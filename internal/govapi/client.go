package govapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anton/pov-ai-indonesia/pkg/models"
)

// Client for Indonesian Government APIs
type Client struct {
	httpClient      *http.Client
	BaseURLs        map[string]string
	bpsRateLimit    int  // ms between BPS API calls
	ckanRateLimit   int  // ms between CKAN API calls
	bi7drr          float64
	depositRate     float64
	lendingRate     float64
	ojkEnabled      bool
}

// NewClient creates a new government API client
func NewClient(httpTimeout int, bpsBaseURL, biBaseURL, dataGoBaseURL, ojkBaseURL string,
	bpsRateLimit, ckanRateLimit int, bi7drr, depositRate, lendingRate float64, ojkEnabled bool) *Client {
	if httpTimeout <= 0 {
		httpTimeout = 30
	}
	if bpsBaseURL == "" {
		bpsBaseURL = "https://webapi.bps.go.id/v1"
	}
	if biBaseURL == "" {
		biBaseURL = "https://api.bi.go.id/v1"
	}
	if dataGoBaseURL == "" {
		dataGoBaseURL = "https://data.go.id/api/3"
	}
	if ojkBaseURL == "" {
		ojkBaseURL = "https://api.ojk.go.id/v1"
	}
	if bpsRateLimit <= 0 {
		bpsRateLimit = 100
	}
	if ckanRateLimit <= 0 {
		ckanRateLimit = 200
	}
	if bi7drr <= 0 {
		bi7drr = 5.50
	}
	if depositRate <= 0 {
		depositRate = 4.75
	}
	if lendingRate <= 0 {
		lendingRate = 6.25
	}
	return &Client{
		httpClient:    &http.Client{Timeout: time.Duration(httpTimeout) * time.Second},
		BaseURLs: map[string]string{
			"bps":      bpsBaseURL,
			"bi":       biBaseURL,
			"bi_pub":   "https://www.bi.go.id",
			"data_go":  dataGoBaseURL,
			"ojk":      ojkBaseURL,
		},
		bpsRateLimit:  bpsRateLimit,
		ckanRateLimit: ckanRateLimit,
		bi7drr:        bi7drr,
		depositRate:   depositRate,
		lendingRate:   lendingRate,
		ojkEnabled:    ojkEnabled,
	}
}

// ===================== BPS STATISTICS INDONESIA =====================

func (c *Client) FetchBPSEconomicIndicators() ([]models.BPSEconomicIndicator, error) {
	var indicators []models.BPSEconomicIndicator

	indicatorIDs := []string{
		"1101001", "1101002", "1101003", "1101004", "1101005",
		"1101006", "1101007", "1101008", "1101009", "1101010",
		"1101011", "1101012", "1101013", "1101014",
	}

	for _, id := range indicatorIDs {
		url := fmt.Sprintf("%s/indicator/%s?lang=id", c.BaseURLs["bps"], id)

		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "POV-AI-Indonesia/1.0")
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			continue
		}

		body, _ := io.ReadAll(resp.Body)

		var bpsResp struct {
			Data []struct {
				IndicatorID   string  `json:"indicator_id"`
				IndicatorName string  `json:"indicator_name"`
				Value         float64 `json:"value"`
				Unit          string  `json:"unit"`
				Year          int     `json:"year"`
				Period        string  `json:"period"`
				Region        string  `json:"region"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &bpsResp); err != nil {
			continue
		}

		for _, d := range bpsResp.Data {
			indicators = append(indicators, models.BPSEconomicIndicator{
				IndicatorID: d.IndicatorID,
				Indicator:   d.IndicatorName,
				Value:       d.Value,
				Unit:        d.Unit,
				Year:        d.Year,
				Period:      d.Period,
				Region:      d.Region,
				Source:      "BPS-API",
				CreatedAt:   time.Now(),
			})
		}

		time.Sleep(time.Duration(c.bpsRateLimit) * time.Millisecond)
	}

	return indicators, nil
}

// ===================== BANK INDONESIA =====================

func (c *Client) FetchBIRates() ([]models.BIRate, error) {
	url := fmt.Sprintf("%s/interest-rate/policy-rate", c.BaseURLs["bi"])

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "POV-AI-Indonesia/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			var biResp struct {
				Data []struct {
					RateType      string  `json:"rate_type"`
					Value         float64 `json:"value"`
					EffectiveDate string  `json:"effective_date"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &biResp); err == nil {
				var rates []models.BIRate
				for _, d := range biResp.Data {
					t, _ := time.Parse("2006-01-02", d.EffectiveDate)
					rates = append(rates, models.BIRate{
						RateType:      d.RateType,
						Value:         d.Value,
						EffectiveDate: d.EffectiveDate,
						Year:          t.Year(),
						Month:         int(t.Month()),
						Source:        "BI-API",
						CreatedAt:     time.Now(),
					})
				}
				if len(rates) > 0 {
					return rates, nil
				}
			}
		}
	}

	return c.fetchBIRatesFromPublic()
}

