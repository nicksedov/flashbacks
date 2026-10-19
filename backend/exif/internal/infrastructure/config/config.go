package config

import (
	"os"
	"strconv"
)

// Config holds all EXIF service configuration
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	ServerHost string
	ServerPort string

	ExiftoolPoolSize int

	LogLevel string
}

// Load reads configuration from environment variables
func Load() *Config {
	return &Config{
		DBHost:           getEnv("DB_HOST", "localhost"),
		DBPort:           getEnv("DB_PORT", "5432"),
		DBUser:           getEnv("DB_USER", "flashbacks"),
		DBPassword:       getEnv("DB_PASSWORD", "postgres"),
		DBName:           getEnv("DB_NAME", "flashbacks"),
		DBSSLMode:        getEnv("DB_SSLMODE", "disable"),
		ServerHost:       getEnv("EXIF_HOST", "0.0.0.0"),
		ServerPort:       getEnv("EXIF_PORT", "5172"),
		ExiftoolPoolSize: getEnvInt("EXIFTOOL_POOL_SIZE", 0),
		LogLevel:         getEnvFallback("EXIF_LOG_LEVEL", "LOG_LEVEL", "info"),
	}
}

// RegistryDefaults returns the bootstrap values the registry may override.
func (c *Config) RegistryDefaults() map[string]string {
	return map[string]string{
		"DB_HOST":            c.DBHost,
		"DB_PORT":            c.DBPort,
		"DB_USER":            c.DBUser,
		"DB_NAME":            c.DBName,
		"DB_SSLMODE":         c.DBSSLMode,
		"EXIF_HOST":          c.ServerHost,
		"EXIF_PORT":          c.ServerPort,
		"EXIFTOOL_POOL_SIZE": strconv.Itoa(c.ExiftoolPoolSize),
		"LOG_LEVEL":          c.LogLevel,
	}
}

// ApplyRegistry overwrites fields present in a resolved registry map.
func (c *Config) ApplyRegistry(values map[string]string) {
	if value, ok := values["DB_HOST"]; ok {
		c.DBHost = value
	}
	if value, ok := values["DB_PORT"]; ok {
		c.DBPort = value
	}
	if value, ok := values["DB_USER"]; ok {
		c.DBUser = value
	}
	if value, ok := values["DB_NAME"]; ok {
		c.DBName = value
	}
	if value, ok := values["DB_SSLMODE"]; ok {
		c.DBSSLMode = value
	}
	if value, ok := values["EXIF_HOST"]; ok {
		c.ServerHost = value
	}
	if value, ok := values["EXIF_PORT"]; ok {
		c.ServerPort = value
	}
	if value, ok := values["EXIFTOOL_POOL_SIZE"]; ok {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			c.ExiftoolPoolSize = parsed
		}
	}
	if value, ok := values["LOG_LEVEL"]; ok {
		c.LogLevel = value
	}
}

// getEnvFallback reads primary, then secondary (for backwards compatibility),
// and finally falls back to defaultValue.
func getEnvFallback(primary, secondary, defaultValue string) string {
	if value, exists := os.LookupEnv(primary); exists {
		return value
	}
	if value, exists := os.LookupEnv(secondary); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil && intVal > 0 {
			return intVal
		}
	}
	return defaultValue
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
