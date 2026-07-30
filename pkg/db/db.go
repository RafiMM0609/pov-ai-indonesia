package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/anton/pov-ai-indonesia/pkg/models"
	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func New(dbPath string, maxOpenConns, maxIdleConns int, connMaxLifetimeMinutes int) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	if maxOpenConns <= 0 {
		maxOpenConns = 1
	}
	if maxIdleConns <= 0 {
		maxIdleConns = 1
	}
	if connMaxLifetimeMinutes <= 0 {
		connMaxLifetimeMinutes = 60
	}

	conn.SetMaxOpenConns(maxOpenConns)
	conn.SetMaxIdleConns(maxIdleConns)
	conn.SetConnMaxLifetime(time.Duration(connMaxLifetimeMinutes) * time.Minute)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS exchange_rates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL UNIQUE,
			rate REAL NOT NULL,
			year INTEGER NOT NULL,
			month INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_er_year_month ON exchange_rates(year, month);
		CREATE INDEX IF NOT EXISTS idx_er_date ON exchange_rates(date);

		CREATE TABLE IF NOT EXISTS fuel_prices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			year INTEGER NOT NULL,
			month INTEGER NOT NULL,
			day INTEGER NOT NULL,
			bbm_type TEXT NOT NULL,
			price REAL NOT NULL,
			region TEXT NOT NULL DEFAULT 'Indonesia',
			source TEXT NOT NULL DEFAULT 'manual',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(date, bbm_type, region)
		);

		CREATE INDEX IF NOT EXISTS idx_fp_date ON fuel_prices(date);
		CREATE INDEX IF NOT EXISTS idx_fp_bbm ON fuel_prices(bbm_type);
		CREATE INDEX IF NOT EXISTS idx_fp_year_month ON fuel_prices(year, month);
		CREATE INDEX IF NOT EXISTS idx_fp_region ON fuel_prices(region);

		-- BPS Economic Indicators
		CREATE TABLE IF NOT EXISTS bps_indicators (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			indicator_id TEXT NOT NULL,
			indicator TEXT NOT NULL,
			value REAL NOT NULL,
			unit TEXT NOT NULL,
			year INTEGER NOT NULL,
			period TEXT NOT NULL,
			region TEXT NOT NULL DEFAULT 'Indonesia',
			source TEXT NOT NULL DEFAULT 'BPS-API',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(indicator_id, year, period, region)
		);

		CREATE INDEX IF NOT EXISTS idx_bps_year ON bps_indicators(year);
		CREATE INDEX IF NOT EXISTS idx_bps_indicator ON bps_indicators(indicator_id);
		CREATE INDEX IF NOT EXISTS idx_bps_region ON bps_indicators(region);

		-- Bank Indonesia Rates
		CREATE TABLE IF NOT EXISTS bi_rates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			rate_type TEXT NOT NULL,
			value REAL NOT NULL,
			effective_date TEXT NOT NULL,
			year INTEGER NOT NULL,
			month INTEGER NOT NULL,
			source TEXT NOT NULL DEFAULT 'BI-API',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(rate_type, effective_date)
		);

		CREATE INDEX IF NOT EXISTS idx_bi_rate_type ON bi_rates(rate_type);
		CREATE INDEX IF NOT EXISTS idx_bi_date ON bi_rates(effective_date);
		CREATE INDEX IF NOT EXISTS idx_bi_year_month ON bi_rates(year, month);

		-- Clean up removed tables
		DROP TABLE IF EXISTS bmkg_weather;

		CREATE TABLE IF NOT EXISTS ckan_datasets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			dataset_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			description TEXT,
			organization TEXT,
			tags TEXT, -- JSON array
			license TEXT,
			url TEXT,
			source TEXT NOT NULL DEFAULT 'data.go.id',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_ckan_org ON ckan_datasets(organization);
		CREATE INDEX IF NOT EXISTS idx_ckan_updated ON ckan_datasets(updated_at);

		-- OJK Financial Entities
		CREATE TABLE IF NOT EXISTS ojk_entities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entity_name TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			license_number TEXT NOT NULL,
			status TEXT NOT NULL,
			address TEXT,
			source TEXT NOT NULL DEFAULT 'OJK',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(license_number)
		);

		CREATE INDEX IF NOT EXISTS idx_ojk_type ON ojk_entities(entity_type);
		CREATE INDEX IF NOT EXISTS idx_ojk_status ON ojk_entities(status);

		-- Commodity Prices (Emas, Beras, Minyak Goreng, dll)
		CREATE TABLE IF NOT EXISTS commodity_prices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			year INTEGER NOT NULL,
			month INTEGER NOT NULL,
			day INTEGER NOT NULL,
			commodity TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT '',
			price REAL NOT NULL,
			unit TEXT NOT NULL DEFAULT 'unit',
			region TEXT NOT NULL DEFAULT 'Indonesia',
			source TEXT NOT NULL DEFAULT 'manual',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(date, commodity, type, region)
		);

		CREATE INDEX IF NOT EXISTS idx_cp_date ON commodity_prices(date);
		CREATE INDEX IF NOT EXISTS idx_cp_commodity ON commodity_prices(commodity);
		CREATE INDEX IF NOT EXISTS idx_cp_year_month ON commodity_prices(year, month);

		CREATE TABLE IF NOT EXISTS scrape_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			status TEXT NOT NULL,
			records_count INTEGER DEFAULT 0,
			error_message TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`)
	return err
}

func (db *DB) Close() error {
	return db.conn.Close()
}

// --- Exchange Rate Operations ---

func (db *DB) UpsertExchangeRate(r models.ExchangeRate) error {
	_, err := db.conn.Exec(`
		INSERT INTO exchange_rates (date, rate, year, month)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET rate = excluded.rate
	`, r.Date, r.Rate, r.Year, r.Month)
	return err
}

func (db *DB) InsertExchangeRates(rates []models.ExchangeRate) (int, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO exchange_rates (date, rate, year, month)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET rate = excluded.rate
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, r := range rates {
		if _, err := stmt.Exec(r.Date, r.Rate, r.Year, r.Month); err != nil {
			continue
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (db *DB) GetExchangeRates(year, month int) ([]models.ExchangeRate, error) {
	query := `SELECT id, date, rate, year, month, created_at FROM exchange_rates WHERE 1=1`
	args := []interface{}{}

	if year > 0 {
		query += ` AND year = ?`
		args = append(args, year)
	}
	if month > 0 {
		query += ` AND month = ?`
		args = append(args, month)
	}
	query += ` ORDER BY date ASC`

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rates := make([]models.ExchangeRate, 0, 365)
	for rows.Next() {
		var r models.ExchangeRate
		var createdAt string
		if err := rows.Scan(&r.ID, &r.Date, &r.Rate, &r.Year, &r.Month, &createdAt); err != nil {
			continue
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rates = append(rates, r)
	}
	return rates, nil
}

func (db *DB) GetLatestExchangeRate() (*models.ExchangeRate, error) {
	var r models.ExchangeRate
	var createdAt string
	err := db.conn.QueryRow(`
		SELECT id, date, rate, year, month, created_at
		FROM exchange_rates ORDER BY date DESC LIMIT 1
	`).Scan(&r.ID, &r.Date, &r.Rate, &r.Year, &r.Month, &createdAt)
	if err != nil {
		return nil, err
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &r, nil
}

func (db *DB) GetExchangeRateYears() ([]int, error) {
	rows, err := db.conn.Query(`SELECT DISTINCT year FROM exchange_rates ORDER BY year ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	years := make([]int, 0, 10)
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			continue
		}
		years = append(years, y)
	}
	return years, nil
}

