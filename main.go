package main

import (
	"context"
	"log"
	"os"
	"time"

	"huddle/db/connection"
	"huddle/pkg/audit"
	"huddle/pkg/router"
	"huddle/pkg/services/order"

	"github.com/joho/godotenv"
)

func main() {
	audit.InitAuditLogger()

	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not loaded, falling back to environment variables:", err)
	}

	if os.Getenv("SECRET_KEY") == "" {
		log.Fatal("SECRET_KEY must be set (in .env or the environment) to sign auth tokens")
	}

	db := connection.ConnectToDb()
	rdb := connection.ConnectToRedis()

	// Releases stock held by unpaid orders once their hold time passes.
	order.StartExpiryWorker(context.Background(), db, time.Minute)

	r := router.RoutesSetup(db, rdb)

	log.Println("Huddle E-Commerce API running on port :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Server startup failed: %v", err)
	}
}