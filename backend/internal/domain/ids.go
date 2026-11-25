package domain

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

// ID prefixes make it obvious what kind of object an ID points to.
const (
	PrefixGeofence  = "geo_"
	PrefixVehicle   = "veh_"
	PrefixAlertRule = "alert_"
	PrefixViolation = "viol_"
	PrefixAlert     = "evt_"
)

var idEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewID returns prefix + 13 random lowercase base32 characters (64 bits of
// randomness), e.g. "geo_mzxw6ytboi4tq". IDs are not sequential, so they
// don't leak how many records exist.
func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error on supported platforms
	return prefix + strings.ToLower(idEncoding.EncodeToString(b))
}
