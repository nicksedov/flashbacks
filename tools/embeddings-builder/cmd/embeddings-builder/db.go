package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// loadEnv loads the .env file, trying common paths relative to the tool location.
func loadEnv() {
	candidates := []string{
		filepath.Join("..", "..", "api-service", ".env"),
		filepath.Join("api-service", ".env"),
		".env",
	}
	for _, path := range candidates {
		if err := godotenv.Load(path); err == nil {
			log.Printf("Loaded environment from %s", path)
			return
		}
	}
	log.Println("No .env file found; using environment variables directly")
}

// getEnv returns the value of an environment variable or a fallback default.
func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// connectDB opens a GORM connection to PostgreSQL using env vars.
// It does NOT run AutoMigrate — the tables are assumed to already exist.
func connectDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "postgres"),
		getEnv("DB_NAME", "api_db"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:      logger.Default.LogMode(logger.Silent),
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return db, nil
}
