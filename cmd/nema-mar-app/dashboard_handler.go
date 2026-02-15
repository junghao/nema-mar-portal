package main

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/GeoNet/kit/weft"
	"github.com/GeoNet/nema-mar-portal/internal/valid"
)

// dashboardHandler serves the read-only dashboard page.
func dashboardHandler(r *http.Request, h http.Header, b *bytes.Buffer, nonce string) error {
	q, err := weft.CheckQueryValid(r, []string{"GET"}, []string{}, []string{"event_title", "version"}, valid.Query)
	if err != nil {
		return err
	}

	page := Page{Nonce: nonce}

	eventTitle := q.Get("event_title")
	versionStr := q.Get("version")

	if eventTitle != "" && versionStr != "" {
		// Show specific version
		version, _ := strconv.Atoi(versionStr)
		_ = version
		// Get the specific version by listing with filter
		eat, err := fsClient.GetLatestVersion(eventTitle)
		if err != nil {
			return weft.StatusError{Code: http.StatusInternalServerError, Err: err}
		}
		page.CurrentEAT = eat
	} else {
		// Show the latest EAT
		events, err := fsClient.ListDistinctEvents(7)
		if err != nil {
			return weft.StatusError{Code: http.StatusInternalServerError, Err: err}
		}
		if len(events) > 0 {
			eat, err := fsClient.GetLatestVersion(events[0])
			if err != nil {
				return weft.StatusError{Code: http.StatusInternalServerError, Err: err}
			}
			page.CurrentEAT = eat
		}
	}

	// Load version history if we have a current EAT
	if page.CurrentEAT != nil {
		eats, err := fsClient.ListEATs(page.CurrentEAT.EventDate.AddDate(0, 0, -1))
		if err == nil {
			for _, e := range eats {
				if e.EventTitle == page.CurrentEAT.EventTitle {
					page.Versions = append(page.Versions, e)
				}
			}
		}
	}

	h.Set("Content-Type", "text/html; charset=utf-8")
	return dashboardTemplate.Execute(b, page)
}
