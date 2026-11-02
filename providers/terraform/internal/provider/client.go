package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

func NewClient(endpoint, apiKey string) *Client {
	return &Client{
		endpoint: endpoint,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("erro ao serializar request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, reqBody)
	if err != nil {
		return fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("erro na requisição HTTP: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("erro ao desserializar resposta: %w", err)
		}
	}

	return nil
}

// Modelos de dados para a API do OdoToken

type ClientKey struct {
	ID          string            `json:"id,omitempty"`
	Name        string            `json:"name"`
	Key         string            `json:"key,omitempty"`
	Scopes      []string          `json:"scopes,omitempty"`
	RateLimit   int               `json:"rate_limit,omitempty"`
	BudgetLimit float64           `json:"budget_limit,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   string            `json:"created_at,omitempty"`
	UpdatedAt   string            `json:"updated_at,omitempty"`
}

type PriceRule struct {
	ID           string  `json:"id,omitempty"`
	ModelPattern string  `json:"model_pattern"`
	ClientID     string  `json:"client_id,omitempty"`
	InputPrice   float64 `json:"input_price"`
	OutputPrice  float64 `json:"output_price"`
	Currency     string  `json:"currency,omitempty"`
	Priority     int     `json:"priority,omitempty"`
	CreatedAt    string  `json:"created_at,omitempty"`
	UpdatedAt    string  `json:"updated_at,omitempty"`
}

type GuardPolicy struct {
	ID              string   `json:"id,omitempty"`
	Name            string   `json:"name"`
	ClientIDs       []string `json:"client_ids,omitempty"`
	BlockPatterns   []string `json:"block_patterns,omitempty"`
	AllowPatterns   []string `json:"allow_patterns,omitempty"`
	MaxTokens       int      `json:"max_tokens,omitempty"`
	MaxRequestsPerMin int    `json:"max_requests_per_min,omitempty"`
	Enabled         bool     `json:"enabled"`
	CreatedAt       string   `json:"created_at,omitempty"`
	UpdatedAt       string   `json:"updated_at,omitempty"`
}

type HealthStatus struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	DBStatus  string `json:"db_status"`
}

// Métodos da API

func (c *Client) CreateClientKey(ctx context.Context, key *ClientKey) (*ClientKey, error) {
	var result ClientKey
	if err := c.doRequest(ctx, http.MethodPost, "/api/client-keys", key, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetClientKey(ctx context.Context, id string) (*ClientKey, error) {
	var result ClientKey
	if err := c.doRequest(ctx, http.MethodGet, "/api/client-keys/"+id, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateClientKey(ctx context.Context, id string, key *ClientKey) (*ClientKey, error) {
	var result ClientKey
	if err := c.doRequest(ctx, http.MethodPut, "/api/client-keys/"+id, key, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteClientKey(ctx context.Context, id string) error {
	return c.doRequest(ctx, http.MethodDelete, "/api/client-keys/"+id, nil, nil)
}

func (c *Client) ListClientKeys(ctx context.Context) ([]ClientKey, error) {
	var result []ClientKey
	if err := c.doRequest(ctx, http.MethodGet, "/api/client-keys", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) CreatePriceRule(ctx context.Context, rule *PriceRule) (*PriceRule, error) {
	var result PriceRule
	if err := c.doRequest(ctx, http.MethodPost, "/api/prices", rule, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetPriceRule(ctx context.Context, id string) (*PriceRule, error) {
	var result PriceRule
	if err := c.doRequest(ctx, http.MethodGet, "/api/prices/"+id, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdatePriceRule(ctx context.Context, id string, rule *PriceRule) (*PriceRule, error) {
	var result PriceRule
	if err := c.doRequest(ctx, http.MethodPut, "/api/prices/"+id, rule, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeletePriceRule(ctx context.Context, id string) error {
	return c.doRequest(ctx, http.MethodDelete, "/api/prices/"+id, nil, nil)
}

func (c *Client) CreateGuardPolicy(ctx context.Context, policy *GuardPolicy) (*GuardPolicy, error) {
	var result GuardPolicy
	if err := c.doRequest(ctx, http.MethodPost, "/api/guard/policies", policy, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetGuardPolicy(ctx context.Context, id string) (*GuardPolicy, error) {
	var result GuardPolicy
	if err := c.doRequest(ctx, http.MethodGet, "/api/guard/policies/"+id, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateGuardPolicy(ctx context.Context, id string, policy *GuardPolicy) (*GuardPolicy, error) {
	var result GuardPolicy
	if err := c.doRequest(ctx, http.MethodPut, "/api/guard/policies/"+id, policy, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteGuardPolicy(ctx context.Context, id string) error {
	return c.doRequest(ctx, http.MethodDelete, "/api/guard/policies/"+id, nil, nil)
}

func (c *Client) GetHealth(ctx context.Context) (*HealthStatus, error) {
	var result HealthStatus
	if err := c.doRequest(ctx, http.MethodGet, "/health", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type ModelsResponse struct {
	Models []string `json:"models"`
}

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	var result ModelsResponse
	if err := c.doRequest(ctx, http.MethodGet, "/api/models", nil, &result); err != nil {
		return nil, err
	}
	return result.Models, nil
}