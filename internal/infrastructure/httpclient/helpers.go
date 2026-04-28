package httpclient

import (
	"strconv"
	"time"
)

// ParseEpochSeconds converts epoch seconds to time.Time.
func ParseEpochSeconds(v int64) time.Time {
	return time.Unix(v, 0)
}

// ParseEpochMillis converts epoch milliseconds to time.Time.
func ParseEpochMillis(v int64) time.Time {
	return time.UnixMilli(v)
}

// ParseRFC3339 parses an RFC3339 formatted string to time.Time.
func ParseRFC3339(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// ParseInt64String parses a string to int64.
func ParseInt64String(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// ParseFloat64String parses a string to float64.
func ParseFloat64String(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}