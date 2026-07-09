// POV AI Indonesia - Main Application JS
const API_BASE = '/api/v1';
const BBM_TYPES = ['Pertalite', 'Solar', 'Pertamax', 'Pertamax Green', 'Pertamax Turbo', 'Dexlite', 'Pertamina Dex'];
let currentYear = new Date().getFullYear();
let currentMonth = new Date().getMonth() + 1; // 1-12
let currentBbmType = 'Pertalite';

document.addEventListener('DOMContentLoaded', () => {
    navigateTo(window.location.pathname);
    setupNavigation();
    window.addEventListener('popstate', (e) => {
        navigateTo(window.location.pathname);
    });
});

function navigateTo(path) {
    if (path.includes('exchange-rate')) {
        setActiveNav('exchange');
        renderExchangeRatePage();
    } else if (path.includes('fuel-price')) {
        setActiveNav('fuel');
        renderFuelPricePage();
    } else if (path.includes('commodities')) {
        setActiveNav('commodities');
        renderCommoditiesPage();
    } else if (path.includes('sources') || path.includes('sumber-data')) {
        setActiveNav('sources');
        renderSourcesPage();
    } else {
        setActiveNav('dashboard');
        renderDashboard();
    }
}

function setActiveNav(page) {
    document.querySelectorAll('.nav-link').forEach(l => l.classList.remove('active'));
    const link = document.querySelector(`.nav-link[data-page="${page}"]`);
    if (link) link.classList.add('active');
}

function setupNavigation() {
    document.querySelectorAll('.nav-link').forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            const page = link.dataset.page;
            let path = '/';
            if (page === 'exchange') path = '/exchange-rate';
            else if (page === 'fuel') path = '/fuel-price';
            else if (page === 'commodities') path = '/commodities';
            else if (page === 'sources') path = '/sources';
            history.pushState({ page }, '', path);
            navigateTo(path);
        });
    });
}

/**
 * Returns HTML for the standard disclaimer banner shown on every page.
 */
function buildDisclaimerHTML() {
    return `
        <div class="disclaimer-banner" id="disclaimer-banner">
            <span class="disclaimer-icon">&#9888;&#65039;</span>
            <div class="disclaimer-text">
                <strong>Transparansi Data &amp; Potensi Bias:</strong>
                Data nilai tukar USD/IDR bersumber dari <strong>ECB via Frankfurter.app</strong> (reference rate harian, bukan market spot).
                Market spot rate (TradingView, Google Finance) dapat berbeda <strong>±0.2%–1%</strong> dari ECB reference rate.
                Data BBM bersumber dari <strong>harga resmi Pertamina</strong> yang dikurasi manual.
                Data BPS, BI, dan BMKG menggunakan <strong>seed data</strong> (bukan live API) karena memerlukan registrasi institusional.
                Seed data dapat berbeda dari nilai aktual terkini. Selalu verifikasi ke
                <a href="https://www.bps.go.id" target="_blank" rel="noopener">bps.go.id</a>,
                <a href="https://www.bi.go.id" target="_blank" rel="noopener">bi.go.id</a>, dan
                <a href="https://www.pertamina.com" target="_blank" rel="noopener">pertamina.com</a>
                untuk keputusan finansial.
                <a href="/sources" onclick="history.pushState({page:'sources'},'', '/sources'); navigateTo('/sources'); return false;">&#8594; Lihat detail sumber data</a>
            </div>
        </div>
    `;
}

async function fetchAPI(endpoint) {
    try {
        const res = await fetch(`${API_BASE}${endpoint}`);
        if (!res.ok) throw new Error(`API error: ${res.status}`);
        return await res.json();
    } catch (err) {
        console.error('API error:', err);
        return null;
    }
}

// ===================== DASHBOARD =====================

