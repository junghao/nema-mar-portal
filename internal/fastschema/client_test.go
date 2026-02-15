package fastschema

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/login" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var req LoginRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Login != "admin" || req.Password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(LoginResponse{
			Data: struct {
				Token string `json:"token"`
			}{Token: "test-jwt-token"},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.Login("admin", "secret")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if c.token != "test-jwt-token" {
		t.Errorf("expected token 'test-jwt-token', got '%s'", c.token)
	}
}

func TestListEATs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		resp := ListResponse{
			Data: ListData{
				Total:       2,
				PerPage:     100,
				CurrentPage: 1,
				LastPage:    1,
				Items: []EAT{
					{ID: 1, EventTitle: "M5.0-Wellington-2026-01-01", Version: 1},
					{ID: 2, EventTitle: "M5.0-Wellington-2026-01-01", Version: 2},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	eats, err := c.ListEATs(time.Now().AddDate(0, 0, -7))
	if err != nil {
		t.Fatalf("ListEATs failed: %v", err)
	}
	if len(eats) != 2 {
		t.Errorf("expected 2 EATs, got %d", len(eats))
	}
}

func TestGetEAT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/content/eat/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := SingleResponse{
			Data: EAT{ID: 42, EventTitle: "M6.0-Kaikoura-2026-02-01", Version: 1},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	eat, err := c.GetEAT(42)
	if err != nil {
		t.Fatalf("GetEAT failed: %v", err)
	}
	if eat.ID != 42 {
		t.Errorf("expected ID 42, got %d", eat.ID)
	}
	if eat.EventTitle != "M6.0-Kaikoura-2026-02-01" {
		t.Errorf("unexpected event title: %s", eat.EventTitle)
	}
}

func TestGetLatestVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ListResponse{
			Data: ListData{
				Total: 1,
				Items: []EAT{
					{ID: 5, EventTitle: "M5.0-Wellington-2026-01-01", Version: 3},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	eat, err := c.GetLatestVersion("M5.0-Wellington-2026-01-01")
	if err != nil {
		t.Fatalf("GetLatestVersion failed: %v", err)
	}
	if eat == nil {
		t.Fatal("expected non-nil EAT")
	}
	if eat.Version != 3 {
		t.Errorf("expected version 3, got %d", eat.Version)
	}
}

func TestListDistinctEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ListResponse{
			Data: ListData{
				Total: 3,
				Items: []EAT{
					{EventTitle: "M5.0-Wellington-2026-01-01"},
					{EventTitle: "M5.0-Wellington-2026-01-01"},
					{EventTitle: "M6.0-Kaikoura-2026-01-02"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	events, err := c.ListDistinctEvents(7)
	if err != nil {
		t.Fatalf("ListDistinctEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 distinct events, got %d", len(events))
	}
}

func TestCreateEAT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var eat EAT
		json.NewDecoder(r.Body).Decode(&eat)

		eat.ID = 99
		eat.CreatedAt = time.Now()
		resp := SingleResponse{Data: eat}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL)
	c.token = "test-token"

	eat := &EAT{
		EventTitle: "M5.0-Test-2026-01-01",
		Location:   "Test",
		Magnitude:  5.0,
		Version:    1,
		Status:     "preliminary",
	}

	created, err := c.CreateEAT(eat)
	if err != nil {
		t.Fatalf("CreateEAT failed: %v", err)
	}
	if created.ID != 99 {
		t.Errorf("expected ID 99, got %d", created.ID)
	}
}

func TestErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	_, err := c.GetEAT(1)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
