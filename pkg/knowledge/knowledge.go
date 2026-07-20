package knowledge

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anton/pov-ai-indonesia/pkg/ai"
	"github.com/anton/pov-ai-indonesia/pkg/models"
)

type Database interface {
	GetExchangeRates(year, month int) ([]models.ExchangeRate, error)
	GetFuelPrices(year, month int, bbmType, region string) ([]models.FuelPrice, error)
	GetBPSIndicators(year int, indicatorID, region string) ([]models.BPSEconomicIndicator, error)
	GetBIRates(year int, rateType string) ([]models.BIRate, error)
	GetCommodityPrices(year, month int, commodity, region string) ([]models.CommodityPrice, error)
}

type KnowledgeManager struct {
	knowledgeDir string
	aiClient     *ai.Client
	db           Database
}

func NewKnowledgeManager(knowledgeDir string, aiClient *ai.Client, db Database) *KnowledgeManager {
	os.MkdirAll(knowledgeDir, 0755)
	return &KnowledgeManager{
		knowledgeDir: knowledgeDir,
		aiClient:     aiClient,
		db:           db,
	}
}

func (km *KnowledgeManager) getMacroContext(year, month int) string {
	if km.db == nil {
		return "Konteks makroekonomi tidak tersedia."
	}
	var sb strings.Builder

	indicators, err := km.db.GetBPSIndicators(year, "", "Indonesia")
	if err == nil && len(indicators) > 0 {
		sb.WriteString("Indikator Ekonomi BPS:\n")
		for _, ind := range indicators {
			sb.WriteString(fmt.Sprintf("- %s (%s, %s): %.2f %s\n", ind.Indicator, ind.Period, getPeriodName(ind.Year, 0), ind.Value, ind.Unit))
		}
		sb.WriteString("\n")
	}

	rates, err := km.db.GetBIRates(year, "")
	if err == nil && len(rates) > 0 {
		sb.WriteString("Suku Bunga Kebijakan Bank Indonesia (BI):\n")
		for _, r := range rates {
			sb.WriteString(fmt.Sprintf("- %s (efektif %s): %.2f%%\n", r.RateType, r.EffectiveDate, r.Value))
		}
		sb.WriteString("\n")
	}

	if sb.Len() == 0 {
		return "Tidak ada data indikator makroekonomi domestik spesifik untuk periode ini."
	}
	return sb.String()
}

func (km *KnowledgeManager) RegenerateAll(years []int) {
	llmOK := km.aiClient.IsEnabled()
	total := 0
	generatedFiles := make(map[string]bool)

	for _, year := range years {
		rates, err := km.db.GetExchangeRates(year, 0)
		if err != nil {
			fmt.Printf("[knowledge] Error getting rates for %d: %v\n", year, err)
		} else if len(rates) > 0 {
			if _, err := km.genExchangeRate(rates, year, 0, llmOK); err == nil {
				total++
				period := fmt.Sprintf("%d", year)
				generatedFiles[fmt.Sprintf("exchange_rate_%s.md", period)] = true
			}
			if year >= 2024 {
				for m := 1; m <= 12; m++ {
					mr, err := km.db.GetExchangeRates(year, m)
					if err != nil {
						continue
					}
					if len(mr) > 0 {
						if _, err := km.genExchangeRate(mr, year, m, llmOK); err == nil {
							total++
							period := fmt.Sprintf("%d-%02d", year, m)
							generatedFiles[fmt.Sprintf("exchange_rate_%s.md", period)] = true
						}
					}
				}
			}
		}

		fuel, err := km.db.GetFuelPrices(year, 0, "", "Indonesia")
		if err != nil {
			fmt.Printf("[knowledge] Error getting fuel for %d: %v\n", year, err)
		} else if len(fuel) > 0 {
			if _, err := km.genFuelPrice(fuel, year, 0, llmOK); err == nil {
				total++
				period := fmt.Sprintf("%d", year)
				generatedFiles[fmt.Sprintf("fuel_price_%s.md", period)] = true
			}
			if year >= 2024 {
				for m := 1; m <= 12; m++ {
					mf, err := km.db.GetFuelPrices(year, m, "", "Indonesia")
					if err != nil {
						continue
					}
					if len(mf) > 0 {
						if _, err := km.genFuelPrice(mf, year, m, llmOK); err == nil {
							total++
							period := fmt.Sprintf("%d-%02d", year, m)
							generatedFiles[fmt.Sprintf("fuel_price_%s.md", period)] = true
						}
					}
				}
			}
		}

		gold, err := km.db.GetCommodityPrices(year, 0, "Emas", "")
		if err != nil {
			fmt.Printf("[knowledge] Error getting gold prices for %d: %v\n", year, err)
		} else if len(gold) > 0 {
			if _, err := km.genGoldPrice(gold, year, 0, llmOK); err == nil {
				total++
				period := fmt.Sprintf("%d", year)
				generatedFiles[fmt.Sprintf("gold_price_%s.md", period)] = true
			}
			if year >= 2024 {
				for m := 1; m <= 12; m++ {
					mg, err := km.db.GetCommodityPrices(year, m, "Emas", "")
					if err != nil {
						continue
					}
					if len(mg) > 0 {
						if _, err := km.genGoldPrice(mg, year, m, llmOK); err == nil {
							total++
							period := fmt.Sprintf("%d-%02d", year, m)
							generatedFiles[fmt.Sprintf("gold_price_%s.md", period)] = true
						}
					}
				}
			}
		}
	}

	entries, err := os.ReadDir(km.knowledgeDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				if !generatedFiles[e.Name()] {
					os.Remove(filepath.Join(km.knowledgeDir, e.Name()))
				}
			}
		}
	}

	fmt.Printf("[knowledge] Generated %d insights (LLM: %v)\n", total, llmOK)
}

