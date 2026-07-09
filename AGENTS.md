# POV AI Indonesia — Knowledge Graph & Project Map

> Generated: 2026-06-18
> Branch: (no git — direktori lokal)
> Tech: Go 1.23 + Gin + modernc.org/sqlite (CGO-free) + OpenRouter AI
> Runtime: Go 1.23+, port 8080 (default), SQLite single-file DB
> Build: `go build -o pov-ai ./cmd/ && PORT=8080 ./pov-ai`

---

## 1. ARSITEKTUR APLIKASI

### Service Overview — Monolith SPA

| Komponen | Deskripsi | Lokasi |
|----------|-----------|--------|
| **Entrypoint** | `cmd/main.go` | Init DB, scraper, AI client, cron, server |
| **Config** | `internal/config/config.go` | Load `.env`, env vars, seed prices |
| **API Server** | `pkg/api/server.go` | Gin HTTP router + handlers |
| **Database** | `pkg/db/db.go` + `pkg/db/govapi_operations.go` | SQLite (modernc.org, CGO-free) |
| **Scraper** | `pkg/scraper/scraper.go` + `pkg/scraper/enhanced.go` | Frankfurter (ECB), Pertamina, web scrape |
| **AI Client** | `pkg/ai/client.go` | OpenRouter API (chat completion) |
| **Knowledge** | `pkg/knowledge/knowledge.go` | Generate insights → markdown files on disk |
| **Models** | `pkg/models/models.go` | ExchangeRate, FuelPrice, etc. |
| **Gov API** | `internal/govapi/client.go` | BPS, BI, BMKG, data.go.id, OJK |
| **Frontend** | `web/templates/*.html` + `web/static/js/app.js` + `web/static/css/style.css` | SPA JavaScript (no framework) |
| **Static Assets** | `web/static/css/style.css` | 526 lines — dark theme, JetBrains Mono |

### Database Strategy
```
SQLite (modernc.org/sqlite — pure Go, no CGO)
  → conn.SetMaxOpenConns(1) — single connection
  → conn.SetMaxIdleConns(1)
  → conn.SetConnMaxLifetime(1h)
```
Tidak ada connection pool — SQLite single-writer. Semua query blocking (sync).

### Directory Structure
```
pov-ai-indonesia/
├── cmd/
│   └── main.go                 # Entrypoint — init, seed, cron, server
├── internal/
│   ├── config/config.go         # .env loader, env vars
│   └── govapi/client.go         # Government API clients (seed fallback)
├── pkg/
│   ├── ai/client.go             # OpenRouter chat completion
│   ├── api/server.go            # Gin HTTP handlers (527 lines)
│   ├── db/db.go                 # SQLite schema, CRUD (744 lines)
│   ├── db/govapi_operations.go  # Gov data DB operations
│   ├── knowledge/knowledge.go   # Insight generation (648 lines)
│   ├── models/models.go         # Data structures
│   └── scraper/
│       ├── scraper.go           # Frankfurter (ECB) + BBM scrapers (381 lines)
│       ├── enhanced.go          # Gov data integration wrapper
│       └── scraper_test.go      # Price parser unit tests
├── web/
│   ├── static/css/style.css     # Dark theme stylesheet
│   ├── static/js/app.js         # SPA JavaScript (938 lines)
│   └── templates/
│       ├── index.html           # Shell template (SPA)
│       ├── exchange_rate.html   # Exchange rate page shell
│       └── fuel_price.html      # Fuel price page shell
├── data/
│   ├── povai.db                 # SQLite database
│   └── knowledge/               # Generated insight markdown files
│       ├── exchange_rate_202*.md
│       └── fuel_price_202*.md
├── scratch/db_check.go          # Standalone DB diagnostic tool
├── go.mod
├── .env                         # API keys (PORT, OPENROUTER_API_KEY, etc.)
├── setup-env.sh                 # Environment setup script
└── restart.sh                   # Kill + rebuild + run
```

---

## 2. DOMAIN ENTITY MAP (Knowledge Graph)

