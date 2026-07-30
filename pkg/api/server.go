package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anton/pov-ai-indonesia/pkg/db"
	"github.com/anton/pov-ai-indonesia/pkg/knowledge"
	"github.com/anton/pov-ai-indonesia/pkg/models"
	"github.com/anton/pov-ai-indonesia/pkg/scraper"
	"github.com/gin-gonic/gin"
)

type Server struct {
	database  *db.DB
	knowledge *knowledge.KnowledgeManager
	scraper   *scraper.Scraper
	aiEnabled bool
	router    *gin.Engine
}

func NewServer(database *db.DB, km *knowledge.KnowledgeManager, sc *scraper.Scraper, aiEnabled bool, corsOrigin, staticDir, templateDir string) *Server {
	router := gin.Default()

	// CORS
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", corsOrigin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	srv := &Server{
		database:  database,
		knowledge: km,
		scraper:   sc,
		aiEnabled: aiEnabled,
		router:    router,
	}
	srv.setupRoutes(staticDir, templateDir)
	return srv
}

func (s *Server) setupRoutes(staticDir, templateDir string) {
	api := s.router.Group("/api/v1")
	{
		api.GET("/dashboard", s.handleDashboard)
		api.GET("/exchange-rates", s.handleExchangeRates)
		api.GET("/fuel-prices", s.handleFuelPrices)
		api.GET("/fuel-prices/latest", s.handleLatestFuelPrices)
		api.GET("/fuel-prices/history", s.handleFuelPriceHistory)
		api.GET("/fuel-types", s.handleFuelTypes)
		api.GET("/insights/:category/:period", s.handleInsight)
		api.GET("/insights", s.handleListInsights)
		api.GET("/admin/scrape-logs", s.handleScrapeLogs)
		api.POST("/admin/regenerate", s.handleRegenerate)
		api.POST("/admin/scrape", s.handleAdminScrape)
		api.GET("/health", s.handleHealth)

		// Government data endpoints
		api.GET("/bps-indicators", s.handleBPSIndicators)
		api.GET("/bi-rates", s.handleBIRates)
		api.GET("/ckan-datasets", s.handleCKANDatasets)
		api.GET("/ojk-entities", s.handleOJKEntities)
		// Commodity & Gold data endpoints
		api.GET("/commodity-prices", s.handleCommodityPrices)
		api.GET("/commodity-latest", s.handleLatestCommodityPrices)
		api.GET("/gold-prices", s.handleGoldPrices)
		api.GET("/gold-prices/latest", s.handleLatestGoldPrices)

		// Data sources transparency endpoint
		api.GET("/data-sources", s.handleDataSources)
	}

	s.router.Static("/static", staticDir)
	s.router.LoadHTMLGlob(templateDir)
	s.router.GET("/", s.handleIndex)
	s.router.GET("/exchange-rate", s.handleExchangeRatePage)
	s.router.GET("/fuel-price", s.handleFuelPricePage)
	s.router.GET("/gold-price", s.handleGoldPricePage)
	s.router.GET("/sources", s.handleSourcesPage)
	s.router.GET("/commodities", s.handleCommoditiesPage) // SPA - same HTML, JS handles routing
}

func (s *Server) Run(port string) error {
	return s.router.Run(":" + port)
}

func parseYearMonth(c *gin.Context) (year, month int) {
	year, _ = strconv.Atoi(c.Query("year"))
	month, _ = strconv.Atoi(c.Query("month"))
	return
}

// periodLabel formats a year/month pair for human-readable messages.
func periodLabel(year, month int) string {
	if year == 0 && month == 0 {
		return "semua periode"
	}
	months := []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	if month > 0 && month <= len(months) {
		return fmt.Sprintf("%s %d", months[month-1], year)
	}
	return fmt.Sprintf("%d", year)
}