func (km *KnowledgeManager) RegenerateAllTemplates(years []int) {
	total := 0

	entries, err := os.ReadDir(km.knowledgeDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				os.Remove(filepath.Join(km.knowledgeDir, e.Name()))
			}
		}
	}

	for _, year := range years {
		rates, err := km.db.GetExchangeRates(year, 0)
		if err != nil {
			fmt.Printf("[knowledge] Error getting rates for %d: %v\n", year, err)
		} else if len(rates) > 0 {
			if _, err := km.genExchangeRate(rates, year, 0, false); err == nil {
				total++
			}
			if year >= 2024 {
				for m := 1; m <= 12; m++ {
					mr, err := km.db.GetExchangeRates(year, m)
					if err != nil {
						continue
					}
					if len(mr) > 0 {
						if _, err := km.genExchangeRate(mr, year, m, false); err == nil {
							total++
						}
					}
				}
			}
		}

		fuel, err := km.db.GetFuelPrices(year, 0, "", "Indonesia")
		if err != nil {
			fmt.Printf("[knowledge] Error getting fuel for %d: %v\n", year, err)
		} else if len(fuel) > 0 {
			if _, err := km.genFuelPrice(fuel, year, 0, false); err == nil {
				total++
			}
			if year >= 2024 {
				for m := 1; m <= 12; m++ {
					mf, err := km.db.GetFuelPrices(year, m, "", "Indonesia")
					if err != nil {
						continue
					}
					if len(mf) > 0 {
						if _, err := km.genFuelPrice(mf, year, m, false); err == nil {
							total++
						}
					}
				}
			}
		}

		gold, err := km.db.GetCommodityPrices(year, 0, "Emas", "")
		if err != nil {
			fmt.Printf("[knowledge] Error getting gold prices for %d: %v\n", year, err)
		} else if len(gold) > 0 {
			if _, err := km.genGoldPrice(gold, year, 0, false); err == nil {
				total++
			}
			if year >= 2024 {
				for m := 1; m <= 12; m++ {
					mg, err := km.db.GetCommodityPrices(year, m, "Emas", "")
					if err != nil {
						continue
					}
					if len(mg) > 0 {
						if _, err := km.genGoldPrice(mg, year, m, false); err == nil {
							total++
						}
					}
				}
			}
		}
	}

	fmt.Printf("[knowledge] Generated %d template insights\n", total)
}

func (km *KnowledgeManager) genExchangeRate(rates []models.ExchangeRate, year, month int, llm bool) (*models.KnowledgeEntry, error) {
	period := fmt.Sprintf("%d", year)
	if month > 0 {
		period = fmt.Sprintf("%d-%02d", year, month)
	}

	var title, summary, analysis string
	var factors []Factor

	if llm {
		dataJSON := formatRatesData(rates)
		macroContext := km.getMacroContext(year, month)
		resp, err := km.aiClient.AnalyzeExchangeRate(dataJSON, period, macroContext)
		if err == nil {
			title = resp.Title
			summary = resp.Summary
			factors = []Factor{}
			for _, f := range resp.Factors {
				factors = append(factors, Factor{Title: f.Title, Description: f.Description})
			}
			analysis = resp.Analysis
		} else {
			fmt.Printf("[knowledge] AI ExchangeRate analysis failed, using fallback: %v\n", err)
		}
	}

	if title == "" {
		title, summary, factors, analysis = exchangeRateFallback(year, month, rates)
	}

	content := buildExchangeRateMarkdown(title, summary, factors, analysis, rates)
	fp := filepath.Join(km.knowledgeDir, fmt.Sprintf("exchange_rate_%s.md", period))
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		return nil, err
	}

	return &models.KnowledgeEntry{
		Title:       title,
		Category:    "exchange_rate",
		Period:      period,
		Summary:     summary,
		Content:     content,
		Factors:     FactorsToStrings(factors),
		GeneratedAt: time.Now(),
	}, nil
}