func (db *DB) CountExchangeRates() (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM exchange_rates`).Scan(&count)
	return count, err
}

// --- Fuel Price Operations ---

func (db *DB) UpsertFuelPrice(p models.FuelPrice) error {
	_, err := db.conn.Exec(`
		INSERT INTO fuel_prices (date, year, month, day, bbm_type, price, region, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date, bbm_type, region) DO UPDATE SET price = excluded.price, source = excluded.source
	`, p.Date, p.Year, p.Month, p.Day, p.BBMType, p.Price, p.Region, p.Source)
	return err
}

func (db *DB) InsertFuelPrices(prices []models.FuelPrice) (int, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO fuel_prices (date, year, month, day, bbm_type, price, region, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date, bbm_type, region) DO UPDATE SET price = excluded.price, source = excluded.source
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, p := range prices {
		if _, err := stmt.Exec(p.Date, p.Year, p.Month, p.Day, p.BBMType, p.Price, p.Region, p.Source); err != nil {
			continue
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (db *DB) GetFuelPrices(year, month int, bbmType, region string) ([]models.FuelPrice, error) {
	query := `SELECT id, date, year, month, day, bbm_type, price, region, source, created_at FROM fuel_prices WHERE 1=1`
	args := []interface{}{}

	if year > 0 {
		query += ` AND year = ?`
		args = append(args, year)
	}
	if month > 0 {
		query += ` AND month = ?`
		args = append(args, month)
	}
	if bbmType != "" {
		query += ` AND bbm_type = ?`
		args = append(args, bbmType)
	}
	if region != "" {
		query += ` AND region = ?`
		args = append(args, region)
	}
	query += ` ORDER BY date ASC, bbm_type ASC`

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make([]models.FuelPrice, 0, 100)
	for rows.Next() {
		var p models.FuelPrice
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Date, &p.Year, &p.Month, &p.Day, &p.BBMType, &p.Price, &p.Region, &p.Source, &createdAt); err != nil {
			continue
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		prices = append(prices, p)
	}
	return prices, nil
}

func (db *DB) GetLatestFuelPrices() ([]models.FuelPrice, error) {
	rows, err := db.conn.Query(`
		SELECT fp.id, fp.date, fp.year, fp.month, fp.day, fp.bbm_type, fp.price, fp.region, fp.source, fp.created_at
		FROM fuel_prices fp
		INNER JOIN (
			SELECT bbm_type, region, MAX(date) as max_date
			FROM fuel_prices
			GROUP BY bbm_type, region
		) latest ON fp.bbm_type = latest.bbm_type AND fp.region = latest.region AND fp.date = latest.max_date
		ORDER BY fp.bbm_type ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make([]models.FuelPrice, 0, 10)
	for rows.Next() {
		var p models.FuelPrice
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Date, &p.Year, &p.Month, &p.Day, &p.BBMType, &p.Price, &p.Region, &p.Source, &createdAt); err != nil {
			continue
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		prices = append(prices, p)
	}
	return prices, nil
}

