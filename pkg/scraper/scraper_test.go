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
