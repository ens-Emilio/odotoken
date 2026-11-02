package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientKeyResource_CRUD(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/client-keys":
			var req ClientKey
			json.NewDecoder(r.Body).Decode(&req)
			resp := ClientKey{
				ID:        "ck-123",
				Name:      req.Name,
				Key:       "sk-test-key",
				CreatedAt: "2026-01-01T00:00:00Z",
				UpdatedAt: "2026-01-01T00:00:00Z",
			}
			json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodGet && r.URL.Path == "/api/client-keys/ck-123":
			resp := ClientKey{
				ID:        "ck-123",
				Name:      "test-key",
				CreatedAt: "2026-01-01T00:00:00Z",
				UpdatedAt: "2026-01-01T00:00:00Z",
			}
			json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodPut && r.URL.Path == "/api/client-keys/ck-123":
			resp := ClientKey{
				ID:        "ck-123",
				Name:      "test-key-updated",
				CreatedAt: "2026-01-01T00:00:00Z",
				UpdatedAt: "2026-01-02T00:00:00Z",
			}
			json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/client-keys/ck-123":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	ctx := context.Background()

	// Test Create
	key := &ClientKey{Name: "test-key"}
	created, err := client.CreateClientKey(ctx, key)
	if err != nil {
		t.Fatalf("CreateClientKey failed: %v", err)
	}
	if created.ID != "ck-123" {
		t.Errorf("expected ID ck-123, got %s", created.ID)
	}
	if created.Key != "sk-test-key" {
		t.Errorf("expected Key sk-test-key, got %s", created.Key)
	}

	// Test Read
	read, err := client.GetClientKey(ctx, "ck-123")
	if err != nil {
		t.Fatalf("GetClientKey failed: %v", err)
	}
	if read.Name != "test-key" {
		t.Errorf("expected Name test-key, got %s", read.Name)
	}

	// Test Update
	updated, err := client.UpdateClientKey(ctx, "ck-123", &ClientKey{Name: "test-key-updated"})
	if err != nil {
		t.Fatalf("UpdateClientKey failed: %v", err)
	}
	if updated.UpdatedAt != "2026-01-02T00:00:00Z" {
		t.Errorf("expected UpdatedAt 2026-01-02T00:00:00Z, got %s", updated.UpdatedAt)
	}

	// Test Delete
	err = client.DeleteClientKey(ctx, "ck-123")
	if err != nil {
		t.Fatalf("DeleteClientKey failed: %v", err)
	}
}

func TestClientKeyResource_ListAndHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/client-keys":
			resp := []ClientKey{
				{ID: "ck-1", Name: "key-1", CreatedAt: "2026-01-01T00:00:00Z"},
				{ID: "ck-2", Name: "key-2", CreatedAt: "2026-01-02T00:00:00Z"},
			}
			json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			resp := HealthStatus{
				Status:   "ok",
				Version:  "0.12.0",
				Uptime:   "1h30m",
				DBStatus: "connected",
			}
			json.NewEncoder(w).Encode(resp)
case r.Method == http.MethodGet && r.URL.Path == "/api/models":
			resp := ModelsResponse{Models: []string{"gpt-4o", "claude-3.5-sonnet"}}
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	ctx := context.Background()

	// Test ListClientKeys
	keys, err := client.ListClientKeys(ctx)
	if err != nil {
		t.Fatalf("ListClientKeys failed: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}

	// Test GetHealth
	health, err := client.GetHealth(ctx)
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}
	if health.Status != "ok" {
		t.Errorf("expected status ok, got %s", health.Status)
	}
	if health.Version != "0.12.0" {
		t.Errorf("expected version 0.12.0, got %s", health.Version)
	}

	// Test ListModels
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}