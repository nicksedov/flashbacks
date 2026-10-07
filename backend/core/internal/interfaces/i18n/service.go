package i18n

import (
	"embed"
	"encoding/json"
	"sync"
)

//go:embed locales/*.json
var localeFiles embed.FS

// Service handles i18n message resolution
type Service struct {
	mu       sync.RWMutex
	locales  map[string]map[string]string
	fallback map[string]string
}

// flattenLocale converts a nested JSON structure into a flat map with dot-separated keys
func flattenLocale(data map[string]interface{}, prefix string, result map[string]string) {
	for key, value := range data {
		// Skip description fields
		if key == "description" || (len(key) > 11 && key[:11] == "description_") {
			continue
		}

		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case string:
			result[fullKey] = v
		case map[string]interface{}:
			flattenLocale(v, fullKey, result)
		}
	}
}

// NewService creates a new i18n service and loads all locale files
func NewService() (*Service, error) {
	s := &Service{
		locales:  make(map[string]map[string]string),
		fallback: make(map[string]string),
	}

	// Load English (default/fallback)
	enData, err := localeFiles.ReadFile("locales/en.json")
	if err != nil {
		return nil, err
	}
	var enRaw map[string]interface{}
	if err := json.Unmarshal(enData, &enRaw); err != nil {
		return nil, err
	}
	flattenLocale(enRaw, "", s.fallback)
	s.locales["en"] = s.fallback

	// Load Russian
	ruData, err := localeFiles.ReadFile("locales/ru.json")
	if err != nil {
		return nil, err
	}
	var ruRaw map[string]interface{}
	if err := json.Unmarshal(ruData, &ruRaw); err != nil {
		return nil, err
	}
	ruLocales := make(map[string]string)
	flattenLocale(ruRaw, "", ruLocales)
	s.locales["ru"] = ruLocales

	return s, nil
}

// GetMessage returns the translated message for the given key and language
// Falls back to English if the key is not found in the requested language
func (s *Service) GetMessage(key MessageKey, lang string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Try requested language
	if locales, ok := s.locales[lang]; ok {
		if msg, ok := locales[string(key)]; ok {
			return msg
		}
	}

	// Fallback to English
	if msg, ok := s.fallback[string(key)]; ok {
		return msg
	}

	// Ultimate fallback: return the key itself
	return string(key)
}