### Central Entity: ExchangeRate
```
ExchangeRate ─┬── date (UNIQUE, TEXT)        — Primary dimension
               ├── rate (REAL)                 — USD/IDR value from ECB
               ├── year (INT)                  — Partition key
               └── month (INT)                 — Partition key
```

### Central Entity: FuelPrice
```
FuelPrice ─┬── date (TEXT)
             ├── bbm_type (TEXT)               — Pertalite, Solar, Pertamax, etc.
             ├── price (REAL)
             ├── year, month, day (INT)
             ├── region (TEXT)                  — Indonesia (default)
             └── source (TEXT)                  — pertamina-official, web-scrape, historical
    UNIQUE(date, bbm_type, region)
```

### Government Data Entities (all seed/static)
```
BPSEconomicIndicator ─── UNIQUE(indicator_id, year, period, region)
BIRate ───────────────── UNIQUE(rate_type, effective_date)
BMKGWeather ──────────── UNIQUE(station_id, date)
CKANDataset ──────────── UNIQUE(dataset_id)
OJKEntity ────────────── UNIQUE(license_number)
CommodityPrice ───────── UNIQUE(date, commodity, type, region)
```

### DashboardResponse (API response composite)
```
DashboardResponse ┬── ExchangeRates []ExchangeRate
                   ├── ExchangeTrend TrendIndicator
                   ├── FuelPrices []FuelPrice
                   ├── FuelTrend TrendIndicator
                   ├── Insights []KnowledgeEntry
                   ├── Filter FilterParams
                   └── DataSources []DataSourceInfo
```

### KnowledgeEntry (generated insight files on disk)
```
KnowledgeEntry ─┬── Category: "exchange_rate" | "fuel_price"
                 ├── Period: "2026" | "2026-06"
                 ├── Title, Summary, Factors, Analysis
                 └── GeneratedAt
    Stored as markdown files: data/knowledge/{category}_{period}.md
```

---

## 3. API ROUTE MAP

| Prefix | Handler Function | Deskripsi |
|--------|-----------------|-----------|
| `GET /api/v1/dashboard` | `handleDashboard` | Ringkasan: rates, fuel, trends, insights, data sources |
| `GET /api/v1/exchange-rates` | `handleExchangeRates` | USD/IDR rates by year/month + trend |
| `GET /api/v1/fuel-prices` | `handleFuelPrices` | BBM prices by year/month/type/region + trend |
| `GET /api/v1/fuel-prices/latest` | `handleLatestFuelPrices` | Latest price for each BBM type |
| `GET /api/v1/fuel-prices/history` | `handleFuelPriceHistory` | Historical prices by type and region |
| `GET /api/v1/fuel-types` | `handleFuelTypes` | Available BBM types |
| `GET /api/v1/insights/:category/:period` | `handleInsight` | Single insight by category+period |
| `GET /api/v1/insights` | `handleListInsights` | List all available insights |
| `GET /api/v1/admin/scrape-logs` | `handleScrapeLogs` | Scrape audit log |
| `POST /api/v1/admin/regenerate` | `handleRegenerate` | Regenerate all insights |
| `GET /api/v1/bps-indicators` | `handleBPSIndicators` | BPS economic indicators |
| `GET /api/v1/bi-rates` | `handleBIRates` | Bank Indonesia rates |
| `GET /api/v1/bmkg-weather` | `handleBMKGWeather` | BMKG weather data |
| `GET /api/v1/ckan-datasets` | `handleCKANDatasets` | data.go.id CKAN datasets |
| `GET /api/v1/ojk-entities` | `handleOJKEntities` | OJK financial entities |
| `GET /api/v1/commodity-prices` | `handleCommodityPrices` | Commodity prices |
| `GET /api/v1/commodity-latest` | `handleLatestCommodityPrices` | Latest commodity prices |
| `GET /api/v1/data-sources` | `handleDataSources` | Data source metadata |
| `GET /api/v1/health` | `handleHealth` | Health check |

