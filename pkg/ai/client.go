package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiKey      string
	baseURL     string
	model       string
	temperature float64
	maxTokens   int
	httpClient  *http.Client
	referer     string
	title       string
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
		maxTokens = 2048
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
	return &Client{
		apiKey:      apiKey,
		baseURL:     baseURL,
		model:       model,
		temperature: temperature,
		maxTokens:   maxTokens,
		httpClient:  &http.Client{Timeout: time.Duration(httpTimeout) * time.Second},
		referer:     referer,
		title:       title,
	}
}

func (c *Client) IsEnabled() bool { return c.apiKey != "" }

func (c *Client) ChatCompletion(systemPrompt, userPrompt string) (string, error) {
	if !c.IsEnabled() {
		return "", fmt.Errorf("AI not enabled")
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": c.model,
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
4. Format Output: Anda WAJIB merespons HANYA dalam format JSON yang valid seperti contoh berikut. Jangan menyertakan teks pembuka atau penutup di luar blok JSON.
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
		return nil, fmt.Errorf("failed to parse LLM JSON: %w (raw: %s)", err, resp[:min(100, len(resp))])
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
5. Format Output: Anda WAJIB merespons HANYA dalam format JSON yang valid seperti contoh berikut. Jangan menyertakan teks pembuka atau penutup di luar blok JSON.
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
		return nil, fmt.Errorf("failed to parse LLM JSON: %w (raw: %s)", err, resp[:min(100, len(resp))])
	}
	return &result, nil
}

func (c *Client) AnalyzeGoldPrice(dataJSON, period, macroContext string) (*LLMResponse, error) {
	system := `Anda adalah POV AI, analis senior pasar komoditas mulia dan makroekonomi terkemuka di Indonesia. Tugas Anda adalah menganalisis pergerakan harga Emas (Antam, Pegadaian, UBS, Spot XAU/IDR) secara profesional, objektif, dan bebas dari bias atau klaim finansial spekulatif.

PANDUAN ANALISIS (MENCEGAH BIAS & MENJAGA TRANSPARANSI):
1. Penggerak Utama Pasar Emas: Jelaskan korelasi harga emas dengan:
   - Suku Bunga Global (The Fed & BI): Suku bunga acuan yang tinggi menaikkan yield obligasi dan menekan harga emas (non-yielding asset), sebaliknya pemangkasan suku bunga meningkatkan daya tarik emas.
   - Pergerakan Dolar AS (DXY) & Nilai Tukar USD/IDR: Emas dunia dihargai dalam USD. Pelemahan Rupiah membuat harga emas dalam Rupiah (IDR) tetap tinggi atau bahkan naik meskipun emas dunia cenderung datar.
   - Inflasi & Hedge Nilai Mata Uang: Fungsi emas sebagai pelindung nilai (inflation hedge) terhadap penurunan daya beli mata uang fiat.
   - Ketidakpastian Geopolitik & Safe Haven: Permintaan aset aman (safe haven) saat konflik internasional atau krisis keuangan global.
   - Pembelian Cadangan Emas Bank Sentral: Akumulasi emas oleh Bank Sentral (PBoC, BI, dll).
2. Pembedaan Jenis & Spread Emas: Jelaskan perbedaan harga fisik Antam/UBS dengan harga spot XAU/IDR, serta pertimbangan spread harga buyback (jual kembali).
3. Hubungkan dengan Ekonomi Nasional: Hubungkan tren emas dengan tingkat inflasi domestik BPS dan suku bunga BI.
4. Format Output: Anda WAJIB merespons HANYA dalam format JSON yang valid seperti contoh berikut. Jangan menyertakan teks pembuka atau penutup di luar blok JSON.
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
		return nil, fmt.Errorf("failed to parse LLM JSON: %w (raw: %s)", err, resp[:min(100, len(resp))])
	}
	return &result, nil
}

// extractJSON tries to find a JSON object in a string (handles markdown code blocks).
func extractJSON(s string) string {
	// Strip markdown code blocks
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+7:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
		return strings.TrimSpace(s)
	}
	if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
		return strings.TrimSpace(s)
	}
	// Find first { to last }
	if idx := strings.Index(s, "{"); idx >= 0 {
		if end := strings.LastIndex(s, "}"); end >= idx {
			return s[idx : end+1]
		}
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
