package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	apiKey       string
	baseURL      string
	model        string
	activeModels []string
	activeIdx    int
	mu           sync.Mutex
	temperature  float64
	maxTokens    int
	httpClient   *http.Client
	referer      string
	title        string
}

func NewClient(apiKey, baseURL, model string, temperature float64, maxTokens int, httpTimeout int, referer, title string) *Client {
	if apiKey == "" {
		fmt.Println("[ai] WARNING: API Key not set - AI insights disabled")
	}
	if model == "" {
		model = "openrouter/owl-alpha"
	}
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	if temperature <= 0 {
		temperature = 0.7
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	if httpTimeout <= 0 {
		httpTimeout = 120
	}
	if referer == "" {
		referer = "https://pov-ai.local"
	}
	if title == "" {
		title = "POV AI Indonesia"
	}

	var activeModels []string
	for _, m := range strings.Split(model, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			activeModels = append(activeModels, m)
		}
	}
	if len(activeModels) == 0 {
		activeModels = []string{"openrouter/owl-alpha"}
	}

	return &Client{
		apiKey:       apiKey,
		baseURL:      baseURL,
		model:        activeModels[0],
		activeModels: activeModels,
		activeIdx:    0,
		temperature:  temperature,
		maxTokens:    maxTokens,
		httpClient:   &http.Client{Timeout: time.Duration(httpTimeout) * time.Second},
		referer:      referer,
		title:        title,
	}
}

func (c *Client) IsEnabled() bool { return c.apiKey != "" }

func (c *Client) GetCurrentModel() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.model
}

func (c *Client) GetActiveModels() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]string, len(c.activeModels))
	copy(cp, c.activeModels)
	return cp
}

type openRouterModel struct {
	ID      string `json:"id"`
	Pricing struct {
		Prompt     interface{} `json:"prompt"`
		Completion interface{} `json:"completion"`
	} `json:"pricing"`
}

type openRouterModelsResp struct {
	Data []openRouterModel `json:"data"`
}

func isPriceZero(val interface{}) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return err == nil && f == 0
	case float64:
		return v == 0
	case int:
		return v == 0
	case int64:
		return v == 0
	}
	return false
}

func isFreeModel(m openRouterModel) bool {
	if strings.HasSuffix(m.ID, ":free") {
		return true
	}
	return isPriceZero(m.Pricing.Prompt) && isPriceZero(m.Pricing.Completion)
}

func (c *Client) FetchTopFreeModels(limit int) ([]string, error) {
	if limit <= 0 {
		limit = 3
	}
	req, err := http.NewRequest("GET", c.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for models: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openrouter models endpoint returned status %d: %s", resp.StatusCode, body[:min(200, len(body))])
	}

	var modelsResp openRouterModelsResp
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode openrouter models response: %w", err)
	}

	var freeModels []string
	for _, m := range modelsResp.Data {
		if isFreeModel(m) && m.ID != "" {
			freeModels = append(freeModels, m.ID)
			if len(freeModels) >= limit {
				break
			}
		}
	}

	if len(freeModels) == 0 {
		return nil, fmt.Errorf("no free models found on OpenRouter")
	}

	return freeModels, nil
}

