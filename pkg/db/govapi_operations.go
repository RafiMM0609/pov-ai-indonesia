package db

import (
	"encoding/json"
	"time"

	"github.com/anton/pov-ai-indonesia/pkg/models"
	_ "modernc.org/sqlite"
)

// --- BPS Economic Indicators Operations ---

func (db *DB) UpsertBPSIndicator(i models.BPSEconomicIndicator) error {
	_, err := db.conn.Exec(`
		INSERT INTO bps_indicators (indicator_id, indicator, value, unit, year, period, region, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(indicator_id, year, period, region) DO UPDATE SET value = excluded.value, source = excluded.source
	`, i.IndicatorID, i.Indicator, i.Value, i.Unit, i.Year, i.Period, i.Region, i.Source)
	return err
}

func (db *DB) InsertBPSIndicators(indicators []models.BPSEconomicIndicator) (int, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO bps_indicators (indicator_id, indicator, value, unit, year, period, region, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(indicator_id, year, period, region) DO UPDATE SET value = excluded.value, source = excluded.source
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, i := range indicators {
		if _, err := stmt.Exec(i.IndicatorID, i.Indicator, i.Value, i.Unit, i.Year, i.Period, i.Region, i.Source); err != nil {
			continue
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (db *DB) GetBPSIndicators(year int, indicatorID, region string) ([]models.BPSEconomicIndicator, error) {
	query := `SELECT id, indicator_id, indicator, value, unit, year, period, region, source, created_at FROM bps_indicators WHERE 1=1`
	args := []interface{}{}

	if year > 0 {
		query += ` AND year = ?`
		args = append(args, year)
	}
	if indicatorID != "" {
		query += ` AND indicator_id = ?`
		args = append(args, indicatorID)
	}
	if region != "" {
		query += ` AND region = ?`
		args = append(args, region)
	}
	query += ` ORDER BY year DESC, period ASC`

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indicators := make([]models.BPSEconomicIndicator, 0, 50)
	for rows.Next() {
		var i models.BPSEconomicIndicator
		var createdAt string
		if err := rows.Scan(&i.ID, &i.IndicatorID, &i.Indicator, &i.Value, &i.Unit, &i.Year, &i.Period, &i.Region, &i.Source, &createdAt); err != nil {
			continue
		}
		i.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		indicators = append(indicators, i)
	}
	return indicators, nil
}

func (db *DB) GetLatestBPSIndicators(indicatorID, region string) ([]models.BPSEconomicIndicator, error) {
	rows, err := db.conn.Query(`
		SELECT bi.id, bi.indicator_id, bi.indicator, bi.value, bi.unit, bi.year, bi.period, bi.region, bi.source, bi.created_at
		FROM bps_indicators bi
		INNER JOIN (
			SELECT indicator_id, region, MAX(year) as max_year
			FROM bps_indicators
			GROUP BY indicator_id, region
		) latest ON bi.indicator_id = latest.indicator_id AND bi.region = latest.region AND bi.year = latest.max_year
		WHERE bi.indicator_id = ? AND bi.region = ?
		ORDER BY bi.period ASC
	`, indicatorID, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indicators := make([]models.BPSEconomicIndicator, 0, 10)
	for rows.Next() {
		var i models.BPSEconomicIndicator
		var createdAt string
		if err := rows.Scan(&i.ID, &i.IndicatorID, &i.Indicator, &i.Value, &i.Unit, &i.Year, &i.Period, &i.Region, &i.Source, &createdAt); err != nil {
			continue
		}
		i.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		indicators = append(indicators, i)
	}
	return indicators, nil
}

func (db *DB) GetBPSIndicatorYears() ([]int, error) {
	rows, err := db.conn.Query(`SELECT DISTINCT year FROM bps_indicators ORDER BY year ASC`)
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

// --- Bank Indonesia Rates Operations ---

func (db *DB) UpsertBIRate(r models.BIRate) error {
	_, err := db.conn.Exec(`
		INSERT INTO bi_rates (rate_type, value, effective_date, year, month, source)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(rate_type, effective_date) DO UPDATE SET value = excluded.value, source = excluded.source
	`, r.RateType, r.Value, r.EffectiveDate, r.Year, r.Month, r.Source)
	return err
}

func (db *DB) InsertBIRates(rates []models.BIRate) (int, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO bi_rates (rate_type, value, effective_date, year, month, source)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(rate_type, effective_date) DO UPDATE SET value = excluded.value, source = excluded.source
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, r := range rates {
		if _, err := stmt.Exec(r.RateType, r.Value, r.EffectiveDate, r.Year, r.Month, r.Source); err != nil {
			continue
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (db *DB) GetBIRates(year int, rateType string) ([]models.BIRate, error) {
	query := `SELECT id, rate_type, value, effective_date, year, month, source, created_at FROM bi_rates WHERE 1=1`
	args := []interface{}{}

	if year > 0 {
		query += ` AND year = ?`
		args = append(args, year)
	}
	if rateType != "" {
		query += ` AND rate_type = ?`
		args = append(args, rateType)
	}
	query += ` ORDER BY effective_date ASC`

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rates := make([]models.BIRate, 0, 20)
	for rows.Next() {
		var r models.BIRate
		var createdAt string
		if err := rows.Scan(&r.ID, &r.RateType, &r.Value, &r.EffectiveDate, &r.Year, &r.Month, &r.Source, &createdAt); err != nil {
			continue
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rates = append(rates, r)
	}
	return rates, nil
}

func (db *DB) GetLatestBIRates() ([]models.BIRate, error) {
	rows, err := db.conn.Query(`
		SELECT br.id, br.rate_type, br.value, br.effective_date, br.year, br.month, br.source, br.created_at
		FROM bi_rates br
		INNER JOIN (
			SELECT rate_type, MAX(effective_date) as max_date
			FROM bi_rates
			GROUP BY rate_type
		) latest ON br.rate_type = latest.rate_type AND br.effective_date = latest.max_date
		ORDER BY br.rate_type ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rates := make([]models.BIRate, 0, 10)
	for rows.Next() {
		var r models.BIRate
		var createdAt string
		if err := rows.Scan(&r.ID, &r.RateType, &r.Value, &r.EffectiveDate, &r.Year, &r.Month, &r.Source, &createdAt); err != nil {
			continue
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rates = append(rates, r)
	}
	return rates, nil
}


// --- CKAN Datasets Operations ---

func (db *DB) UpsertCKANDataset(d models.CKANDataset) error {
	tagsJSON, _ := json.Marshal(d.Tags)
	_, err := db.conn.Exec(`
		INSERT INTO ckan_datasets (dataset_id, title, description, organization, tags, license, url, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(dataset_id) DO UPDATE SET 
			title = excluded.title,
			description = excluded.description,
			organization = excluded.organization,
			tags = excluded.tags,
			license = excluded.license,
			url = excluded.url,
			source = excluded.source,
			updated_at = excluded.updated_at
	`, d.DatasetID, d.Title, d.Description, d.Organization, string(tagsJSON), d.License, d.URL, d.Source, d.CreatedAt.Format(time.RFC3339), d.UpdatedAt.Format(time.RFC3339))
	return err
}

func (db *DB) InsertCKANDatasets(datasets []models.CKANDataset) (int, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO ckan_datasets (dataset_id, title, description, organization, tags, license, url, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(dataset_id) DO UPDATE SET 
			title = excluded.title,
			description = excluded.description,
			organization = excluded.organization,
			tags = excluded.tags,
			license = excluded.license,
			url = excluded.url,
			source = excluded.source,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, d := range datasets {
		tagsJSON, _ := json.Marshal(d.Tags)
		if _, err := stmt.Exec(d.DatasetID, d.Title, d.Description, d.Organization, string(tagsJSON), d.License, d.URL, d.Source, d.CreatedAt.Format(time.RFC3339), d.UpdatedAt.Format(time.RFC3339)); err != nil {
			continue
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (db *DB) GetCKANDatasets(organization string, limit int) ([]models.CKANDataset, error) {
	query := `SELECT id, dataset_id, title, description, organization, tags, license, url, source, created_at, updated_at FROM ckan_datasets WHERE 1=1`
	args := []interface{}{}

	if organization != "" {
		query += ` AND organization = ?`
		args = append(args, organization)
	}
	query += ` ORDER BY updated_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	datasets := make([]models.CKANDataset, 0, limit)
	for rows.Next() {
		var d models.CKANDataset
		var tagsJSON, createdAt, updatedAt string
		if err := rows.Scan(&d.ID, &d.DatasetID, &d.Title, &d.Description, &d.Organization, &tagsJSON, &d.License, &d.URL, &d.Source, &createdAt, &updatedAt); err != nil {
			continue
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		d.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		json.Unmarshal([]byte(tagsJSON), &d.Tags)
		datasets = append(datasets, d)
	}
	return datasets, nil
}

// --- OJK Entities Operations ---

func (db *DB) UpsertOJKEntity(e models.OJKEntity) error {
	_, err := db.conn.Exec(`
		INSERT INTO ojk_entities (entity_name, entity_type, license_number, status, address, source)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(license_number) DO UPDATE SET 
			entity_name = excluded.entity_name,
			entity_type = excluded.entity_type,
			status = excluded.status,
			address = excluded.address,
			source = excluded.source
	`, e.EntityName, e.EntityType, e.LicenseNumber, e.Status, e.Address, e.Source)
	return err
}

func (db *DB) InsertOJKEntities(entities []models.OJKEntity) (int, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO ojk_entities (entity_name, entity_type, license_number, status, address, source)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(license_number) DO UPDATE SET 
			entity_name = excluded.entity_name,
			entity_type = excluded.entity_type,
			status = excluded.status,
			address = excluded.address,
			source = excluded.source
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, e := range entities {
		if _, err := stmt.Exec(e.EntityName, e.EntityType, e.LicenseNumber, e.Status, e.Address, e.Source); err != nil {
			continue
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (db *DB) GetOJKEntities(entityType, status string, limit int) ([]models.OJKEntity, error) {
	query := `SELECT id, entity_name, entity_type, license_number, status, address, source, created_at FROM ojk_entities WHERE 1=1`
	args := []interface{}{}

	if entityType != "" {
		query += ` AND entity_type = ?`
		args = append(args, entityType)
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY entity_name ASC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entities := make([]models.OJKEntity, 0, limit)
	for rows.Next() {
		var e models.OJKEntity
		var createdAt string
		if err := rows.Scan(&e.ID, &e.EntityName, &e.EntityType, &e.LicenseNumber, &e.Status, &e.Address, &e.Source, &createdAt); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		entities = append(entities, e)
	}
	return entities, nil
}

// --- Commodity Price Operations ---

func (db *DB) UpsertCommodityPrice(c models.CommodityPrice) error {
	_, err := db.conn.Exec(`
		INSERT INTO commodity_prices (date, year, month, day, commodity, type, price, unit, region, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date, commodity, type, region) DO UPDATE SET price = excluded.price, source = excluded.source
	`, c.Date, c.Year, c.Month, c.Day, c.Commodity, c.Type, c.Price, c.Unit, c.Region, c.Source)
	return err
}

func (db *DB) InsertCommodityPrices(prices []models.CommodityPrice) (int, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO commodity_prices (date, year, month, day, commodity, type, price, unit, region, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date, commodity, type, region) DO UPDATE SET price = excluded.price, source = excluded.source
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, c := range prices {
		if _, err := stmt.Exec(c.Date, c.Year, c.Month, c.Day, c.Commodity, c.Type, c.Price, c.Unit, c.Region, c.Source); err != nil {
			continue
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (db *DB) GetCommodityPrices(year, month int, commodity, region string) ([]models.CommodityPrice, error) {
	query := `SELECT id, date, year, month, day, commodity, type, price, unit, region, source, created_at FROM commodity_prices WHERE 1=1`
	args := []interface{}{}

	if year > 0 {
		query += ` AND year = ?`
		args = append(args, year)
	}
	if month > 0 {
		query += ` AND month = ?`
		args = append(args, month)
	}
	if commodity != "" {
		query += ` AND commodity = ?`
		args = append(args, commodity)
	}
	if region != "" {
		query += ` AND region = ?`
		args = append(args, region)
	}
	query += ` ORDER BY date ASC, commodity ASC`

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make([]models.CommodityPrice, 0, 50)
	for rows.Next() {
		var c models.CommodityPrice
		var createdAt string
		if err := rows.Scan(&c.ID, &c.Date, &c.Year, &c.Month, &c.Day, &c.Commodity, &c.Type, &c.Price, &c.Unit, &c.Region, &c.Source, &createdAt); err != nil {
			continue
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		prices = append(prices, c)
	}
	return prices, nil
}

func (db *DB) GetLatestCommodityPrices() ([]models.CommodityPrice, error) {
	rows, err := db.conn.Query(`
		SELECT cp.id, cp.date, cp.year, cp.month, cp.day, cp.commodity, cp.type, cp.price, cp.unit, cp.region, cp.source, cp.created_at
		FROM commodity_prices cp
		INNER JOIN (
			SELECT commodity, type, region, MAX(date) as max_date
			FROM commodity_prices
			GROUP BY commodity, type, region
		) latest ON cp.commodity = latest.commodity AND cp.type = latest.type AND cp.region = latest.region AND cp.date = latest.max_date
		ORDER BY cp.commodity ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make([]models.CommodityPrice, 0, 20)
	for rows.Next() {
		var c models.CommodityPrice
		var createdAt string
		if err := rows.Scan(&c.ID, &c.Date, &c.Year, &c.Month, &c.Day, &c.Commodity, &c.Type, &c.Price, &c.Unit, &c.Region, &c.Source, &createdAt); err != nil {
			continue
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		prices = append(prices, c)
	}
	return prices, nil
}

func (db *DB) GetCommodityTypes() ([]string, error) {
	rows, err := db.conn.Query(`SELECT DISTINCT commodity FROM commodity_prices ORDER BY commodity ASC`)
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