package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/anton/pov-ai-indonesia/internal/config"
	"github.com/anton/pov-ai-indonesia/pkg/ai"
	"github.com/anton/pov-ai-indonesia/pkg/api"
	"github.com/anton/pov-ai-indonesia/pkg/db"
	"github.com/anton/pov-ai-indonesia/pkg/knowledge"
	"github.com/anton/pov-ai-indonesia/pkg/scraper"
	"github.com/robfig/cron/v3"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)

	cfg := config.Load()
	os.MkdirAll(cfg.DataDir, 0755)
	os.MkdirAll(cfg.KnowledgeDir, 0755)

	// --- Database ---
	fmt.Println("[main] Opening SQLite database...")
	database, err := db.New(cfg.DBPath, cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxLifetime)
	if err != nil {
		log.Fatalf("Database init failed: %v", err)
	}
	defer database.Close()

	// --- Components ---
	aiClient := ai.NewClient(cfg.OpenRouterAPIKey, cfg.OpenRouterBaseURL, cfg.POVModel, cfg.AITemperature, cfg.AIMaxTokens, cfg.AIHTTPTimeout, cfg.AIHTTPReferer, cfg.AIHTTPTitle)

	sc := scraper.New(
		cfg.SeedPrices,
		cfg.FrankfurterStartDate,
		cfg.ScraperHTTPTimeout,
		cfg.FrankfurterBaseURL,
		cfg.SearchTimeout,
		cfg.SearchEngineURL,
		cfg.SearchDomainFilter,
		cfg.PertaminaDirectURL,
		cfg.PertaminaDirectToken,
		cfg.PertaminaDirectRegion,
	)
	es := scraper.NewEnhancedScraper(cfg)
	km := knowledge.NewKnowledgeManager(cfg.KnowledgeDir, aiClient, database)

	// --- Seed data if empty ---
	seedData(database, sc, es)

	// --- Pre-generate insights BEFORE serving (templates first = fast, LLM in background) ---
	fmt.Println("[main] Pre-generating template insights (fast)...")
	start := time.Now()
	years := getAllYears(database)
	km.RegenerateAllTemplates(years)
	fmt.Printf("[main] Template insights ready in %v\n", time.Since(start))

	// --- Start server (all reads from disk, zero LLM during HTTP) ---
	server := api.NewServer(database, km, sc, aiClient.IsEnabled(), cfg.CORSAllowOrigin, cfg.StaticDir, cfg.TemplateDir)

	go func() {
		fmt.Printf("[main] Server starting on port %s\n", cfg.Port)
		fmt.Printf("[main] Dashboard: http://localhost:%s\n", cfg.Port)
		if err := server.Run(cfg.Port); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	time.Sleep(2 * time.Second)

	// Enrich with LLM in background (slow, non-blocking)
	if aiClient.IsEnabled() {
		fmt.Println("[main] Starting background LLM enrichment (slow, non-blocking)...")
		go func() {
			km.RegenerateAll(years)
			fmt.Println("[main] LLM enrichment complete!")
		}()
	}

	// --- Cron jobs ---
	c := cron.New()

	// Daily data scrape + knowledge refresh
	c.AddFunc(fmt.Sprintf("0 */%d * * *", cfg.ScrapeInterval), func() {
		fmt.Println("[cron] Daily data scrape...")
		scrapeData(database, sc, km, aiClient.IsEnabled())
	})

	// AI knowledge refresh (configurable cron)
	if cfg.CronAIRefresh != "" {
		c.AddFunc(cfg.CronAIRefresh, func() {
			if aiClient.IsEnabled() {
				fmt.Println("[cron] AI knowledge refresh...")
				years, err := database.GetExchangeRateYears()
				if err != nil {
					fmt.Printf("[cron] Error getting exchange rate years: %v\n", err)
					return
				}
				km.RegenerateAll(years)
			}
		})
	}

	// Weekly government data collection (configurable cron)
	if cfg.CronGovData != "" {
		c.AddFunc(cfg.CronGovData, func() {
			fmt.Println("[cron] Government data collection...")
			if err := collectGovernmentData(database, es); err != nil {
				fmt.Printf("[cron] Government data collection failed: %v\n", err)
			} else {
				if aiClient.IsEnabled() {
					years := getAllYears(database)
					km.RegenerateAll(years)
				}
			}
		})
	}

	c.Start()
	fmt.Printf("[main] Cron: scrape every %dh, AI refresh: %s, gov data: %s\n", cfg.ScrapeInterval, cfg.CronAIRefresh, cfg.CronGovData)

	select {}
}

