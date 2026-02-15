package fastschema

import (
	"testing"
	"time"
)

func TestFormatEventTitle(t *testing.T) {
	tests := []struct {
		magnitude float32
		location  string
		date      time.Time
		expected  string
	}{
		{5.0, "Wellington", time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC), "M5.0-Wellington-2026-01-15"},
		{6.2, "Kaikoura", time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), "M6.2-Kaikoura-2026-03-01"},
		{0.5, "Test Location", time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC), "M0.5-Test Location-2025-12-31"},
	}

	for _, tt := range tests {
		result := FormatEventTitle(tt.magnitude, tt.location, tt.date)
		if result != tt.expected {
			t.Errorf("FormatEventTitle(%v, %q, %v) = %q, want %q",
				tt.magnitude, tt.location, tt.date, result, tt.expected)
		}
	}
}
