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
	httpClient            *http.Client
	officialPrices        map[string]float64
	frankfurterBaseURL    string
	frankfurterStartDate  string
	searchEnabled         bool
	searchTimeout         int
	searchEngineURL       string
	searchDomainFilter    []string
	pertaminaDirectURL    string
	pertaminaDirectToken  string
	pertaminaDirectRegion string
}

func New(officialPrices map[string]float64, startDate string, httpTimeout int, frankfurterBaseURL string, searchTimeout int, searchEngineURL, searchDomainFilter string, pertaminaDirectURL, pertaminaDirectToken, pertaminaDirectRegion string) *Scraper {
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
		httpClient:            &http.Client{Timeout: time.Duration(httpTimeout) * time.Second},
		officialPrices:        officialPrices,
		frankfurterBaseURL:    frankfurterBaseURL,
		frankfurterStartDate:  startDate,
		searchEnabled:         true,
		searchTimeout:         searchTimeout,
		searchEngineURL:       searchEngineURL,
		searchDomainFilter:    domainFilter,
		pertaminaDirectURL:    pertaminaDirectURL,
		pertaminaDirectToken:  pertaminaDirectToken,
		pertaminaDirectRegion: pertaminaDirectRegion,
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
	// Get region from config if available (default: Yogyakarta)
	region := s.pertaminaDirectRegion
	if region == "" {
		region = "Yogyakarta"
	}

	now := time.Now()
	date := now.Format("2006-01-02")
	year := now.Year()
	month := int(now.Month())
	day := now.Day()

	// Try direct Pertamina Patra Niaga API source first
	if s.pertaminaDirectURL != "" {
		prices := s.pertaminaDirectPrices(date, year, month, day, region)
		if len(prices) > 0 {
			return prices, nil
		}
	}

	// Always use official Pertamina prices as the primary source
	// Web scraping is supplementary and often returns stale data
	prices := s.pertaminaOfficialPrices(date, year, month, day, region)
	if len(prices) > 0 {
		return prices, nil
	}

	// Fallback: try web scrape
	prices, err := s.scrapeFromWeb()
	if err == nil && len(prices) > 0 {
		for i := range prices {
			prices[i].Date = date
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

func extractBBMTypeFromName(name string) string {
	n := strings.ToLower(name)
	if strings.Contains(n, "pertalite") {
		return "Pertalite"
	} else if strings.Contains(n, "solar") || strings.Contains(n, "bio solar") {
		return "Solar"
	} else if strings.Contains(n, "pertamax") {
		return "Pertamax"
	} else if strings.Contains(n, "dexlite") {
		return "Dexlite"
	} else if strings.Contains(n, "dex") || strings.Contains(n, "pertamina dex") {
		return "Pertamina Dex"
	}
	return ""
}

func (s *Scraper) pertaminaDirectPrices(date string, year, month, day int, region string) []models.FuelPrice {
	if s.pertaminaDirectURL == "" {
		return nil
	}

	apiURL := s.pertaminaDirectURL
	if strings.Contains(apiURL, "pertaminapatraniaga.com/page/") {
		slug := apiURL[strings.LastIndex(apiURL, "/")+1:]
		apiURL = fmt.Sprintf("https://pertaminapatraniaga.com/api/api/v1/post/get-by-slug/page/%s", slug)
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	return parsePertaminaPatraNiagaJSON(body, date, year, month, day, region)
}

func parsePertaminaPatraNiagaJSON(jsonData []byte, date string, year, month, day int, targetRegion string) []models.FuelPrice {
	var respData struct {
		Data struct {
			Content map[string]struct {
				DisplayName string `json:"displayName"`
				Type        struct {
					ResolvedName string `json:"resolvedName"`
				} `json:"type"`
				Props struct {
					Items []struct {
						Title string                   `json:"title"`
						Data  []map[string]interface{} `json:"data"`
					} `json:"items"`
				} `json:"props"`
			} `json:"content"`
		} `json:"data"`
	}

	if err := json.Unmarshal(jsonData, &respData); err != nil {
		return nil
	}

	if targetRegion == "" {
		targetRegion = "Yogyakarta"
	}
	targetLower := strings.ToLower(targetRegion)

	var prices []models.FuelPrice
	seenBBM := make(map[string]bool)

	for _, node := range respData.Data.Content {
		if node.DisplayName != "ProductTable" && node.Type.ResolvedName != "ProductTable" {
			continue
		}

		for _, item := range node.Props.Items {
			for _, row := range item.Data {
				wilayah, ok := row["WILAYAH"].(string)
				if !ok {
					continue
				}

				wilayahLower := strings.ToLower(wilayah)
				if !strings.Contains(wilayahLower, targetLower) &&
					!(targetLower == "yogyakarta" && (strings.Contains(wilayahLower, "di yogyakarta") || strings.Contains(wilayahLower, "jogja"))) {
					continue
				}

				for k, v := range row {
					if k == "WILAYAH" {
						continue
					}
					valStr, ok := v.(string)
					if !ok {
						continue
					}

					priceVal, err := parsePrice(strings.TrimSpace(valStr))
					if err != nil || priceVal <= 0 {
						continue
					}

					bbmType := mapImageURLToBBMType(k)
					if bbmType != "" && !seenBBM[bbmType] {
						seenBBM[bbmType] = true
						prices = append(prices, models.FuelPrice{
							Date:    date,
							Year:    year,
							Month:   month,
							Day:     day,
							BBMType: bbmType,
							Price:   priceVal,
							Region:  targetRegion,
							Source:  "pertamina-patra-niaga",
						})
					}
				}
			}
		}
	}

	return prices
}

func mapImageURLToBBMType(key string) string {
	k := strings.ToLower(key)
	if strings.Contains(k, "pertamax-turbo") {
		return "Pertamax Turbo"
	}
	if strings.Contains(k, "pertamax-green") {
		return "Pertamax Green"
	}
	if strings.Contains(k, "product-table-pertamax.png") || strings.Contains(k, "harga-produk-pertamax") {
		return "Pertamax"
	}
	if strings.Contains(k, "pertalite") {
		return "Pertalite"
	}
	if strings.Contains(k, "pertamina-dex") {
		return "Pertamina Dex"
	}
	if strings.Contains(k, "dexlite") {
		return "Dexlite"
	}
	if strings.Contains(k, "bio-solar") || strings.Contains(k, "biosolar") || strings.Contains(k, "solar") {
		return "Solar"
	}
	return ""
}

func (s *Scraper) pertaminaOfficialPrices(date string, year, month, day int, region string) []models.FuelPrice {
	var prices []models.FuelPrice

	// Try region-specific direct source first (e.g. Yogyakarta)
	if region == "Yogyakarta" || s.pertaminaDirectURL != "" {
		directPrices := s.pertaminaDirectPrices(date, year, month, day, region)
		if len(directPrices) > 0 {
			return directPrices
		}
	}

	// Fallback to official seed prices
	for bbmType, price := range s.officialPrices {
		prices = append(prices, models.FuelPrice{
			Date:    date,
			Year:    year,
			Month:   month,
			Day:     day,
			BBMType: bbmType,
			Price:   price,
			Region:  region,
			Source:  "pertamina-official",
		})
	}

	return prices
}

func SeedHistoricalFuelPrices() []models.FuelPrice {
	var prices []models.FuelPrice
	now := time.Now()
	for year := 2020; year <= now.Year(); year++ {
		for month := 1; month <= 12; month++ {
			if year == now.Year() && month > int(now.Month()) {
				break
			}
			date := fmt.Sprintf("%d-%02d-01", year, month)
			prices = append(prices, []models.FuelPrice{
				{Date: date, Year: year, Month: month, Day: 1, BBMType: "Pertalite", Price: 10000, Region: "Yogyakarta", Source: "historical"},
				{Date: date, Year: year, Month: month, Day: 1, BBMType: "Solar", Price: 6800, Region: "Yogyakarta", Source: "historical"},
				{Date: date, Year: year, Month: month, Day: 1, BBMType: "Pertamax", Price: 13900, Region: "Yogyakarta", Source: "historical"},
				{Date: date, Year: year, Month: month, Day: 1, BBMType: "Pertamax Turbo", Price: 15900, Region: "Yogyakarta", Source: "historical"},
				{Date: date, Year: year, Month: month, Day: 1, BBMType: "Dexlite", Price: 16150, Region: "Yogyakarta", Source: "historical"},
				{Date: date, Year: year, Month: month, Day: 1, BBMType: "Pertamina Dex", Price: 16850, Region: "Yogyakarta", Source: "historical"},
			}...)
		}
	}
	return prices
}