func seedData(database *db.DB, sc *scraper.Scraper, es *scraper.EnhancedScraper) {
	// Exchange rates
	erCount, err := database.CountExchangeRates()
	if err != nil {
		fmt.Printf("[main] Error counting exchange rates: %v\n", err)
	}
	if erCount == 0 {
		fmt.Println("[main] Seeding exchange rates...")
		rates, err := sc.ScrapeExchangeRates()
		if err != nil {
			fmt.Printf("[main]   Exchange rate scrape failed: %v\n", err)
			database.LogScrape("frankfurter", "failed", 0, err.Error())
		} else {
			n, err := database.InsertExchangeRates(rates)
			if err != nil {
				fmt.Printf("[main]   Exchange rate seeding failed: %v\n", err)
			}
			fmt.Printf("[main]   Inserted %d exchange rate records\n", n)
			database.LogScrape("frankfurter", "success", n, "")
		}
	} else {
		fmt.Printf("[main]   Exchange rates already seeded: %d records\n", erCount)
	}

	// Fuel prices
	fpCount, err := database.CountFuelPrices()
	if err != nil {
		fmt.Printf("[main] Error counting fuel prices: %v\n", err)
	}
	if fpCount == 0 {
		fmt.Println("[main] Seeding fuel prices...")
		historical := scraper.SeedHistoricalFuelPrices()
		n, err := database.InsertFuelPrices(historical)
		if err != nil {
			fmt.Printf("[main]   Historical seed failed: %v\n", err)
		} else {
			fmt.Printf("[main]   Inserted %d historical fuel price records\n", n)
		}
		livePrices, err := sc.ScrapeFuelPrices()
		if err == nil && len(livePrices) > 0 {
			n, err := database.InsertFuelPrices(livePrices)
			if err != nil {
				fmt.Printf("[main]   Live fuel prices seeding failed: %v\n", err)
			}
			fmt.Printf("[main]   Inserted %d live fuel price records\n", n)
			database.LogScrape("fuel-scraper", "success", n, "")
		} else {
			fmt.Printf("[main]   Live fuel scrape: %v (using seed data)\n", err)
			var errMsg string
			if err != nil {
				errMsg = err.Error()
			}
			database.LogScrape("fuel-scraper", "seed-fallback", len(historical), errMsg)
		}
	} else {
		fmt.Printf("[main]   Fuel prices already seeded: %d records\n", fpCount)
	}

	// Government data (BPS, BI, data.go.id, OJK)
	fmt.Println("[main] Seeding government data...")
	govResult := es.CollectGovernmentData()
	if len(govResult.BPSIndicators) > 0 {
		if n, err := database.InsertBPSIndicators(govResult.BPSIndicators); err != nil {
			fmt.Printf("[main]   BPS indicators seeding failed: %v\n", err)
		} else {
			fmt.Printf("[main]   Inserted %d BPS indicators\n", n)
		}
		database.LogScrape("bps-indicators", "success", len(govResult.BPSIndicators), "")
	}
	if len(govResult.BIRates) > 0 {
		if n, err := database.InsertBIRates(govResult.BIRates); err != nil {
			fmt.Printf("[main]   BI rates seeding failed: %v\n", err)
		} else {
			fmt.Printf("[main]   Inserted %d BI rates\n", n)
		}
		database.LogScrape("bi-rates", "success", len(govResult.BIRates), "")
	}
	if len(govResult.CKANDatasets) > 0 {
		if n, err := database.InsertCKANDatasets(govResult.CKANDatasets); err != nil {
			fmt.Printf("[main]   CKAN datasets seeding failed: %v\n", err)
		} else {
			fmt.Printf("[main]   Inserted %d CKAN datasets\n", n)
		}
		database.LogScrape("ckan-datasets", "success", len(govResult.CKANDatasets), "")
	}
	if len(govResult.OJKEntities) > 0 {
		if n, err := database.InsertOJKEntities(govResult.OJKEntities); err != nil {
			fmt.Printf("[main]   OJK entities seeding failed: %v\n", err)
		} else {
			fmt.Printf("[main]   Inserted %d OJK entities\n", n)
		}
		database.LogScrape("ojk-entities", "success", len(govResult.OJKEntities), "")
	}
	// Commodity prices
	if len(govResult.CommodityPrices) > 0 {
		if n, err := database.InsertCommodityPrices(govResult.CommodityPrices); err != nil {
			fmt.Printf("[main]   Commodity prices seeding failed: %v\n", err)
		} else {
			fmt.Printf("[main]   Inserted %d commodity price records\n", n)
		}
		database.LogScrape("commodity-prices", "success", len(govResult.CommodityPrices), "")
	}
	if len(govResult.Errors) > 0 {
		for _, e := range govResult.Errors {
			fmt.Printf("[main]   Gov API error: %s\n", e)
		}
	}
}