### Static Routes
| Route | File |
|-------|------|
| `/` | `index.html` (SPA shell) |
| `/exchange-rate` | `exchange_rate.html` |
| `/fuel-price` | `fuel_price.html` |
| `/static/*` | `web/static/` |
| `/sources` | Same SPA shell — JS routing |

### Query Parameters Convention
- `year` (int, optional) — filter by year, 0 = all
- `month` (int, optional) — filter by month, 0 = all
- `bbm_type` (string) — fuel type filter
- `region` (string, default "Indonesia")
- `limit` (int, default 50)
- `indicator_id`, `rate_type`, `province`, `organization`, `commodity`, `entity_type`, `status`

---

## 4. DATABASE SCHEMA

### Core Tables

**exchange_rates** — 1655 records
| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PK | Auto-increment |
| date | TEXT UNIQUE | "2026-06-17" |
| rate | REAL | USD/IDR — ECB reference rate |
| year | INT | Partition |
| month | INT | 1-12 |
| created_at | TEXT | datetime('now') |

**fuel_prices** — 152 records
| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PK | |
| date | TEXT | |
| year, month, day | INT | |
| bbm_type | TEXT | "Pertalite", "Pertamax", dll |
| price | REAL | IDR per liter |
| region | TEXT | Default "Indonesia" |
| source | TEXT | "pertamina-official", "web-scrape", "historical" |
| UNIQUE | | (date, bbm_type, region) |

### Government Tables

**bps_indicators** — UNIQUE(indicator_id, year, period, region)
**bi_rates** — UNIQUE(rate_type, effective_date)
**bmkg_weather** — UNIQUE(station_id, date)
**ckan_datasets** — UNIQUE(dataset_id)
**ojk_entities** — UNIQUE(license_number)
**commodity_prices** — UNIQUE(date, commodity, type, region)
**scrape_log** — Audit trail for data collection

### Common Column Pattern
- `id` (INTEGER PK AUTOINCREMENT)
- `created_at` (TEXT, datetime('now'))
- All tables use UNIQUE constraints for upsert (ON CONFLICT DO UPDATE)
- No soft-delete (data is additive via upsert)

---

## 5. CORE UTILITIES

| File | Purpose |
|------|---------|
| `internal/config/config.go` | Load `.env`, env vars, seed prices map |
| `internal/govapi/client.go` | Government API clients (BPS, BI, BMKG, data.go.id, OJK) — mostly seed fallback |
| `pkg/ai/client.go` | OpenRouter chat completion, `AnalyzeExchangeRate`, `AnalyzeFuelPrice`, JSON response extraction |
| `pkg/db/db.go` | SQLite schema migration, CRUD for all entities, trend calculation |
| `pkg/db/govapi_operations.go` | Gov-specific DB operations (BPS, BI, BMKG, CKAN, OJK) |
| `pkg/knowledge/knowledge.go` | Insight generation: template-based + LLM enrichment, file I/O |
| `pkg/scraper/scraper.go` | Frankfurter (ECB) scraping, BBM web scraping from Bisnis.com & CNBC |
| `pkg/scraper/enhanced.go` | Government data collection orchestration |

### AI Client (OpenRouter)
- Base URL: `https://openrouter.ai/api/v1`
- Model: configurable via `OPENROUTER_API_KEY` + `POV_AI_MODEL` env vars (default: `openrouter/owl-alpha`)
- Response format: structured JSON (`{title, summary, factors, analysis}`)
- Fallback: template-based insights when AI disabled or fails
- Timeout: 120s

### Data Sources — KEY NOTE
| Source | Type | Update Cycle | Akurasi |
|--------|------|-------------|---------|
| Frankfurter.app (ECB) | REST API publik | Harian (~16:00 CET) | Reference rate — ±0.2-1% dari market spot |
| Pertamina Official | Static seed + web scrape | Manual / as announced | Harga resmi |
| BBM Web Scrape | HTML scraping (Bisnis/CNBC) | Per cron (6 jam) | Perlu regex update jika DOM berubah |
| BPS, BI, BMKG | Seed data (static) | Manual | ⚠️ Bisa outdated |
| Commodity Prices | Curated manual | Manual | ⚠️ Survei pasar |