func (km *KnowledgeManager) genFuelPrice(prices []models.FuelPrice, year, month int, llm bool) (*models.KnowledgeEntry, error) {
	period := fmt.Sprintf("%d", year)
	if month > 0 {
		period = fmt.Sprintf("%d-%02d", year, month)
	}

	var title, summary, analysis string
	var factors []Factor

	if llm {
		dataJSON := formatFuelData(prices)
		macroContext := km.getMacroContext(year, month)
		resp, err := km.aiClient.AnalyzeFuelPrice(dataJSON, period, macroContext)
		if err == nil {
			title = resp.Title
			summary = resp.Summary
			factors = []Factor{}
			for _, f := range resp.Factors {
				factors = append(factors, Factor{Title: f.Title, Description: f.Description})
			}
			analysis = resp.Analysis
		} else {
			fmt.Printf("[knowledge] AI FuelPrice analysis failed, using fallback: %v\n", err)
		}
	}

	if title == "" {
		title, summary, factors, analysis = fuelPriceFallback(year, month, prices)
	}

	content := buildFuelPriceMarkdown(title, summary, factors, analysis, prices)
	fp := filepath.Join(km.knowledgeDir, fmt.Sprintf("fuel_price_%s.md", period))
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		return nil, err
	}

	return &models.KnowledgeEntry{
		Title:       title,
		Category:    "fuel_price",
		Period:      period,
		Summary:     summary,
		Content:     content,
		Factors:     FactorsToStrings(factors),
		GeneratedAt: time.Now(),
	}, nil
}

func (km *KnowledgeManager) genGoldPrice(prices []models.CommodityPrice, year, month int, llm bool) (*models.KnowledgeEntry, error) {
	period := fmt.Sprintf("%d", year)
	if month > 0 {
		period = fmt.Sprintf("%d-%02d", year, month)
	}

	var title, summary, analysis string
	var factors []Factor

	if llm {
		dataJSON := formatGoldData(prices)
		macroContext := km.getMacroContext(year, month)
		resp, err := km.aiClient.AnalyzeGoldPrice(dataJSON, period, macroContext)
		if err == nil {
			title = resp.Title
			summary = resp.Summary
			factors = []Factor{}
			for _, f := range resp.Factors {
				factors = append(factors, Factor{Title: f.Title, Description: f.Description})
			}
			analysis = resp.Analysis
		} else {
			fmt.Printf("[knowledge] AI GoldPrice analysis failed, using fallback: %v\n", err)
		}
	}

	if title == "" {
		title, summary, factors, analysis = goldPriceFallback(year, month, prices)
	}

	content := buildGoldPriceMarkdown(title, summary, factors, analysis, prices)
	fp := filepath.Join(km.knowledgeDir, fmt.Sprintf("gold_price_%s.md", period))
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		return nil, err
	}

	return &models.KnowledgeEntry{
		Title:       title,
		Category:    "gold_price",
		Period:      period,
		Summary:     summary,
		Content:     content,
		Factors:     FactorsToStrings(factors),
		GeneratedAt: time.Now(),
	}, nil
}

func (km *KnowledgeManager) GenerateExchangeRateInsight(rates []models.ExchangeRate, year, month int) (*models.KnowledgeEntry, error) {
	return km.genExchangeRate(rates, year, month, km.aiClient.IsEnabled())
}

func (km *KnowledgeManager) GenerateFuelPriceInsight(prices []models.FuelPrice, year, month int) (*models.KnowledgeEntry, error) {
	return km.genFuelPrice(prices, year, month, km.aiClient.IsEnabled())
}

func (km *KnowledgeManager) GenerateGoldPriceInsight(prices []models.CommodityPrice, year, month int) (*models.KnowledgeEntry, error) {
	return km.genGoldPrice(prices, year, month, km.aiClient.IsEnabled())
}

func (km *KnowledgeManager) LoadInsight(category, period string) (*models.KnowledgeEntry, error) {
	fp := filepath.Join(km.knowledgeDir, fmt.Sprintf("%s_%s.md", category, period))
	content, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("not found")
	}
	return &models.KnowledgeEntry{
		Category: category,
		Period:   period,
		Content:  string(content),
	}, nil
}