// handleDashboard reads insights from disk (NO LLM calls)
func (s *Server) handleDashboard(c *gin.Context) {
	year, month := parseYearMonth(c)

	rates, err := s.database.GetExchangeRates(year, month)
	if err != nil {
		fmt.Printf("[api] Gagal mengambil exchange rates: %v\n", err)
	}
	fuel, err := s.database.GetFuelPrices(year, month, "", "Indonesia")
	if err != nil {
		fmt.Printf("[api] Gagal mengambil harga BBM: %v\n", err)
	}

	// Fall back to the latest available period when the requested period is empty,
	// so the dashboard never renders with all-zero/empty data.
	var resolvedYear, resolvedMonth int
	var note string
	if len(rates) == 0 && len(fuel) == 0 {
		ry, rm, ok := s.database.GetLatestExchangePeriod()
		if !ok {
			ry, rm, ok = s.database.GetLatestFuelPeriod()
		}
		if ok {
			resolvedYear, resolvedMonth = ry, rm
			rates, err = s.database.GetExchangeRates(ry, rm)
			if err != nil {
				fmt.Printf("[api] Gagal mengambil exchange rates (fallback): %v\n", err)
			}
			fuel, err = s.database.GetFuelPrices(ry, rm, "", "Indonesia")
			if err != nil {
				fmt.Printf("[api] Gagal mengambil harga BBM (fallback): %v\n", err)
			}
			note = fmt.Sprintf("Menampilkan data terbaru (%s) — periode %s belum tersedia",
				periodLabel(ry, rm), periodLabel(year, month))
		}
	} else {
		resolvedYear, resolvedMonth = year, month
	}

	exTrend := s.database.CalculateExchangeTrend(rates, resolvedYear, resolvedMonth)
	fuelTrend := s.database.CalculateFuelTrend(fuel, resolvedYear, resolvedMonth, "", "Indonesia")

	// Read pre-generated insights from disk - NO LLM calls
	insights := make([]models.KnowledgeEntry, 0, 3)

	// Try insight for the (possibly fallback) period first, then fall back to the
	// year-level insight so a missing month-specific insight still shows something.
	loadInsightFor := func(category string) (*models.KnowledgeEntry, bool) {
		rperiod := fmt.Sprintf("%d", resolvedYear)
		if resolvedMonth > 0 {
			rperiod = fmt.Sprintf("%d-%02d", resolvedYear, resolvedMonth)
		}
		if ir, err := s.knowledge.LoadInsight(category, rperiod); err == nil {
			return ir, true
		}
		if ir, err := s.knowledge.LoadInsight(category, fmt.Sprintf("%d", resolvedYear)); err == nil {
			return ir, true
		}
		return nil, false
	}

	if ir, ok := loadInsightFor("exchange_rate"); ok && len(rates) > 0 {
		insights = append(insights, *ir)
	}

	if fi, ok := loadInsightFor("fuel_price"); ok && len(fuel) > 0 {
		insights = append(insights, *fi)
	}

	if gi, ok := loadInsightFor("gold_price"); ok {
		insights = append(insights, *gi)
	}

	// Get data sources for transparency
	dataSources := s.getDataSources()

	c.JSON(http.StatusOK, models.DashboardResponse{
		ExchangeRates: rates,
		ExchangeTrend: exTrend,
		FuelPrices:    fuel,
		FuelTrend:     fuelTrend,
		Insights:      insights,
		Filter: models.FilterParams{
			Year:          year,
			Month:         month,
			ResolvedYear:  resolvedYear,
			ResolvedMonth: resolvedMonth,
			Note:          note,
		},
		DataSources: dataSources,
	})
}

func (s *Server) handleExchangeRates(c *gin.Context) {
	year, month := parseYearMonth(c)
	rates, err := s.database.GetExchangeRates(year, month)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resolvedYear, resolvedMonth := year, month
	note := ""
	if len(rates) == 0 {
		ry, rm, ok := s.database.GetLatestExchangePeriod()
		if ok {
			resolvedYear, resolvedMonth = ry, rm
			rates, err = s.database.GetExchangeRates(ry, rm)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			note = fmt.Sprintf("Periode %s belum tersedia — menampilkan data terbaru (%s)",
				periodLabel(year, month), periodLabel(ry, rm))
		}
	}
	trend := s.database.CalculateExchangeTrend(rates, resolvedYear, resolvedMonth)
	c.JSON(http.StatusOK, gin.H{
		"data":          rates,
		"trend":         trend,
		"resolved_year": resolvedYear,
		"resolved_month": resolvedMonth,
		"note":          note,
	})
}