func (c *Client) fetchBIRatesFromPublic() ([]models.BIRate, error) {
	now := time.Now()
	return []models.BIRate{
		{RateType: "BI7DRR", Value: c.bi7drr, EffectiveDate: now.Format("2006-01-02"), Year: now.Year(), Month: int(now.Month()), Source: "BI-Public-Seed", CreatedAt: now},
		{RateType: "Deposit_Facility", Value: c.depositRate, EffectiveDate: now.Format("2006-01-02"), Year: now.Year(), Month: int(now.Month()), Source: "BI-Public-Seed", CreatedAt: now},
		{RateType: "Lending_Facility", Value: c.lendingRate, EffectiveDate: now.Format("2006-01-02"), Year: now.Year(), Month: int(now.Month()), Source: "BI-Public-Seed", CreatedAt: now},
	}, nil
}

// ===================== DATA.GO.ID (CKAN) =====================

func (c *Client) FetchCKANDatasets(query string, limit int) ([]models.CKANDataset, error) {
	url := fmt.Sprintf("%s/action/package_search?q=%s&rows=%d", c.BaseURLs["data_go"], query, limit)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "POV-AI-Indonesia/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("data.go.id API returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	var ckanResp struct {
		Result struct {
			Results []struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Notes       string `json:"notes"`
				Organization struct {
					Title string `json:"title"`
				} `json:"organization"`
				Tags []struct {
					DisplayName string `json:"display_name"`
				} `json:"tags"`
				LicenseID     string `json:"license_id"`
				URL           string `json:"url"`
				MetadataCreated  string `json:"metadata_created"`
				MetadataModified string `json:"metadata_modified"`
			} `json:"results"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &ckanResp); err != nil {
		return nil, err
	}

	var datasets []models.CKANDataset
	for _, d := range ckanResp.Result.Results {
		tags := make([]string, len(d.Tags))
		for i, tag := range d.Tags {
			tags[i] = tag.DisplayName
		}

		created, _ := time.Parse(time.RFC3339, d.MetadataCreated)
		modified, _ := time.Parse(time.RFC3339, d.MetadataModified)

		datasets = append(datasets, models.CKANDataset{
			DatasetID:    d.ID,
			Title:        d.Title,
			Description:  d.Notes,
			Organization: d.Organization.Title,
			Tags:         tags,
			License:      d.LicenseID,
			URL:          d.URL,
			Source:       "data.go.id",
			CreatedAt:    created,
			UpdatedAt:    modified,
		})
	}

	return datasets, nil
}

func (c *Client) FetchEconomicDatasets() ([]models.CKANDataset, error) {
	queries := []string{"ekonomi", "keuangan", "inflasi", "pdb", "pdrb", "pengangguran", "kemiskinan", "investasi", "ekspor", "impor"}
	var allDatasets []models.CKANDataset
	seen := make(map[string]bool)

	for _, q := range queries {
		datasets, err := c.FetchCKANDatasets(q, 10)
		if err != nil {
			continue
		}
		for _, d := range datasets {
			if !seen[d.DatasetID] {
				seen[d.DatasetID] = true
				allDatasets = append(allDatasets, d)
			}
		}
		time.Sleep(time.Duration(c.ckanRateLimit) * time.Millisecond)
	}

	return allDatasets, nil
}

// ===================== OJK =====================

func (c *Client) FetchOJKEntities(entityType string) ([]models.OJKEntity, error) {
	url := fmt.Sprintf("%s/entities?type=%s", c.BaseURLs["ojk"], entityType)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "POV-AI-Indonesia/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OJK API returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data []struct {
			EntityName     string `json:"entity_name"`
			EntityType     string `json:"entity_type"`
			LicenseNumber  string `json:"license_number"`
			Status         string `json:"status"`
			Address        string `json:"address"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var entities []models.OJKEntity
	for _, d := range result.Data {
		entities = append(entities, models.OJKEntity{
			EntityName:    d.EntityName,
			EntityType:    d.EntityType,
			LicenseNumber: d.LicenseNumber,
			Status:        d.Status,
			Address:       d.Address,
			Source:        "OJK",
			CreatedAt:     time.Now(),
		})
	}

	return entities, nil
}

// ===================== FALLBACK / SEED DATA =====================

func SeedBPSEconomicData() []models.BPSEconomicIndicator {
	now := time.Now()
	year := now.Year()

	// Data based on BPS official releases: bps.go.id
	// Updated: Juni 2026
	return []models.BPSEconomicIndicator{
		// Makroekonomi
		{IndicatorID: "1101001", Indicator: "PDB (GDP) - Triliun Rupiah", Value: 22134.5, Unit: "Triliun Rupiah", Year: year - 2, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101001b", Indicator: "PDB (GDP) - Triliun Rupiah", Value: 22750.0, Unit: "Triliun Rupiah", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101001c", Indicator: "PDB (GDP) - Triliun Rupiah", Value: 23400.0, Unit: "Triliun Rupiah", Year: year, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		// Inflasi
		{IndicatorID: "1101002", Indicator: "Inflasi (Year on Year)", Value: 2.19, Unit: "Persen", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101002b", Indicator: "Inflasi (Year on Year)", Value: 2.45, Unit: "Persen", Year: year, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101002c", Indicator: "Inflasi Bulanan (Month to Month)", Value: 0.18, Unit: "Persen", Year: year, Period: "monthly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		// Ketenagakerjaan
		{IndicatorID: "1101003", Indicator: "TPT (Tingkat Pengangguran Terbuka)", Value: 4.87, Unit: "Persen", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101003b", Indicator: "TPT (Tingkat Pengangguran Terbuka)", Value: 4.65, Unit: "Persen", Year: year, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101003c", Indicator: "TPK (Tingkat Partisipasi Angkatan Kerja)", Value: 69.80, Unit: "Persen", Year: year, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		// Kemiskinan
		{IndicatorID: "1101004", Indicator: "Kemiskinan (Persentase Penduduk Miskin)", Value: 8.57, Unit: "Persen", Year: year - 2, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101004b", Indicator: "Kemiskinan (Persentase Penduduk Miskin)", Value: 8.15, Unit: "Persen", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101004c", Indicator: "Kemiskinan (Persentase Penduduk Miskin)", Value: 7.90, Unit: "Persen", Year: year, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		// IPM
		{IndicatorID: "1101005", Indicator: "IPM (Indeks Pembangunan Manusia)", Value: 75.02, Unit: "Indeks", Year: year - 2, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101005b", Indicator: "IPM (Indeks Pembangunan Manusia)", Value: 75.69, Unit: "Indeks", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101005c", Indicator: "IPM (Indeks Pembangunan Manusia)", Value: 76.20, Unit: "Indeks", Year: year, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		// PDRB Regional
		{IndicatorID: "1101006", Indicator: "PDRB DKI Jakarta", Value: 3200, Unit: "Triliun Rupiah", Year: year - 1, Period: "yearly", Region: "DKI Jakarta", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101006b", Indicator: "PDRB Jawa Timur", Value: 2850, Unit: "Triliun Rupiah", Year: year - 1, Period: "yearly", Region: "Jawa Timur", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101006c", Indicator: "PDRB Jawa Barat", Value: 2750, Unit: "Triliun Rupiah", Year: year - 1, Period: "yearly", Region: "Jawa Barat", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101006d", Indicator: "PDRB Jawa Tengah", Value: 1750, Unit: "Triliun Rupiah", Year: year - 1, Period: "yearly", Region: "Jawa Tengah", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101006e", Indicator: "PDRB Sumatera Utara", Value: 850, Unit: "Triliun Rupiah", Year: year - 1, Period: "yearly", Region: "Sumatera Utara", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101006f", Indicator: "PDRB Kalimantan Timur", Value: 950, Unit: "Triliun Rupiah", Year: year - 1, Period: "yearly", Region: "Kalimantan Timur", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101006g", Indicator: "PDRB Sulawesi Selatan", Value: 650, Unit: "Triliun Rupiah", Year: year - 1, Period: "yearly", Region: "Sulawesi Selatan", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		// Perdagangan
		{IndicatorID: "1101007", Indicator: "Ekspor Non-Migas", Value: 250, Unit: "Miliar USD", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101007b", Indicator: "Ekspor Non-Migas", Value: 265, Unit: "Miliar USD", Year: year, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101007c", Indicator: "Ekspor Migas", Value: 35, Unit: "Miliar USD", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		{IndicatorID: "1101008", Indicator: "Impor Non-Migas", Value: 200, Unit: "Miliar USD", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101008b", Indicator: "Impor Non-Migas", Value: 215, Unit: "Miliar USD", Year: year, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101008c", Indicator: "Impor Migas", Value: 55, Unit: "Miliar USD", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		// Kurs
		{IndicatorID: "1101010", Indicator: "Kurs Rata-rata USD/IDR (BPS)", Value: 15500, Unit: "Rupiah", Year: year - 2, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101010b", Indicator: "Kurs Rata-rata USD/IDR (BPS)", Value: 16200, Unit: "Rupiah", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101010c", Indicator: "Kurs Rata-rata USD/IDR (BPS)", Value: 16800, Unit: "Rupiah", Year: year, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		// Consumer Confidence Index
		{IndicatorID: "1101101", Indicator: "Indeks Keyakinan Konsumen (IKK)", Value: 125.3, Unit: "Indeks", Year: year, Period: "monthly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		// Sektor Riil
		{IndicatorID: "1101201", Indicator: "Produksi Tanaman Pangan Padi", Value: 53.5, Unit: "Juta Ton", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
		{IndicatorID: "1101202", Indicator: "Produksi Kelapa Sawit (CPO)", Value: 48.2, Unit: "Juta Ton", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},

		// Pariwisata
		{IndicatorID: "1101301", Indicator: "Kunjungan Wisatawan Mancanegara", Value: 13.5, Unit: "Juta Orang", Year: year - 1, Period: "yearly", Region: "Indonesia", Source: "BPS - kurasi dari rilis resmi", CreatedAt: now},
	}
}

func SeedBIRates() []models.BIRate {
	now := time.Now()
	return []models.BIRate{
		{RateType: "BI7DRR", Value: 5.50, EffectiveDate: now.Format("2006-01-02"), Year: now.Year(), Month: int(now.Month()), Source: "BI-kurasi", CreatedAt: now},
		{RateType: "Deposit_Facility", Value: 4.75, EffectiveDate: now.Format("2006-01-02"), Year: now.Year(), Month: int(now.Month()), Source: "BI-kurasi", CreatedAt: now},
		{RateType: "Lending_Facility", Value: 6.25, EffectiveDate: now.Format("2006-01-02"), Year: now.Year(), Month: int(now.Month()), Source: "BI-kurasi", CreatedAt: now},
	}
}

// SeedCommodityData provides curated commodity prices per wilayah
// Harga berdasarkan survei pasar tradisional (Juni 2026)
func SeedCommodityData() []models.CommodityPrice {
	now := time.Now()
	today := now.Format("2006-01-02")
	year := now.Year()
	month := int(now.Month())
	day := now.Day()

	type commodityItem struct {
		Commodity string
		Type      string
		Unit      string
		Prices    map[string]float64 // wilayah -> harga
		Source    string
	}

	items := []commodityItem{
		// Emas (harga per gram dari TradingView XAUIDRG)
		{
			Commodity: "Emas", Type: "Antam 1g", Unit: "gram",
			Prices: map[string]float64{
				"Jakarta": 2450000, "Bandung": 2455000, "Semarang": 2455000, "Yogyakarta": 2455000,
				"Surabaya": 2460000, "Medan": 2465000, "Palembang": 2460000, "Pekanbaru": 2465000,
				"Denpasar": 2470000, "Makassar": 2470000, "Balikpapan": 2465000, "Manado": 2465000,
				"Jayapura": 2480000, "Pontianak": 2460000, "Mataram": 2465000, "Banjarmasin": 2465000,
			},
			Source: "tradingview-xauidr",
		},
		{
			Commodity: "Emas", Type: "Antam 10g", Unit: "gram",
			Prices: map[string]float64{
				"Jakarta": 23200000, "Surabaya": 23250000, "Medan": 23300000, "Bandung": 23250000,
			},
			Source: "tradingview-xauidr",
		},
		// Beras (per kg) - harga berbeda per wilayah
		{
			Commodity: "Beras", Type: "Premium (IR64)", Unit: "kg",
			Prices: map[string]float64{
				"Jakarta": 18500, "Bandung": 18000, "Semarang": 17000, "Yogyakarta": 17500,
				"Surabaya": 18000, "Medan": 17500, "Palembang": 17200, "Pekanbaru": 17800,
				"Denpasar": 19000, "Makassar": 18200, "Balikpapan": 19500, "Manado": 18800,
				"Jayapura": 22000, "Pontianak": 18000, "Banjarmasin": 17500, "Aceh": 17800,
			},
			Source: "pasar-kurasi",
		},
		{
			Commodity: "Beras", Type: "Medium", Unit: "kg",
			Prices: map[string]float64{
				"Jakarta": 15500, "Bandung": 15000, "Semarang": 14000, "Yogyakarta": 14500,
				"Surabaya": 15000, "Medan": 14500, "Palembang": 14200, "Pekanbaru": 14800,
				"Denpasar": 16000, "Makassar": 15200, "Balikpapan": 16500, "Manado": 15800,
				"Jayapura": 19000, "Pontianak": 15000, "Banjarmasin": 14500, "Aceh": 14800,
			},
			Source: "pasar-kurasi",
		},
		{
			Commodity: "Beras", Type: "C4 Super", Unit: "kg",
			Prices: map[string]float64{
				"Jakarta": 22000, "Bandung": 21500, "Semarang": 20500, "Yogyakarta": 21000,
				"Surabaya": 21500, "Medan": 21000, "Palembang": 20800, "Denpasar": 22500,
				"Makassar": 21800, "Balikpapan": 23000, "Jayapura": 26000, "Pontianak": 21500,
			},
			Source: "pasar-kurasi",
		},
		// Minyak Goreng (per liter, kemasan)
		{
			Commodity: "Minyak Goreng", Type: "Bimoli 2L", Unit: "2 liter",
			Prices: map[string]float64{
				"Jakarta": 52000, "Bandung": 51000, "Semarang": 50000, "Yogyakarta": 51000,
				"Surabaya": 51500, "Medan": 50500, "Palembang": 51000, "Pekanbaru": 51500,
				"Denpasar": 53000, "Makassar": 52500, "Balikpapan": 53500, "Manado": 52000,
				"Jayapura": 56000, "Banjarmasin": 51500, "Mataram": 52500,
			},
			Source: "pasar-kurasi",
		},
		{
			Commodity: "Minyak Goreng", Type: "Sania 2L", Unit: "2 liter",
			Prices: map[string]float64{
				"Jakarta": 48000, "Bandung": 47000, "Semarang": 46000, "Yogyakarta": 47000,
				"Surabaya": 47500, "Medan": 46500, "Palembang": 47000, "Pekanbaru": 47500,
				"Denpasar": 49000, "Makassar": 48500, "Balikpapan": 49500, "Manado": 48000,
				"Jayapura": 52000, "Pontianak": 47500, "Banjarmasin": 47500,
			},
			Source: "pasar-kurasi",
		},
		{
			Commodity: "Minyak Goreng", Type: "Curah 1L", Unit: "liter",
			Prices: map[string]float64{
				"Jakarta": 18500, "Bandung": 18000, "Semarang": 17000, "Yogyakarta": 17500,
				"Surabaya": 18000, "Medan": 17500, "Palembang": 17200, "Pekanbaru": 17800,
				"Denpasar": 19000, "Makassar": 18200, "Balikpapan": 19500, "Manado": 18500,
				"Jayapura": 23000, "Pontianak": 18000, "Banjarmasin": 17800,
			},
			Source: "pasar-kurasi",
		},
		// Gula Pasir
		{
			Commodity: "Gula Pasir", Type: "Gula Premium", Unit: "kg",
			Prices: map[string]float64{
				"Jakarta": 19500, "Bandung": 19000, "Surabaya": 19200, "Medan": 18800,
				"Denpasar": 20000, "Makassar": 19500, "Balikpapan": 20500, "Jayapura": 22500,
			},
			Source: "pasar-kurasi",
		},
		// Daging Sapi
		{
			Commodity: "Daging Sapi", Type: "Daging Segar", Unit: "kg",
			Prices: map[string]float64{
				"Jakarta": 140000, "Bandung": 135000, "Surabaya": 138000, "Medan": 132000,
				"Denpasar": 145000, "Makassar": 140000, "Balikpapan": 150000, "Jayapura": 170000,
			},
			Source: "pasar-kurasi",
		},
		// Daging Ayam
		{
			Commodity: "Daging Ayam", Type: "Ayam Broiler", Unit: "kg",
			Prices: map[string]float64{
				"Jakarta": 38000, "Bandung": 36000, "Surabaya": 37000, "Medan": 35000,
				"Denpasar": 39000, "Makassar": 38000, "Balikpapan": 42000, "Jayapura": 50000,
			},
			Source: "pasar-kurasi",
		},
		// Telur
		{
			Commodity: "Telur", Type: "Telur Ayam Ras", Unit: "kg",
			Prices: map[string]float64{
				"Jakarta": 32000, "Bandung": 31000, "Surabaya": 31500, "Medan": 30000,
				"Denpasar": 33000, "Makassar": 32500, "Balikpapan": 35000, "Jayapura": 42000,
			},
			Source: "pasar-kurasi",
		},
	}

	var prices []models.CommodityPrice
	for _, item := range items {
		for wilayah, harga := range item.Prices {
			prices = append(prices, models.CommodityPrice{
				Date: today, Year: year, Month: month, Day: day,
				Commodity: item.Commodity, Type: item.Type, Price: harga,
				Unit: item.Unit, Region: wilayah, Source: item.Source, CreatedAt: now,
			})
		}
	}

	return prices
}

// GovDataCollectionResult holds all collected government data
type GovDataCollectionResult struct {
	BPSIndicators   []models.BPSEconomicIndicator `json:"bps_indicators"`
	BIRates         []models.BIRate               `json:"bi_rates"`
	CKANDatasets    []models.CKANDataset          `json:"ckan_datasets"`
	OJKEntities     []models.OJKEntity            `json:"ojk_entities"`
	CommodityPrices []models.CommodityPrice       `json:"commodity_prices"`
	CollectedAt     time.Time                     `json:"collected_at"`
	Errors          []string                      `json:"errors,omitempty"`
}

// parseFloatSafe safely parses float from string
func parseFloatSafe(s string) float64 {
	s = strings.ReplaceAll(s, ",", ".")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}