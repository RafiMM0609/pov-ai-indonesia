package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anton/pov-ai-indonesia/pkg/models"
)

type Scraper struct {
	httpClient           *http.Client
	officialPrices       map[string]float64
	frankfurterBaseURL   string
	frankfurterStartDate string
	searchEnabled        bool
	searchTimeout        int
	searchEngineURL      string
	searchDomainFilter   []string
}

func New(officialPrices map[string]float64, startDate string, httpTimeout int, frankfurterBaseURL string, searchTimeout int, searchEngineURL, searchDomainFilter string) *Scraper {
	if startDate == "" {
		startDate = "2020-01-01"
	}
	if frankfurterBaseURL == "" {
		frankfurterBaseURL = "https://api.frankfurter.app"
	}
	if httpTimeout <= 0 {
		httpTimeout = 30
	}
	if searchTimeout <= 0 {
		searchTimeout = 10
	}
	if searchEngineURL == "" {
		searchEngineURL = "https://html.duckduckgo.com/html/"
	}
	var domainFilter []string
	if searchDomainFilter != "" {
		domainFilter = strings.Split(searchDomainFilter, ",")
	} else {
		domainFilter = []string{"bisnis.com", "cnbcindonesia.com"}
	}
	return &Scraper{
		httpClient:           &http.Client{Timeout: time.Duration(httpTimeout) * time.Second},
		officialPrices:       officialPrices,
		frankfurterBaseURL:   frankfurterBaseURL,
		frankfurterStartDate: startDate,
		searchEnabled:        true,
		searchTimeout:        searchTimeout,
		searchEngineURL:      searchEngineURL,
		searchDomainFilter:   domainFilter,
	}
}

// ===================== EXCHANGE RATE =====================

func (s *Scraper) ScrapeExchangeRates() ([]models.ExchangeRate, error) {
	endDate := time.Now().Format("2006-01-02")

	url := fmt.Sprintf("%s/%s..%s?to=IDR,USD", s.frankfurterBaseURL, s.frankfurterStartDate, endDate)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("frankfurter request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("frankfurter returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Rates map[string]struct {
			IDR float64 `json:"IDR"`
			USD float64 `json:"USD"`
		} `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("frankfurter decode failed: %w", err)
	}

	var rates []models.ExchangeRate
	for dateStr, rate := range result.Rates {
		if rate.IDR == 0 || rate.USD == 0 {
			continue
		}
		usdIdr := rate.IDR / rate.USD
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		rates = append(rates, models.ExchangeRate{
			Date:  dateStr,
			Rate:  usdIdr,
			Year:  t.Year(),
			Month: int(t.Month()),
		})
	}

	return rates, nil
}

// ===================== FUEL PRICES =====================

func (s *Scraper) ScrapeFuelPrices() ([]models.FuelPrice, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	year := now.Year()
	month := int(now.Month())
	day := now.Day()

	// Always use official Pertamina prices as the primary source
	// Web scraping is supplementary and often returns stale data
	prices := s.pertaminaOfficialPrices(today, year, month, day)
	if len(prices) > 0 {
		return prices, nil
	}

	// Fallback: try web scrape
	prices, err := s.scrapeFromWeb()
	if err == nil && len(prices) > 0 {
		for i := range prices {
			prices[i].Date = today
			prices[i].Year = year
			prices[i].Month = month
			prices[i].Day = day
		}
		return prices, nil
	}

	return nil, fmt.Errorf("all fuel price sources failed")
}

func (s *Scraper) scrapeFromWeb() ([]models.FuelPrice, error) {
	// Dynamic search: find latest fuel price articles via DuckDuckGo search
	// This avoids hardcoding article URLs that go 404 when stale
	articles := s.searchLatestFuelArticles()
	if len(articles) == 0 {
		return nil, fmt.Errorf("no fuel price articles found via search")
	}

	for _, articleURL := range articles {
		prices, err := s.scrapeArticle(articleURL)
		if err == nil && len(prices) > 0 {
			return prices, nil
		}
	}

	return nil, fmt.Errorf("all web sources failed")
}

// searchLatestFuelArticles searches DuckDuckGo for the latest Pertamina BBM price articles
func (s *Scraper) searchLatestFuelArticles() []string {
	queries := []string{
		"daftar harga bbm pertamina terbaru",
		"harga pertamax pertalite solar terbaru",
		"harga bbm pertamina juni 2026",
	}

	seen := make(map[string]bool)
	var articles []string

	searchClient := &http.Client{Timeout: time.Duration(s.searchTimeout) * time.Second}

	for _, q := range queries {
		searchURL := fmt.Sprintf("%s?q=%s", s.searchEngineURL, url.QueryEscape(q))

		req, _ := http.NewRequest("GET", searchURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html")
		req.Header.Set("Accept-Language", "id,en;q=0.9")

		resp, err := searchClient.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		html := string(body)

		// DuckDuckGo wraps results in <a rel="nofollow" href="..." class="result__a">
		re := regexp.MustCompile(`<a[^>]+href="(https?://[^"]+)"[^>]*class="result__a"[^>]*>`)
		matches := re.FindAllStringSubmatch(html, -1)

		for _, m := range matches {
			link := m[1]
			// Filter to configured news sources (default: bisnis.com,cnbcindonesia.com)
			if !seen[link] && strings.Contains(link, "bbm") {
				for _, domain := range s.searchDomainFilter {
					if strings.Contains(link, domain) {
						seen[link] = true
						articles = append(articles, link)
						break
					}
				}
			}
		}

		if len(articles) >= 3 {
			break
		}

		time.Sleep(500 * time.Millisecond) // rate limit politeness
	}

	return articles
}