func (s *Server) handleFuelPrices(c *gin.Context) {
	year, month := parseYearMonth(c)
	bbmType := c.Query("bbm_type")
	region := c.Query("region")
	prices, err := s.database.GetFuelPrices(year, month, bbmType, region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resolvedYear, resolvedMonth := year, month
	note := ""
	if len(prices) == 0 {
		ry, rm, ok := s.database.GetLatestFuelPeriod()
		if ok {
			resolvedYear, resolvedMonth = ry, rm
			prices, err = s.database.GetFuelPrices(ry, rm, bbmType, region)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			note = fmt.Sprintf("Periode %s belum tersedia — menampilkan data terbaru (%s)",
				periodLabel(year, month), periodLabel(ry, rm))
		}
	}
	trend := s.database.CalculateFuelTrend(prices, resolvedYear, resolvedMonth, bbmType, region)
	c.JSON(http.StatusOK, gin.H{
		"data":           prices,
		"trend":          trend,
		"resolved_year":  resolvedYear,
		"resolved_month": resolvedMonth,
		"note":           note,
	})
}

func (s *Server) handleLatestFuelPrices(c *gin.Context) {
	prices, err := s.database.GetLatestFuelPrices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": prices})
}

func (s *Server) handleFuelPriceHistory(c *gin.Context) {
	bbmType := c.Query("bbm_type")
	region := c.Query("region")
	if bbmType == "" {
		bbmType = "Pertalite"
	}
	if region == "" {
		region = "Indonesia"
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	prices, err := s.database.GetFuelPriceHistory(bbmType, region, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	trend := s.database.CalculateFuelTrend(prices, 0, 0, bbmType, region)
	c.JSON(http.StatusOK, gin.H{"data": prices, "trend": trend})
}

func (s *Server) handleFuelTypes(c *gin.Context) {
	types, err := s.database.GetFuelTypes()
	if err != nil {
		fmt.Printf("[api] Gagal mengambil tipe BBM: %v\n", err)
	}
	regions, err := s.database.GetFuelRegions()
	if err != nil {
		fmt.Printf("[api] Gagal mengambil wilayah BBM: %v\n", err)
	}
	c.JSON(http.StatusOK, gin.H{"types": types, "regions": regions})
}

// handleInsight reads from disk (NO LLM calls)
func (s *Server) handleInsight(c *gin.Context) {
	category := c.Param("category")
	period := c.Param("period")

	insight, err := s.knowledge.LoadInsight(category, period)
	if err != nil {
		// Fallback: try yearly if monthly not found
		parts := strings.Split(period, "-")
		if len(parts) > 1 {
			insight, err = s.knowledge.LoadInsight(category, parts[0])
		}
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":    "insight not found",
				"detail":   "Insight not pre-generated yet. Wait for background regeneration or trigger POST /api/v1/admin/regenerate",
				"category": category,
				"period":   period,
			})
			return
		}
	}
	c.JSON(http.StatusOK, insight)
}

