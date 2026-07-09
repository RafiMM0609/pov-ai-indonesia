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

type LLMResponse struct {
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Factors  []string `json:"factors"`
	Analysis string   `json:"analysis"`
}

func (c *Client) AnalyzeExchangeRate(dataJSON, period string) (*LLMResponse, error) {
	system := `You are POV AI, an expert Indonesian economic analyst. Analyze USD/IDR exchange rate data and provide deep insights in Indonesian. Respond ONLY with valid JSON: {"title":"...","summary":"...","factors":["..."],"analysis":"..."}`
	user := fmt.Sprintf("Analyze USD/IDR for period %s. Data:\n%s\nRespond in Indonesian.", period, dataJSON)

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

func (c *Client) AnalyzeFuelPrice(dataJSON, period string) (*LLMResponse, error) {
	system := `You are POV AI, an expert Indonesian energy analyst. Analyze fuel price (BBM) data and provide deep insights in Indonesian. Respond ONLY with valid JSON: {"title":"...","summary":"...","factors":["..."],"analysis":"..."}`
	user := fmt.Sprintf("Analyze Indonesian fuel prices for period %s. Data:\n%s\nRespond in Indonesian.", period, dataJSON)

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
