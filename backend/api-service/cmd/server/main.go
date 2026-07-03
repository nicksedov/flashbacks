package main

import (
	"log"

	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}
}

func main() {
	app, err := InitializeApp()
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	defer app.Shutdown()

	app.Init()
	app.PrintBanner()

	if err := app.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