func (s *Server) handleListInsights(c *gin.Context) {
	insights, err := s.knowledge.ListInsights()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []models.KnowledgeEntry{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": insights})
}

func (s *Server) handleScrapeLogs(c *gin.Context) {
	logs, err := s.database.GetRecentScrapeLogs(20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func (s *Server) handleRegenerate(c *gin.Context) {
	years, err := s.database.GetExchangeRateYears()
	if err != nil {
		fmt.Printf("[api] Gagal mengambil tahun kurs untuk regenerasi: %v\n", err)
	}
	fy, err := s.database.GetFuelPriceYears()
	if err != nil {
		fmt.Printf("[api] Gagal mengambil tahun BBM untuk regenerasi: %v\n", err)
	}
	for _, y := range fy {
		found := false
		for _, ey := range years {
			if ey == y {
				found = true
				break
			}
		}
		if !found {
			years = append(years, y)
		}
	}

	go s.knowledge.RegenerateAll(years)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Knowledge regeneration started in background. Insights will be available in ~20s per year.",
	})
}

// handleAdminScrape triggers an on-demand data refresh (exchange rates + fuel prices).
// Unlike the background cron, this blocks until the scrape completes so the caller
// gets an accurate count of what was fetched. Insights are refreshed in the background.
func (s *Server) handleAdminScrape(c *gin.Context) {
	if s.scraper == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "scraper not configured"})
		return
	}

	resp := gin.H{"status": "ok"}

	rates, err := s.scraper.ScrapeExchangeRates()
	if err == nil && len(rates) > 0 {
		n, insErr := s.database.InsertExchangeRates(rates)
		if insErr != nil {
			resp["exchange_error"] = insErr.Error()
		} else {
			resp["exchange_inserted"] = n
		}
		s.database.LogScrape("frankfurter", "success", len(rates), "")
	} else {
		var msg string
		if err != nil {
			msg = err.Error()
		}
		resp["exchange_error"] = msg
		s.database.LogScrape("frankfurter", "failed", 0, msg)
	}

	fuelPrices, err := s.scraper.ScrapeFuelPrices()
	if err == nil && len(fuelPrices) > 0 {
		n, insErr := s.database.InsertFuelPrices(fuelPrices)
		if insErr != nil {
			resp["fuel_error"] = insErr.Error()
		} else {
			resp["fuel_inserted"] = n
		}
		s.database.LogScrape("fuel-scraper", "success", len(fuelPrices), "")
	} else {
		var msg string
		if err != nil {
			msg = err.Error()
		}
		resp["fuel_error"] = msg
		s.database.LogScrape("fuel-scraper", "failed", 0, msg)
	}

	// Refresh insights in the background using the latest available data.
	if s.aiEnabled {
		years := getAllYearsSafe(s.database)
		go s.knowledge.RegenerateAll(years)
	}

	c.JSON(http.StatusOK, resp)
}

// getAllYearsSafe mirrors main.getAllYears without importing main (no import cycle).
func getAllYearsSafe(database *db.DB) []int {
	erYears, err := database.GetExchangeRateYears()
	if err != nil {
		fmt.Printf("[api] Error getting exchange rate years: %v\n", err)
	}
	fpYears, err := database.GetFuelPriceYears()
	if err != nil {
		fmt.Printf("[api] Error getting fuel price years: %v\n", err)
	}
	yearSet := make(map[int]bool, len(erYears)+len(fpYears))
	for _, y := range erYears {
		yearSet[y] = true
	}
	for _, y := range fpYears {
		yearSet[y] = true
	}
	years := make([]int, 0, len(yearSet))
	for y := range yearSet {
		years = append(years, y)
	}
	sort.Ints(years)
	return years
}

func (s *Server) handleHealth(c *gin.Context) {
	erCount, err := s.database.CountExchangeRates()
	if err != nil {
		fmt.Printf("[api] Gagal menghitung kurs di health check: %v\n", err)
	}
	fpCount, err := s.database.CountFuelPrices()
	if err != nil {
		fmt.Printf("[api] Gagal menghitung BBM di health check: %v\n", err)
	}
	types, err := s.database.GetFuelTypes()
	if err != nil {
		fmt.Printf("[api] Gagal mengambil tipe BBM di health check: %v\n", err)
	}
	insights, err := s.knowledge.ListInsights()
	if err != nil {
		fmt.Printf("[api] Gagal mengambil insights di health check: %v\n", err)
	}
	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"service":        "pov-ai-indonesia",
		"exchange_rates": erCount,
		"fuel_prices":    fpCount,
		"fuel_types":     types,
		"insights":       len(insights),
	})
}

func (s *Server) handleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{"title": "POV AI Indonesia"})
}

