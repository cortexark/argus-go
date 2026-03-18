package config

import (
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	DataDir              string
	DBPath               string
	LogDir               string
	IPCSocketPath        string
	WebPort              int
	ScanInterval         time.Duration
	NetworkScanInterval  time.Duration
	CleanupInterval      time.Duration
	NotificationThrottle time.Duration
	MaxLogSizeMB         int
}

func Load() *Config {
	home, _ := os.UserHomeDir()
	dataDir := getEnv("ARGUS_DATA_DIR", filepath.Join(home, ".argus"))

	return &Config{
		DataDir:              dataDir,
		DBPath:               getEnv("ARGUS_DB_PATH", filepath.Join(dataDir, "data.db")),
		LogDir:               filepath.Join(dataDir, "logs"),
		IPCSocketPath:        filepath.Join(dataDir, "argus.sock"),
		WebPort:              3131,
		ScanInterval:         30 * time.Second,
		NetworkScanInterval:  15 * time.Second,
		CleanupInterval:      24 * time.Hour,
		NotificationThrottle: 5 * time.Minute,
		MaxLogSizeMB:         10,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
