package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GeoNet/kit/weft/wefttest"
	"github.com/GeoNet/nema-mar-portal/internal/fastschema"
)

var ts *httptest.Server

func TestMain(m *testing.M) {
	// Set up a mock FastSchema server
	mockFS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/login" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(fastschema.LoginResponse{
				Data: struct {
					Token string `json:"token"`
				}{Token: "test-token"},
			})

		case r.URL.Path == "/api/content/eat" && r.Method == http.MethodGet:
			resp := fastschema.ListResponse{
				Data: fastschema.ListData{
					Total: 1,
					Items: []fastschema.EAT{
						{
							ID:         1,
							EventTitle: "M5.0-Wellington-2026-01-01",
							Location:   "Wellington",
							EventDate:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
							Magnitude:  5.0,
							Version:    1,
							Status:     "preliminary",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/api/content/eat" && r.Method == http.MethodPost:
			var eat fastschema.EAT
			json.NewDecoder(r.Body).Decode(&eat)
			eat.ID = 99
			eat.CreatedAt = time.Now()
			resp := fastschema.SingleResponse{Data: eat}
			json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/api/schema" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	fsClient = fastschema.NewClient(mockFS.URL)

	// Load templates from the local templates directory
	templateDir := filepath.Join("templates")
	if err := loadTemplates(templateDir); err != nil {
		// Try relative from test execution directory
		templateDir = filepath.Join(".", "templates")
		if err := loadTemplates(templateDir); err != nil {
			panic("could not load templates: " + err.Error())
		}
	}

	ts = httptest.NewServer(mux)

	code := m.Run()

	ts.Close()
	mockFS.Close()
	os.Exit(code)
}

func TestRoutes(t *testing.T) {
	routes := wefttest.Requests{
		{ID: wefttest.L(), URL: "/soh/up"},
		{ID: wefttest.L(), URL: "/soh"},
		{ID: wefttest.L(), URL: "/gha-portal"},
		{ID: wefttest.L(), URL: "/dashboard"},
		{ID: wefttest.L(), URL: "/api/events", Content: "application/json"},
	}
	if err := routes.DoAll(ts.URL); err != nil {
		t.Error(err)
	}
}

func TestRoutes404(t *testing.T) {
	routes := wefttest.Requests{
		{ID: wefttest.L(), URL: "/nonexistent", Status: http.StatusNotFound},
	}
	if err := routes.DoAll(ts.URL); err != nil {
		t.Error(err)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	// /api/events uses TextError which returns text/plain for errors
	r := wefttest.Request{
		ID:      wefttest.L(),
		URL:     "/api/events",
		Content: "text/plain; charset=utf-8",
	}
	if _, err := r.MethodNotAllowed(ts.URL, []string{"GET"}); err != nil {
		t.Error(err)
	}
}
