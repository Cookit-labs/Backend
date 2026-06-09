package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Cookit-labs/Backend/internal/config"
	"github.com/Cookit-labs/Backend/internal/db"
	"github.com/Cookit-labs/Backend/internal/server"
)

func main() {
	cfg := config.Load()

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	if err := database.Health(); err != nil {
		log.Fatalf("database health check failed: %v", err)
	}

	if err := database.Migrate(); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	srv := server.New(cfg, database)

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Println("server stopped")
}