async function renderDashboard() {
    const main = document.getElementById('main-content');
    main.innerHTML = buildDisclaimerHTML() + '<div class="loading">Memuat data</div>';

    const params = new URLSearchParams();
    if (currentYear) params.set('year', currentYear);
    if (currentMonth) params.set('month', currentMonth);

    const data = await fetchAPI(`/dashboard?${params}`);
    if (!data) {
        main.innerHTML = '<div class="error-msg">Gagal memuat data. Pastikan server berjalan.</div>';
        return;
    }

    const et = data.exchange_trend;
    const ft = data.fuel_trend;
    const resolvedYear = data.filter && data.filter.resolved_year ? data.filter.resolved_year : currentYear;
    const resolvedMonth = data.filter && data.filter.resolved_month ? data.filter.resolved_month : currentMonth;
    const dataNote = data.filter && data.filter.note ? data.filter.note : '';

    // Get latest exchange rate
    let latestRate = et.current_value;
    if (data.exchange_rates && data.exchange_rates.length > 0) {
        const sorted = [...data.exchange_rates].sort((a, b) => b.date.localeCompare(a.date));
        latestRate = sorted[0].rate;
    }

    // Get latest fuel price (Pertalite)
    let latestFuel = ft.current_value;
    if (data.fuel_prices && data.fuel_prices.length > 0) {
        const pertalite = data.fuel_prices.filter(p => p.bbm_type === 'Pertalite');
        if (pertalite.length > 0) {
            const sorted = [...pertalite].sort((a, b) => b.date.localeCompare(a.date));
            latestFuel = sorted[0].price;
        } else {
            const sorted = [...data.fuel_prices].sort((a, b) => b.date.localeCompare(a.date));
            latestFuel = sorted[0].price;
        }
    }

    let html = `
        <div class="page-title">
            <h2>Dashboard</h2>
            <p>Ringkasan nilai tukar Rupiah dan harga BBM Indonesia</p>
        </div>
        ${dataNote ? `<div class="data-note-banner">&#8505;&#65039; ${dataNote}</div>` : ''}
        <div class="filter-bar">
            <div class="filter-group"><label>Tahun</label>
                <select class="filter-select" id="filter-year">
                    <option value="0">Semua</option>
                    ${[2020,2021,2022,2023,2024,2025,2026].map(y => `<option value="${y}" ${y === currentYear ? 'selected' : ''}>${y}</option>`).join('')}
                </select>
            </div>
            <div class="filter-group"><label>Bulan</label>
                <select class="filter-select" id="filter-month">
                    <option value="0">Semua</option>
                    ${['Januari','Februari','Maret','April','Mei','Juni','Juli','Agustus','September','Oktober','November','Desember'].map((m,i) => `<option value="${i+1}" ${i+1 === currentMonth ? 'selected' : ''}>${m}</option>`).join('')}
                </select>
            </div>
            <button class="filter-btn" id="btn-apply-filter">Terapkan Filter</button>
        </div>
        <div class="cards-grid">
            <div class="stat-card">
                <div class="stat-card-header"><span class="stat-card-label">USD/IDR</span>
                    <span class="stat-card-badge badge-${et.direction}">${et.direction === 'up' ? 'Lemah' : et.direction === 'down' ? 'Kuat' : 'Stabil'}</span>
                </div>
                <div class="stat-card-value">Rp ${fmt(latestRate)}</div>
                <div class="stat-card-sub">Nilai tukar terbaru</div>
                <div class="stat-card-change change-${et.direction}">
                    ${et.direction === 'up' ? '&#9660;' : et.direction === 'down' ? '&#9650;' : '&#9644;'}
                    ${Math.abs(et.change_percent).toFixed(2)}% vs ${resolvedMonth ? 'bulan sebelumnya' : 'sebelumnya'}
                </div>
            </div>
            <div class="stat-card">
                <div class="stat-card-header"><span class="stat-card-label">Pertalite</span>
                    <span class="stat-card-badge badge-${ft.direction}">${ft.direction === 'up' ? 'Naik' : ft.direction === 'down' ? 'Turun' : 'Stabil'}</span>
                </div>
                <div class="stat-card-value">Rp ${fmt(latestFuel)}</div>
                <div class="stat-card-sub">Harga terbaru per liter</div>
                <div class="stat-card-change change-${ft.direction}">
                    ${ft.direction === 'up' ? '&#9660;' : ft.direction === 'down' ? '&#9650;' : '&#9644;'}
                    ${Math.abs(ft.change_percent).toFixed(2)}% vs ${resolvedMonth ? 'bulan sebelumnya' : 'sebelumnya'}
                </div>
            </div>
            <div class="stat-card">
                <div class="stat-card-header"><span class="stat-card-label">Data Points</span></div>
                <div class="stat-card-value">${(data.exchange_rates ? data.exchange_rates.length : 0) + (data.fuel_prices ? data.fuel_prices.length : 0)}</div>
                <div class="stat-card-sub">Total data point${currentMonth ? ' bulan ini' : ''}</div>
            </div>
        </div>
    `;

    // Data sources summary
    if (data.data_sources && data.data_sources.length > 0) {
        html += `
            <div class="data-sources-summary">
                <h3>&#128202; Sumber Data</h3>
                <div class="source-tags">
                    ${data.data_sources.map(s => `
                        <span class="source-tag ${s.access_type === 'manual' ? 'seed' : s.access_type}">${s.name}</span>
                    `).join('')}
                </div>
                <a href="/sources" class="source-detail-link" onclick="history.pushState({page:'sources'},'', '/sources'); navigateTo('/sources'); return false;">Lihat detail sumber data &rarr;</a>
            </div>
        `;
    }

    html += `
        <div class="chart-container"><div class="chart-header"><span class="chart-title">Tren USD/IDR</span></div>
            <div id="exchange-chart" class="chart-canvas"></div>
        </div>
        <div class="chart-container">
            <div class="chart-header">
                <span class="chart-title">Tren Harga BBM</span>
                <select class="chart-select" id="dashboard-filter-bbm">
                    ${BBM_TYPES.map(bbm => `<option value="${bbm}" ${bbm === currentBbmType ? 'selected' : ''}>${bbm}</option>`).join('')}
                </select>
            </div>
            <div id="fuel-chart" class="chart-canvas"></div>
        </div>
    `;

    if (data.insights && data.insights.length > 0) {
        data.insights.forEach(insight => { html += buildInsightHTML(insight); });
    }

    main.innerHTML = buildDisclaimerHTML() + html;
    setupFilterEvents();

    requestAnimationFrame(() => {
        if (data.exchange_rates && data.exchange_rates.length > 0) {
            drawLineChart('exchange-chart', data.exchange_rates.map(r => ({date: r.date, value: r.rate})), '#4a9eff');
        }
        if (data.fuel_prices && data.fuel_prices.length > 0) {
            const updateDashboardFuelChart = () => {
                const selectedPrices = data.fuel_prices.filter(p => p.bbm_type === currentBbmType);
                const dataToPlot = selectedPrices.length > 0 ? selectedPrices : data.fuel_prices.filter(p => p.bbm_type === 'Pertalite');
                const fuelByDate = {};
                dataToPlot.forEach(p => {
                    if (!fuelByDate[p.date]) fuelByDate[p.date] = [];
                    fuelByDate[p.date].push(p.price);
                });
                const fuelAvg = Object.entries(fuelByDate).map(([date, prices]) => ({
                    date, value: prices.reduce((a,b) => a+b, 0) / prices.length
                }));
                
                const fuelColors = {
                    'Pertalite': '#00d68f',
                    'Solar': '#ffa502',
                    'Pertamax': '#4a9eff',
                    'Pertamax Green': '#10b981',
                    'Pertamax Turbo': '#ff4757',
                    'Dexlite': '#a855f7',
                    'Pertamina Dex': '#ec4899'
                };
                const chartColor = fuelColors[currentBbmType] || '#00d68f';
                drawLineChart('fuel-chart', fuelAvg, chartColor);
            };

            updateDashboardFuelChart();

            const chartSelect = document.getElementById('dashboard-filter-bbm');
            if (chartSelect) {
                chartSelect.addEventListener('change', (e) => {
                    currentBbmType = e.target.value;
                    updateDashboardFuelChart();
                });
            }
        }
    });
}

// ===================== EXCHANGE RATE PAGE =====================

async function renderExchangeRatePage() {
    const main = document.getElementById('main-content');
    main.innerHTML = buildDisclaimerHTML() + '<div class="loading">Memuat data nilai tukar</div>';

    const params = new URLSearchParams();
    if (currentYear) params.set('year', currentYear);
    if (currentMonth) params.set('month', currentMonth);

    const data = await fetchAPI(`/exchange-rates?${params}`);
    const resolvedYear = data && data.resolved_year ? data.resolved_year : currentYear;
    const resolvedMonth = data && data.resolved_month ? data.resolved_month : currentMonth;
    const dataNote = data && data.note ? data.note : '';
    const insight = await fetchAPI(`/insights/exchange_rate/${resolvedYear}${resolvedMonth ? '-'+String(resolvedMonth).padStart(2,'0') : ''}`);

    if (!data || !data.data) {
        main.innerHTML = '<div class="error-msg">Gagal memuat data.</div>';
        return;
    }

    const rates = data.data;
    const trend = data.trend;

    // Get latest rate
    let latestRate = trend.current_value;
    if (rates.length > 0) {
        const sorted = [...rates].sort((a, b) => b.date.localeCompare(a.date));
        latestRate = sorted[0].rate;
    }

    let html = `
        <div class="page-title"><h2>Nilai Tukar USD/IDR</h2><p>Pergerakan nilai tukar Rupiah terhadap Dolar AS</p></div>
        ${dataNote ? `<div class="data-note-banner">&#8505;&#65039; ${dataNote}</div>` : ''}
        <div class="filter-bar">
            <div class="filter-group"><label>Tahun</label>
                <select class="filter-select" id="filter-year"><option value="0">Semua</option>
                    ${[2020,2021,2022,2023,2024,2025,2026].map(y => `<option value="${y}" ${y === currentYear ? 'selected' : ''}>${y}</option>`).join('')}
                </select>
            </div>
            <div class="filter-group"><label>Bulan</label>
                <select class="filter-select" id="filter-month"><option value="0">Semua</option>
                    ${['Januari','Februari','Maret','April','Mei','Juni','Juli','Agustus','September','Oktober','November','Desember'].map((m,i) => `<option value="${i+1}" ${i+1 === currentMonth ? 'selected' : ''}>${m}</option>`).join('')}
                </select>
            </div>
            <button class="filter-btn" id="btn-apply-filter">Terapkan Filter</button>
        </div>
        <div class="cards-grid">
            <div class="stat-card">
                <div class="stat-card-header"><span class="stat-card-label">Rate Terbaru</span>
                    <span class="stat-card-badge badge-${trend.direction}">${trend.direction === 'up' ? 'Lemah' : trend.direction === 'down' ? 'Kuat' : 'Stabil'}</span>
                </div>
                <div class="stat-card-value">Rp ${fmt(latestRate)}</div>
                <div class="stat-card-sub">per 1 USD</div>
            </div>
            <div class="stat-card">
                <div class="stat-card-header"><span class="stat-card-label">Perubahan</span></div>
                <div class="stat-card-value ${trend.direction === 'up' ? 'change-up' : trend.direction === 'down' ? 'change-down' : ''}">${trend.change_percent > 0 ? '+' : ''}${trend.change_percent.toFixed(2)}%</div>
                <div class="stat-card-sub">vs sebelumnya</div>
            </div>
            <div class="stat-card">
                <div class="stat-card-header"><span class="stat-card-label">Rata-rata</span></div>
                <div class="stat-card-value">Rp ${fmt(rates.reduce((a,r) => a + r.rate, 0) / rates.length)}</div>
                <div class="stat-card-sub">rata-rata periode</div>
            </div>
        </div>
        <div class="chart-container"><div class="chart-header"><span class="chart-title">Grafik USD/IDR</span></div>
            <div id="exchange-detail-chart" class="chart-canvas"></div>
        </div>
    `;

    if (insight) html += buildInsightHTML(insight);

    html += `
        <div class="data-table-container">
            <div class="data-table-header"><span class="data-table-title">Data Historis</span>
                <span style="color:var(--text-muted);font-size:12px">${rates.length} records</span>
            </div>
            <table class="data-table"><thead><tr><th>Tanggal</th><th>Rate (IDR)</th></tr></thead>
            <tbody>${rates.slice(0,50).map(r => `<tr><td>${r.date}</td><td>Rp ${fmt(r.rate)}</td></tr>`).join('')}</tbody></table>
        </div>
    `;

    main.innerHTML = buildDisclaimerHTML() + html;
    setupFilterEvents();
    requestAnimationFrame(() => {
        drawLineChart('exchange-detail-chart', rates.map(r => ({date: r.date, value: r.rate})), '#4a9eff');
    });
}

// ===================== FUEL PRICE PAGE =====================

async function renderFuelPricePage() {
    const main = document.getElementById('main-content');
    main.innerHTML = buildDisclaimerHTML() + '<div class="loading">Memuat data harga BBM</div>';

    const params = new URLSearchParams();
    if (currentYear) params.set('year', currentYear);
    if (currentMonth) params.set('month', currentMonth);

    const data = await fetchAPI(`/fuel-prices?${params}`);
    const resolvedYear = data && data.resolved_year ? data.resolved_year : currentYear;
    const resolvedMonth = data && data.resolved_month ? data.resolved_month : currentMonth;
    const dataNote = data && data.note ? data.note : '';
    const latest = await fetchAPI('/fuel-prices/latest');
    const insight = await fetchAPI(`/insights/fuel_price/${resolvedYear}${resolvedMonth ? '-'+String(resolvedMonth).padStart(2,'0') : ''}`);

    if (!data || !data.data) {
        main.innerHTML = '<div class="error-msg">Gagal memuat data.</div>';
        return;
    }

    const prices = data.data;
    const trend = data.trend;

    // Group by type - get latest price for each BBM type
    const latestByBbm = {};
    prices.forEach(p => {
        if (!latestByBbm[p.bbm_type] || p.date > latestByBbm[p.bbm_type].date) {
            latestByBbm[p.bbm_type] = p;
        }
    });

    // Get latest Pertalite price
    let latestPertalite = trend.current_value;
    if (latestByBbm['Pertalite']) {
        latestPertalite = latestByBbm['Pertalite'].price;
    }

    let html = `
        <div class="page-title"><h2>Harga BBM Indonesia</h2><p>Pergerakan harga Bahan Bakar Minyak di Indonesia</p></div>
        ${dataNote ? `<div class="data-note-banner">&#8505;&#65039; ${dataNote}</div>` : ''}
        <div class="filter-bar">
            <div class="filter-group"><label>Tahun</label>
                <select class="filter-select" id="filter-year"><option value="0">Semua</option>
                    ${[2020,2021,2022,2023,2024,2025,2026].map(y => `<option value="${y}" ${y === currentYear ? 'selected' : ''}>${y}</option>`).join('')}
                </select>
            </div>
            <div class="filter-group"><label>Bulan</label>
                <select class="filter-select" id="filter-month"><option value="0">Semua</option>
                    ${['Januari','Februari','Maret','April','Mei','Juni','Juli','Agustus','September','Oktober','November','Desember'].map((m,i) => `<option value="${i+1}" ${i+1 === currentMonth ? 'selected' : ''}>${m}</option>`).join('')}
                </select>
            </div>
            <button class="filter-btn" id="btn-apply-filter">Terapkan Filter</button>
        </div>
        <div class="cards-grid">
            <div class="stat-card">
                <div class="stat-card-header"><span class="stat-card-label">Pertalite</span>
                    <span class="stat-card-badge badge-${trend.direction}">${trend.direction === 'up' ? 'Naik' : trend.direction === 'down' ? 'Turun' : 'Stabil'}</span>
                </div>
                <div class="stat-card-value">Rp ${fmt(latestPertalite)}</div>
                <div class="stat-card-sub">harga terbaru per liter</div>
            </div>
            <div class="stat-card">
                <div class="stat-card-header"><span class="stat-card-label">Jenis BBM</span></div>
                <div class="stat-card-value">${Object.keys(latestByBbm).length}</div>
                <div class="stat-card-sub">jenis tercatat</div>
            </div>
            <div class="stat-card">
                <div class="stat-card-header"><span class="stat-card-label">Perubahan</span></div>
                <div class="stat-card-value ${trend.direction === 'up' ? 'change-up' : trend.direction === 'down' ? 'change-down' : ''}">${trend.change_percent > 0 ? '+' : ''}${trend.change_percent.toFixed(2)}%</div>
                <div class="stat-card-sub">vs sebelumnya</div>
            </div>
        </div>
    `;

    // Price per BBM type - show latest price for each
    html += '<div class="cards-grid">';
    const bbmOrder = ['Pertalite', 'Solar', 'Pertamax', 'Pertamax Green', 'Pertamax Turbo', 'Dexlite', 'Pertamina Dex'];
    for (const bbm of bbmOrder) {
        if (latestByBbm[bbm]) {
            html += `<div class="stat-card"><div class="stat-card-header"><span class="stat-card-label">${bbm}</span></div>
                <div class="stat-card-value" style="font-size:24px">Rp ${fmt(latestByBbm[bbm].price)}</div>
                <div class="stat-card-sub">harga terbaru per liter</div></div>`;
        }
    }
    html += '</div>';

    // Latest prices table
    if (latest && latest.data && latest.data.length > 0) {
        html += `
            <div class="data-table-container">
                <div class="data-table-header"><span class="data-table-title">Harga BBM Terbaru</span></div>
                <table class="data-table"><thead><tr><th>Jenis BBM</th><th>Harga (Rp)</th><th>Tanggal</th><th>Sumber</th></tr></thead>
                <tbody>${latest.data.map(p => `<tr><td>${p.bbm_type}</td><td>Rp ${fmt(p.price)}</td><td>${p.date}</td><td>${p.source}</td></tr>`).join('')}</tbody></table>
            </div>
        `;
    }

    html += `
        <div class="chart-container">
            <div class="chart-header">
                <span class="chart-title">Grafik Harga BBM</span>
                <select class="chart-select" id="chart-filter-bbm">
                    ${BBM_TYPES.map(bbm => `<option value="${bbm}" ${bbm === currentBbmType ? 'selected' : ''}>${bbm}</option>`).join('')}
                </select>
            </div>
            <div id="fuel-detail-chart" class="chart-canvas"></div>
        </div>
    `;

    if (insight) html += buildInsightHTML(insight);

    html += `
        <div class="data-table-container">
            <div class="data-table-header"><span class="data-table-title">Data Historis</span>
                <span style="color:var(--text-muted);font-size:12px">${prices.length} records</span>
            </div>
            <table class="data-table"><thead><tr><th>Tanggal</th><th>Jenis BBM</th><th>Harga (Rp)</th></tr></thead>
            <tbody>${prices.slice(0,50).map(p => `<tr><td>${p.date}</td><td>${p.bbm_type}</td><td>Rp ${fmt(p.price)}</td></tr>`).join('')}</tbody></table>
        </div>
    `;

    main.innerHTML = buildDisclaimerHTML() + html;
    setupFilterEvents();
    requestAnimationFrame(() => {
        const updateChart = () => {
            const selectedPrices = prices.filter(p => p.bbm_type === currentBbmType);
            const dataToPlot = selectedPrices.length > 0 ? selectedPrices : prices.filter(p => p.bbm_type === 'Pertalite');
            const fuelByDate = {};
            dataToPlot.forEach(p => { if (!fuelByDate[p.date]) fuelByDate[p.date] = []; fuelByDate[p.date].push(p.price); });
            const fuelAvg = Object.entries(fuelByDate).map(([date, prs]) => ({date, value: prs.reduce((a,b)=>a+b,0)/prs.length}));
            
            const fuelColors = {
                'Pertalite': '#00d68f',
                'Solar': '#ffa502',
                'Pertamax': '#4a9eff',
                'Pertamax Green': '#10b981',
                'Pertamax Turbo': '#ff4757',
                'Dexlite': '#a855f7',
                'Pertamina Dex': '#ec4899'
            };
            const chartColor = fuelColors[currentBbmType] || '#00d68f';
            drawLineChart('fuel-detail-chart', fuelAvg, chartColor);
        };

        updateChart();

        const chartSelect = document.getElementById('chart-filter-bbm');
        if (chartSelect) {
            chartSelect.addEventListener('change', (e) => {
                currentBbmType = e.target.value;
                updateChart();
            });
        }
    });
}

// ===================== COMMODITIES PAGE =====================

let currentWilayah = 'Jakarta';

async function renderCommoditiesPage() {
    const main = document.getElementById('main-content');
    main.innerHTML = '<div class="loading">Memuat data harga kebutuhan pokok</div>';

    // Fetch latest prices for selected wilayah
    const latest = await fetchAPI(`/commodity-latest?region=${currentWilayah}`);

    if (!latest || !latest.data) {
        main.innerHTML = '<div class="error-msg">Gagal memuat data komoditas.</div>';
        return;
    }

    const prices = latest.data;

    // Group by commodity
    const byCommodity = {};
    prices.forEach(p => {
        if (!byCommodity[p.commodity]) byCommodity[p.commodity] = [];
        byCommodity[p.commodity].push(p);
    });

    const wilayahOptions = ['Jakarta', 'Jawa Tengah', 'Yogyakarta', 'Jawa Timur'];

    let html = `
        <div class="page-title">
            <h2>Harga Kebutuhan Pokok</h2>
            <p>Harga emas, beras, dan minyak goreng di pasar tradisional</p>
        </div>
        <div class="filter-bar">
            <div class="filter-group"><label>Wilayah</label>
                <select class="filter-select" id="filter-wilayah">
                    ${wilayahOptions.map(w => `<option value="${w}" ${w === currentWilayah ? 'selected' : ''}>${w}</option>`).join('')}
                </select>
            </div>
            <span style="color:var(--text-muted);font-size:12px;margin-left:auto">Data kurasi manual — harga bisa berbeda dengan pasar terdekat</span>
        </div>
    `;

    // TradingView Gold Chart Widget
    html += `
        <div class="tradingview-widget-container">
            <div class="tradingview-widget-header">
                <span class="tradingview-title">Live Gold Price (XAU/IDR per gram)</span>
                <a href="https://id.tradingview.com/symbols/XAUIDRG/" target="_blank" rel="noopener" class="tradingview-link">Buka di TradingView</a>
            </div>
            <div id="tradingview-gold-chart" class="tradingview-chart"></div>
        </div>
    `;

    // Cards per commodity group
    const commodityOrder = ['Emas', 'Beras', 'Minyak Goreng'];

    for (const commodity of commodityOrder) {
        if (!byCommodity[commodity]) continue;
        const items = byCommodity[commodity];

        html += `
            <div class="commodity-section">
                <div class="commodity-header">
                    <h3>${commodity}</h3>
                </div>
                <div class="cards-grid">
        `;

        // Special handling for Emas - show 1g, 5g, 10g
        if (commodity === 'Emas' && items.length > 0) {
            const pricePerGram = items[0].price;
            const multipliers = [
                { label: '1 gram', mult: 1 },
                { label: '5 gram', mult: 5 },
                { label: '10 gram', mult: 10 },
            ];
            for (const m of multipliers) {
                const totalPrice = pricePerGram * m.mult;
                html += `
                    <div class="stat-card">
                        <div class="stat-card-header">
                            <span class="stat-card-label">Emas ${m.label}</span>
                            <span class="stat-card-source">${items[0].source}</span>
                        </div>
                        <div class="stat-card-value">${formatPrice(totalPrice)}</div>
                        <div class="stat-card-sub">@ ${formatPrice(pricePerGram)}/gram — ${items[0].region}</div>
                    </div>
                `;
            }
        } else {
            for (const item of items) {
                html += `
                    <div class="stat-card">
                        <div class="stat-card-header">
                            <span class="stat-card-label">${item.type}</span>
                            <span class="stat-card-source">${item.source}</span>
                        </div>
                        <div class="stat-card-value">${formatPrice(item.price)}</div>
                        <div class="stat-card-sub">per ${item.unit} — ${item.region}</div>
                    </div>
                `;
            }
        }

        html += '</div></div>';
    }

    // Price comparison table
    html += `
        <div class="data-table-container">
            <div class="data-table-header"><span class="data-table-title">Tabel Harga ${currentWilayah}</span></div>
            <table class="data-table">
                <thead><tr><th>Komoditas</th><th>Jenis</th><th>Harga (Rp)</th><th>Unit</th></tr></thead>
                <tbody>${prices.map(p => `<tr><td>${p.commodity}</td><td>${p.type}</td><td>${formatPrice(p.price)}</td><td>${p.unit}</td></tr>`).join('')}</tbody>
            </table>
        </div>
        <div class="disclaimer-banner" style="margin-top:24px">
            <span class="disclaimer-icon">i</span>
            <div class="disclaimer-text">
                <strong>Catatan:</strong> Harga di atas adalah data kurasi manual berdasarkan survei pasar tradisional ${currentWilayah} dan bisa berbeda dengan harga real-time. Selalu cek harga terkini di toko/pedagang terdekat.
                <a href="/sources" onclick="history.pushState({page:'sources'},'', '/sources'); navigateTo('/sources'); return false;">Lihat semua sumber data</a>
            </div>
        </div>
    `;

    main.innerHTML = html;

    // Setup wilayah filter
    const wilayahSelect = document.getElementById('filter-wilayah');
    if (wilayahSelect) {
        wilayahSelect.addEventListener('change', (e) => {
            currentWilayah = e.target.value;
            renderCommoditiesPage();
        });
    }

    // Load TradingView widget
    loadTradingViewWidget();
}

function loadTradingViewWidget() {
    const container = document.getElementById('tradingview-gold-chart');
    if (!container) return;

    // Create TradingView widget
    const script = document.createElement('script');
    script.src = 'https://s3.tradingview.com/external-embedding/embed-widget-mini-symbol-overview.js';
    script.async = true;
    script.innerHTML = JSON.stringify({
        "symbol": "FX_IDC:XAUIDRG",
        "width": "100%",
        "height": "400",
        "locale": "id",
        "dateRange": "1M",
        "colorTheme": "dark",
        "isTransparent": true,
        "autosize": false,
        "largeChartUrl": "https://id.tradingview.com/symbols/XAUIDRG/",
        "noTimeScale": false,
        "chartOnly": false
    });

    container.innerHTML = '';
    container.appendChild(script);
}

function formatPrice(num) {
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'jt';
    if (num >= 1000) return (num / 1000).toFixed(0) + 'rb';
    return num.toString();
}

// ===================== INSIGHT =====================

function buildInsightHTML(insight) {
    const icon = insight.category === 'exchange_rate' ? '&#128176;' : '&#128663;';
    let factorsHTML = '';
    if (insight.factors && insight.factors.length > 0) {
        factorsHTML = '<h3>Faktor-faktor Penyebab:</h3><ul>' + insight.factors.map(f => `<li>${f}</li>`).join('') + '</ul>';
    }
    let contentHTML = parseMarkdown(insight.content);
    return `
        <div class="pov-section">
            <div class="pov-section-header">
                <span class="pov-ai-badge">POV AI</span>
                <span class="pov-section-title">${insight.title || 'AI Insight'}</span>
                <span style="margin-left:auto;font-size:11px;color:var(--text-muted)">&#9888; Insight dihasilkan AI berdasarkan data historis — bukan saran keuangan</span>
            </div>
            <div class="pov-section-body">${contentHTML}</div>
        </div>
    `;
}

// ===================== SOURCES PAGE =====================

async function renderSourcesPage() {
    const main = document.getElementById('main-content');
    main.innerHTML = '<div class="loading">Memuat informasi sumber data</div>';

    const data = await fetchAPI('/data-sources');

    const accessLabel = { public: 'Publik (Live)', official: 'Resmi (Dikurasi)', manual: 'Kurasi Manual (Statis)' };
    const accessClass = { public: 'public', official: 'official', manual: 'seed' };

    let sourcesHTML = '';
    if (data && data.data) {
        sourcesHTML = data.data.map(src => `
            <div class="source-card access-${accessClass[src.access_type] || 'seed'}">
                <div class="source-card-header">
                    <div class="source-name">${src.name}</div>
                    <span class="access-badge ${accessClass[src.access_type] || 'seed'}">${accessLabel[src.access_type] || src.access_type}</span>
                </div>
                <div class="source-description">${src.description}</div>
                <div class="source-meta">
                    <div class="source-meta-row">
                        <span class="source-meta-key">URL</span>
                        <span class="source-meta-val"><a href="${src.url.split(' ')[0]}" target="_blank" rel="noopener">${src.url}</a></span>
                    </div>
                    <div class="source-meta-row">
                        <span class="source-meta-key">Diperbarui</span>
                        <span class="source-meta-val">${src.last_updated}</span>
                    </div>
                    <div class="source-meta-row">
                        <span class="source-meta-key">Siklus</span>
                        <span class="source-meta-val">${src.refresh_cycle}</span>
                    </div>
                </div>
                ${src.notes ? `<div class="source-notes">&#128204; ${src.notes}</div>` : ''}
            </div>
        `).join('');
    }

    const html = `
        <div class="page-title">
            <h2>Transparansi Sumber Data</h2>
            <p>Daftar lengkap sumber data yang digunakan beserta potensi bias dan keterbatasannya</p>
        </div>

        <div class="legend-bar">
            <div class="legend-item"><div class="legend-dot green"></div>Live API (data real-time)</div>
            <div class="legend-item"><div class="legend-dot blue"></div>Data Resmi (dikurasi dari pengumuman resmi)</div>
            <div class="legend-item"><div class="legend-dot yellow"></div>Kurasi Manual (data statis, perlu verifikasi ke sumber resmi)</div>
        </div>

        <div class="bias-section">
            <h3>&#9888;&#65039; Potensi Bias &amp; Keterbatasan Data</h3>
            <p class="bias-subtitle">Pahami keterbatasan ini sebelum menggunakan data untuk keputusan finansial atau riset.</p>
            <ul class="bias-list">
                <li>
                    <span class="bias-icon">&#128197;</span>
                    <div class="bias-content">
                        <strong>Data Historis BBM — Estimasi, Bukan Arsip Resmi</strong>
                        Data harga BBM 2020–2025 merupakan data <em>seed</em> yang direkonstruksi dari pemberitaan media (Bisnis.com, CNBC Indonesia) dan pengumuman Pertamina.
                        Nilai spesifik per bulan adalah interpolasi — bukan tanggal perubahan harga yang tepat. Gunakan hanya sebagai gambaran tren.
                    </div>
                </li>
                <li>
                    <span class="bias-icon">&#128200;</span>
                    <div class="bias-content">
                        <strong>Nilai Tukar USD/IDR — ECB Reference, Bukan Market Spot</strong>
                        Data kurs bersumber dari <strong>European Central Bank (ECB)</strong> via Frankfurter.app — reference rate harian, diperbarui 1x sehari jam ~16:00 CET.
                        <strong>Market spot rate</strong> (yang terlihat di TradingView, Google Finance, atau berita Kontan/CNBC) <strong>bisa berbeda ±0.2%–1%</strong> dari ECB. Ini wajar karena:
                        <ul style="margin:6px 0 0 18px;padding:0;font-size:13px;line-height:1.6">
                            <li>ECB = official fixing rate (mid-rate dari survey market maker)</li>
                            <li>Market spot = bid/ask real-time dari liquidity provider</li>
                            <li>Spread wajar ±50–150 pips tergantung likuiditas</li>
                        </ul>
                        Contoh 18 Jun 2026: ECB 17.803 vs Google Finance 17.850 vs TradingView 17.748.
                    </div>
                </li>
                <li>
                    <span class="bias-icon">&#128202;</span>
                    <div class="bias-content">
                        <strong>Data BPS (Inflasi, GDP, Pengangguran) — Seed Statis</strong>
                        Indikator BPS yang digunakan adalah <em>seed data</em>: inflasi estimasi ~2.19% (tren 2026), TPT 4.87% (Sakernas Feb 2026), kemiskinan 8.57% (Mar 2025), IPM 75.02 (2024), PDB Rp22.134,5T (2024).
                        Nilai aktual dapat berbeda karena data ini tidak diperbarui otomatis. BPS merilis data resmi di
                        <a href="https://www.bps.go.id" target="_blank" rel="noopener">bps.go.id</a>
                        secara berkala (inflasi bulanan, PDB triwulanan).
                    </div>
                </li>
                <li>
                    <span class="bias-icon">&#127981;</span>
                    <div class="bias-content">
                        <strong>Suku Bunga Bank Indonesia — Seed, Bukan Live</strong>
                        Seed data BI: BI7DRR <strong>5.50%</strong>, Deposit Facility <strong>4.75%</strong>, Lending Facility <strong>6.25%</strong>
                        (berlaku sejak RDG Januari 2025). BI mengumumkan perubahan suku bunga setiap Rapat Dewan Gubernur (RDG) bulanan.
                        Cek keputusan terbaru di <a href="https://www.bi.go.id" target="_blank" rel="noopener">bi.go.id</a>.
                    </div>
                </li>
                <li>
                    <span class="bias-icon">&#127774;</span>
                    <div class="bias-content">
                        <strong>Cuaca BMKG — Seed saat API Tidak Tersedia</strong>
                        BMKG public API terkadang tidak dapat diakses (HTTP 404). Aplikasi otomatis menggunakan seed data cuaca kota besar.
                        Seed cuaca (suhu, kelembaban) adalah nilai representatif, bukan pembacaan real-time.
                    </div>
                </li>
                <li>
                    <span class="bias-icon">&#129302;</span>
                    <div class="bias-content">
                        <strong>Insight AI — Analisis Berbasis Template, Bukan Prediksi</strong>
                        Insight yang ditampilkan dihasilkan dari template berbasis aturan (rule-based) menggunakan data historis.
                        Bukan prediksi pasar atau saran investasi. Selalu konsultasikan keputusan finansial dengan profesional.
                    </div>
                </li>
            </ul>
        </div>

        <div class="page-title" style="margin-bottom:16px">
            <h2 style="font-size:20px">Detail Sumber Data</h2>
        </div>
        <div class="sources-grid">
            ${sourcesHTML || '<div style="color:var(--text-muted);font-size:13px">Gagal memuat detail sumber data.</div>'}
        </div>

        <div class="data-table-container">
            <div class="data-table-header">
                <span class="data-table-title">Cara Kami Mengumpulkan Data</span>
            </div>
            <table class="data-table">
                <thead><tr><th>Sumber</th><th>Metode</th><th>Frekuensi Update</th><th>Verifikasi</th></tr></thead>
                <tbody>
                    <tr><td>Frankfurter.app (ECB)</td><td>REST API publik</td><td>Harian (hari kerja ECB)</td><td>&#9989; Data ECB resmi</td></tr>
                    <tr><td>Pertamina (harga BBM terbaru)</td><td>Harga resmi dikurasi manual</td><td>Per pengumuman resmi</td><td>&#9989; Dikonfirmasi dari Pertamina.com</td></tr>
                    <tr><td>Histori BBM 2020–2025</td><td>Rekonstruksi dari berita &amp; pengumuman</td><td>Tidak otomatis</td><td>&#9888; Estimasi tren, bukan tanggal tepat</td></tr>
                    <tr><td>BPS Indikator Ekonomi</td><td>Seed data manual</td><td>Tidak otomatis</td><td>&#9888; Perlu verifikasi ke bps.go.id</td></tr>
                    <tr><td>Bank Indonesia Rates</td><td>Seed data manual</td><td>Tidak otomatis</td><td>&#9888; Perlu verifikasi ke bi.go.id</td></tr>
                    <tr><td>BMKG Cuaca</td><td>Public API (fallback ke seed)</td><td>Per jam (jika API aktif)</td><td>&#9888; Seed digunakan saat API gagal</td></tr>
                    <tr><td>data.go.id (CKAN)</td><td>Public API</td><td>Per request</td><td>&#9989; Portal data terbuka resmi</td></tr>
                </tbody>
            </table>
        </div>

        <div class="pov-section" style="margin-top:8px">
            <div class="pov-section-header">
                <span class="pov-ai-badge">KONTRIBUSI</span>
                <span class="pov-section-title">Bantu Kami Meningkatkan Akurasi Data</span>
            </div>
            <div class="pov-section-body">
                <p>Jika Anda menemukan data yang tidak akurat atau ingin berkontribusi memperbarui seed data,
                silakan buka issue atau pull request di repositori proyek ini.
                Data yang lebih akurat membantu semua pengguna mendapatkan gambaran ekonomi Indonesia yang lebih baik.</p>
                <p>Sumber data prioritas untuk diperbarui: <strong>histori harga BBM tahunan</strong>,
                <strong>indikator BPS terbaru</strong>, dan <strong>keputusan RDG Bank Indonesia</strong>.</p>
            </div>
        </div>
    `;

    main.innerHTML = html;
}

// ===================== UTILITIES =====================

function parseMarkdown(md) {
    if (!md) return '';
    let html = md
        .replace(/^### (.+)$/gm, '<h3>$1</h3>')
        .replace(/^## (.+)$/gm, '<h3>$1</h3>')
        .replace(/^# (.+)$/gm, '<h2>$1</h2>')
        .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.+?)\*/g, '<em>$1</em>')
        .replace(/^- (.+)$/gm, '<li>$1</li>')
        .replace(/^\d+\. (.+)$/gm, '<li>$1</li>')
        .replace(/\n\n/g, '</p><p>')
        .replace(/\n/g, '<br>');
    html = html.replace(/(<li>.*<\/li>)+/g, '<ul>$&</ul>');
    html = html.replace(/(<tr>.*<\/tr>)+/g, '<table class="data-table">$&</table>');
    return html;
}

function setupFilterEvents() {
    const btn = document.getElementById('btn-apply-filter');
    if (btn) {
        btn.addEventListener('click', () => {
            const y = document.getElementById('filter-year');
            const m = document.getElementById('filter-month');
            currentYear = parseInt(y?.value) || 0;
            currentMonth = parseInt(m?.value) || 0;
            const active = document.querySelector('.nav-link.active');
            if (active) {
                const page = active.dataset.page;
                if (page === 'dashboard') renderDashboard();
                else if (page === 'exchange') renderExchangeRatePage();
                else if (page === 'fuel') renderFuelPricePage();
            }
        });
    }
}

function drawLineChart(containerId, data, color) {
    const container = document.getElementById(containerId);
    if (!container || !data || data.length === 0) return;

    const width = container.clientWidth || 800;
    const height = 300;
    const pad = {top:20, right:20, bottom:40, left:70};
    const cw = width - pad.left - pad.right;
    const ch = height - pad.top - pad.bottom;

    const vals = data.map(d => d.value);
    const minV = Math.min(...vals) * 0.998;
    const maxV = Math.max(...vals) * 1.002;
    const range = maxV - minV;

    const pts = data.map((d, i) => {
        const x = pad.left + (i / (data.length - 1 || 1)) * cw;
        const y = pad.top + ch - ((d.value - minV) / range) * ch;
        return {x, y, date: d.date, value: d.value};
    });

    const pathD = pts.map((p,i) => `${i===0?'M':'L'} ${p.x} ${p.y}`).join(' ');
    const areaD = pathD + ` L ${pts[pts.length-1].x} ${pad.top+ch} L ${pts[0].x} ${pad.top+ch} Z`;

    const yLabels = [];
    for (let i = 0; i <= 5; i++) {
        yLabels.push({val: minV + (range*i/5), y: pad.top + ch - (i/5)*ch});
    }

    const xStep = Math.max(1, Math.floor(data.length / 6));
    const xLabels = [];
    for (let i = 0; i < data.length; i += xStep) {
        xLabels.push({date: pts[i].date, x: pts[i].x});
    }

    const svg = `<svg class="chart-svg" viewBox="0 0 ${width} ${height}">
        <defs><linearGradient id="cg-${containerId}" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="${color}" stop-opacity="0.2"/>
            <stop offset="100%" stop-color="${color}" stop-opacity="0"/>
        </linearGradient></defs>
        ${yLabels.map(l => `<line class="chart-grid-line" x1="${pad.left}" y1="${l.y}" x2="${width-pad.right}" y2="${l.y}"/>`).join('')}
        <path class="chart-area" d="${areaD}" fill="url(#cg-${containerId})"/>
        <path class="chart-line" d="${pathD}" stroke="${color}"/>
        ${pts.map(p => `<circle class="chart-dot" cx="${p.x}" cy="${p.y}" r="2.5"><title>${p.date}: ${fmt(p.value)}</title></circle>`).join('')}
        ${yLabels.map(l => `<text class="chart-axis-label" x="${pad.left-10}" y="${l.y+4}" text-anchor="end">${fmt(l.val)}</text>`).join('')}
        ${xLabels.map(l => `<text class="chart-axis-label" x="${l.x}" y="${height-10}" text-anchor="middle">${l.date.substring(0,7)}</text>`).join('')}
    </svg>`;

    container.innerHTML = svg;
}

function fmt(num) {
    if (num === undefined || num === null) return '0';
    return Math.round(num).toString().replace(/\B(?=(\d{3})+(?!\d))/g, '.');
}