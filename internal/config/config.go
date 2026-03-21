package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	KnownMacs           []string
	NotificationService string
	AlwaysNotify        bool
	RememberNewDevices  bool
	RemoveOldDevices    bool
	RequireIP           bool
	RemoveDelay         int64
	DatabasePath        string
	WSEventDelay        int  // seconds to wait after a WS event before querying the API
	FallbackInterval    int  // seconds for fallback checks; -1 = disabled, default = 60
	Verbose             bool // if true, log diagnostic/polling details; default = false
}

func Load() Config {
	cfg := Config{
		NotificationService: "Telegram",
		RememberNewDevices:  true,
		DatabasePath:        "/data/knownMacs.db",
		WSEventDelay:        3,
		FallbackInterval:    60, // default: fallback every 60 seconds
	}

	if v := os.Getenv("KNOWN_MACS"); v != "" {
		for _, mac := range strings.Split(v, ",") {
			if trimmed := strings.TrimSpace(mac); trimmed != "" {
				cfg.KnownMacs = append(cfg.KnownMacs, trimmed)
			}
		}
	}

	if v := os.Getenv("NOTIFICATION_SERVICE"); v != "" {
		cfg.NotificationService = v
	}

	if v := os.Getenv("DATABASE_PATH"); v != "" {
		cfg.DatabasePath = v
	}

	cfg.AlwaysNotify = parseBool(os.Getenv("ALWAYS_NOTIFY"), false)
	cfg.RememberNewDevices = parseBool(os.Getenv("REMEMBER_NEW_DEVICES"), true)
	cfg.RemoveOldDevices = parseBool(os.Getenv("REMOVE_OLD_DEVICES"), false)
	cfg.RequireIP = parseBool(os.Getenv("REQUIRE_IP"), false)

	if v := os.Getenv("REMOVE_DELAY"); v != "" {
		if n, ok := parseDuration(v); ok {
			cfg.RemoveDelay = n
		} else {
			log.Printf("Warning: REMOVE_DELAY has invalid format %q, ignoring (using default: 0)", v)
		}
	}

	if v := os.Getenv("WS_EVENT_DELAY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.WSEventDelay = n
		}
	}

	if v := os.Getenv("FALLBACK_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && (n == -1 || n > 0) {
			cfg.FallbackInterval = n
		}
	}

	cfg.Verbose = parseBool(os.Getenv("VERBOSE"), false)

	return cfg
}

func parseBool(s string, defaultVal bool) bool {
	if s == "" {
		return defaultVal
	}
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}

// parseDuration converts a duration string to seconds.
// Accepts raw integer (e.g., "86400") or suffixed values (e.g., "30s", "24h", "7d", "2w").
// Returns (seconds, success).
func parseDuration(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, true
	}

	// Try raw integer first (backward compat for raw seconds)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n >= 0 {
		return n, true
	}

	// Try suffix-based parsing
	suffixes := map[string]int64{"s": 1, "m": 60, "h": 3600, "d": 86400, "w": 604800}
	for suffix, mult := range suffixes {
		if strings.HasSuffix(s, suffix) {
			if n, err := strconv.ParseInt(s[:len(s)-1], 10, 64); err == nil && n >= 0 {
				return n * mult, true
			}
		}
	}

	return 0, false
}

// HumanDuration formats a duration in seconds to a human-readable string.
// Examples: "30s", "2m", "1h", "7d", "2w".
func HumanDuration(seconds int64) string {
	if seconds <= 0 {
		return "0s"
	}

	type unit struct {
		name     string
		duration int64
	}
	units := []unit{
		{"w", 604800},
		{"d", 86400},
		{"h", 3600},
		{"m", 60},
		{"s", 1},
	}

	var parts []string
	remaining := seconds
	for _, u := range units {
		if remaining >= u.duration {
			count := remaining / u.duration
			remaining = remaining % u.duration
			parts = append(parts, strconv.FormatInt(count, 10)+u.name)
		}
	}

	if len(parts) == 0 {
		return "0s"
	}

	if len(parts) == 1 {
		return parts[0]
	}

	// Return top 2 units for readability
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, " ")
}