func (km *KnowledgeManager) ListInsights() ([]models.KnowledgeEntry, error) {
	entries, err := os.ReadDir(km.knowledgeDir)
	if err != nil {
		return nil, err
	}
	var insights []models.KnowledgeEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		content, _ := os.ReadFile(filepath.Join(km.knowledgeDir, e.Name()))
		name := strings.TrimSuffix(e.Name(), ".md")
		parts := strings.SplitN(name, "_", 2)
		insights = append(insights, models.KnowledgeEntry{
			Category: parts[0],
			Period:   parts[len(parts)-1],
			Content:  string(content),
		})
	}
	return insights, nil
}

// --- Factor type for rich analysis ---

type Factor struct {
	Title       string
	Description string
}

func FactorsFromStrings(ss []string) []Factor {
	var factors []Factor
	for _, s := range ss {
		factors = append(factors, Factor{Title: s, Description: ""})
	}
	return factors
}

func FactorsToStrings(factors []Factor) []string {
	var ss []string
	for _, f := range factors {
		ss = append(ss, f.Title)
	}
	return ss
}

// --- Data formatting ---

func formatRatesData(rates []models.ExchangeRate) string {
	var lines []string
	for _, r := range rates {
		lines = append(lines, fmt.Sprintf("%s: Rp %.0f", r.Date, r.Rate))
	}
	return strings.Join(lines, "\n")
}

func formatFuelData(prices []models.FuelPrice) string {
	keyMap := make(map[string][]float64)
	for _, p := range prices {
		key := fmt.Sprintf("%s|%s", p.Date, p.BBMType)
		keyMap[key] = append(keyMap[key], p.Price)
	}
	var lines []string
	for key, prs := range keyMap {
		parts := strings.SplitN(key, "|", 2)
		avg := 0.0
		for _, p := range prs {
			avg += p
		}
		avg /= float64(len(prs))
		lines = append(lines, fmt.Sprintf("%s %s: Rp %.0f", parts[0], parts[1], avg))
	}
	return strings.Join(lines, "\n")
}

func getPeriodName(year, month int) string {
	if month <= 0 {
		return fmt.Sprintf("%d", year)
	}
	months := []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	if month >= 1 && month <= 12 {
		return fmt.Sprintf("%s %d", months[month], year)
	}
	return fmt.Sprintf("%d-%02d", year, month)
}

// --- Exchange Rate Fallback Analysis ---

