package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/api"
	"gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/app"
	"gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/workflow"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/provider"
	"go.temporal.io/sdk/worker"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/config"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
)

var isVerbose bool = false

func main() {
	args := os.Args[1:]
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	// TODO(refan): Better command args handler
	logLevel := &slog.LevelVar{}
	// You can enable debug mode by specifying -v for verbosity.
	if slices.Contains(args, "-v") {
		isVerbose = true
		logLevel.Set(slog.LevelDebug)
	}

	// TODO(refan): Further extend this functionality for writing out to file
	// - Also for potential integration with any logging service in the future.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel,
	})))

	slog.Debug(
		"os.Args",
		"args", args,
	)

	slog.Debug("Loading environment variables...")
	cfgOptions := config.LoadConfig(".env")
	slog.Debug("Environment variables loaded.")

	httpApp := app.MakeHttpApp(cfgOptions)

	temporalService, err := service.MakeTemporalService(cfgOptions.SERVICE_NAME, cfgOptions)
	if err != nil {
		slog.Error("Error while creating temporal service",
			"err", err,
		)
		panic(err)
	}

	workflow.RegisterWorkflows(temporalService, httpApp.ServiceManager)

	dbProvider := new(provider.DBProvider)

	httpApp.RegisterService("TemporalService", temporalService)
	httpApp.RegisterService("DBProvider", dbProvider)

	go runHttpApp(httpApp)
	go runTemporalService(temporalService)

	_ = <-sigs // blocks the main thread until an interrupt is received.
	slog.Info("=== received SIGINT ===")

	slog.Info("Gracefully shutting down...")
	// You can execute task to release resources here.

	// Shutdown Temporal service
	temporalService.Close()

	// Shutdown HTTP app
	if err := httpApp.Shutdown(); err != nil {
		slog.Error(
			"Error while shutting down fiber",
			"error", err,
		)
	}

	slog.Info("Program successfully shutdown gracefully.")
}

func runHttpApp(httpApp *app.HttpApp) {
	api.RegisterRoutes(httpApp.App, httpApp.ServiceManager)

	if err := httpApp.Run(context.Background()); err != nil {
		slog.Error(
			"Error while trying to run http application",
			"err", err,
		)
		panic(err)
	}
}

func runTemporalService(temporalService service.TemporalServicer) {
	if err := temporalService.Run(context.Background(), worker.InterruptCh()); err != nil {
		slog.Error("Error while trying to run temporal service",
			"err", err,
		)
		panic(err)
	}
}