func (c *Client) doChatCompletion(targetModel, systemPrompt, userPrompt string) (string, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": targetModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": c.temperature,
		"max_tokens":  c.maxTokens,
	})

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("HTTP-Referer", c.referer)
	req.Header.Set("X-Title", c.title)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, body[:min(200, len(body))])
	}

	var result struct {
		Choices []struct {
			Message struct{ Content string `json:"content"` } `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse failed: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response")
	}
	return result.Choices[0].Message.Content, nil
}

func (c *Client) ChatCompletion(systemPrompt, userPrompt string) (string, error) {
	if !c.IsEnabled() {
		return "", fmt.Errorf("AI not enabled")
	}

	c.mu.Lock()
	candidates := make([]string, len(c.activeModels))
	copy(candidates, c.activeModels)
	startIdx := c.activeIdx
	c.mu.Unlock()

	var lastErr error

	// 1. Try current candidate models
	for i := 0; i < len(candidates); i++ {
		idx := (startIdx + i) % len(candidates)
		targetModel := candidates[idx]

		content, err := c.doChatCompletion(targetModel, systemPrompt, userPrompt)
		if err == nil {
			c.mu.Lock()
			c.activeIdx = idx
			c.model = targetModel
			c.mu.Unlock()
			return content, nil
		}

		fmt.Printf("[ai] Model '%s' failed: %v. Trying next candidate model...\n", targetModel, err)
		lastErr = err
	}

	// 2. All active models failed — fetch top 3 free models from OpenRouter API to override POVModel
	fmt.Printf("[ai] All %d active models failed. Fetching top 3 free models from OpenRouter API...\n", len(candidates))
	newFreeModels, fetchErr := c.FetchTopFreeModels(3)
	if fetchErr != nil {
		return "", fmt.Errorf("all active models failed (%v) and fallback model fetch failed: %w", lastErr, fetchErr)
	}

	fmt.Printf("[ai] Overriding POVModel with new OpenRouter free models: %v\n", newFreeModels)

	c.mu.Lock()
	c.activeModels = newFreeModels
	c.activeIdx = 0
	c.model = newFreeModels[0]
	c.mu.Unlock()

	// 3. Retry completion with the new top 3 free models
	for i, targetModel := range newFreeModels {
		content, err := c.doChatCompletion(targetModel, systemPrompt, userPrompt)
		if err == nil {
			c.mu.Lock()
			c.activeIdx = i
			c.model = targetModel
			c.mu.Unlock()
			return content, nil
		}
		fmt.Printf("[ai] Newly fetched model '%s' failed: %v. Trying next...\n", targetModel, err)
		lastErr = err
	}

	return "", fmt.Errorf("all newly fetched free models failed, last error: %w", lastErr)
}

type LLMFactor struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type LLMResponse struct {
	Title    string      `json:"title"`
	Summary  string      `json:"summary"`
	Factors  []LLMFactor `json:"factors"`
	Analysis string      `json:"analysis"`
}

func (c *Client) AnalyzeExchangeRate(dataJSON, period, macroContext string) (*LLMResponse, error) {
	system := `Anda adalah POV AI, analis ekonomi makro terkemuka di Indonesia. Tugas Anda adalah menganalisis data nilai tukar USD/IDR secara objektif, mendalam, dan bebas dari bias politik atau partisan.

PANDUAN ANALISIS (MENCEGAH BIAS & MENJAGA TRANSPARANSI):
1. Transparansi Sumber Data: Jelaskan bahwa data ini berasal dari European Central Bank (ECB) via Frankfurter.app. Berikan catatan bahwa nilai ini adalah kurs referensi harian dan mungkin berbeda sekitar 0,2% - 1% dari kurs spot pasar langsung atau kurs JISDOR Bank Indonesia.
2. Perspektif Berimbang (Balanced View): Hindari menafsirkan pergerakan rupiah (depresiasi maupun apresiasi) secara satu sisi atau menyederhanakannya sebagai kegagalan/keberhasilan kebijakan domestik saja. Berikan analisis dari kedua sisi:
   - Depresiasi Rupiah: Jelaskan tekanan dari faktor eksternal (kebijakan The Fed/suku bunga AS tinggi, ketidakpastian geopolitik global) dan internal (kebutuhan likuiditas dolar, neraca dagang). Sebutkan implikasi berimbang (membantu daya saing eksportir, namun meningkatkan biaya impor bagi industri dan konsumen).
   - Apresiasi Rupiah: Jelaskan aliran modal masuk (inflow), intervensi BI, atau pelemahan dolar AS. Sebutkan juga implikasi ganda (menekan inflasi impor, tetapi berisiko menekan margin pendapatan eksportir).
3. Hubungkan dengan Data Makroekonomi Indonesia: Gunakan data indikator makroekonomi domestik (seperti data inflasi BPS dan suku bunga BI-Rate) yang disediakan dalam input untuk menjelaskan korelasi logis pergerakan nilai tukar dengan ekonomi lokal.
4. Format Output: Anda WAJIB merespons HANYA dalam format JSON yang valid. Jika Anda menggunakan pemikiran internal (<think>), buat pemikiran singkat dan langsung keluarkan JSON.

Format JSON:
{
  "title": "[Judul Analisis yang Informatif dan Netral]",
  "summary": "[Ringkasan Analisis dalam 2-3 kalimat]",
  "factors": [
    {
      "title": "[Nama Faktor]",
      "description": "[Penjelasan mendalam dan objektif mengenai faktor ini]"
    }
  ],
  "analysis": "[Analisis detail dalam beberapa paragraf, termasuk implikasi sektoral secara berimbang]"
}`

	user := fmt.Sprintf("Analisis pergerakan nilai tukar USD/IDR untuk periode %s.\n\nData Nilai Tukar:\n%s\n\nIndikator Makroekonomi Domestik (Konteks):\n%s\n\nBerikan analisis yang objektif dan berimbang dalam bahasa Indonesia sesuai format JSON yang ditentukan.", period, dataJSON, macroContext)

	resp, err := c.ChatCompletion(system, user)
	if err != nil {
		return nil, err
	}

	cleaned := extractJSON(resp)
	var result LLMResponse
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM JSON: %w (raw cleaned: '%s', full resp: '%s')", err, cleaned, resp[:min(200, len(resp))])
	}
	return &result, nil
}

func (c *Client) AnalyzeFuelPrice(dataJSON, period, macroContext string) (*LLMResponse, error) {
	system := `Anda adalah POV AI, pakar kebijakan energi dan ekonomi makro terkemuka di Indonesia. Tugas Anda adalah menganalisis pergerakan harga BBM secara netral, komprehensif, dan bebas dari opini politik/partisan.

PANDUAN ANALISIS (MENCEGAH BIAS & MENJAGA TRANSPARANSI):
1. Pembedaan Jenis BBM: Jelaskan perbedaan karakteristik secara jelas antara BBM bersubsidi/kompensasi (Pertalite, Solar/Biosolar) yang harganya ditetapkan pemerintah atas pertimbangan sosial-politik, dan BBM non-subsidi (Pertamax, Dexlite, Pertamax Turbo, dll.) yang harganya berfluktuasi secara berkala mengikuti mekanisme pasar global.
2. Analisis Trade-off Kebijakan Subsidi: Bahas kebijakan subsidi BBM secara objektif dari dua sudut pandang:
   - Sisi Sosial: Menjaga daya beli masyarakat menengah ke bawah dan mengendalikan inflasi biaya transportasi.
   - Sisi Fiskal: Dampak beban subsidi terhadap APBN, defisit fiskal, serta risiko salah sasaran subsidi.
3. Driver Utama Harga: Jelaskan korelasi harga BBM domestik dengan pergerakan harga minyak mentah internasional (ICP, Brent/WTI), nilai tukar USD/IDR (karena transaksi minyak menggunakan dolar), dan biaya logistik distribusi.
4. Hubungkan dengan Ekonomi Nasional: Hubungkan tren harga BBM dengan tingkat inflasi domestik BPS dan kebijakan suku bunga BI untuk memberikan analisis yang kaya konteks.
5. Format Output: Anda WAJIB merespons HANYA dalam format JSON yang valid. Jika Anda menggunakan pemikiran internal (<think>), buat pemikiran singkat dan langsung keluarkan JSON.

Format JSON:
{
  "title": "[Judul Analisis yang Informatif dan Netral]",
  "summary": "[Ringkasan Analisis dalam 2-3 kalimat]",
  "factors": [
    {
      "title": "[Nama Faktor]",
      "description": "[Penjelasan mendalam dan objektif mengenai faktor ini]"
    }
  ],
  "analysis": "[Analisis detail dalam beberapa paragraf, termasuk implikasi sektoral secara berimbang]"
}`

	user := fmt.Sprintf("Analisis harga BBM di Indonesia untuk periode %s.\n\nData Harga BBM:\n%s\n\nIndikator Makroekonomi Domestik (Konteks):\n%s\n\nBerikan analisis yang objektif dan berimbang dalam bahasa Indonesia sesuai format JSON yang ditentukan.", period, dataJSON, macroContext)

	resp, err := c.ChatCompletion(system, user)
	if err != nil {
		return nil, err
	}

	cleaned := extractJSON(resp)
	var result LLMResponse
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM JSON: %w (raw cleaned: '%s', full resp: '%s')", err, cleaned, resp[:min(200, len(resp))])
	}
	return &result, nil
}

func (c *Client) AnalyzeGoldPrice(dataJSON, period, macroContext string) (*LLMResponse, error) {
	system := `Anda adalah POV AI, analis senior pasar komoditas mulia dan makroekonomi terkemuka di Indonesia. Tugas Anda adalah menganalisis pergerakan harga Emas (Antam, Pegadaian, UBS, Spot XAU/IDR) secara profesional, objektif, dan bebas dari bias atau klaim finansial spekulatif.

PANDUAN ANALISIS (MENCEGAH BIAS & MENJAGA TRANSPARANSI):
1. Penggerak Utama Pasar Emas: Jelaskan korelasi harga emas dengan suku bunga global, pergerakan Dolar AS (DXY), inflasi, ketidakpastian geopolitik, dan cadangan emas Bank Sentral.
2. Pembedaan Jenis & Spread Emas: Jelaskan perbedaan harga fisik Antam/UBS dengan harga spot XAU/IDR, serta pertimbangan spread harga buyback.
3. Hubungkan dengan Ekonomi Nasional: Hubungkan tren emas dengan tingkat inflasi domestik BPS dan suku bunga BI.
4. Format Output: Anda WAJIB merespons HANYA dalam format JSON yang valid. Jika Anda menggunakan pemikiran internal (<think>), buat pemikiran singkat dan langsung keluarkan JSON.

Format JSON:
{
  "title": "[Judul Analisis Emas yang Informatif dan Netral]",
  "summary": "[Ringkasan Analisis Emas dalam 2-3 kalimat]",
  "factors": [
    {
      "title": "[Nama Faktor]",
      "description": "[Penjelasan mendalam dan objektif mengenai faktor ini]"
    }
  ],
  "analysis": "[Analisis detail dalam beberapa paragraf, termasuk pertimbangan investasi & spread buyback]"
}`

	user := fmt.Sprintf("Analisis pergerakan harga emas di Indonesia untuk periode %s.\n\nData Harga Emas:\n%s\n\nIndikator Makroekonomi Domestik (Konteks):\n%s\n\nBerikan analisis yang objektif dan berimbang dalam bahasa Indonesia sesuai format JSON yang ditentukan.", period, dataJSON, macroContext)

	resp, err := c.ChatCompletion(system, user)
	if err != nil {
		return nil, err
	}

	cleaned := extractJSON(resp)
	var result LLMResponse
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM JSON: %w (raw cleaned: '%s', full resp: '%s')", err, cleaned, resp[:min(200, len(resp))])
	}
	return &result, nil
}

// extractJSON tries to find a JSON object in a string (handles thinking tokens and markdown code blocks).
func extractJSON(s string) string {
	// Strip <think> ... </think> blocks if present
	for {
		start := strings.Index(s, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(s, "</think>")
		if end != -1 && end > start {
			s = s[:start] + s[end+8:]
		} else {
			// Unterminated <think>: just remove the "<think>" tag itself
			s = s[:start] + s[start+7:]
			break
		}
	}

	// Handle markdown code blocks ```json ... ``` (even if unclosed)
	if idx := strings.Index(s, "```json"); idx >= 0 {
		sub := s[idx+7:]
		if end := strings.Index(sub, "```"); end >= 0 {
			sub = sub[:end]
		}
		s = sub
	} else if idx := strings.Index(s, "```"); idx >= 0 {
		sub := s[idx+3:]
		if end := strings.Index(sub, "```"); end >= 0 {
			sub = sub[:end]
		}
		s = sub
	}

	// Find first { to last }
	if idx := strings.Index(s, "{"); idx >= 0 {
		if end := strings.LastIndex(s, "}"); end >= idx {
			return strings.TrimSpace(s[idx : end+1])
		}
	}
	return strings.TrimSpace(s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

