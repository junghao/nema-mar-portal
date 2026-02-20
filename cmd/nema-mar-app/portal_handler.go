package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/GeoNet/kit/weft"
	"github.com/GeoNet/nema-mar-portal/internal/fastschema"
)

// portalPageHandler serves the EAT editor page.
func portalPageHandler(r *http.Request, h http.Header, b *bytes.Buffer, nonce string) error {
	if err := weft.CheckQuery(r, []string{"GET"}, []string{}, []string{}); err != nil {
		return err
	}

	page := Page{Nonce: nonce}

	// Load recent events for dropdown
	events, err := fsClient.ListDistinctEvents(7)
	if err != nil {
		// Non-fatal: show empty dropdown
		events = nil
	}
	page.Events = events

	h.Set("Content-Type", "text/html; charset=utf-8")
	return editorTemplate.ExecuteTemplate(b, "base", page)
}

// portalPreviewHandler renders a dashboard-like preview from form data.
func portalPreviewHandler(r *http.Request, h http.Header, b *bytes.Buffer, nonce string) error {
	if err := weft.CheckQuery(r, []string{"POST"}, []string{}, []string{}); err != nil {
		return err
	}

	if err := r.ParseForm(); err != nil {
		return weft.StatusError{Code: http.StatusBadRequest, Err: err}
	}

	mag, _ := strconv.ParseFloat(r.FormValue("magnitude"), 32)
	eventDate, _ := time.Parse("2006-01-02T15:04", r.FormValue("event_date"))

	eat := &fastschema.EAT{
		Location:          r.FormValue("location"),
		EventDate:         eventDate,
		Magnitude:         float32(mag),
		EarthquakeURL:     r.FormValue("earthquake_url"),
		EventComments:     r.FormValue("event_comments"),
		BeachMarineThreat: r.FormValue("beach_marine_threat") == "on",
		LandThreat:        r.FormValue("land_threat") == "on",
		Status:            r.FormValue("status"),
		TEPActivated:      r.FormValue("tep_activated") == "on",
	}

	eat.EventTitle = fastschema.FormatEventTitle(eat.Magnitude, eat.Location, eat.EventDate)

	// Parse uploaded file references if present
	if filesJSON := r.FormValue("uploaded_files"); filesJSON != "" {
		var files []fastschema.File
		if err := json.Unmarshal([]byte(filesJSON), &files); err == nil {
			eat.Attachments = files
		}
	}

	page := Page{
		Nonce:      nonce,
		CurrentEAT: eat,
	}

	h.Set("Content-Type", "text/html; charset=utf-8")
	return previewTemplate.ExecuteTemplate(b, "base", page)
}