func exchangeRateFallback(year, month int, rates []models.ExchangeRate) (string, string, []Factor, string) {
	if len(rates) == 0 {
		return "Analisis USD/IDR", "Data tidak tersedia", []Factor{}, "Tidak ada data untuk dianalisis."
	}

	lastRate := rates[len(rates)-1].Rate
	avgRate := calcAvg(rates)
	minRate, maxRate := calcMinMax(rates)

	trend := "stabil"
	trendDesc := "pergerakan sideways"
	if len(rates) >= 2 {
		ch := ((rates[len(rates)-1].Rate - rates[0].Rate) / rates[0].Rate) * 100
		if ch > 5 {
			trend = "pelemahan"
			trendDesc = fmt.Sprintf("pelemahan Rupiah sebesar %.1f%%", ch)
		} else if ch > 2 {
			trend = "pelemahan ringan"
			trendDesc = fmt.Sprintf("pelemahan ringan Rupiah sebesar %.1f%%", ch)
		} else if ch < -5 {
			trend = "penguatan"
			trendDesc = fmt.Sprintf("penguatan Rupiah sebesar %.1f%%", math.Abs(ch))
		} else if ch < -2 {
			trend = "penguatan ringan"
			trendDesc = fmt.Sprintf("penguatan ringan Rupiah sebesar %.1f%%", math.Abs(ch))
		}
	}

	deviation := ((lastRate - avgRate) / avgRate) * 100
	volatility := ((maxRate - minRate) / avgRate) * 100

	periodName := getPeriodName(year, month)

	// Build factors with detailed explanations
	var factors []Factor

	factors = append(factors, Factor{
		Title: "Kebijakan Moneter The Fed (Federal Reserve)",
		Description: "The Fed menahan suku bunga di level tinggi untuk mengendalikan inflasi AS. Suku bunga tinggi membuat obligasi AS lebih menarik, mengalirkan dolar dari pasar emerging market termasuk Indonesia. Kebijakan moneter ketat The Fed menjadi tekanan utama bagi nilai tukar negara berkembang, di mana aliran modal asing keluar dari pasar saham dan SBN Indonesia menekan Rupiah.",
	})

	factors = append(factors, Factor{
		Title: "Kondisi Ekonomi Domestik & Defisit Fiskal",
		Description: "Pertumbuhan ekonomi Indonesia diproyeksikan di kisaran 5,0%-5,2% dengan target pemerintah yang ambitius. Defisit fiskal yang melebar untuk membiayai berbagai program prioritas nasional menambah stok utang pemerintah. Investor memperhatikan keberlanjutan fiskal, yang berdampak pada sentimen terhadap Rupiah.",
	})

	factors = append(factors, Factor{
		Title: "Harga Komoditas Internasional",
		Description: "Harga CPO (minyak kelapa sawit) dan batu bara — dua komoditas ekspor utama Indonesia — berfluktuasi di pasar global. Permintaan dari negara tujuan ekspor (China, India, Uni Eropa) dan kebijakan energi global mempengaruhi harga komoditas. Fluktuasi harga komoditas berdampak pada pendapatan ekspor dan posisi neraca perdagangan Indonesia.",
	})

	if deviation > 3 {
		factors = append(factors, Factor{
			Title: "Tekanan Geopolitik & Ketidakpastian Global",
			Description: fmt.Sprintf("Konflik Timur Middle (Israel-Iran) dan perang Rusia-Ukraina yang belum selesai meningkatkan risk-off sentiment. Investor beralih ke aset safe-haven seperti dolar AS dan emas. Rupiah sebagai mata uang emerging market tertekan, dengan volatilitas mencapai %.1f%% di atas rata-rata historis.", volatility),
		})
	}

	if volatility > 5 {
		factors = append(factors, Factor{
			Title: "Volatilitas Tinggi — Waspadai Fluktuasi",
			Description: fmt.Sprintf("Rentang pergerakan Rp %.0f - Rp %.0f (selisih Rp %.0f) menunjukkan volatilitas %.1f%%. Volatilitas tinggi membuat perencanaan bisnis dan keuangan menjadi lebih sulit. Pelaku usaha impor dan eksportir perlu melakukan hedging untuk mengurangi risiko nilai tukar.", minRate, maxRate, maxRate-minRate, volatility),
		})
	}

	// Monthly comparison
	var monthlyComp string
	if len(rates) >= 2 {
		prevRate := rates[len(rates)-2].Rate
		monthCh := ((lastRate - prevRate) / prevRate) * 100
		if monthCh > 0 {
			monthlyComp = fmt.Sprintf("Rate naik %.1f%% dari periode sebelumnya (Rp %.0f), menunjukkan tekanan beli dolar meningkat.", monthCh, prevRate)
		} else if monthCh < 0 {
			monthlyComp = fmt.Sprintf("Rate turun %.1f%% dari periode sebelumnya (Rp %.0f), Rupiah sedikit menguat.", math.Abs(monthCh), prevRate)
		} else {
			monthlyComp = fmt.Sprintf("Rate stabil dari periode sebelumnya (Rp %.0f).", prevRate)
		}
	}

	title := fmt.Sprintf("Analisis USD/IDR %s", periodName)
	summary := fmt.Sprintf("Nilai tukar USD/IDR %s: tren %s. Rate terakhir Rp %.0f, rata-rata Rp %.0f (deviasi %.1f%%). Rentang: Rp %.0f - Rp %.0f (volatilitas %.1f%%).",
		periodName, trend, lastRate, avgRate, deviation, minRate, maxRate, volatility)

	analysis := fmt.Sprintf(`**Ringkasan Pergerakan:**
- Rate terakhir: Rp %.0f per 1 USD
- Rata-rata periode: Rp %.0f
- Rate terendah: Rp %.0f
- Rate tertinggi: Rp %.0f
- Selisih min-max: Rp %.0f (%.1f%% dari rata-rata)
- Tren: %s (%s)
- %s

**Implikasi untuk Masyarakat:**
- Harga barang impor (elektronik, kendaraan, bahan pokok impor) cenderung naik
- Daya beli masyarakat tertekan, terutama untuk produk berbasis impor
- Biaya pendidikan dan perjalanan ke luar negeri lebih mahal
- Bagi eksportir, pelemahan Rupiah menguntungkan karena pendapatan dalam dolar lebih besar

**Rekomendasi:**
- Bagi yang memiliki utang dalam dolar, pertimbangkan hedging
- Bagi calon mahasiswa luar negeri, anggaran perlu ditambah 5-10%% dari perkiraan awal
- Bagi pelaku UMKM impor, cari alternatif lokal atau negosiasi ulang harga dengan supplier`,
		lastRate, avgRate, minRate, maxRate, maxRate-minRate, volatility,
		trend, trendDesc, monthlyComp,
	)

	return title, summary, factors, analysis
}

// --- Fuel Price Fallback Analysis ---

