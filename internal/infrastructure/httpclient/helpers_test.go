package httpclient

import (
	"testing"
	"time"
)

func TestParseEpochSeconds(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected time.Time
	}{
		{"zero", 0, time.Unix(0, 0)},
		{"positive", 1609459200, time.Unix(1609459200, 0)},
		{"negative", -86400, time.Unix(-86400, 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEpochSeconds(tt.input)
			if !got.Equal(tt.expected) {
				t.Errorf("ParseEpochSeconds(%d) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseEpochMillis(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected time.Time
	}{
		{"zero", 0, time.UnixMilli(0)},
		{"positive", 1609459200000, time.UnixMilli(1609459200000)},
		{"negative", -86400000, time.UnixMilli(-86400000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEpochMillis(tt.input)
			if !got.Equal(tt.expected) {
				t.Errorf("ParseEpochMillis(%d) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseRFC3339(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    time.Time
		expectError bool
	}{
		{
			name:        "valid",
			input:       "2021-01-01T00:00:00Z",
			expected:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			expectError: false,
		},
		{
			name:        "valid with timezone",
			input:       "2021-01-01T12:30:00+08:00",
			expected:    time.Date(2021, 1, 1, 12, 30, 0, 0, time.FixedZone("", 8*3600)),
			expectError: false,
		},
		{
			name:        "invalid format",
			input:       "2021/01/01",
			expected:    time.Time{},
			expectError: true,
		},
		{
			name:        "invalid string",
			input:       "not a date",
			expected:    time.Time{},
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expected:    time.Time{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRFC3339(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseRFC3339(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseRFC3339(%q) unexpected error: %v", tt.input, err)
				return
			}
			if !got.Equal(tt.expected) {
				t.Errorf("ParseRFC3339(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseInt64String(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int64
		expectError bool
	}{
		{"zero", "0", 0, false},
		{"positive", "12345", 12345, false},
		{"negative", "-98765", -98765, false},
		{"max int64", "9223372036854775807", 9223372036854775807, false},
		{"invalid string", "abc", 0, true},
		{"float string", "1.5", 0, true},
		{"empty string", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInt64String(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseInt64String(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseInt64String(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseInt64String(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseFloat64String(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    float64
		expectError bool
	}{
		{"zero", "0", 0, false},
		{"positive", "123.456", 123.456, false},
		{"negative", "-987.654", -987.654, false},
		{"integer string", "100", 100, false},
		{"scientific", "1.23e+10", 1.23e+10, false},
		{"invalid string", "abc", 0, true},
		{"empty string", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFloat64String(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseFloat64String(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseFloat64String(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseFloat64String(%q) = %f, want %f", tt.input, got, tt.expected)
			}
		})
	}
}