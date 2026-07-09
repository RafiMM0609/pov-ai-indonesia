package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// Server
	Port        string
	DataDir     string
	KnowledgeDir string
	DBPath      string
	StaticDir   string
	TemplateDir string
	CORSAllowOrigin string

	// Database
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime int // minutes

	// Scraper
	ScrapeInterval      int
	FrankfurterBaseURL  string
	FrankfurterStartDate string
	WebScraperEnabled   bool
	SearchTimeout       int
	SearchEngineURL     string
	SearchDomainFilter  string
	ScraperHTTPTimeout  int // seconds

	// AI Client
	OpenRouterAPIKey  string
	OpenRouterBaseURL string
	POVModel          string
	AITemperature     float64
	AIMaxTokens       int
	AIHTTPTimeout     int    // seconds
	AIHTTPReferer     string
	AIHTTPTitle       string

	// Seed Prices
	SeedPrices map[string]float64

	// Cron
	CronAIRefresh string // cron expression
	CronGovData   string // cron expression

	// Gov API
	GovHTTPTimeout  int // seconds
	BPSBaseURL      string
	BIBaseURL       string
DataGoBaseURL   string
	OJKBaseURL      string
	BPSAPIRateLimit  int // milliseconds
	CKANAPIRateLimit int // milliseconds
	OJKEnabled      bool
	BIBI7DRR        float64
	BIDepositRate   float64
	BILendingRate   float64
}

func Load() *Config {
	loadEnvFile(".env")
	loadEnvFile("../.env")

	seedPrices := map[string]float64{
		"Pertalite":      10000,
		"Solar":          6800,
		"Pertamax":       16250,
		"Pertamax Green": 17000,
		"Pertamax Turbo": 20750,
		"Dexlite":        23000,
		"Pertamina Dex":  24800,
	}

	for k := range seedPrices {
		envKey := "SEED_PRICE_" + strings.ToUpper(strings.ReplaceAll(k, " ", "_"))
		if val := os.Getenv(envKey); val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				seedPrices[k] = f
			}
		}
	}

	return &Config{
		// Server
		Port:             getEnv("PORT", "8080"),
		DataDir:          getEnv("DATA_DIR", "./data"),
		KnowledgeDir:     getEnv("KNOWLEDGE_DIR", "./data/knowledge"),
		DBPath:           getEnv("DB_PATH", "./data/povai.db"),
		StaticDir:        getEnv("STATIC_DIR", "./web/static"),
		TemplateDir:      getEnv("TEMPLATE_DIR", "web/templates/*"),
		CORSAllowOrigin:  getEnv("CORS_ALLOW_ORIGIN", "*"),

		// Database
		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 1),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 1),
		DBConnMaxLifetime: getEnvInt("DB_CONN_MAX_LIFETIME", 60),

		// Scraper
		ScrapeInterval:       getEnvInt("SCRAPE_INTERVAL", 6),
		FrankfurterBaseURL:   getEnv("FRANKFURTER_BASE_URL", "https://api.frankfurter.app"),
		FrankfurterStartDate: getEnv("FRANKFURTER_START_DATE", "2020-01-01"),
		WebScraperEnabled:    getEnvBool("WEB_SCRAPER_ENABLED", true),
		SearchTimeout:        getEnvInt("SEARCH_TIMEOUT", 10),
		SearchEngineURL:      getEnv("SEARCH_ENGINE_URL", "https://html.duckduckgo.com/html/"),
		SearchDomainFilter:   getEnv("SEARCH_DOMAIN_FILTER", "bisnis.com,cnbcindonesia.com"),
		ScraperHTTPTimeout:   getEnvInt("SCRAPER_HTTP_TIMEOUT", 30),

		// AI Client
		OpenRouterAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterBaseURL: getEnv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		POVModel:          getEnv("POV_AI_MODEL", "openrouter/owl-alpha"),
		AITemperature:     getEnvFloat("AI_TEMPERATURE", 0.7),
		AIMaxTokens:       getEnvInt("AI_MAX_TOKENS", 2048),
		AIHTTPTimeout:     getEnvInt("AI_HTTP_TIMEOUT", 120),
		AIHTTPReferer:     getEnv("AI_HTTP_REFERER", "https://pov-ai.local"),
		AIHTTPTitle:       getEnv("AI_HTTP_TITLE", "POV AI Indonesia"),

		// Seed Prices
		SeedPrices: seedPrices,

		// Cron
		CronAIRefresh: getEnv("CRON_AI_REFRESH", "0 3 * * *"),
		CronGovData:   getEnv("CRON_GOV_DATA", "0 2 * * 0"),

		// Gov API
		GovHTTPTimeout:   getEnvInt("GOV_HTTP_TIMEOUT", 30),
		BPSBaseURL:       getEnv("BPS_BASE_URL", "https://webapi.bps.go.id/v1"),
		BIBaseURL:        getEnv("BI_BASE_URL", "https://api.bi.go.id/v1"),
		DataGoBaseURL:    getEnv("DATA_GO_BASE_URL", "https://data.go.id/api/3"),
		OJKBaseURL:       getEnv("OJK_BASE_URL", "https://api.ojk.go.id/v1"),
		BPSAPIRateLimit:  getEnvInt("BPS_API_RATE_LIMIT", 100),
		CKANAPIRateLimit: getEnvInt("CKAN_API_RATE_LIMIT", 200),
		OJKEnabled:       getEnvBool("OJK_ENABLED", false),
		BIBI7DRR:         getEnvFloat("BI_BI7DRR_RATE", 5.50),
		BIDepositRate:    getEnvFloat("BI_DEPOSIT_RATE", 4.75),
		BILendingRate:    getEnvFloat("BI_LENDING_RATE", 6.25),
	}
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		val = strings.ToLower(strings.TrimSpace(val))
		return val == "true" || val == "1" || val == "yes"
	}
	return defaultVal
}
