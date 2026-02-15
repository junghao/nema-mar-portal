package main

import (
	"fmt"
	"html/template"
	"path/filepath"
	"time"

	"github.com/GeoNet/nema-mar-portal/internal/fastschema"
)

// Page holds data passed to HTML templates.
type Page struct {
	Nonce        string
	Events       []string         // distinct event titles for dropdown
	CurrentEAT   *fastschema.EAT  // current EAT for display/edit
	Versions     []fastschema.EAT // version history
	IsNewEvent   bool
	IsNewVersion bool
	Error        string
	Success      string
	HasNewBanner bool // show "new event/version" banner on dashboard
}

var (
	editorTemplate    *template.Template
	dashboardTemplate *template.Template
	previewTemplate   *template.Template
)

var funcMap = template.FuncMap{
	"formatDate": func(t time.Time) string {
		return t.UTC().Format("2006-01-02T15:04")
	},
	"formatDateDisplay": func(t time.Time) string {
		return t.UTC().Format("2006-01-02 15:04 UTC")
	},
	"formatMagnitude": func(m float32) string {
		return fmt.Sprintf("%.1f", m)
	},
	"boolYesNo": func(b bool) string {
		if b {
			return "Yes"
		}
		return "No"
	},
	"isImage": func(mimeType string) bool {
		switch mimeType {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
			return true
		}
		return false
	},
	"isPDF": func(mimeType string) bool {
		return mimeType == "application/pdf"
	},
}

func loadTemplates(dir string) error {
	base := filepath.Join(dir, "base.html")

	var err error

	editorTemplate, err = template.New("base.html").Funcs(funcMap).ParseFiles(base, filepath.Join(dir, "editor.html"))
	if err != nil {
		return fmt.Errorf("parsing editor template: %w", err)
	}

	dashboardTemplate, err = template.New("base.html").Funcs(funcMap).ParseFiles(base, filepath.Join(dir, "dashboard.html"))
	if err != nil {
		return fmt.Errorf("parsing dashboard template: %w", err)
	}

	previewTemplate, err = template.New("base.html").Funcs(funcMap).ParseFiles(base, filepath.Join(dir, "preview.html"))
	if err != nil {
		return fmt.Errorf("parsing preview template: %w", err)
	}

	return nil
}
