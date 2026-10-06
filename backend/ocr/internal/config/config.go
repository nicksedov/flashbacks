package config

import (
	"os"
)

// Config holds application configuration.
type Config struct {
	Port string
}

// Load loads configuration from environment variables.
// The port is read from OCR_PORT. For backwards compatibility, PORT is used as
// a fallback when OCR_PORT is not set. Defaults to port 5174 if neither is set.
func Load() *Config {
	port := os.Getenv("OCR_PORT")
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "5174"
	}
	return &Config{
		Port: port,
	}
}
