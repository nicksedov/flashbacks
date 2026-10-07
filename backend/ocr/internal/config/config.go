package config

import (
	"os"
)

// Config holds application configuration.
type Config struct {
	Host string
	Port string
}

// Load loads configuration from environment variables.
// The bind address is read from OCR_HOST and defaults to 0.0.0.0 (all
// interfaces), mirroring the EXIF service's EXIF_HOST behavior.
// The port is read from OCR_PORT. For backwards compatibility, PORT is used as
// a fallback when OCR_PORT is not set. Defaults to port 5174 if neither is set.
func Load() *Config {
	host := os.Getenv("OCR_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	port := os.Getenv("OCR_PORT")
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "5174"
	}
	return &Config{
		Host: host,
		Port: port,
	}
}