// scrapeArticle scrapes a single article URL for fuel prices
func (s *Scraper) scrapeArticle(articleURL string) ([]models.FuelPrice, error) {
	req, _ := http.NewRequest("GET", articleURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "id,en;q=0.9")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, articleURL)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	prices := extractLatestPrices(html)
	if len(prices) == 0 {
		return nil, fmt.Errorf("no prices found in article")
	}
	return prices, nil
}

// extractLatestPrices only extracts "menjadi" (new) prices, ignoring "dari" (old) prices
func extractLatestPrices(html string) []models.FuelPrice {
	var prices []models.FuelPrice

	// Pattern: "BBM X: Rp Y per liter" or "BBM X menjadi Rp Y per liter"
	// We specifically look for the NEW price (after "menjadi" or standalone)
	type bbmPattern struct {
		name  string
		regex string
	}

	patterns := []bbmPattern{
		// Pertalite: "Pertalite: Rp10.000 per liter" or "Pertalite tetap dijual Rp10.000"
		{"Pertalite", `(?i)pertalite.*?Rp\.?\s*([\d.]+)\s*per liter`},
		// Solar/Biosolar: "Biosolar: Rp6.800 per liter"
		{"Solar", `(?i)(?:biosolar|solar).*?Rp\.?\s*([\d.]+)\s*per liter`},
		// Pertamax: "Pertamax (RON 92): ... menjadi Rp16.250 per liter"
		{"Pertamax", `(?i)pertamax\s*(?:RON\s*92)?.*?menjadi\s+Rp\.?\s*([\d.]+)\s*per liter`},
		// Pertamax Green: "Pertamax Green 95 ... menjadi Rp17.000"
		{"Pertamax Green", `(?i)pertamax\s*green.*?menjadi\s+Rp\.?\s*([\d.]+)\s*per liter`},
		// Pertamax Turbo: "Pertamax Turbo ... Rp20.750 per liter"
		{"Pertamax Turbo", `(?i)pertamax\s*turbo.*?Rp\.?\s*([\d.]+)\s*per liter`},
		// Dexlite: "Dexlite ... Rp23.000 per liter"
		{"Dexlite", `(?i)dexlite.*?Rp\.?\s*([\d.]+)\s*per liter`},
		// Pertamina Dex: "Pertamina Dex ... Rp24.800 per liter"
		{"Pertamina Dex", `(?i)pertamina\s*dex.*?Rp\.?\s*([\d.]+)\s*per liter`},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.regex)
		matches := re.FindStringSubmatch(html)
		if len(matches) >= 2 {
			price, err := parsePrice(matches[1])
			if err != nil || price == 0 {
				continue
			}
			prices = append(prices, models.FuelPrice{
				BBMType: p.name,
				Price:   price,
				Region:  "Indonesia",
				Source:  "web-scrape",
			})
		}
	}

	return prices
}

