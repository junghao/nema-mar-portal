package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/GeoNet/kit/weft"
	"github.com/GeoNet/nema-mar-portal/internal/email"
	"github.com/GeoNet/nema-mar-portal/internal/fastschema"
	"github.com/GeoNet/nema-mar-portal/internal/pdf"
	"github.com/GeoNet/nema-mar-portal/internal/valid"
)

// apiEventsHandler returns a JSON list of distinct event titles from the last 7 days.
func apiEventsHandler(r *http.Request, h http.Header, b *bytes.Buffer) error {
	if _, err := weft.CheckQueryValid(r, []string{"GET"}, []string{}, []string{"days"}, valid.Query); err != nil {
		return err
	}

	days := 7
	events, err := fsClient.ListDistinctEvents(days)
	if err != nil {
		return weft.StatusError{Code: http.StatusInternalServerError, Err: err}
	}

	h.Set("Content-Type", "application/json")
	return json.NewEncoder(b).Encode(events)
}

// apiEATHandler returns a single EAT as JSON.
func apiEATHandler(r *http.Request, h http.Header, b *bytes.Buffer) error {
	q, err := weft.CheckQueryValid(r, []string{"GET"}, []string{}, []string{"id", "event_title"}, valid.Query)
	if err != nil {
		return err
	}

	var eat *fastschema.EAT

	if idStr := q.Get("id"); idStr != "" {
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		eat, err = fsClient.GetEAT(id)
	} else if title := q.Get("event_title"); title != "" {
		eat, err = fsClient.GetLatestVersion(title)
	} else {
		return weft.StatusError{Code: http.StatusBadRequest, Err: errors.New("id or event_title required")}
	}

	if err != nil {
		return weft.StatusError{Code: http.StatusInternalServerError, Err: err}
	}
	if eat == nil {
		return weft.StatusError{Code: http.StatusNotFound, Err: errors.New("EAT not found")}
	}

	h.Set("Content-Type", "application/json")
	return json.NewEncoder(b).Encode(eat)
}

// publishRequest is the JSON payload for the publish endpoint.
type publishRequest struct {
	Mode              string            `json:"mode"` // "new_event" or "new_version"
	Location          string            `json:"location"`
	EventDate         string            `json:"event_date"` // ISO datetime-local format
	Magnitude         float32           `json:"magnitude"`
	EarthquakeURL     string            `json:"earthquake_url"`
	EventComments     string            `json:"event_comments"`
	BeachMarineThreat bool              `json:"beach_marine_threat"`
	LandThreat        bool              `json:"land_threat"`
	Status            string            `json:"status"`
	TEPActivated      bool              `json:"tep_activated"`
	Attachments       []fastschema.File `json:"attachments"`
	ExistingEATID     int               `json:"existing_eat_id"`
}

// publishResponse is the JSON response from the publish endpoint.
type publishResponse struct {
	Success bool   `json:"success"`
	ID      int    `json:"id,omitempty"`
	Version int    `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// apiPublishHandler saves an EAT, generates a PDF, and sends email.
func apiPublishHandler(r *http.Request, h http.Header, b *bytes.Buffer) error {
	if err := weft.CheckQuery(r, []string{"POST"}, []string{}, []string{}); err != nil {
		return err
	}

	var req publishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return weft.StatusError{Code: http.StatusBadRequest, Err: fmt.Errorf("invalid JSON: %w", err)}
	}

	// Validate required fields
	if req.Location == "" {
		return writePublishError(b, h, "location is required")
	}
	if req.EventDate == "" {
		return writePublishError(b, h, "event_date is required")
	}
	if req.Status != "preliminary" && req.Status != "confirmed" {
		return writePublishError(b, h, "status must be 'preliminary' or 'confirmed'")
	}

	eventDate, err := time.Parse("2006-01-02T15:04", req.EventDate)
	if err != nil {
		return writePublishError(b, h, "invalid event_date format")
	}

	eat := &fastschema.EAT{
		Location:          req.Location,
		EventDate:         eventDate,
		Magnitude:         req.Magnitude,
		EarthquakeURL:     req.EarthquakeURL,
		EventComments:     req.EventComments,
		BeachMarineThreat: req.BeachMarineThreat,
		LandThreat:        req.LandThreat,
		Status:            req.Status,
		TEPActivated:      req.TEPActivated,
		Attachments:       req.Attachments,
	}

	if req.Mode == "new_event" {
		eat.EventTitle = fastschema.FormatEventTitle(eat.Magnitude, eat.Location, eat.EventDate)
		eat.Version = 1
	} else {
		// New version: look up existing event title and increment version
		if req.ExistingEATID > 0 {
			existing, err := fsClient.GetEAT(req.ExistingEATID)
			if err != nil {
				return writePublishError(b, h, "failed to look up existing EAT")
			}
			eat.EventTitle = existing.EventTitle
		} else {
			eat.EventTitle = fastschema.FormatEventTitle(eat.Magnitude, eat.Location, eat.EventDate)
		}

		latest, err := fsClient.GetLatestVersion(eat.EventTitle)
		if err != nil {
			return writePublishError(b, h, "failed to look up latest version")
		}
		if latest != nil {
			eat.Version = latest.Version + 1
		} else {
			eat.Version = 1
		}
	}

	// Save to FastSchema
	created, err := fsClient.CreateEAT(eat)
	if err != nil {
		return writePublishError(b, h, "failed to save EAT: "+err.Error())
	}

	// Generate PDF
	pdfBytes, err := pdf.GenerateEATPDF(created)
	if err != nil {
		log.Printf("warning: PDF generation failed: %v", err)
	}

	// Send email
	emailCfg, err := email.ConfigFromEnv()
	if err != nil {
		log.Printf("warning: email not configured: %v", err)
	} else if pdfBytes != nil {
		if err := email.SendEATEmail(emailCfg, created, pdfBytes); err != nil {
			log.Printf("warning: email send failed: %v", err)
		}
	}

	h.Set("Content-Type", "application/json")
	return json.NewEncoder(b).Encode(publishResponse{
		Success: true,
		ID:      created.ID,
		Version: created.Version,
	})
}

func writePublishError(b *bytes.Buffer, h http.Header, msg string) error {
	h.Set("Content-Type", "application/json")
	return json.NewEncoder(b).Encode(publishResponse{Error: msg})
}

// apiUploadHandler handles file uploads and stores them via FastSchema.
func apiUploadHandler(r *http.Request, w http.ResponseWriter) (int64, error) {
	if r.Method != http.MethodPost {
		return 0, weft.StatusError{Code: http.StatusMethodNotAllowed}
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		return 0, weft.StatusError{Code: http.StatusBadRequest, Err: err}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return 0, weft.StatusError{Code: http.StatusBadRequest, Err: err}
	}
	defer file.Close()

	uploaded, err := fsClient.UploadFile(header.Filename, file)
	if err != nil {
		return 0, weft.StatusError{Code: http.StatusInternalServerError, Err: err}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	respBytes, err := json.Marshal(uploaded)
	if err != nil {
		return 0, err
	}

	n, err := w.Write(respBytes)
	return int64(n), err
}
