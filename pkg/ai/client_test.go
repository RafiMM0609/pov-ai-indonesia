package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pure json",
			input:    `{"foo": "bar"}`,
			expected: `{"foo": "bar"}`,
		},
		{
			name:     "markdown json block",
			input:    "```json\n{\n  \"foo\": \"bar\"\n}\n```",
			expected: "{\n  \"foo\": \"bar\"\n}",
		},
		{
			name:     "markdown standard block",
			input:    "```\n{\n  \"foo\": \"bar\"\n}\n```",
			expected: "{\n  \"foo\": \"bar\"\n}",
		},
		{
			name:     "wrapped in text",
			input:    `Here is your JSON: {"foo": "bar"} Hope you like it!`,
			expected: `{"foo": "bar"}`,
		},
		{
			name:     "no JSON",
			input:    `plain text`,
			expected: `plain text`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.expected {
				t.Errorf("extractJSON() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsFreeModel(t *testing.T) {
	tests := []struct {
		name     string
		model    openRouterModel
		expected bool
	}{
		{
			name: "ID ends with :free",
			model: openRouterModel{
				ID: "google/gemini-2.0-flash-exp:free",
			},
			expected: true,
		},
		{
			name: "Pricing string zeros",
			model: openRouterModel{
				ID: "meta-llama/llama-3.3-70b-instruct",
				Pricing: struct {
					Prompt     interface{} `json:"prompt"`
					Completion interface{} `json:"completion"`
				}{
					Prompt:     "0",
					Completion: "0.0",
				},
			},
			expected: true,
		},
		{
			name: "Paid model",
			model: openRouterModel{
				ID: "openai/gpt-4o",
				Pricing: struct {
					Prompt     interface{} `json:"prompt"`
					Completion interface{} `json:"completion"`
				}{
					Prompt:     "0.0000025",
					Completion: "0.00001",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFreeModel(tt.model)
			if got != tt.expected {
				t.Errorf("isFreeModel(%s) = %v, want %v", tt.model.ID, got, tt.expected)
			}
		})
	}
}

func TestFetchTopFreeModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		resp := openRouterModelsResp{
			Data: []openRouterModel{
				{ID: "paid/model-1", Pricing: struct {
					Prompt     interface{} `json:"prompt"`
					Completion interface{} `json:"completion"`
				}{Prompt: "0.01", Completion: "0.02"}},
				{ID: "free/model-1:free"},
				{ID: "free/model-2", Pricing: struct {
					Prompt     interface{} `json:"prompt"`
					Completion interface{} `json:"completion"`
				}{Prompt: "0", Completion: "0"}},
				{ID: "free/model-3:free"},
				{ID: "free/model-4:free"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, "initial/model", 0.7, 100, 5, "", "")
	freeModels, err := client.FetchTopFreeModels(3)
	if err != nil {
		t.Fatalf("FetchTopFreeModels failed: %v", err)
	}

	expected := []string{"free/model-1:free", "free/model-2", "free/model-3:free"}
	if !reflect.DeepEqual(freeModels, expected) {
		t.Errorf("FetchTopFreeModels() = %v, want %v", freeModels, expected)
	}
}

func TestChatCompletionModelRotationAndOverride(t *testing.T) {
	attemptedModels := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			resp := openRouterModelsResp{
				Data: []openRouterModel{
					{ID: "new-free/model-1:free"},
					{ID: "new-free/model-2:free"},
					{ID: "new-free/model-3:free"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.URL.Path == "/chat/completions" {
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			model, _ := req["model"].(string)
			attemptedModels = append(attemptedModels, model)

			// Initial models fail
			if model == "old/model-1" || model == "old/model-2" || model == "old/model-3" {
				http.Error(w, `{"error":"model unavailable"}`, http.StatusBadRequest)
				return
			}

			// New model succeeds
			if model == "new-free/model-1:free" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"choices":[{"message":{"content":"success from new free model"}}]}`))
				return
			}
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, "old/model-1,old/model-2,old/model-3", 0.7, 100, 5, "", "")

	res, err := client.ChatCompletion("sys", "user")
	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}

	if res != "success from new free model" {
		t.Errorf("got response %q, want %q", res, "success from new free model")
	}

	expectedActive := []string{"new-free/model-1:free", "new-free/model-2:free", "new-free/model-3:free"}
	if !reflect.DeepEqual(client.GetActiveModels(), expectedActive) {
		t.Errorf("GetActiveModels() = %v, want %v", client.GetActiveModels(), expectedActive)
	}

	if client.GetCurrentModel() != "new-free/model-1:free" {
		t.Errorf("GetCurrentModel() = %q, want %q", client.GetCurrentModel(), "new-free/model-1:free")
	}
}
