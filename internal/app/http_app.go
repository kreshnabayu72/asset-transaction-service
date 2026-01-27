package app

import (
	"context"
	"fmt"
	"log/slog"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/config"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
	"github.com/gofiber/fiber/v2"
)

// HttpApp is the http application, you can extend this to suit your usecase.
type HttpApp struct {
	App            *fiber.App
	Config         config.ConfigOptions
	ServiceManager service.Managerer
}

// MakeHttpApp creates a new HttpApp instance.
//
//	httpApp := app.MakeHttpApp()
//
// You can pass optional configuration options by passing a ConfigOptions struct.
func MakeHttpApp(config ...config.ConfigOptions) *HttpApp {
	slog.Debug("Creating http application...")

	httpApp := &HttpApp{
		App:            fiber.New(),
		ServiceManager: service.MakeManager(),
	}

	if len(config) > 0 {
		slog.Debug("http application configuration found.")
		httpApp.Config = config[0]
	}

	slog.Debug("Created http application.")
	return httpApp
}

// Run runs the HttpApp application, may return an error if fails.
func (a *HttpApp) Run(ctx context.Context) error {
	slog.Debug("Fiber application listening on port: " + a.Config.PORT)
	return a.App.Listen(":" + a.Config.PORT)
}

// Shutdown shuts the HttpApp application down, may return an error if fails.
func (a *HttpApp) Shutdown() error {
	slog.Debug("Shutting down Fiber application...")
	if err := a.App.Shutdown(); err != nil {
		return err
	}
	slog.Debug("Fiber was successfully shutdown.")

	return nil
}

// RegisterService registers external service to the application.
func (a *HttpApp) RegisterService(name string, service service.Servicer) {
	slog.Debug(fmt.Sprintf("Registering service '%s'...", name))
	a.ServiceManager.Register(name, service)
	slog.Debug(fmt.Sprintf("Registered service '%s'.", name))
}

// UnregisterService unregisters registered service from the application.
func (a *HttpApp) UnregisterService(name string) {
	slog.Debug(fmt.Sprintf("Unregistering service '%s'...", name))
	a.ServiceManager.Unregister(name)
	slog.Debug(fmt.Sprintf("Unregistered service '%s'.", name))
}
