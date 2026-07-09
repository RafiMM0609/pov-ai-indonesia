package models

import "time"

type ExchangeRate struct {
	ID        int64     `json:"id"`
	Date      string    `json:"date"`
	Rate      float64   `json:"rate"`
	Year      int       `json:"year"`
	Month     int       `json:"month"`
	CreatedAt time.Time `json:"created_at"`
}

type FuelPrice struct {
	ID        int64     `json:"id"`
	Date      string    `json:"date"`
	Year      int       `json:"year"`
	Month     int       `json:"month"`
	Day       int       `json:"day"`
	BBMType   string    `json:"bbm_type"`
	Price     float64   `json:"price"`
	Region    string    `json:"region"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

// CommodityPrice untuk harga kebutuhan pokok (emas, beras, minyak goreng, dll)
type CommodityPrice struct {
	ID          int64     `json:"id"`
	Date        string    `json:"date"`
	Year        int       `json:"year"`
	Month       int       `json:"month"`
	Day         int       `json:"day"`
	Commodity   string    `json:"commodity"`   // "Emas", "Beras Premium", "Beras Medium", "Minyak Goreng"
	Type        string    `json:"type"`        // jenis spesifik
	Price       float64   `json:"price"`       // dalam Rupiah
	Unit        string    `json:"unit"`        // "gram", "kg", "liter"
	Region      string    `json:"region"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
}

type TrendIndicator struct {
	CurrentValue  float64 `json:"current_value"`
	PreviousValue float64 `json:"previous_value"`
	ChangePercent float64 `json:"change_percent"`
	Direction     string  `json:"direction"`
}

type FilterParams struct {
	Year     int    `form:"year" json:"year"`
	Month    int    `form:"month" json:"month"`
	YearFrom int    `form:"year_from" json:"year_from"`
	YearTo   int    `form:"year_to" json:"year_to"`
	BBMType  string `form:"bbm_type" json:"bbm_type"`
	Region   string `form:"region" json:"region"`
	// ResolvedYear/ResolvedMonth report the period the backend actually used to
	// serve the request. When the requested period has no data, the backend falls
	// back to the latest available period and sets Note accordingly so the UI can
	// inform the user instead of showing empty charts.
	ResolvedYear  int    `json:"resolved_year"`
	ResolvedMonth int    `json:"resolved_month"`
	Note          string `json:"note,omitempty"`
}

type KnowledgeEntry struct {
	Title       string    `json:"title"`
	Category    string    `json:"category"`
	Period      string    `json:"period"`
	Summary     string    `json:"summary"`
	Content     string    `json:"content"`
	Factors     []string  `json:"factors"`
	GeneratedAt time.Time `json:"generated_at"`
}

// BPS Economic Indicator
type BPSEconomicIndicator struct {
	ID          int64     `json:"id"`
	IndicatorID string    `json:"indicator_id"`
	Indicator   string    `json:"indicator"`
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	Year        int       `json:"year"`
	Period      string    `json:"period"`
	Region      string    `json:"region"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
}

// Bank Indonesia Rate
type BIRate struct {
	ID            int64     `json:"id"`
	RateType      string    `json:"rate_type"`
	Value         float64   `json:"value"`
	EffectiveDate string    `json:"effective_date"`
	Year          int       `json:"year"`
	Month         int       `json:"month"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
}

// CKAN Dataset
type CKANDataset struct {
	ID           int64     `json:"id"`
	DatasetID    string    `json:"dataset_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Organization string    `json:"organization"`
	Tags         []string  `json:"tags"`
	License      string    `json:"license"`
	URL          string    `json:"url"`
	Source       string    `json:"source"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// OJK Entity
type OJKEntity struct {
	ID            int64     `json:"id"`
	EntityName    string    `json:"entity_name"`
	EntityType    string    `json:"entity_type"`
	LicenseNumber string    `json:"license_number"`
	Status        string    `json:"status"`
	Address       string    `json:"address"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
}

type DashboardResponse struct {
	ExchangeRates  []ExchangeRate      `json:"exchange_rates"`
	ExchangeTrend  TrendIndicator      `json:"exchange_trend"`
	FuelPrices     []FuelPrice         `json:"fuel_prices"`
	FuelTrend      TrendIndicator      `json:"fuel_trend"`
	Insights       []KnowledgeEntry    `json:"insights"`
	Filter         FilterParams        `json:"filter"`
	DataSources    []DataSourceInfo    `json:"data_sources"`
}

// DataSourceInfo provides transparency about data sources used
type DataSourceInfo struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	URL          string `json:"url"`
	AccessType   string `json:"access_type"` // "public", "manual", "official"
	LastUpdated  string `json:"last_updated"`
	RefreshCycle string `json:"refresh_cycle"`
	Notes        string `json:"notes,omitempty"`
}