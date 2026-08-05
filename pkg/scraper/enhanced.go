package scraper

import (
	"fmt"
	"time"

	"github.com/anton/pov-ai-indonesia/internal/config"
	"github.com/anton/pov-ai-indonesia/internal/govapi"
	"github.com/anton/pov-ai-indonesia/pkg/models"
)

// EnhancedScraper wraps the original scraper with government data integration
// Uses seed data for APIs requiring registration (BPS, BI, data.go.id, OJK)
// Uses live public APIs where available (Frankfurter, Pertamina)
type EnhancedScraper struct {
	*Scraper
	govClient  *govapi.Client
	ojkEnabled bool
}

// NewEnhancedScraper creates a scraper with government data integration
func NewEnhancedScraper(cfg *config.Config) *EnhancedScraper {
	return &EnhancedScraper{
		Scraper: New(
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
		),
		govClient: govapi.NewClient(
			cfg.GovHTTPTimeout,
			cfg.BPSBaseURL, cfg.BIBaseURL, cfg.DataGoBaseURL, cfg.OJKBaseURL,
			cfg.BPSAPIRateLimit, cfg.CKANAPIRateLimit,
			cfg.BIBI7DRR, cfg.BIDepositRate, cfg.BILendingRate,
			cfg.OJKEnabled,
		),
		ojkEnabled: cfg.OJKEnabled,
	}
}

// CollectGovernmentData collects government data using seed data for restricted APIs
// This avoids registration requirements while providing accurate reference data
func (es *EnhancedScraper) CollectGovernmentData() *govapi.GovDataCollectionResult {
	result := &govapi.GovDataCollectionResult{
		CollectedAt: time.Now(),
		Errors:      []string{},
	}

	// BPS - Use curated seed data (API requires registration)
	fmt.Println("[enhanced-scraper] Loading BPS economic indicators (seed data)...")
	bpsData := govapi.SeedBPSEconomicData()
	result.BPSIndicators = bpsData
	fmt.Printf("[enhanced-scraper] Loaded %d BPS indicators (seed)\n", len(bpsData))

	// Bank Indonesia - Use seed data (API requires corporate registration)
	fmt.Println("[enhanced-scraper] Loading Bank Indonesia rates (seed data)...")
	biRates := govapi.SeedBIRates()
	result.BIRates = biRates
	fmt.Printf("[enhanced-scraper] Loaded %d BI rates (seed)\n", len(biRates))

	// data.go.id - Try public API
	fmt.Println("[enhanced-scraper] Fetching data.go.id datasets...")
	ckanDatasets, err := es.govClient.FetchEconomicDatasets()
	if err != nil {
		fmt.Printf("[enhanced-scraper] data.go.id API limited: %v\n", err)
		result.Errors = append(result.Errors, fmt.Sprintf("data.go.id: %v", err))
	}
	result.CKANDatasets = ckanDatasets
	fmt.Printf("[enhanced-scraper] Loaded %d CKAN datasets\n", len(ckanDatasets))

	// OJK - only if enabled via config
	if es.ojkEnabled {
		fmt.Println("[enhanced-scraper] Fetching OJK entities...")
		ojkEntities, err := es.govClient.FetchOJKEntities("bank")
		if err != nil {
			fmt.Printf("[enhanced-scraper] OJK API unavailable: %v\n", err)
			result.Errors = append(result.Errors, fmt.Sprintf("OJK: %v", err))
		}
		result.OJKEntities = ojkEntities
		fmt.Printf("[enhanced-scraper] Loaded %d OJK entities\n", len(ojkEntities))
	} else {
		result.OJKEntities = []models.OJKEntity{}
	}

	// Commodity prices (emas, beras, minyak goreng)
	fmt.Println("[enhanced-scraper] Loading commodity prices...")
	commodityData := govapi.SeedCommodityData()
	result.CommodityPrices = commodityData
	fmt.Printf("[enhanced-scraper] Loaded %d commodity price records\n", len(commodityData))

	return result
}

// CheckAPIAvailability checks which public APIs are accessible
func (es *EnhancedScraper) CheckAPIAvailability() map[string]bool {
	status := make(map[string]bool)

	// Frankfurter (always works)
	status["frankfurter"] = true

	// data.go.id
	if _, err := es.govClient.FetchCKANDatasets("ekonomi", 1); err == nil {
		status["data_go_id"] = true
	} else {
		status["data_go_id"] = false
	}

	// Seed-based sources (always available)
	status["bps_seed"] = true
	status["bi_seed"] = true
	status["pertamina_official"] = true
	status["historical_fuel"] = true

	return status
}