func (db *DB) GetFuelPriceYears() ([]int, error) {
	rows, err := db.conn.Query(`SELECT DISTINCT year FROM fuel_prices ORDER BY year ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	years := make([]int, 0, 10)
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			continue
		}
		years = append(years, y)
	}
	return years, nil
}

func (db *DB) GetFuelTypes() ([]string, error) {
	rows, err := db.conn.Query(`SELECT DISTINCT bbm_type FROM fuel_prices ORDER BY bbm_type ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	types := make([]string, 0, 10)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			continue
		}
		types = append(types, t)
	}
	return types, nil
}

func (db *DB) GetFuelRegions() ([]string, error) {
	rows, err := db.conn.Query(`SELECT DISTINCT region FROM fuel_prices ORDER BY region ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	regions := make([]string, 0, 10)
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			continue
		}
		regions = append(regions, r)
	}
	return regions, nil
}

func (db *DB) CountFuelPrices() (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM fuel_prices`).Scan(&count)
	return count, err
}

// GetLatestExchangePeriod returns the most recent (year, month) present in exchange_rates.
// Used to fall back gracefully when a requested period has no data (e.g. the current
// month has not been scraped yet).
func (db *DB) GetLatestExchangePeriod() (year, month int, ok bool) {
	row := db.conn.QueryRow(`SELECT year, month FROM exchange_rates ORDER BY date DESC LIMIT 1`)
	var y, m int
	if err := row.Scan(&y, &m); err != nil {
		return 0, 0, false
	}
	return y, m, true
}

// GetLatestFuelPeriod returns the most recent (year, month) present in fuel_prices.
func (db *DB) GetLatestFuelPeriod() (year, month int, ok bool) {
	row := db.conn.QueryRow(`SELECT year, month FROM fuel_prices ORDER BY date DESC LIMIT 1`)
	var y, m int
	if err := row.Scan(&y, &m); err != nil {
		return 0, 0, false
	}
	return y, m, true
}