---

## 6. BUSINESS RULES & FLOWS

### Flow 1: Startup & Seed Data
```
main() ─┬── config.Load()                     — Load .env
         ├── db.New(dbPath)                    — Open/migrate SQLite
         ├── ai.NewClient()                     — OpenRouter client
         ├── scraper.New() + EnhancedScraper    — Scraper instances
         ├── seedData()                         — If DB empty:
         │   ├── Frankfurter API (2020-today)   → exchange_rates
         │   ├── Historical BBM seed            → fuel_prices
         │   ├── Live BBM scrape                → fuel_prices
         │   └── Government seed data           → bps, bi, bmkg, etc.
         ├── km.RegenerateAllTemplates()        — Fast template insights
         ├── server.Run(port)                   — Gin HTTP server
         └── km.RegenerateAll() (background)    — LLM-enriched insights
```

### Flow 2: Daily Scrape (Cron)
```
cron: every N hours (default 6) ─┬── sc.ScrapeExchangeRates()
                                   │       → Frankfurter API → INSERT/UPDATE exchange_rates
                                   ├── sc.ScrapeFuelPrices()
                                   │       → Official prices → web scrape fallback → UPSERT fuel_prices
                                   └── km.RegenerateAll() (if AI enabled)
                                           → Regenerate insight markdown files
                                           
cron: 03:00 daily (AI only) ─────── km.RegenerateAll() — Full LLM refresh
cron: Sunday 02:00 ──────────────── es.CollectGovernmentData() — Government data refresh
```

### Flow 3: Dashboard Request (HTTP)
```
GET /api/v1/dashboard?year=2026&month=6
  └── handleDashboard()
      ├── db.GetExchangeRates(year, month)     → rates[]
      ├── db.GetFuelPrices(year, month)        → fuel[]
      ├── db.CalculateExchangeTrend()          → TrendIndicator
      ├── db.CalculateFuelTrend()              → TrendIndicator
      ├── km.LoadInsight("exchange_rate",…)    → KnowledgeEntry (from disk)
      ├── km.LoadInsight("fuel_price",…)       → KnowledgeEntry (from disk)
      └── JSON response (NO LLM calls during HTTP)
```

### Flow 4: Insight Generation (2-phase)
```
Phase 1 (startup, fast):
  RegenerateAllTemplates() ─── genExchangeRate(…, llm=false)
                                └── exchangeRateFallback() — rule-based template
                                genFuelPrice(…, llm=false)
                                └── fuelPriceFallback() — rule-based template
                              
Phase 2 (background, can be slow):
  RegenerateAll() ───────────── genExchangeRate(…, llm=true)
                                └── OpenRouter API → structured JSON
                                genFuelPrice(…, llm=true)
                                └── OpenRouter API → structured JSON
```

### Flow 5: BBM Price Resolution
```
ScrapeFuelPrices()
  ├── pertaminaOfficialPrices()   — Always primary (seed prices from .env/config)
  ├── scrapeFromWeb()             — Fallback: Bisnis.com → CNBC Indonesia
  │     └── extractLatestPrices()    — Regex patterns for each BBM type
  └── Jika keduanya gagal → return error (cron logged as "failed")
```

---

## 7. FRONTEND ARCHITECTURE

### SPA Routing (no framework)
- `app.js` uses `history.pushState()` + `popstate` event
- Path-based routing: `/`, `/exchange-rate`, `/fuel-price`, `/sources`, `/commodities`
- All templates (`index.html`, `exchange_rate.html`, `fuel_price.html`) share the same layout shell
- JS dynamically fetches data from API and renders HTML into `#main-content`

### Key Frontend Functions
| Function | Page | Description |
|----------|------|-------------|
| `renderDashboard()` | `/` | Summary cards, charts, insights |
| `renderExchangeRatePage()` | `/exchange-rate` | Rate chart, table, insight |
| `renderFuelPricePage()` | `/fuel-price` | BBM cards, chart, insight |
| `renderSourcesPage()` | `/sources` | Data source transparency |
| `renderCommoditiesPage()` | `/commodities` | Commodity prices |
| `buildDisclaimerHTML()` | All | Data transparency banner |
| `drawLineChart(canvasId, data, color)` | Dashboard/Detail | Canvas-based line chart |
| `buildInsightHTML(insight)` | Dashboard/Detail | Insight card renderer |

