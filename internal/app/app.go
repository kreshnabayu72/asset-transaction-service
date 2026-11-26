package app

import (
	"context"
	"log"
	"log/slog"
	"net"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	temporalclient "go.temporal.io/sdk/client"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/config"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/consumer"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/group_guard"
	groupGuardService "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/group_guard/service"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings"
	transactionSettingsHandler "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/handler"
	transactionSettingsProvider "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/provider"
	transactionSettingsRepository "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/repository"
	transactionSettingsService "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/repository"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/pkg/migrations"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/tenant"
	coreConfig "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/config"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
)

// App owns the HTTP server, tenant migration worker, iam event consumer and their lifecycle.
type App struct {
	httpApp         *fiber.App
	port            string
	consumerCancel  context.CancelFunc
	temporalClient  temporalclient.Client
	temporalService *coreService.TemporalService
}

// New provisions existing tenant schemas before creating the HTTP application and routes.
func New(appConfig config.AppConfig) *App {
	ctx := context.Background()
	var tenantStatus *tenant.Status
	var temporalService *coreService.TemporalService

	if appConfig.PostgresDB != "" && appConfig.ServiceToken != "" {
		tenantMig, err := migrations.Tenant(appConfig.PostgresDSN())
		if err != nil {
			log.Fatalf("tenant migrations init failed: %v", err)
		}
		migAct := migrations.NewActivities(tenantMig)

		if appConfig.TemporalHost == "" || appConfig.TemporalTaskQueue == "" {
			log.Fatalf("TEMPORAL_HOST and TEMPORAL_TASK_QUEUE are required for tenant migration provisioning")
		}
		temporalService, err = coreService.MakeTemporalService(appConfig.TemporalTaskQueue, coreConfig.ConfigOptions{
			TEMPORAL_CLIENT_HOST: appConfig.TemporalHost,
			TEMPORAL_CLIENT_PORT: appConfig.TemporalPort,
			TEMPORAL_NAMESPACE:   appConfig.TemporalNamespace,
		})
		if err != nil {
			log.Fatalf("tenant migration worker init failed: %v", err)
		}
		temporalService.RegisterActivity(migAct.MigrateTenant)
		temporalService.RegisterActivity(migAct.MigrateExisting)
		temporalService.RegisterActivity(migAct.Verify)
		temporalService.RegisterActivity(migAct.DropTenantSchema)

		tenantStatus, err = tenant.Start(ctx, migAct, appConfig.EnterpriseManagementBaseURL, appConfig.ServiceToken)
		if err != nil {
			log.Fatalf("tenant provisioning failed: %v", err)
		}
	}

	fiberApp := fiber.New()
	fiberApp.Use(requestid.New())
	fiberApp.Use(logger.New())
	fiberApp.Use(recover.New())

	if tenantStatus != nil {
		fiberApp.Get("/health", tenantStatus.Health)
	} else {
		fiberApp.Get("/health", func(ctx fiber.Ctx) error {
			return ctx.JSON(fiber.Map{"status": "ok"})
		})
	}

	group_guard.SetupInternalRoutes(fiberApp, groupGuardService.NewService(), appConfig.ServiceToken)

	// Keeps modified_by_name current when iam renames a user. Stops on Shutdown.
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	userRenamed := consumer.NewUserRenamedConsumer(appConfig.PostgresDB, repository.NewTransactionSettingsRepository(), appConfig.KafkaBrokers)
	go func() {
		defer func() { _ = userRenamed.Close() }()
		userRenamed.Run(consumerCtx)
	}()

	// ADR 013: the snapshot loader mounts once, before every protected route.
	fiberApp.Use(tsMiddleware.LoadUserRoles(appConfig.IAMBaseURL, appConfig.ServiceToken))

	var temporalClient temporalclient.Client
	if appConfig.TemporalHost != "" && appConfig.AnalyticsReportingTaskQueue != "" {
		connected, err := temporalclient.Dial(temporalclient.Options{
			HostPort:  net.JoinHostPort(appConfig.TemporalHost, appConfig.TemporalPort),
			Namespace: appConfig.TemporalNamespace,
		})
		if err != nil {
			slog.Warn("Temporal client unavailable; Transaction Settings change logs will be skipped", "error", err)
		} else {
			temporalClient = connected
		}
	}

	// Rules and version come from the tenant database. Owner-service providers answer categories, assets,
	// override targets, Group scope and deny contacts; the plan is read per request from the plan provider
	// (EMS subscriptions and capacities, IAM approver presence), and a provider failure answers 500.
	settingsRepo := transactionSettingsRepository.NewTransactionSettingsRepository(transactionSettingsRepository.NewTenantServicerFactory(appConfig.PostgresDB))
	settingsSvc := transactionSettingsService.NewService(settingsRepo, transactionSettingsProvider.NewWorkbookRenderer())
	txProviders := transactionSettingsProvider.NewProviders(transactionSettingsProvider.Endpoints{
		MasterAssetRegistry:  appConfig.MasterAssetRegistryBaseURL,
		EnterpriseManagement: appConfig.EnterpriseManagementBaseURL,
		IAM:                  appConfig.IAMBaseURL,
	}, appConfig.ServiceToken)
	settingsSvc = settingsSvc.
		WithCategoryDirectory(txProviders.Categories).
		WithAssetDirectory(txProviders.Assets).
		WithOverrideTargetDirectory(txProviders.OverrideTargets).
		WithGroupScopeDirectory(txProviders.GroupScope).
		WithDenyContactResolver(txProviders.DenyContacts)
	if temporalClient != nil {
		txChangeLogs := transactionSettingsProvider.NewChangeLogDispatcher(temporalClient, appConfig.AnalyticsReportingTaskQueue)
		settingsSvc = settingsSvc.WithChangeLogDispatcher(txChangeLogs).WithActivityEntryDispatcher(txChangeLogs)
	}
	transaction_settings.SetupRoutes(fiberApp, transactionSettingsHandler.NewHandler(settingsSvc,
		transactionSettingsHandler.PlanFromProvider(txProviders.Plans.Plan)).WithEntity(txProviders.Entities.EntityType).WithClientStatus(txProviders.Entities.ClientStatus))

	return &App{
		httpApp:         fiberApp,
		port:            appConfig.Port,
		consumerCancel:  consumerCancel,
		temporalClient:  temporalClient,
		temporalService: temporalService,
	}
}

// RunWorker runs the tenant migration activities on this service's Temporal queue until ctx is done.
func (a *App) RunWorker(ctx context.Context) error {
	if a.temporalService == nil {
		<-ctx.Done()
		return nil
	}
	return a.temporalService.Run(ctx, nil)
}

// Run starts the HTTP server.
func (a *App) Run() error {
	return a.httpApp.Listen(":" + a.port)
}

// Shutdown stops the HTTP server using the provided context.
func (a *App) Shutdown(ctx context.Context) error {
	a.consumerCancel()
	err := a.httpApp.ShutdownWithContext(ctx)
	if a.temporalClient != nil {
		a.temporalClient.Close()
	}
	if a.temporalService != nil {
		a.temporalService.Close()
	}
	return err
}
