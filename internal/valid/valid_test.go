package valid

import (
	"net/url"
	"testing"
)

func TestQuery(t *testing.T) {
	tests := []struct {
		name    string
		values  url.Values
		wantErr bool
	}{
		{
			name:    "empty values OK",
			values:  url.Values{},
			wantErr: false,
		},
		{
			name:    "valid id",
			values:  url.Values{"id": {"42"}},
			wantErr: false,
		},
		{
			name:    "invalid id",
			values:  url.Values{"id": {"abc"}},
			wantErr: true,
		},
		{
			name:    "valid days",
			values:  url.Values{"days": {"7"}},
			wantErr: false,
		},
		{
			name:    "days too high",
			values:  url.Values{"days": {"100"}},
			wantErr: true,
		},
		{
			name:    "days zero",
			values:  url.Values{"days": {"0"}},
			wantErr: true,
		},
		{
			name:    "invalid days",
			values:  url.Values{"days": {"abc"}},
			wantErr: true,
		},
		{
			name:    "valid version",
			values:  url.Values{"version": {"3"}},
			wantErr: false,
		},
		{
			name:    "invalid version zero",
			values:  url.Values{"version": {"0"}},
			wantErr: true,
		},
		{
			name:    "invalid version negative",
			values:  url.Values{"version": {"-1"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Query(tt.values)
			if (err != nil) != tt.wantErr {
				t.Errorf("Query() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEventTitle(t *testing.T) {
	tests := []struct {
		name    string
		values  url.Values
		wantErr bool
	}{
		{
			name:    "non-empty event_title OK",
			values:  url.Values{"event_title": {"M5.0-Wellington-2026-01-01"}},
			wantErr: false,
		},
		{
			name:    "empty event_title",
			values:  url.Values{"event_title": {""}},
			wantErr: true,
		},
		{
			name:    "missing event_title",
			values:  url.Values{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EventTitle(tt.values)
			if (err != nil) != tt.wantErr {
				t.Errorf("EventTitle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
