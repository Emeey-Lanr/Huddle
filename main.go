package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"huddle/db/connection"
	"huddle/pkg/audit"
	"huddle/pkg/config"
	"huddle/pkg/mail"
	"huddle/pkg/router"
	"huddle/pkg/services/notification"
	"huddle/pkg/services/order"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	audit.InitAuditLogger()

	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not loaded, falling back to environment variables:", err)
	}

	// Stop now on a bad setting rather than misbehave later. Messages name
	// settings, never their values.
	problems, warnings := config.Check(os.Getenv)
	for _, w := range warnings {
		log.Printf("Warning: %s", w)
	}
	if len(problems) > 0 {
		log.Fatalf("Fix these settings (in .env or the environment) before starting:\n- %s", strings.Join(problems, "\n- "))
	}
	if config.IsProduction(os.Getenv) {
		gin.SetMode(gin.ReleaseMode)
	}

	db := connection.ConnectToDb()
	rdb := connection.ConnectToRedis()

	// Done on Ctrl+C or SIGTERM (sent by Docker and most hosts to stop the API).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Releases stock held by unpaid orders once their hold time passes.
	workers := []<-chan struct{}{order.StartExpiryWorker(ctx, db, time.Minute)}

	// Emails sellers about their paid orders. Without SMTP settings the
	// emails stay queued and go out once it is configured.
	if sender, err := mail.FromEnv(); err != nil {
		log.Printf("Seller emails are not sent: %v", err)
	} else {
		workers = append(workers, notification.StartWorker(ctx, db, sender, time.Minute))
	}

	port := config.Port(os.Getenv)
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Server startup failed: %v", err)
	}
	log.Printf("Huddle E-Commerce API running on port :%s", port)
	if err := serve(ctx, newHTTPServer(router.RoutesSetup(db, rdb)), ln, shutdownTimeout); err != nil {
		log.Fatalf("Server stopped with an error: %v", err)
	}

	// Requests are done; let the workers finish their current run before
	// the database goes away.
	log.Println("Shutting down: waiting for background workers")
	if !waitFor(time.Now().Add(shutdownTimeout), workers...) {
		log.Println("Background workers did not finish in time")
	}
	if err := rdb.Close(); err != nil {
		log.Printf("Closing Redis: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
	log.Println("Huddle E-Commerce API stopped")
}