func fuelPriceFallback(year, month int, prices []models.FuelPrice) (string, string, []Factor, string) {
	if len(prices) == 0 {
		return "Analisis Harga BBM", "Data tidak tersedia", []Factor{}, "Tidak ada data untuk dianalisis."
	}

	typeStats := make(map[string][]float64)
	for _, p := range prices {
		typeStats[p.BBMType] = append(typeStats[p.BBMType], p.Price)
	}

	var bbmTypes []string
	for bbm := range typeStats {
		bbmTypes = append(bbmTypes, bbm)
	}
	sort.Strings(bbmTypes)

	var avgLines []string
	var totalAvg float64
	var count int
	for _, bbm := range bbmTypes {
		prs := typeStats[bbm]
		avg := 0.0
		for _, p := range prs {
			avg += p
		}
		avg /= float64(len(prs))
		avgLines = append(avgLines, fmt.Sprintf("%s: Rp %.0f/liter", bbm, avg))
		totalAvg += avg
		count++
	}
	if count > 0 {
		totalAvg /= float64(count)
	}

	var cheapest, mostExpensive string
	var cheapestPrice, mostExpensivePrice float64
	first := true
	for bbm, prs := range typeStats {
		avg := 0.0
		for _, p := range prs {
			avg += p
		}
		avg /= float64(len(prs))
		if first || avg < cheapestPrice {
			cheapestPrice = avg
			cheapest = bbm
		}
		if first || avg > mostExpensivePrice {
			mostExpensivePrice = avg
			mostExpensive = bbm
		}
		first = false
	}

	periodName := getPeriodName(year, month)
	title := fmt.Sprintf("Analisis Harga BBM %s", periodName)
	summary := fmt.Sprintf("Harga BBM %s: rata-rata Rp %.0f/liter. Termurah: %s (Rp %.0f), termahal: %s (Rp %.0f).",
		periodName, totalAvg, cheapest, cheapestPrice, mostExpensive, mostExpensivePrice)

	var factors []Factor

	factors = append(factors, Factor{
		Title: "Kebijakan Subsidi & Harga Jual Pemerintah",
		Description: fmt.Sprintf("Pemerintah menetapkan harga jual BBM melalui Pertamina dengan mempertimbangkan harga minyak mentah global (ICP), nilai tukar Rupiah, dan margin distribusi. Perubahan harga BBM selalu melalui kajian dan persetujuan Kementerian ESDM. Subsidi dialokasikan untuk Pertalite dan Solar yang dinikmati masyarakat berpenghasilan menengah ke bawah."),
	})

	factors = append(factors, Factor{
		Title: "Harga Minyak Mentah Global (WTI & Brent)",
		Description: "Harga minyak mentah global menjadi penentu utama harga BBM. WTI (West Texas Intermediate) dan Brent Crude berfluktuasi berdasarkan keputusan OPEC+ tentang kuota produksi, permintaan global (terutama dari China dan AS), serta kondisi geopolitik. Fluktuasi harga minyak mentah berdampak langsung pada biaya produksi BBM dalam negeri.",
	})

	factors = append(factors, Factor{
		Title: "Nilai Tukar Rupiah terhadap USD",
		Description: "Karena minyak mentah diperdagangkan dalam dolar AS, pelemahan Rupiah langsung meningkatkan biaya impor BBM. Setiap pelemahan Rp 1.000/USD berpotensi menaikkan harga BBM Rp 300-500/liter. Fluktuasi nilai tukar menjadi faktor penting dalam penentuan harga BBM domestik.",
	})

	factors = append(factors, Factor{
		Title: "Permintaan Domestik & Musiman",
		Description: "Konsumsi BBM Indonesia termasuk yang tertinggi di Asia Tenggara dengan tren meningkat setiap tahun. Permintaan meningkat signifikan saat musim mudik Lebaran, Natal/Tahun Baru, dan libur panjang nasional. Kenaikan permintaan musiman ini seringkali menyebabkan kelangkaan di daerah terpencil dan tekanan pada harga.",
	})

	analysis := fmt.Sprintf(`**Rata-rata Harga BBM:**
%s

**Perbandingan:**
- BBM termurah: %s (Rp %.0f/liter)
- BBM termahal: %s (Rp %.0f/liter)
- Selisih: Rp %.0f/liter

**Implikasi untuk Masyarakat:**
- Biaya transportasi sehari-hari terpengaruh langsung
- Harga barang kebutuhan pokok (beras, sayuran, daging) ikut naik karena biaya distribusi
- Daya beli masyarakat tertekan, terutama kelompok menengah ke bawah
- Bagi pengendara motor, biaya BBM bulanan bisa mencapai Rp 300.000-500.000

**Rekomendasi:**
- Pertimbangkan penggunaan transportasi umum untuk menghemat biaya
- Bisa membeli BBM di SPBU resmi untuk mendapatkan harga terjamin
- Pantau informasi kebijakan BBM terbaru dari Kementerian ESDM`,
		strings.Join(avgLines, "\n"),
		cheapest, cheapestPrice, mostExpensive, mostExpensivePrice,
		mostExpensivePrice-cheapestPrice,
	)

	return title, summary, factors, analysis
}