### CSS Architecture
- CSS custom properties (variables) for theming
- Dark theme: `--bg-primary: #0a0a0f`
- Accent colors: blue (#exchange), green (#fuel), red (#down/weak), yellow (#up/strong)
- No CSS framework — 100% custom styles (526 lines)

---

## 8. DATA SOURCE TRANSPARENCY

### USD/IDR Rate — Penting!
- **Sumber: European Central Bank (ECB)** via Frankfurter.app — reference rate harian
- BUKAN market spot rate (TradingView, Google Finance)
- Market spot rate bisa berbeda **±0.2%–1%** dari ECB (~50-150 pips)
- Contoh 18 Jun 2026: ECB 17.803 vs Google Finance 17.850 vs TradingView 17.748
- Untuk referensi eksak: cek kurs tengah BI (JISDOR) atau TradingView
- Alasan tetap pakai ECB: satu-satunya API gratis yang menyediakan data historis 2020-sekarang dalam bulk

### Access Types
| Label | Arti | Contoh |
|-------|------|--------|
| **Publik (Live)** | REST API publik, auto-update | Frankfurter (ECB) |
| **Resmi (Dikurasi)** | Data resmi, diupdate manual | Pertamina Official |
| **Kurasi Manual (Statis)** | Data statis, perlu verifikasi | BPS, BI, BMKG, Commodity |

---

## 9. CRITICAL PITFALLS & PATTERNS

### Known Issues
1. **ECB vs Market Spot** — Displayed rate (ECB) will always differ slightly from rates in news headlines (Kontan, CNBC). Already disclaimed in UI.
2. **BBM Web Scrape Fragility** — `extractLatestPrices()` uses regex patterns on Bisnis.com/CNBC HTML. If these news sites change their HTML structure, scraping breaks silently (logged as failed).
3. **SQLite Single Connection** — `SetMaxOpenConns(1)`. Background cron/insight generation blocks during HTTP requests and vice versa. OK for low traffic but will queue under load.
4. **BMKG API Unstable** — Public BMKG API frequently returns 404. Falls back to seed data silently.
5. **Government Data All Seed** — BPS, BI, OJK APIs require corporate registration. Current seed data may be outdated.
6. **No Git** — Project has no `.git` directory. No version history or branching.
7. **Knowledge Files Out of Sync** — Insight `.md` files on disk may not reflect latest DB state if cron hasn't run.

### Common Patterns
- **UPSERT everywhere** — All INSERT operations use `ON CONFLICT DO UPDATE`
- **Two-phase insight gen** — Templates first (fast, no LLM), then LLM enrichment (slow, background)
- **Seed data for restricted APIs** — When gov API unavailable, fallback to pre-compiled seed data
- **Disclaimer on every page** — `buildDisclaimerHTML()` prepended to all page renders
- **SPA with pushState** — No page reloads; JS handles all routing
- **Rate from ECB via EUR cross** — Frankfurter provides USD→IDR via USD→EUR→IDR cross rate

---

## 10. FILE REFERENCE INDEX

### Entrypoint & Config
| File | Purpose | Key Functions |
|------|---------|--------------|
| `cmd/main.go` | App entrypoint | `main()`, `seedData()`, `scrapeData()`, `collectGovernmentData()` |
| `internal/config/config.go` | Config loader | `Load()`, `getEnv()` |

### API Layer
| File | Purpose | Key Handlers |
|------|---------|-------------|
| `pkg/api/server.go` | Gin server + routes | `Dashboard`, `ExchangeRates`, `FuelPrices`, `DataSources`, + gov endpoints |

### Database
| File | Purpose | Key Functions |
|------|---------|--------------|
| `pkg/db/db.go` | SQLite operations | `New()`, `migrate()`, `Upsert/Insert/Get/Count` for all entities, trend calc |
| `pkg/db/govapi_operations.go` | Gov DB operations | BPS/BI/BMKG/CKAN/OJK CRUD |

### Business Logic
| File | Purpose | Key Functions |
|------|---------|--------------|
| `pkg/knowledge/knowledge.go` | Insight engine | `RegenerateAll()`, `genExchangeRate()`, `genFuelPrice()`, `exchangeRateFallback()`, `fuelPriceFallback()` |
| `pkg/scraper/scraper.go` | Data collection | `ScrapeExchangeRates()`, `ScrapeFuelPrices()`, `extractLatestPrices()` |
| `pkg/scraper/enhanced.go` | Gov data collector | `CollectGovernmentData()` |
| `pkg/ai/client.go` | OpenRouter client | `ChatCompletion()`, `AnalyzeExchangeRate()`, `extractJSON()` |

### Data Models
| File | Purpose |
|------|---------|
| `pkg/models/models.go` | All structs: ExchangeRate, FuelPrice, CommodityPrice, TrendIndicator, KnowledgeEntry, DashboardResponse, DataSourceInfo |

### Government API
| File | Purpose |
|------|---------|
| `internal/govapi/client.go` | Gov API clients + seed data functions (SeedBPSEconomicData, SeedBIRates, SeedBMKGWeather, dll) |

### Frontend
| File | Size | Role |
|------|------|------|
| `web/static/js/app.js` | 938 lines | SPA logic, API fetches, rendering, charts |
| `web/static/css/style.css` | 526 lines | Dark theme, custom components |
| `web/templates/index.html` | 35 lines | SPA shell (header, nav, footer, #main-content) |
| `web/templates/exchange_rate.html` | 36 lines | Shell for exchange rate page |
| `web/templates/fuel_price.html` | 31 lines | Shell for fuel price page |

### Tools
| File | Purpose |
|------|---------|
| `scratch/db_check.go` | Standalone DB diagnostic (build: `go run scratch/db_check.go`) |

---

## 11. TROUBLESHOOTING

### How to add a new data source
1. Add model struct in `pkg/models/models.go`
2. Add table in `pkg/db/db.go` `migrate()` — include indices + UNIQUE for upsert
3. Add DB CRUD functions in `pkg/db/db.go` (Upsert, Insert, Get, Count)
4. Add scraper in `pkg/scraper/` — implement scrape + fallback
5. Add endpoint in `pkg/api/server.go` — handler + route registration
6. Add API fetch + render in `web/static/js/app.js`
7. Add data source entry in `getDataSources()` in server.go
8. Add bias note in `renderSourcesPage()` in app.js

### How to fix a bug
1. **Wrong rate displayed** → Check `ScrapeExchangeRates()` in `pkg/scraper/scraper.go` — Frankfurter API endpoint
2. **BBM scraping broken** → Check regex patterns in `extractLatestPrices()` — news site may have changed HTML
3. **Insight not generating** → Check `data/knowledge/` directory exists, check AI client enabled, check model name
4. **API returning empty** → Check year/month filter params, check DB has data for that period
5. **Frontend not rendering** → Check browser console, check API endpoint returns valid JSON
6. **Cron not firing** → Check `SCRAPE_INTERVAL` env var, check cron expression in `cmd/main.go`
7. **SQLite locked** → app has `MaxOpenConns(1)` — background cron blocks during HTTP. Wait or restart.

### How to debug background tasks
1. Startup seed logs appear in stdout immediately
2. Cron logs appear every N hours — check "Cron" prefix in stdout
3. Knowledge generation logs show "knowledge" prefix + file count
4. AI errors show "ai" prefix with API response details
5. Scrape audit trail in `scrape_log` table via `/api/v1/admin/scrape-logs`

### How to rebuild and run
```bash
cd /home/anton/Koding/ngawur/pov-ai-indonesia
go build -o pov-ai ./cmd/
PORT=8080 ./pov-ai
```

Untuk quick restart (kill + rebuild + run):
```bash
bash restart.sh
```
