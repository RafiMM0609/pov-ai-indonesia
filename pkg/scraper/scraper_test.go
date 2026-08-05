package scraper

import "testing"

func TestParsePrice(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		hasError bool
	}{
		{"10.000", 10000, false},
		{"6.800", 6800, false},
		{"16.250", 16250, false},
		{"12,500", 12500, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parsePrice(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("parsePrice(%q) error = %v, want error = %v", tt.input, err, tt.hasError)
			}
			if got != tt.expected {
				t.Errorf("parsePrice(%q) = %f, want %f", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractLatestPrices(t *testing.T) {
	htmlSample := `
		<html>
		<body>
			<p>Pemerintah menyesuaikan harga BBM. Pertalite dijual tetap Rp 10.000 per liter.</p>
			<p>Harga Biosolar tetap Rp 6.800 per liter.</p>
			<p>Sementara itu, Pertamax (RON 92) naik menjadi Rp 16.250 per liter dari sebelumnya Rp12.300.</p>
			<p>Pertamax Green 95 naik menjadi Rp 17.000 per liter.</p>
			<p>Pertamax Turbo juga disesuaikan menjadi Rp 20.750 per liter.</p>
			<p>Dexlite menjadi Rp 23.000 per liter.</p>
			<p>Pertamina Dex saat ini dijual Rp 24.800 per liter.</p>
		</body>
		</html>
	`

	prices := extractLatestPrices(htmlSample)
	expectedMap := map[string]float64{
		"Pertalite":      10000,
		"Solar":          6800,
		"Pertamax":       16250,
		"Pertamax Green": 17000,
		"Pertamax Turbo": 20750,
		"Dexlite":        23000,
		"Pertamina Dex":  24800,
	}

	if len(prices) != len(expectedMap) {
		t.Errorf("Expected %d prices extracted, got %d", len(expectedMap), len(prices))
	}

	for _, p := range prices {
		expVal, exists := expectedMap[p.BBMType]
		if !exists {
			t.Errorf("Unexpected BBMType extracted: %s", p.BBMType)
			continue
		}
		if p.Price != expVal {
			t.Errorf("BBMType %s: expected price %f, got %f", p.BBMType, expVal, p.Price)
		}
		if p.Region != "Indonesia" {
			t.Errorf("Expected region 'Indonesia', got %q", p.Region)
		}
		if p.Source != "web-scrape" {
			t.Errorf("Expected source 'web-scrape', got %q", p.Source)
		}
	}
}

func TestParsePertaminaPatraNiagaJSON(t *testing.T) {
	jsonSample := []byte(`{
		"data": {
			"content": {
				"node1": {
					"displayName": "ProductTable",
					"props": {
						"items": [
							{
								"title": "Gasoline",
								"data": [
									{
										"WILAYAH": "Prov. DI Yogyakarta",
										"https://pertaminapatraniaga.com/file/files/2024/08/product-table-pertamax-turbo.png": " 18,300 ",
										"https://pertaminapatraniaga.com/file/files/2024/08/product-table-pertamax-green-95.png": " 16,600 ",
										"https://pertaminapatraniaga.com/file/files/2024/08/product-table-pertamax.png": " 15,950 ",
										"https://pertaminapatraniaga.com/file/files/2024/08/product-table-pertalite.png": " 10,000 "
									}
								]
							},
							{
								"title": "Gasoil",
								"data": [
									{
										"WILAYAH": "Prov. DI Yogyakarta",
										"https://pertaminapatraniaga.com/file/files/2024/08/product-table-pertamina-dex.png": " 21,150 ",
										"https://pertaminapatraniaga.com/file/files/2024/08/product-table-dexlite.png": " 19,700 ",
										"https://pertaminapatraniaga.com/file/files/2026/05/harga-produk-bio-solar-subsidi.jpg": " 6,800 "
									}
								]
							}
						]
					}
				}
			}
		}
	}`)

	prices := parsePertaminaPatraNiagaJSON(jsonSample, "2026-08-05", 2026, 8, 5, "Yogyakarta")
	expectedMap := map[string]float64{
		"Pertamax Turbo": 18300,
		"Pertamax Green": 16600,
		"Pertamax":       15950,
		"Pertalite":      10000,
		"Pertamina Dex":  21150,
		"Dexlite":        19700,
		"Solar":          6800,
	}

	if len(prices) != len(expectedMap) {
		t.Errorf("Expected %d fuel prices, got %d", len(expectedMap), len(prices))
	}

	for _, p := range prices {
		expVal, exists := expectedMap[p.BBMType]
		if !exists {
			t.Errorf("Unexpected BBMType extracted: %s", p.BBMType)
			continue
		}
		if p.Price != expVal {
			t.Errorf("BBMType %s: expected price %f, got %f", p.BBMType, expVal, p.Price)
		}
		if p.Region != "Yogyakarta" {
			t.Errorf("Expected region 'Yogyakarta', got %q", p.Region)
		}
		if p.Source != "pertamina-patra-niaga" {
			t.Errorf("Expected source 'pertamina-patra-niaga', got %q", p.Source)
		}
	}
}

func TestLivePertaminaDirectScrape(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in short mode")
	}
	s := New(
		map[string]float64{"Pertalite": 10000},
		"2020-01-01",
		30,
		"https://api.frankfurter.app",
		10,
		"https://html.duckduckgo.com/html/",
		"bisnis.com,cnbcindonesia.com",
		"https://pertaminapatraniaga.com/page/harga-terbaru-bbm",
		"",
		"Yogyakarta",
	)

	prices, err := s.ScrapeFuelPrices()
	if err != nil {
		t.Fatalf("ScrapeFuelPrices live failed: %v", err)
	}
	if len(prices) == 0 {
		t.Fatalf("Expected non-empty live fuel prices for Yogyakarta")
	}
	t.Logf("Successfully fetched %d live fuel prices for Yogyakarta:", len(prices))
	for _, p := range prices {
		t.Logf(" - %s: Rp %.2f (Region: %s, Source: %s)", p.BBMType, p.Price, p.Region, p.Source)
	}
}
