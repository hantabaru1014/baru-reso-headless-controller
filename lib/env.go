package lib

import (
	"os"
	"time"
)

// GetEnvDuration returns a duration from an environment variable, or the default value if not set or invalid.
func GetEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}
