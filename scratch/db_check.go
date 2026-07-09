package main

import (
	"fmt"
	"log"

	"github.com/anton/pov-ai-indonesia/pkg/db"
)

func main() {
	database, err := db.New("data/povai.db", 1, 1, 60)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	erCount, _ := database.CountExchangeRates()
	fpCount, _ := database.CountFuelPrices()
	years, _ := database.GetExchangeRateYears()
	fuelYears, _ := database.GetFuelPriceYears()

	fmt.Printf("Exchange rates count: %d\n", erCount)
	fmt.Printf("Fuel prices count: %d\n", fpCount)
	fmt.Printf("Exchange rate years: %v\n", years)
	fmt.Printf("Fuel price years: %v\n", fuelYears)
}
