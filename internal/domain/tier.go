package domain

import "strings"

// Tier represents a supported quota time window.
// Only these three tiers are supported; all others must be omitted.
type Tier string

const (
	Tier5H Tier = "5H" // 5-hour window
	Tier1W Tier = "1W" // 1-week window
	Tier1M Tier = "1M" // 1-month window
)

// AllSupportedTiers is the list of all supported tier values.
var AllSupportedTiers = []Tier{Tier5H, Tier1W, Tier1M}

// IsSupported returns true if the tier is a supported tier.
func (t Tier) IsSupported() bool {
	switch t {
	case Tier5H, Tier1W, Tier1M:
		return true
	default:
		return false
	}
}

// String returns the string representation of the tier.
func (t Tier) String() string {
	return string(t)
}

// ParseTier attempts to parse a tier from a string.
// It returns the tier if it's a supported tier, otherwise returns false.
func ParseTier(s string) (Tier, bool) {
	t := Tier(strings.ToUpper(s))
	return t, t.IsSupported()
}