func (db *DB) GetFuelPriceHistory(bbmType, region string, limit int) ([]models.FuelPrice, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := db.conn.Query(`
		SELECT id, date, year, month, day, bbm_type, price, region, source, created_at
		FROM fuel_prices
		WHERE bbm_type = ? AND region = ?
		ORDER BY date DESC LIMIT ?
	`, bbmType, region, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make([]models.FuelPrice, 0, limit)
	for rows.Next() {
		var p models.FuelPrice
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Date, &p.Year, &p.Month, &p.Day, &p.BBMType, &p.Price, &p.Region, &p.Source, &createdAt); err != nil {
			continue
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		prices = append(prices, p)
	}
	return prices, nil
}

// --- Scrape Log ---

func (db *DB) LogScrape(source, status string, records int, errMsg string) error {
	_, err := db.conn.Exec(`
		INSERT INTO scrape_log (source, status, records_count, error_message)
		VALUES (?, ?, ?, ?)
	`, source, status, records, errMsg)
	return err
}

func (db *DB) GetRecentScrapeLogs(limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.conn.Query(`
		SELECT source, status, records_count, error_message, created_at
		FROM scrape_log ORDER BY id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]map[string]interface{}, 0, limit)
	for rows.Next() {
		var source, status, errMsg, createdAt string
		var records int
		if err := rows.Scan(&source, &status, &records, &errMsg, &createdAt); err != nil {
			continue
		}
		logs = append(logs, map[string]interface{}{
			"source":        source,
			"status":        status,
			"records_count": records,
			"error_message": errMsg,
			"created_at":    createdAt,
		})
	}
	return logs, nil
}

// --- Trend Helpers ---

func (db *DB) CalculateExchangeTrend(rates []models.ExchangeRate, year, month int) models.TrendIndicator {
	if len(rates) == 0 {
		return models.TrendIndicator{Direction: "stable"}
	}

	var currentVal, prevVal float64

	if year > 0 {
		if month > 0 {
			// Month filter active: show LATEST value in the month vs previous month's latest
			currentVal = rates[len(rates)-1].Rate // rates already sorted ASC by date

			prevYear, prevMonth := year, month-1
			if prevMonth == 0 {
				prevMonth = 12
				prevYear = year - 1
			}
			prevRates, _ := db.GetExchangeRates(prevYear, prevMonth)
			if len(prevRates) > 0 {
				prevVal = prevRates[len(prevRates)-1].Rate
			} else {
				prevVal = currentVal
			}
		} else {
			// Year filter only: avg for the year vs previous year avg
			var sum float64
			for _, r := range rates {
				sum += r.Rate
			}
			currentVal = sum / float64(len(rates))

			prevRates, _ := db.GetExchangeRates(year-1, 0)
			if len(prevRates) > 0 {
				var pSum float64
				for _, r := range prevRates {
					pSum += r.Rate
				}
				prevVal = pSum / float64(len(prevRates))
			} else {
				prevVal = currentVal
			}
		}
	} else {
		// No filter: show latest value vs second-to-last
		currentVal = rates[len(rates)-1].Rate
		if len(rates) >= 2 {
			prevVal = rates[len(rates)-2].Rate
		} else {
			prevVal = currentVal
		}
	}

	change := 0.0
	if prevVal > 0 {
		change = ((currentVal - prevVal) / prevVal) * 100
	}

	direction := "stable"
	if currentVal > prevVal {
		direction = "up"
	} else if currentVal < prevVal {
		direction = "down"
	}

	return models.TrendIndicator{
		CurrentValue:  currentVal,
		PreviousValue: prevVal,
		ChangePercent: round2(change),
		Direction:     direction,
	}
}

func (db *DB) CalculateFuelTrend(prices []models.FuelPrice, year, month int, bbmType, region string) models.TrendIndicator {
	if len(prices) == 0 {
		return models.TrendIndicator{Direction: "stable"}
	}

	calcBBMType := bbmType
	if calcBBMType == "" {
		calcBBMType = "Pertalite"
	}

	filtered := make([]models.FuelPrice, 0, len(prices))
	for _, p := range prices {
		if p.BBMType == calcBBMType {
			filtered = append(filtered, p)
		}
	}

	prices = filtered
	if len(prices) == 0 {
		return models.TrendIndicator{Direction: "stable"}
	}

	var currentVal, prevVal float64

	if year > 0 {
		if month > 0 {
			// Month filter: show LATEST price in the month vs previous month's latest
			currentVal = prices[len(prices)-1].Price

			prevYear, prevMonth := year, month-1
			if prevMonth == 0 {
				prevMonth = 12
				prevYear = year - 1
			}
			prevPrices, _ := db.GetFuelPrices(prevYear, prevMonth, calcBBMType, region)
			if len(prevPrices) > 0 {
				prevVal = prevPrices[len(prevPrices)-1].Price
			} else {
				prevVal = currentVal
			}
		} else {
			// Year filter only: avg for the year vs prev year avg
			var sum float64
			for _, p := range prices {
				sum += p.Price
			}
			currentVal = sum / float64(len(prices))

			prevPrices, _ := db.GetFuelPrices(year-1, 0, calcBBMType, region)
			if len(prevPrices) > 0 {
				var pSum float64
				for _, p := range prevPrices {
					pSum += p.Price
				}
				prevVal = pSum / float64(len(prevPrices))
			} else {
				prevVal = currentVal
			}
		}
	} else {
		// No filter: latest date's price vs second-to-last date
		latestDate := prices[len(prices)-1].Date
		var prevDate string
		for i := len(prices) - 1; i >= 0; i-- {
			if prices[i].Date != latestDate {
				prevDate = prices[i].Date
				break
			}
		}

		var latestSum, prevSum float64
		var latestCount, prevCount int
		for _, p := range prices {
			if p.Date == latestDate {
				latestSum += p.Price
				latestCount++
			} else if p.Date == prevDate {
				prevSum += p.Price
				prevCount++
			}
		}
		if latestCount > 0 {
			currentVal = latestSum / float64(latestCount)
		}
		if prevCount > 0 {
			prevVal = prevSum / float64(prevCount)
		} else {
			prevVal = currentVal
		}
	}

	change := 0.0
	if prevVal > 0 {
		change = ((currentVal - prevVal) / prevVal) * 100
	}

	direction := "stable"
	if currentVal > prevVal {
		direction = "up"
	} else if currentVal < prevVal {
		direction = "down"
	}

	return models.TrendIndicator{
		CurrentValue:  currentVal,
		PreviousValue: prevVal,
		ChangePercent: round2(change),
		Direction:     direction,
	}
}

func round2(v float64) float64 {
	if v >= 0 {
		return float64(int64(v*100+0.5)) / 100
	}
	return float64(int64(v*100-0.5)) / 100
}

func (db *DB) CalculateGoldTrend(prices []models.CommodityPrice, year, month int) models.TrendIndicator {
	if len(prices) == 0 {
		return models.TrendIndicator{Direction: "stable"}
	}

	goldPrices := make([]models.CommodityPrice, 0, len(prices))
	for _, p := range prices {
		if p.Commodity == "Emas" && (p.Type == "Antam 1g" || p.Type == "Spot XAU/IDR") {
			goldPrices = append(goldPrices, p)
		}
	}
	if len(goldPrices) == 0 {
		// Fallback to any Emas item
		for _, p := range prices {
			if p.Commodity == "Emas" {
				goldPrices = append(goldPrices, p)
			}
		}
	}

	if len(goldPrices) == 0 {
		return models.TrendIndicator{Direction: "stable"}
	}

	currentVal := goldPrices[len(goldPrices)-1].Price
	prevVal := currentVal

	if len(goldPrices) >= 2 {
		prevVal = goldPrices[len(goldPrices)-2].Price
	}

	change := 0.0
	if prevVal > 0 {
		change = ((currentVal - prevVal) / prevVal) * 100
	}

	direction := "stable"
	if currentVal > prevVal {
		direction = "up"
	} else if currentVal < prevVal {
		direction = "down"
	}

	return models.TrendIndicator{
		CurrentValue:  currentVal,
		PreviousValue: prevVal,
		ChangePercent: round2(change),
		Direction:     direction,
	}
}

