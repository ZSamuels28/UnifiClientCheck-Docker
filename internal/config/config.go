package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	KnownMacs             []string
	CheckInterval         int
	NotificationService   string
	AlwaysNotify          bool
	RememberNewDevices    bool
	TeleportNotifications bool
	RemoveOldDevices      bool
	RequireIP             bool
	RemoveDelay           int64
	DatabasePath          string
}

func Load() Config {
	cfg := Config{
		CheckInterval:       60,
		NotificationService: "Telegram",
		RememberNewDevices:  true,
		DatabasePath:        "/data/knownMacs.db",
	}

	if v := os.Getenv("KNOWN_MACS"); v != "" {
		for _, mac := range strings.Split(v, ",") {
			if trimmed := strings.TrimSpace(mac); trimmed != "" {
				cfg.KnownMacs = append(cfg.KnownMacs, trimmed)
			}
		}
	}

	if v := os.Getenv("CHECK_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.CheckInterval = n
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
	cfg.TeleportNotifications = parseBool(os.Getenv("TELEPORT_NOTIFICATIONS"), false)
	cfg.RemoveOldDevices = parseBool(os.Getenv("REMOVE_OLD_DEVICES"), false)
	cfg.RequireIP = parseBool(os.Getenv("REQUIRE_IP"), false)

	if v := os.Getenv("REMOVE_DELAY"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.RemoveDelay = n
		}
	}

	return cfg
}

func parseBool(s string, defaultVal bool) bool {
	if s == "" {
		return defaultVal
	}
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}
