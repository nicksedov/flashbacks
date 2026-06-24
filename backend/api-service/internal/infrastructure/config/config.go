package config

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// AppConfig holds all application configuration
type AppConfig struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	ServerHost  string
	ServerPort  string
	CORSOrigins []string

	ScanWorkers int

	// OCR classifier configuration
	OCREnabled            bool
	OCRServiceURL         string
	OCRCheckInterval      int
	OCRConcurrentRequests int // Max concurrent OCR requests (default: 4)

	// Auth configuration
	BootstrapLogin      string
	BootstrapPassword   string
	SessionIdleHours    int
	SessionAbsoluteDays int

	// Thumbnail cache configuration
	ThumbnailCacheEnabled       bool
	ThumbnailCachePath          string
	ThumbnailCacheMaxSize       int
	ThumbnailCacheQuality       int
	ThumbnailCachePreloadOnScan bool

	// Background sync configuration
	BackgroundSyncEnabled bool

	// VL LLM configuration
	LlmMaxImageMegapixels float64

	// Agent configuration
	AgentMaxConversationTokens int

	// EXIF service configuration
	ExifServiceURL string
}

// LoadConfig reads configuration from environment variables
func LoadConfig() *AppConfig {
	originsStr := getEnv("CORS_ORIGINS", "http://localhost:5180")
	origins := strings.Split(originsStr, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	scanWorkers := runtime.NumCPU()
	if v := getEnv("SCAN_WORKERS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			scanWorkers = n
		}
	}

	return &AppConfig{
		DBHost:                      getEnv("DB_HOST", "localhost"),
		DBPort:                      getEnv("DB_PORT", "5432"),
		DBUser:                      getEnv("DB_USER", "postgres"),
		DBPassword:                  getEnv("DB_PASSWORD", "postgres"),
		DBName:                      getEnv("DB_NAME", "image_dedup"),
		ServerHost:                  getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:                  getEnv("SERVER_PORT", "5170"),
		CORSOrigins:                 origins,
		ScanWorkers:                 scanWorkers,
		OCREnabled:                  getEnv("OCR_ENABLED", "true") == "true",
		OCRServiceURL:               getEnv("OCR_SERVICE_URL", "http://localhost:5174"),
		OCRCheckInterval:            getEnvInt("OCR_CHECK_INTERVAL", 10),
		OCRConcurrentRequests:       getEnvInt("OCR_CONCURRENT_REQUESTS", 4),
		BootstrapLogin:              getEnv("BOOTSTRAP_LOGIN", "admin"),
		BootstrapPassword:           getEnv("BOOTSTRAP_PASSWORD", "admin"),
		SessionIdleHours:            getEnvInt("SESSION_IDLE_HOURS", 720),   // 30 days
		SessionAbsoluteDays:         getEnvInt("SESSION_ABSOLUTE_DAYS", 90), // 90 days
		ThumbnailCacheEnabled:       getEnv("THUMBNAIL_CACHE_ENABLED", "true") == "true",
		ThumbnailCachePath:          getEnv("THUMBNAIL_CACHE_PATH", ""),
		ThumbnailCacheMaxSize:       getEnvInt("THUMBNAIL_CACHE_MAX_SIZE", 320),
		ThumbnailCacheQuality:       getEnvInt("THUMBNAIL_CACHE_QUALITY", 80),
		ThumbnailCachePreloadOnScan: getEnv("THUMBNAIL_CACHE_PRELOAD_ON_SCAN", "true") == "true",
		BackgroundSyncEnabled:       getEnv("BACKGROUND_SYNC_ENABLED", "true") == "true",
		LlmMaxImageMegapixels:       getEnvFloat("LLM_MAX_IMAGE_MEGAPIXELS", 2.0),
		AgentMaxConversationTokens:  getEnvInt("AGENT_MAX_CONVERSATION_TOKENS", 128000),
		ExifServiceURL:              getEnv("EXIF_SERVICE_URL", "http://localhost:5172"),
	}
}

// getEnvInt gets environment variable as int with a default value
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return defaultValue
}

// getEnvFloat gets environment variable as float64 with a default value
func getEnvFloat(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if n, err := strconv.ParseFloat(value, 64); err == nil && n > 0 {
			return n
		}
	}
	return defaultValue
}

// getEnv gets environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