func calcAvg(rates []models.ExchangeRate) float64 {
	if len(rates) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range rates {
		sum += r.Rate
	}
	return sum / float64(len(rates))
}

func calcMinMax(rates []models.ExchangeRate) (min, max float64) {
	if len(rates) == 0 {
		return 0, 0
	}
	min = rates[0].Rate
	max = rates[0].Rate
	for _, r := range rates {
		if r.Rate < min {
			min = r.Rate
		}
		if r.Rate > max {
			max = r.Rate
		}
	}
	return min, max
}

// --- Markdown builders ---

func buildExchangeRateMarkdown(title, summary string, factors []Factor, analysis string, rates []models.ExchangeRate) string {
	md := fmt.Sprintf("# %s\n\n*Generated by POV AI - %s*\n\n---\n\n## Ringkasan\n\n%s\n\n## Faktor-faktor Penyebab\n\n",
		title, time.Now().Format("2006-01-02 15:04:00"), summary)
	for i, f := range factors {
		md += fmt.Sprintf("### %d. %s\n\n%s\n\n", i+1, f.Title, f.Description)
	}
	if analysis != "" {
		md += fmt.Sprintf("## Analisis Detail\n\n%s\n\n", analysis)
	}
	md += "\n## Data Nilai Tukar\n\n| Tanggal | Rate (IDR/USD) |\n|---------|----------------|\n"
	for _, r := range rates {
		md += fmt.Sprintf("| %s | Rp %.0f |\n", r.Date, r.Rate)
	}
	md += "\n---\n\n*Analisis ini dihasilkan oleh POV AI berdasarkan data historis. Bukan saran keuangan.*\n"
	return md
}

func buildFuelPriceMarkdown(title, summary string, factors []Factor, analysis string, prices []models.FuelPrice) string {
	typeStats := make(map[string][]float64)
	for _, p := range prices {
		typeStats[p.BBMType] = append(typeStats[p.BBMType], p.Price)
	}

	md := fmt.Sprintf("# %s\n\n*Generated by POV AI - %s*\n\n---\n\n## Ringkasan\n\n%s\n\n## Faktor-faktor Penyebab\n\n",
		title, time.Now().Format("2006-01-02 15:04:00"), summary)
	for i, f := range factors {
		md += fmt.Sprintf("### %d. %s\n\n%s\n\n", i+1, f.Title, f.Description)
	}
	if analysis != "" {
		md += fmt.Sprintf("## Analisis Detail\n\n%s\n\n", analysis)
	}
	md += "\n## Data Harga BBM\n\n| Jenis BBM | Harga Rata-rata (Rp/liter) |\n|-----------|--------------------------|\n"
	for bbm, prs := range typeStats {
		avg := 0.0
		for _, p := range prs {
			avg += p
		}
		avg /= float64(len(prs))
		md += fmt.Sprintf("| %s | Rp %.0f |\n", bbm, avg)
	}
	md += "\n---\n\n*Analisis ini dihasilkan oleh POV AI berdasarkan data historis. Bukan saran keuangan.*\n"
	return md
}

func formatGoldData(prices []models.CommodityPrice) string {
	var lines []string
	for _, p := range prices {
		lines = append(lines, fmt.Sprintf("%s - %s (%s, %s): Rp %.0f", p.Date, p.Commodity, p.Type, p.Region, p.Price))
	}
	return strings.Join(lines, "\n")
}