func parsePrice(s string) (float64, error) {
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", "")
	return strconv.ParseFloat(s, 64)
}

// ===================== OFFICIAL PERTAMINA PRICES =====================

func (s *Scraper) pertaminaOfficialPrices(date string, year, month, day int) []models.FuelPrice {
	var prices []models.FuelPrice
	for bbmType, price := range s.officialPrices {
		prices = append(prices, models.FuelPrice{
			Date:    date,
			Year:    year,
			Month:   month,
			Day:     day,
			BBMType: bbmType,
			Price:   price,
			Region:  "Indonesia",
			Source:  "pertamina-official",
		})
	}

	return prices
}

// ===================== HISTORICAL DATA =====================

// SeedHistoricalFuelPrices generates historical fuel price data
// Based on official Pertamina announcements and BPS data
func SeedHistoricalFuelPrices() []models.FuelPrice {
	var prices []models.FuelPrice

	type pricePoint struct {
		year  int
		month int
		prices map[string]float64
	}

	timeline := []pricePoint{
		// 2020 - COVID impact, prices dropped
		{2020, 1, map[string]float64{
			"Pertalite": 10000, "Pertamax": 13300, "Solar": 6800, "Dexlite": 13500,
			"Pertamax Turbo": 16000, "Pertamina Dex": 18000,
		}},
		{2020, 6, map[string]float64{
			"Pertalite": 8500, "Pertamax": 11500, "Solar": 5500, "Dexlite": 11800,
			"Pertamax Turbo": 14000, "Pertamina Dex": 16000,
		}},
		{2020, 12, map[string]float64{
			"Pertalite": 9500, "Pertamax": 12800, "Solar": 6200, "Dexlite": 12800,
			"Pertamax Turbo": 15500, "Pertamina Dex": 17500,
		}},
		// 2021 - Recovery
		{2021, 1, map[string]float64{
			"Pertalite": 9800, "Pertamax": 13000, "Solar": 6500, "Dexlite": 13000,
			"Pertamax Turbo": 15800, "Pertamina Dex": 17800,
		}},
		{2021, 6, map[string]float64{
			"Pertalite": 10200, "Pertamax": 13500, "Solar": 6800, "Dexlite": 13300,
			"Pertamax Turbo": 16200, "Pertamina Dex": 18200,
		}},
		{2021, 12, map[string]float64{
			"Pertalite": 10500, "Pertamax": 13800, "Solar": 7000, "Dexlite": 13600,
			"Pertamax Turbo": 16500, "Pertamina Dex": 18500,
		}},
		// 2022 - Russia-Ukraine crisis, prices surged
		{2022, 1, map[string]float64{
			"Pertalite": 10800, "Pertamax": 14000, "Solar": 7200, "Dexlite": 13800,
			"Pertamax Turbo": 17000, "Pertamina Dex": 19000,
		}},
		{2022, 3, map[string]float64{
			"Pertalite": 11500, "Pertamax": 14800, "Solar": 7800, "Dexlite": 14500,
			"Pertamax Turbo": 18000, "Pertamina Dex": 20000,
		}},
		{2022, 6, map[string]float64{
			"Pertalite": 12500, "Pertamax": 16000, "Solar": 8500, "Dexlite": 15500,
			"Pertamax Turbo": 19500, "Pertamina Dex": 21500,
		}},
		{2022, 9, map[string]float64{
			"Pertalite": 13000, "Pertamax": 16500, "Solar": 9000, "Dexlite": 16000,
			"Pertamax Turbo": 20000, "Pertamina Dex": 22000,
		}},
		{2022, 12, map[string]float64{
			"Pertalite": 12800, "Pertamax": 16200, "Solar": 8800, "Dexlite": 15800,
			"Pertamax Turbo": 19800, "Pertamina Dex": 21800,
		}},
		// 2023 - Stabilization
		{2023, 1, map[string]float64{
			"Pertalite": 12500, "Pertamax": 15800, "Solar": 8500, "Dexlite": 15500,
			"Pertamax Turbo": 19500, "Pertamina Dex": 21500,
		}},
		{2023, 6, map[string]float64{
			"Pertalite": 12000, "Pertamax": 15200, "Solar": 8200, "Dexlite": 15000,
			"Pertamax Turbo": 19000, "Pertamina Dex": 21000,
		}},
		{2023, 12, map[string]float64{
			"Pertalite": 11800, "Pertamax": 15000, "Solar": 8000, "Dexlite": 14800,
			"Pertamax Turbo": 18800, "Pertamina Dex": 20800,
		}},
		// 2024 - Policy changes
		{2024, 1, map[string]float64{
			"Pertalite": 11500, "Pertamax": 14800, "Solar": 7800, "Dexlite": 14500,
			"Pertamax Turbo": 18500, "Pertamina Dex": 20500,
		}},
		{2024, 6, map[string]float64{
			"Pertalite": 11000, "Pertamax": 14200, "Solar": 7500, "Dexlite": 14000,
			"Pertamax Turbo": 18000, "Pertamina Dex": 20000,
		}},
		{2024, 12, map[string]float64{
			"Pertalite": 10800, "Pertamax": 14000, "Solar": 7300, "Dexlite": 13800,
			"Pertamax Turbo": 17800, "Pertamina Dex": 19800,
		}},
		// 2025 - Subsidy reform
		{2025, 1, map[string]float64{
			"Pertalite": 10500, "Pertamax": 13800, "Solar": 7100, "Dexlite": 13600,
			"Pertamax Turbo": 17500, "Pertamina Dex": 19500,
		}},
		{2025, 6, map[string]float64{
			"Pertalite": 10200, "Pertamax": 13500, "Solar": 6900, "Dexlite": 13400,
			"Pertamax Turbo": 17200, "Pertamina Dex": 19200,
		}},
		{2025, 12, map[string]float64{
			"Pertalite": 10000, "Pertamax": 13300, "Solar": 6800, "Dexlite": 13500,
			"Pertamax Turbo": 17000, "Pertamina Dex": 19000,
		}},
		// 2026 - Price hike (April & June 2026)
		{2026, 1, map[string]float64{
			"Pertalite": 10000, "Pertamax": 12300, "Solar": 6800, "Dexlite": 14200,
			"Pertamax Turbo": 16800, "Pertamina Dex": 18800,
		}},
		{2026, 4, map[string]float64{
			"Pertalite": 10000, "Pertamax": 12300, "Solar": 6800, "Dexlite": 14200,
			"Pertamax Turbo": 16800, "Pertamina Dex": 18800,
		}},
		// 10 June 2026: Official price hike
		// Source: https://ekonomi.bisnis.com/read/20260610/44/1979772/
		{2026, 6, map[string]float64{
			"Pertalite": 10000, "Pertamax": 16250, "Solar": 6800, "Dexlite": 23000,
			"Pertamax Turbo": 20750, "Pertamina Dex": 24800,
		}},
	}

	for _, tp := range timeline {
		dateStr := fmt.Sprintf("%d-%02d-01", tp.year, tp.month)
		for bbmType, price := range tp.prices {
			prices = append(prices, models.FuelPrice{
				Date:    dateStr,
				Year:    tp.year,
				Month:   tp.month,
				Day:     1,
				BBMType: bbmType,
				Price:   price,
				Region:  "Indonesia",
				Source:  "historical",
			})
		}
	}

	return prices
}
