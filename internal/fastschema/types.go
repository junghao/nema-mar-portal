package fastschema

import (
	"fmt"
	"time"
)

// EAT represents an Emergency Advisory Text record.
type EAT struct {
	ID                int       `json:"id"`
	EventTitle        string    `json:"event_title"`
	Location          string    `json:"location"`
	EventDate         time.Time `json:"event_date"`
	Magnitude         float32   `json:"magnitude"`
	EarthquakeURL     string    `json:"earthquake_url"`
	Version           int       `json:"version"`
	EventComments     string    `json:"event_comments"`
	BeachMarineThreat bool      `json:"beach_marine_threat"`
	LandThreat        bool      `json:"land_threat"`
	Status            string    `json:"status"` // "preliminary" or "confirmed"
	TEPActivated      bool      `json:"tep_activated"`
	Attachments       []File    `json:"attachments,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// File represents an attachment stored in FastSchema's object store.
type File struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	URL  string `json:"url,omitempty"`
	Size int64  `json:"size,omitempty"`
	Type string `json:"type,omitempty"` // MIME type
}

// ListResponse wraps the paginated list response from FastSchema.
type ListResponse struct {
	Data ListData `json:"data"`
}

// ListData holds the pagination info and items.
type ListData struct {
	Total       int   `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
	Items       []EAT `json:"items"`
}

// SingleResponse wraps a single record response from FastSchema.
type SingleResponse struct {
	Data EAT `json:"data"`
}

// FileResponse wraps a file upload response from FastSchema.
type FileResponse struct {
	Data File `json:"data"`
}

// LoginRequest is the payload for FastSchema authentication.
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// LoginResponse wraps the authentication response.
type LoginResponse struct {
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}

// FormatEventTitle computes the event title from its components.
func FormatEventTitle(magnitude float32, location string, eventDate time.Time) string {
	return fmt.Sprintf("M%.1f-%s-%s", magnitude, location, eventDate.UTC().Format("2006-01-02"))
}
