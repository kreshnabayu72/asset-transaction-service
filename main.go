package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/app"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/config"
)

func main() {
	serviceApp := app.New(config.LoadConfig())

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- serviceApp.Run()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatalf("server error: %v", err)
	case <-signals:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := serviceApp.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}
}
