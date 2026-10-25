package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TokenCounterClient is a client for the odotoken token counter API.
type TokenCounterClient struct {
	baseURL string
	client  *http.Client
}

func NewTokenCounterClient(baseURL string) *TokenCounterClient {
	return &TokenCounterClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *TokenCounterClient) do(method, path string, body any) ([]byte, error) {
	var buf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, buf)
	if err != nil {
		return nil, err
	}
	if buf != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: status %d", method, path, resp.StatusCode)
	}

	var out bytes.Buffer
	if _, err := out.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type CountTextResponse struct {
	TotalTokens int    `json:"total_tokens"`
	Method      string `json:"method"`
}

func (c *TokenCounterClient) CountText(model, text, provider string) (map[string]any, error) {
	payload := map[string]any{"model": model, "text": text}
	if provider != "" {
		payload["provider"] = provider
	}
	data, err := c.do("POST", "/v1/count", payload)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

type CountMessagesRequest struct {
	Model    string           `json:"model"`
	Messages []map[string]any `json:"messages"`
	Provider string           `json:"provider,omitempty"`
	Tools    []map[string]any `json:"tools,omitempty"`
}

func (c *TokenCounterClient) CountMessages(model string, messages []map[string]any, provider string, tools []map[string]any) (map[string]any, error) {
	req := CountMessagesRequest{
		Model:    model,
		Messages: messages,
		Provider: provider,
		Tools:    tools,
	}
	data, err := c.do("POST", "/v1/count", req)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

type CountBatchRequest struct {
	Items []map[string]any `json:"items"`
}

type CountBatchResponse struct {
	TotalItems      int              `json:"total_items"`
	SuccessfulItems int              `json:"successful_items"`
	Results         []map[string]any `json:"results"`
}

func (c *TokenCounterClient) CountBatch(items []map[string]any) (*CountBatchResponse, error) {
	data, err := c.do("POST", "/v1/count/batch", CountBatchRequest{Items: items})
	if err != nil {
		return nil, err
	}
	var v CountBatchResponse
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

type ListModelsResponse struct {
	Total int `json:"total"`
}

func (c *TokenCounterClient) ListModels(provider string) (map[string]any, error) {
	path := "/v1/models"
	if provider != "" {
		path += "?provider=" + provider
	}
	data, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func (c *TokenCounterClient) GetModelInfo(modelID string) (map[string]any, error) {
	data, err := c.do("GET", "/v1/models/"+modelID, nil)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func (c *TokenCounterClient) Health() (map[string]any, error) {
	data, err := c.do("GET", "/health", nil)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func (c *TokenCounterClient) Ready() (map[string]any, error) {
	data, err := c.do("GET", "/ready", nil)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func (c *TokenCounterClient) Metrics() (map[string]any, error) {
	data, err := c.do("GET", "/metrics", nil)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func main() {
	client := NewTokenCounterClient("http://127.0.0.1:8081")

	fmt.Println("=== Health Check ===")
	health, err := client.Health()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Status:", health["status"])
	fmt.Println("Version:", health["version"])

	fmt.Println("\n=== List Models ===")
	models, err := client.ListModels("")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Total models:", models["total"])

	fmt.Println("\n=== Count Text ===")
	result, err := client.CountText("gpt-4o-mini", "Hello world!", "")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Tokens: %v (method: %v)\n", result["total_tokens"], result["method"])

	fmt.Println("\n=== Count Chat ===")
	messages := []map[string]any{
		{"role": "system", "content": "You are helpful"},
		{"role": "user", "content": "What is Python?"},
	}
	result, err = client.CountMessages("gpt-4o-mini", messages, "", nil)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Tokens: %v (overhead: %v)\n", result["total_tokens"], result["overhead_tokens"])

	fmt.Println("\n=== Batch Count ===")
	batch := []map[string]any{
		{"model": "gpt-4o-mini", "text": "Test 1"},
		{"model": "gpt-4o-mini", "text": "Test 2"},
	}
	batchResult, err := client.CountBatch(batch)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Total items: %d, Successful: %d\n", batchResult.TotalItems, batchResult.SuccessfulItems)
	for i, r := range batchResult.Results {
		fmt.Printf("  Item %d: %v tokens (%v)\n", i, r["total_tokens"], r["method"])
	}
}
