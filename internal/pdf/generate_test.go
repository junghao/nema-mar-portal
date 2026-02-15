package pdf

import (
	"testing"
	"time"

	"github.com/GeoNet/nema-mar-portal/internal/fastschema"
)

func TestGenerateEATPDF(t *testing.T) {
	eat := &fastschema.EAT{
		ID:                1,
		EventTitle:        "M5.0-Wellington-2026-01-15",
		Location:          "Wellington",
		EventDate:         time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Magnitude:         5.0,
		EarthquakeURL:     "https://www.geonet.org.nz/earthquake/2026p001",
		Version:           1,
		EventComments:     "Test earthquake event.\nMultiple lines of comments.",
		BeachMarineThreat: true,
		LandThreat:        false,
		Status:            "preliminary",
		TEPActivated:      true,
		Attachments: []fastschema.File{
			{Name: "map.png", Type: "image/png"},
			{Name: "report.pdf", Type: "application/pdf"},
		},
	}

	pdfBytes, err := GenerateEATPDF(eat)
	if err != nil {
		t.Fatalf("GenerateEATPDF failed: %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Fatal("expected non-empty PDF bytes")
	}

	// Check PDF header
	if len(pdfBytes) < 4 || string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:min(10, len(pdfBytes))]))
	}
}

func TestGenerateEATPDF_MinimalData(t *testing.T) {
	eat := &fastschema.EAT{
		EventTitle: "M3.0-Test-2026-01-01",
		Location:   "Test",
		Magnitude:  3.0,
		Version:    1,
		Status:     "confirmed",
	}

	pdfBytes, err := GenerateEATPDF(eat)
	if err != nil {
		t.Fatalf("GenerateEATPDF failed: %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Fatal("expected non-empty PDF bytes")
	}
}
