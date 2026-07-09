package db

import (
	"testing"

	"github.com/anton/pov-ai-indonesia/pkg/models"
)

func TestRound2(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.2345, 1.23},
		{1.2356, 1.24},
		{-1.2345, -1.23},
		{-1.2356, -1.24},
		{0.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			got := round2(tt.input)
			if got != tt.expected {
				t.Errorf("round2(%f) = %f, want %f", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCalculateExchangeTrend(t *testing.T) {
	dbInstance, err := New(":memory:", 1, 1, 60)
	if err != nil {
		t.Fatal(err)
	}
	defer dbInstance.Close()

	tests := []struct {
		name      string
		rates     []models.ExchangeRate
		expected  string
		expectedC float64
	}{
		{
			name:      "empty",
			rates:     nil,
			expected:  "stable",
			expectedC: 0.0,
		},
		{
			name:      "single element",
			rates:     []models.ExchangeRate{{Rate: 15000}},
			expected:  "stable",
			expectedC: 0.0,
		},
		{
			name: "up change < 0.5% (now returns up because different value)",
			rates: []models.ExchangeRate{
				{Rate: 15000, Date: "2024-01-01"},
				{Rate: 15010, Date: "2024-01-02"}, // ~0.07% increase
			},
			expected:  "up",
			expectedC: 0.07,
		},
		{
			name: "up change > 0.5%",
			rates: []models.ExchangeRate{
				{Rate: 15000, Date: "2024-01-01"},
				{Rate: 15100, Date: "2024-01-02"}, // ~0.67% increase
			},
			expected:  "up",
			expectedC: 0.67,
		},
		{
			name: "down change < -0.5%",
			rates: []models.ExchangeRate{
				{Rate: 15000, Date: "2024-01-01"},
				{Rate: 14900, Date: "2024-01-02"}, // ~-0.67% decrease
			},
			expected:  "down",
			expectedC: -0.67,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dbInstance.CalculateExchangeTrend(tt.rates, 0, 0)
			if got.Direction != tt.expected {
				t.Errorf("CalculateExchangeTrend() direction = %q, want %q", got.Direction, tt.expected)
			}
			if len(tt.rates) >= 2 && got.ChangePercent != tt.expectedC {
				t.Errorf("CalculateExchangeTrend() change percent = %f, want %f", got.ChangePercent, tt.expectedC)
			}
		})
	}
}

func TestCalculateFuelTrend(t *testing.T) {
	dbInstance, err := New(":memory:", 1, 1, 60)
	if err != nil {
		t.Fatal(err)
	}
	defer dbInstance.Close()

	tests := []struct {
		name      string
		prices    []models.FuelPrice
		expected  string
		expectedC float64
	}{
		{
			name:     "empty",
			prices:   nil,
			expected: "stable",
		},
		{
			name: "up change > 0.5%",
			prices: []models.FuelPrice{
				{BBMType: "Pertalite", Price: 10000, Date: "2024-01-01"},
				{BBMType: "Pertalite", Price: 11000, Date: "2024-01-02"},
			},
			expected:  "up",
			expectedC: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dbInstance.CalculateFuelTrend(tt.prices, 0, 0, "", "")
			if got.Direction != tt.expected {
				t.Errorf("CalculateFuelTrend() direction = %q, want %q", got.Direction, tt.expected)
			}
			if len(tt.prices) >= 2 && got.ChangePercent != tt.expectedC {
				t.Errorf("CalculateFuelTrend() change percent = %f, want %f", got.ChangePercent, tt.expectedC)
			}
		})
	}
}
