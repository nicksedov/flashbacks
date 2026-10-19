package config

import (
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/flashbacks/core/internal/domain"
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
	AccountCreationMode string // "self_service" | "self_service_with_approval" | "admin_only"

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

	accountCreationMode := getEnv("ACCOUNT_CREATION_MODE", domain.AccountCreationModeAdminOnly)
	switch accountCreationMode {
	case domain.AccountCreationModeSelfService, domain.AccountCreationModeSelfServiceWithApproval, domain.AccountCreationModeAdminOnly:
	default:
		log.Fatalf("invalid ACCOUNT_CREATION_MODE %q: must be one of %q, %q, %q",
			accountCreationMode,
			domain.AccountCreationModeSelfService,
			domain.AccountCreationModeSelfServiceWithApproval,
			domain.AccountCreationModeAdminOnly)
	}

	return &AppConfig{
		DBHost:                      getEnv("DB_HOST", "localhost"),
		DBPort:                      getEnv("DB_PORT", "5432"),
		DBUser:                      getEnv("DB_USER", "flashbacks"),
		DBPassword:                  getEnv("DB_PASSWORD", "postgres"),
		DBName:                      getEnv("DB_NAME", "flashbacks"),
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
		AccountCreationMode:         accountCreationMode,
		ThumbnailCacheEnabled:       getEnv("THUMBNAIL_CACHE_ENABLED", "true") == "true",
		ThumbnailCachePath:          getEnv("THUMBNAIL_CACHE_PATH", ""),
		ThumbnailCacheMaxSize:       getEnvInt("THUMBNAIL_CACHE_MAX_SIZE", 320),
		ThumbnailCacheQuality:       getEnvInt("THUMBNAIL_CACHE_QUALITY", 80),
		ThumbnailCachePreloadOnScan: getEnv("THUMBNAIL_CACHE_PRELOAD_ON_SCAN", "true") == "true",
		BackgroundSyncEnabled:       getEnv("BACKGROUND_SYNC_ENABLED", "true") == "true",
		LlmMaxImageMegapixels:       getEnvFloat("LLM_MAX_IMAGE_MEGAPIXELS", 2.4),
		AgentMaxConversationTokens:  getEnvInt("AGENT_MAX_CONVERSATION_TOKENS", 128000),
		ExifServiceURL:              getEnv("EXIF_SERVICE_URL", "http://localhost:5172"),
	}
}

// RegistryDefaults returns the bootstrap values the registry may override.
// Secrets are intentionally excluded and read through the secrets API.
func (c *AppConfig) RegistryDefaults() map[string]string {
	return map[string]string{
		"DB_HOST":                         c.DBHost,
		"DB_PORT":                         c.DBPort,
		"DB_USER":                         c.DBUser,
		"DB_NAME":                         c.DBName,
		"SERVER_HOST":                     c.ServerHost,
		"SERVER_PORT":                     c.ServerPort,
		"CORS_ORIGINS":                    strings.Join(c.CORSOrigins, ","),
		"SCAN_WORKERS":                    strconv.Itoa(c.ScanWorkers),
		"OCR_ENABLED":                     strconv.FormatBool(c.OCREnabled),
		"OCR_SERVICE_URL":                 c.OCRServiceURL,
		"OCR_CHECK_INTERVAL":              strconv.Itoa(c.OCRCheckInterval),
		"OCR_CONCURRENT_REQUESTS":         strconv.Itoa(c.OCRConcurrentRequests),
		"BOOTSTRAP_LOGIN":                 c.BootstrapLogin,
		"SESSION_IDLE_HOURS":              strconv.Itoa(c.SessionIdleHours),
		"SESSION_ABSOLUTE_DAYS":           strconv.Itoa(c.SessionAbsoluteDays),
		"ACCOUNT_CREATION_MODE":           c.AccountCreationMode,
		"THUMBNAIL_CACHE_ENABLED":         strconv.FormatBool(c.ThumbnailCacheEnabled),
		"THUMBNAIL_CACHE_PATH":            c.ThumbnailCachePath,
		"THUMBNAIL_CACHE_MAX_SIZE":        strconv.Itoa(c.ThumbnailCacheMaxSize),
		"THUMBNAIL_CACHE_QUALITY":         strconv.Itoa(c.ThumbnailCacheQuality),
		"THUMBNAIL_CACHE_PRELOAD_ON_SCAN": strconv.FormatBool(c.ThumbnailCachePreloadOnScan),
		"BACKGROUND_SYNC_ENABLED":         strconv.FormatBool(c.BackgroundSyncEnabled),
		"LLM_MAX_IMAGE_MEGAPIXELS":        strconv.FormatFloat(c.LlmMaxImageMegapixels, 'f', -1, 64),
		"AGENT_MAX_CONVERSATION_TOKENS":   strconv.Itoa(c.AgentMaxConversationTokens),
		"EXIF_SERVICE_URL":                c.ExifServiceURL,
	}
}