func scrapeData(database *db.DB, sc *scraper.Scraper, km *knowledge.KnowledgeManager, llmOK bool) {
	rates, err := sc.ScrapeExchangeRates()
	if err == nil && len(rates) > 0 {
		if n, err := database.InsertExchangeRates(rates); err != nil {
			fmt.Printf("[cron] Error inserting exchange rates: %v\n", err)
		} else if n > 0 {
			fmt.Printf("[cron] Updated %d exchange rates\n", n)
		}
		database.LogScrape("frankfurter", "success", len(rates), "")
	} else {
		var errMsg string
		if err != nil {
			errMsg = err.Error()
		}
		database.LogScrape("frankfurter", "failed", 0, errMsg)
	}

	fuelPrices, err := sc.ScrapeFuelPrices()
	if err == nil && len(fuelPrices) > 0 {
		if n, err := database.InsertFuelPrices(fuelPrices); err != nil {
			fmt.Printf("[cron] Error inserting fuel prices: %v\n", err)
		} else if n > 0 {
			fmt.Printf("[cron] Updated %d fuel prices\n", n)
		}
		database.LogScrape("fuel-scraper", "success", len(fuelPrices), "")
	} else {
		var errMsg string
		if err != nil {
			errMsg = err.Error()
		}
		database.LogScrape("fuel-scraper", "failed", 0, errMsg)
	}

	if llmOK {
		years := getAllYears(database)
		km.RegenerateAll(years)
	}
}

// getAllYears returns all unique years from both exchange_rates and fuel_prices tables
func getAllYears(database *db.DB) []int {
	erYears, err := database.GetExchangeRateYears()
	if err != nil {
		fmt.Printf("[main] Error getting exchange rate years: %v\n", err)
	}
	fpYears, err := database.GetFuelPriceYears()
	if err != nil {
		fmt.Printf("[main] Error getting fuel price years: %v\n", err)
	}

	yearSet := make(map[int]bool)
	for _, y := range erYears {
		yearSet[y] = true
	}
	for _, y := range fpYears {
		yearSet[y] = true
	}

	var years []int
	for y := range yearSet {
		years = append(years, y)
	}
	sort.Ints(years)
	return years
}

// collectGovernmentData collects data from Indonesian government APIs
func collectGovernmentData(database *db.DB, es *scraper.EnhancedScraper) error {
	govResult := es.CollectGovernmentData()

	if len(govResult.BPSIndicators) > 0 {
		if n, err := database.InsertBPSIndicators(govResult.BPSIndicators); err != nil {
			fmt.Printf("[cron] BPS indicators update failed: %v\n", err)
		} else if n > 0 {
			fmt.Printf("[cron] Updated %d BPS indicators\n", n)
		}
		database.LogScrape("bps-indicators", "success", len(govResult.BPSIndicators), "")
	}
	if len(govResult.BIRates) > 0 {
		if n, err := database.InsertBIRates(govResult.BIRates); err != nil {
			fmt.Printf("[cron] BI rates update failed: %v\n", err)
		} else if n > 0 {
			fmt.Printf("[cron] Updated %d BI rates\n", n)
		}
		database.LogScrape("bi-rates", "success", len(govResult.BIRates), "")
	}
	if len(govResult.CKANDatasets) > 0 {
		if n, err := database.InsertCKANDatasets(govResult.CKANDatasets); err != nil {
			fmt.Printf("[cron] CKAN datasets update failed: %v\n", err)
		} else if n > 0 {
			fmt.Printf("[cron] Updated %d CKAN datasets\n", n)
		}
		database.LogScrape("ckan-datasets", "success", len(govResult.CKANDatasets), "")
	}
	if len(govResult.OJKEntities) > 0 {
		if n, err := database.InsertOJKEntities(govResult.OJKEntities); err != nil {
			fmt.Printf("[cron] OJK entities update failed: %v\n", err)
		} else if n > 0 {
			fmt.Printf("[cron] Updated %d OJK entities\n", n)
		}
		database.LogScrape("ojk-entities", "success", len(govResult.OJKEntities), "")
	}

	if len(govResult.Errors) > 0 {
		for _, e := range govResult.Errors {
			fmt.Printf("[cron] Gov API error: %s\n", e)
		}
	}

	return nil
}
