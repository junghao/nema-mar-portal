package valid

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/GeoNet/kit/weft"
)

// Query validates query parameters for the API and page handlers.
func Query(v url.Values) error {
	if id := v.Get("id"); id != "" {
		if _, err := strconv.Atoi(id); err != nil {
			return weft.StatusError{Code: http.StatusBadRequest, Err: fmt.Errorf("invalid id: %s", id)}
		}
	}

	if days := v.Get("days"); days != "" {
		d, err := strconv.Atoi(days)
		if err != nil || d < 1 || d > 90 {
			return weft.StatusError{Code: http.StatusBadRequest, Err: fmt.Errorf("invalid days: %s (must be 1-90)", days)}
		}
	}

	if version := v.Get("version"); version != "" {
		v, err := strconv.Atoi(version)
		if err != nil || v < 1 {
			return weft.StatusError{Code: http.StatusBadRequest, Err: fmt.Errorf("invalid version: %s", version)}
		}
	}

	return nil
}

// EventTitle validates that an event_title parameter is non-empty.
func EventTitle(v url.Values) error {
	if et := v.Get("event_title"); et == "" {
		return weft.StatusError{Code: http.StatusBadRequest, Err: errors.New("event_title is required")}
	}
	return nil
}