// ApplyRegistry overwrites fields present in a resolved registry map.
func (c *AppConfig) ApplyRegistry(values map[string]string) {
	applyString := func(key string, target *string) {
		if value, ok := values[key]; ok {
			*target = value
		}
	}
	applyPositiveInt := func(key string, target *int) {
		if value, ok := values[key]; ok {
			if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
				*target = parsed
			}
		}
	}
	applyBool := func(key string, target *bool) {
		if value, ok := values[key]; ok {
			if parsed, err := strconv.ParseBool(value); err == nil {
				*target = parsed
			}
		}
	}
	applyFloat := func(key string, target *float64) {
		if value, ok := values[key]; ok {
			if parsed, err := strconv.ParseFloat(value, 64); err == nil && parsed > 0 {
				*target = parsed
			}
		}
	}

	applyString("DB_HOST", &c.DBHost)
	applyString("DB_PORT", &c.DBPort)
	applyString("DB_USER", &c.DBUser)
	applyString("DB_NAME", &c.DBName)
	applyString("SERVER_HOST", &c.ServerHost)
	applyString("SERVER_PORT", &c.ServerPort)
	applyString("OCR_SERVICE_URL", &c.OCRServiceURL)
	applyString("BOOTSTRAP_LOGIN", &c.BootstrapLogin)
	applyString("THUMBNAIL_CACHE_PATH", &c.ThumbnailCachePath)
	applyString("EXIF_SERVICE_URL", &c.ExifServiceURL)

	if value, ok := values["CORS_ORIGINS"]; ok {
		origins := strings.Split(value, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		c.CORSOrigins = origins
	}

	applyPositiveInt("SCAN_WORKERS", &c.ScanWorkers)
	applyPositiveInt("OCR_CHECK_INTERVAL", &c.OCRCheckInterval)
	applyPositiveInt("OCR_CONCURRENT_REQUESTS", &c.OCRConcurrentRequests)
	applyPositiveInt("SESSION_IDLE_HOURS", &c.SessionIdleHours)
	applyPositiveInt("SESSION_ABSOLUTE_DAYS", &c.SessionAbsoluteDays)
	applyPositiveInt("THUMBNAIL_CACHE_MAX_SIZE", &c.ThumbnailCacheMaxSize)
	applyPositiveInt("THUMBNAIL_CACHE_QUALITY", &c.ThumbnailCacheQuality)
	applyPositiveInt("AGENT_MAX_CONVERSATION_TOKENS", &c.AgentMaxConversationTokens)

	applyBool("OCR_ENABLED", &c.OCREnabled)
	applyBool("THUMBNAIL_CACHE_ENABLED", &c.ThumbnailCacheEnabled)
	applyBool("THUMBNAIL_CACHE_PRELOAD_ON_SCAN", &c.ThumbnailCachePreloadOnScan)
	applyBool("BACKGROUND_SYNC_ENABLED", &c.BackgroundSyncEnabled)

	applyFloat("LLM_MAX_IMAGE_MEGAPIXELS", &c.LlmMaxImageMegapixels)

	if value, ok := values["ACCOUNT_CREATION_MODE"]; ok {
		switch value {
		case domain.AccountCreationModeSelfService,
			domain.AccountCreationModeSelfServiceWithApproval,
			domain.AccountCreationModeAdminOnly:
			c.AccountCreationMode = value
		}
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