func (s *Server) handleExchangeRatePage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{"title": "USD/IDR - POV AI Indonesia"})
}

func (s *Server) handleFuelPricePage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{"title": "Harga BBM - POV AI Indonesia"})
}

func (s *Server) handleSourcesPage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{"title": "Sumber Data - POV AI Indonesia"})
}

func (s *Server) handleCommoditiesPage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{"title": "Kebutuhan Pokok - POV AI Indonesia"})
}

func (s *Server) handleGoldPricePage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{"title": "Harga Emas - POV AI Indonesia"})
}

// Gold price handlers
func (s *Server) handleGoldPrices(c *gin.Context) {
	year, month := parseYearMonth(c)
	region := c.Query("region")
	prices, err := s.database.GetCommodityPrices(year, month, "Emas", region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	trend := s.database.CalculateGoldTrend(prices, year, month)
	c.JSON(http.StatusOK, gin.H{
		"data":          prices,
		"trend":         trend,
		"resolved_year": year,
		"resolved_month": month,
	})
}

func (s *Server) handleLatestGoldPrices(c *gin.Context) {
	prices, err := s.database.GetLatestCommodityPrices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	goldPrices := make([]models.CommodityPrice, 0, len(prices))
	for _, p := range prices {
		if p.Commodity == "Emas" {
			goldPrices = append(goldPrices, p)
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": goldPrices})
}


// Government data handlers
func (s *Server) handleBPSIndicators(c *gin.Context) {
	year, _ := strconv.Atoi(c.Query("year"))
	indicatorID := c.Query("indicator_id")
	region := c.DefaultQuery("region", "Indonesia")

	indicators, err := s.database.GetBPSIndicators(year, indicatorID, region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": indicators})
}

func (s *Server) handleBIRates(c *gin.Context) {
	year, _ := strconv.Atoi(c.Query("year"))
	rateType := c.Query("rate_type")

	rates, err := s.database.GetBIRates(year, rateType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rates})
}

func (s *Server) handleCKANDatasets(c *gin.Context) {
	organization := c.Query("organization")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	datasets, err := s.database.GetCKANDatasets(organization, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": datasets})
}

func (s *Server) handleOJKEntities(c *gin.Context) {
	entityType := c.Query("entity_type")
	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	entities, err := s.database.GetOJKEntities(entityType, status, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entities})
}

// getDataSources returns metadata about all data sources for transparency
func (s *Server) getDataSources() []models.DataSourceInfo {
	now := time.Now().Format("2006-01-02 15:04:05")
	return []models.DataSourceInfo{
		{
			Name:         "Frankfurter.app (ECB)",
			Description:  "European Central Bank reference exchange rates",
			URL:          "https://api.frankfurter.app",
			AccessType:   "public",
			LastUpdated:  now,
			RefreshCycle: "Daily (ECB business days, ~16:00 CET)",
			Notes:        "Official ECB reference rate — BUKAN market spot rate. Digunakan untuk chart historis dan analisis tren. Market spot (TradingView, Google Finance) bisa berbeda ±0.2%–1% dari ECB. Untuk referensi eksak harian, cek kurs tengah BI (JISDOR) atau TradingView.",
		},
		{
			Name:         "TradingView (XAUIDRG)",
			Description:  "Live gold price chart (XAU/IDR per gram) dari FX_IDC",
			URL:          "https://id.tradingview.com/symbols/XAUIDRG/",
			AccessType:   "public",
			LastUpdated:  now,
			RefreshCycle: "Real-time (market hours)",
			Notes:        "Live gold price widget dari TradingView. Harga XAU/IDR per gram dari FX_IDC (ICE). Bisa berbeda dari harga Antam/pegadaian.",
		},
		{
			Name:         "Pertamina Official",
			Description:  "Official BBM retail prices from Pertamina",
			URL:          "https://www.pertamina.com",
			AccessType:   "official",
			LastUpdated:  now,
			RefreshCycle: "As announced (typically monthly)",
			Notes:        "Primary source for current prices: Pertalite, Pertamax, Solar, Dexlite, Pertamax Turbo, Pertamina Dex, Pertamax Green.",
		},
		{
			Name:         "Harga Kebutuhan Pokok (Kurasi Manual)",
			Description:  "Emas (Antam), Beras (Premium/Medium/C4), Minyak Goreng (Bimoli/Sania/Curah) — per wilayah",
			URL:          "https://www.antam.com + pasar tradisional",
			AccessType:   "manual",
			LastUpdated:  "2026-06 (survei harga pasar)",
			RefreshCycle: "Update manual berkala",
			Notes:        "⚠️ DATA KURASI MANUAL — Harga emas berdasarkan Antam, harga beras dan minyak goreng berdasarkan pasar tradisional per wilayah (Jakarta, Jateng, Jogja, Jatim). Bisa berbeda dengan harga real-time di daerah lain.",
		},
		{
			Name:         "BPS (Badan Pusat Statistik)",
			Description:  "Indonesian official statistics - GDP, Inflation, Unemployment, Poverty, HDI, Trade, Investment",
			URL:          "https://www.bps.go.id",
			AccessType:   "manual",
			LastUpdated:  "2024-2026 (per BPS publications)",
			RefreshCycle: "Quarterly (GDP), Monthly (Inflation), Semi-annual (Unemployment), Annually (Poverty, HDI)",
			Notes:        "⚠️ DATA KURASI MANUAL — Tidak pakai live API (perlu registrasi institusi). Nilai dari rilis pers resmi BPS. Bisa berbeda dari data terkini.",
		},
		{
			Name:         "Bank Indonesia",
			Description:  "Monetary policy rates (BI7DRR, Deposit Facility, Lending Facility)",
			URL:          "https://www.bi.go.id",
			AccessType:   "manual",
			LastUpdated:  "Jan 2025 (RDG Jan 2025)",
			RefreshCycle: "Monthly (BI Board of Governors / RDG meetings)",
			Notes:        "⚠️ DATA KURASI MANUAL — Nilai dari keputusan RDG. Bisa berbeda dari keputusan terbaru.",
		},
		{
			Name:         "data.go.id (Satu Data Indonesia)",
			Description:  "National open data portal (CKAN-based)",
			URL:          "https://data.go.id",
			AccessType:   "public",
			LastUpdated:  now,
			RefreshCycle: "Varies by dataset",
			Notes:        "Free registration for API key (higher limits). Frontend migrated to Next.js - API access may be limited.",
		},
		{
			Name:         "OJK (Otoritas Jasa Keuangan)",
			Description:  "Licensed financial institutions registry",
			URL:          "https://www.ojk.go.id",
			AccessType:   "manual",
			LastUpdated:  "Not available",
			RefreshCycle: "As published by OJK",
			Notes:        "❌ TIDAK TERSEDIA — API restricted to registered entities. Data entitas keuangan OJK tidak ditampilkan.",
		},
	}
}

// handleDataSources returns metadata about all data sources
func (s *Server) handleDataSources(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data":        s.getDataSources(),
		"generated_at": time.Now().Format(time.RFC3339),
		"version":     "1.0",
		"note":        "Source transparency for user trust. Kurasi manual digunakan untuk data yang API-nya memerlukan registrasi.",
	})
}

// Commodity price handlers
func (s *Server) handleCommodityPrices(c *gin.Context) {
	year, _ := strconv.Atoi(c.Query("year"))
	month, _ := strconv.Atoi(c.Query("month"))
	commodity := c.Query("commodity")
	region := c.DefaultQuery("region", "Indonesia")

	prices, err := s.database.GetCommodityPrices(year, month, commodity, region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": prices})
}

func (s *Server) handleLatestCommodityPrices(c *gin.Context) {
	region := c.Query("region")
	
	var prices []models.CommodityPrice
	var err error
	
	if region != "" {
		prices, err = s.database.GetCommodityPrices(0, 0, "", region)
	} else {
		prices, err = s.database.GetLatestCommodityPrices()
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": prices})
}


