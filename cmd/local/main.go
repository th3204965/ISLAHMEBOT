package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/th3204965/islahmebot/telegram"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables.")
	} else {
		log.Println("Loaded .env file successfully.")
	}

	// Verify required keys exist
	requiredEnvVars := []string{"TELEGRAM_BOT_TOKEN", "GROQ_API_KEY", "GEMINI_API_KEY"}
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("Fatal: %s is not set.", envVar)
		}
	}
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", telegram.HandleWebhook)

	log.Printf("Starting local development server on :%s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
