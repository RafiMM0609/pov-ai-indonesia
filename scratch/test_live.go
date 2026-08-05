package main

import (
	"fmt"
	"log"

	"github.com/anton/pov-ai-indonesia/internal/config"
	"github.com/anton/pov-ai-indonesia/pkg/db"
	"github.com/anton/pov-ai-indonesia/pkg/scraper"
)

func main() {
	cfg := config.Load()
	database, err := db.New(cfg.DBPath, 1, 1, 60)
	if err != nil {
		log.Fatalf("DB open error: %v", err)
	}
	defer database.Close()

	sc := scraper.New(
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
	)

	prices, err := sc.ScrapeFuelPrices()
	if err != nil {
		log.Fatalf("ScrapeFuelPrices error: %v", err)
	}
	fmt.Printf("Scraped %d live fuel prices for region: %s\n", len(prices), cfg.PertaminaDirectRegion)
	for _, p := range prices {
		fmt.Printf(" - %s: Rp %.2f (Region: %s, Source: %s, Date: %s)\n", p.BBMType, p.Price, p.Region, p.Source, p.Date)
	}

	n, err := database.InsertFuelPrices(prices)
	if err != nil {
		log.Fatalf("InsertFuelPrices error: %v", err)
	}
	fmt.Printf("Inserted/Updated %d fuel price records in SQLite database!\n", n)
}
