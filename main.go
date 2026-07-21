package main

import (
	"log"
	"huddle/db/connection"
	"huddle/pkg/audit"
	"huddle/pkg/router"

	"github.com/joho/godotenv"
)

func main() {
	audit.InitAuditLogger()

	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not loaded, falling back to environment variables:", err)
	}

	db := connection.ConnectToDb()
	rdb := connection.ConnectToRedis()

	r := router.RoutesSetup(db, rdb)

	log.Println("Huddle E-Commerce API running on port :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Server startup failed: %v", err)
	}
}