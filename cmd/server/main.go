package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"go-mac/internal/db"
	"go-mac/internal/poller"
	"go-mac/internal/web"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	"github.com/joho/godotenv"
)

// getEnv fetches environment variable or returns fallback
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	// Load .env if exists
	_ = godotenv.Load()

	// Configurable values from env
	host := getEnv("WEB_HOST", "0.0.0.0")
	port := getEnv("WEB_PORT", "8080")
	pollIntervalSec, _ := strconv.Atoi(getEnv("POLL_INTERVAL", "600"))
	dbPath := getEnv("DB_PATH", "/tmp/gomac1910.db")

	// Initialize database
	db.InitDB(dbPath)

	// Start background SNMP poller
	go poller.StartBackgroundPolling(time.Duration(pollIntervalSec)*time.Second, dbPath)

	// Setup template engine
	engine := html.New("./internal/web/templates", ".html")
	engine.AddFunc("mod", func(a, b int) int { return a % b })

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	web.SetupRoutes(app)

	log.Printf("Server running at http://%s:%s\n", host, port)
	log.Fatal(app.Listen(host + ":" + port))
}
