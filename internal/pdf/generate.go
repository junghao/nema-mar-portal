package pdf

import (
	"bytes"
	"fmt"
	"time"

	"github.com/GeoNet/nema-mar-portal/internal/fastschema"
	"github.com/go-pdf/fpdf"
)

// GenerateEATPDF creates a PDF representation of an EAT.
func GenerateEATPDF(eat *fastschema.EAT) ([]byte, error) {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(15, 15, 15)
	p.AddPage()

	// Title
	p.SetFont("Helvetica", "B", 18)
	p.Cell(0, 10, "Emergency Advisory Text")
	p.Ln(15)

	// Event title
	p.SetFont("Helvetica", "B", 14)
	p.Cell(0, 8, eat.EventTitle)
	p.Ln(12)

	// Version and status
	p.SetFont("Helvetica", "", 11)
	p.Cell(0, 6, fmt.Sprintf("Version: %d | Status: %s", eat.Version, eat.Status))
	p.Ln(10)

	addField(p, "Location", eat.Location)
	addField(p, "Event Date (UTC)", eat.EventDate.UTC().Format(time.RFC3339))
	addField(p, "Magnitude", fmt.Sprintf("%.1f", eat.Magnitude))
	if eat.EarthquakeURL != "" {
		addField(p, "Earthquake URL", eat.EarthquakeURL)
	}
	addField(p, "Beach/Marine Threat", boolStr(eat.BeachMarineThreat))
	addField(p, "Land Threat", boolStr(eat.LandThreat))
	addField(p, "TEP Activated", boolStr(eat.TEPActivated))

	// Event comments
	if eat.EventComments != "" {
		p.Ln(5)
		p.SetFont("Helvetica", "B", 11)
		p.Cell(0, 6, "Event Comments:")
		p.Ln(7)
		p.SetFont("Helvetica", "", 10)
		p.MultiCell(0, 5, eat.EventComments, "", "L", false)
	}

	// Attachments list
	if len(eat.Attachments) > 0 {
		p.Ln(5)
		p.SetFont("Helvetica", "B", 11)
		p.Cell(0, 6, "Attachments:")
		p.Ln(7)
		p.SetFont("Helvetica", "", 10)
		for _, a := range eat.Attachments {
			p.Cell(0, 5, fmt.Sprintf("- %s (%s)", a.Name, a.Type))
			p.Ln(6)
		}
	}

	// Footer
	p.Ln(10)
	p.SetFont("Helvetica", "I", 8)
	p.Cell(0, 4, fmt.Sprintf("Generated: %s", time.Now().UTC().Format(time.RFC3339)))

	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}

	return buf.Bytes(), nil
}

func addField(p *fpdf.Fpdf, label, value string) {
	p.SetFont("Helvetica", "B", 10)
	p.Cell(50, 6, label+":")
	p.SetFont("Helvetica", "", 10)
	p.Cell(0, 6, value)
	p.Ln(7)
}

func boolStr(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