func goldPriceFallback(year, month int, prices []models.CommodityPrice) (string, string, []Factor, string) {
	if len(prices) == 0 {
		return "Analisis Harga Emas", "Data tidak tersedia", []Factor{}, "Tidak ada data untuk dianalisis."
	}

	var latestAntam1g float64 = 2450000
	var latestSpot float64 = 2380000
	for _, p := range prices {
		if p.Type == "Antam 1g" && p.Price > 0 {
			latestAntam1g = p.Price
		}
		if p.Type == "Spot XAU/IDR" && p.Price > 0 {
			latestSpot = p.Price
		}
	}

	buybackEst := latestAntam1g * 0.895
	spread := latestAntam1g - buybackEst

	periodName := getPeriodName(year, month)
	title := fmt.Sprintf("Analisis Pasar Emas & Aset Safe Haven %s", periodName)
	summary := fmt.Sprintf("Harga Emas Antam 1g %s berada di kisaran Rp %.0f/gram dengan estimasi harga buyback Rp %.0f (spread Rp %.0f/g atau ~10.5%%). Emas tetap menjadi instrumen lindung nilai (inflation hedge) favorit masyarakat Indonesia.",
		periodName, latestAntam1g, buybackEst, spread)

	var factors []Factor

	factors = append(factors, Factor{
		Title: "Ekspektasi Suku Bunga The Fed & Kebijakan BI-Rate",
		Description: "Kebijakan suku bunga acuan bank sentral global (Federal Reserve) dan Bank Indonesia berdampak langsung pada opportunity cost memegang emas. Saat ekspektasi pemangkasan suku bunga meningkat, yield obligasi menurun sehingga daya tarik emas sebagai aset non-yielding meningkat.",
	})

	factors = append(factors, Factor{
		Title: "Fluktuasi Nilai Tukar USD/IDR & Indeks Dolar (DXY)",
		Description: fmt.Sprintf("Emas dunia didagang dalam USD (XAU/USD). Pelemahan Rupiah terhadap USD secara teknis menjaga harga emas Antam (IDR) tetap tinggi di pasar domestik, meskipun emas global sedang mengalami koreksi tipis. Saat ini Spot XAU/IDR berada di Rp %.0f/gram.", latestSpot),
	})

	factors = append(factors, Factor{
		Title: "Permintaan Safe Haven & Konflik Geopolitik Global",
		Description: "Ketegangan geopolitik internasional serta ketidakpastian pasar saham mendorong arus modal asing dan domestik ke aset aman (safe haven). Akumulasi cadangan emas oleh bank-bank sentral dunia juga memberikan landasan harga emas yang kuat.",
	})

	factors = append(factors, Factor{
		Title: "Margin Spread Buyback & Pajak PPh 22",
		Description: fmt.Sprintf("Spread buyback Antam berada di kisaran ~10.5%% (Rp %.0f/g). Berdasarkan PMK No. 34/PMK.10/2017, penjualan kembali emas ke PT Antam Tbk dengan nominal di atas Rp 10 juta dikenakan PPh 22 sebesar 1,5%% untuk pemegang NPWP dan 3%% untuk non-NPWP.", spread),
	})

	analysis := fmt.Sprintf(`**Detail Harga & Spread Emas %s:**
- **Harga Jual Antam 1g:** Rp %.0f
- **Estimasi Harga Buyback:** Rp %.0f
- **Selisih Spread Buyback:** Rp %.0f/gram (~10.5%%)
- **Live Spot XAU/IDR:** Rp %.0f/gram

**Implikasi Investasi untuk Masyarakat:**
- Emas fisik sangat cocok untuk investasi jangka menengah-panjang (>2-3 tahun) untuk menutup beban *spread buyback*.
- Untuk keperluan likuiditas jangka pendek (< 1 tahun), perhatikan bahwa *spread* jual-beli memerlukan kenaikan harga minimal 10-11%% agar mencapai titik *break-even*.
- Pembelian pecahan besar (50g, 100g, 1000g) memiliki premi cetak per gram yang jauh lebih rendah dibanding pecahan kecil (0.5g atau 1g).

**Rekomendasi Strategi Alokasi:**
- Pertahankan alokasi emas 10%% - 20%% dari total portofolio investasi sebagai benteng pertahanan risiko inflasi dan devaluasi mata uang.
- Manfaatkan metode Dollar Cost Averaging (DCA) atau cicil emas untuk memitigasi risiko volatilitas harga harian.`,
		periodName, latestAntam1g, buybackEst, spread, latestSpot,
	)

	return title, summary, factors, analysis
}

func buildGoldPriceMarkdown(title, summary string, factors []Factor, analysis string, prices []models.CommodityPrice) string {
	md := fmt.Sprintf("# %s\n\n*Generated by POV AI - %s*\n\n---\n\n## Ringkasan\n\n%s\n\n## Faktor-faktor Penyebab\n\n",
		title, time.Now().Format("2006-01-02 15:04:00"), summary)
	for i, f := range factors {
		md += fmt.Sprintf("### %d. %s\n\n%s\n\n", i+1, f.Title, f.Description)
	}
	if analysis != "" {
		md += fmt.Sprintf("## Analisis Detail\n\n%s\n\n", analysis)
	}
	md += "\n## Data Harga Emas & Pecahan\n\n| Jenis Emas | Harga (Rp) | Unit | Sumber |\n|------------|------------|------|--------|\n"
	for _, p := range prices {
		md += fmt.Sprintf("| %s | Rp %.0f | %s | %s |\n", p.Type, p.Price, p.Unit, p.Source)
	}
	md += "\n---\n\n*Analisis ini dihasilkan oleh POV AI berdasarkan data pasar historis. Bukan saran keuangan.*\n"
	return md
}