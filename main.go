package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"donetick.com/core/config"
	docs "donetick.com/core/docs"
	"donetick.com/core/external/payment"
	"donetick.com/core/frontend"
	auth "donetick.com/core/internal/auth"
	"donetick.com/core/internal/auth/apple"
	"donetick.com/core/internal/chore"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/circle"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/database"
	"donetick.com/core/internal/device"
	dRepo "donetick.com/core/internal/device/repo"
	"donetick.com/core/internal/email"
	"donetick.com/core/internal/events"
	label "donetick.com/core/internal/label"
	lRepo "donetick.com/core/internal/label/repo"
	"donetick.com/core/internal/mfa"
	"donetick.com/core/internal/project"
	pjRepo "donetick.com/core/internal/project/repo"

	sRepo "donetick.com/core/external/payment/repo"
	sService "donetick.com/core/external/payment/service"
	notifier "donetick.com/core/internal/notifier"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	discord "donetick.com/core/internal/notifier/service/discord"
	"donetick.com/core/internal/notifier/service/fcm"
	"donetick.com/core/internal/notifier/service/pushover"
	telegram "donetick.com/core/internal/notifier/service/telegram"
	pRepo "donetick.com/core/internal/points/repo"
	"donetick.com/core/internal/realtime"
	"donetick.com/core/internal/resource"
	"donetick.com/core/internal/storage"
	storageRepo "donetick.com/core/internal/storage/repo"
	spRepo "donetick.com/core/internal/subtask/repo"
	"donetick.com/core/internal/thing"
	tRepo "donetick.com/core/internal/thing/repo"
	"donetick.com/core/internal/user"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
	"donetick.com/core/migrations"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"gorm.io/gorm"

	"donetick.com/core/internal/filter"
	fRepo "donetick.com/core/internal/filter/repo"
	"donetick.com/core/internal/reward"
	rRepo "donetick.com/core/internal/reward/repo"
)

var (
	Version   = "dev"
	Commit    = "dev"
	BuildDate = "dev"
)

func backupSQLiteDatabase(cfg *config.Config) error {
	// Determine the SQLite database path
	dbPath := os.Getenv("DT_SQLITE_PATH")
	if dbPath == "" {
		dbPath = "donetick.db"
	}

	// Check if database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// Database doesn't exist yet, nothing to backup
		return nil
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.bak-%s", dbPath, timestamp)

	// Open source file
	srcFile, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer srcFile.Close()

	// Create backup file
	backupFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer backupFile.Close()

	// Copy the database file to backup
	if _, err := io.Copy(backupFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy database to backup: %w", err)
	}

	// Ensure backup is written to disk
	if err := backupFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync backup file: %w", err)
	}

	logging.DefaultLogger().Infof("SQLite database backed up to: %s", backupPath)
	return nil
}

func main() {
	// Load configuration first
	cfg := config.LoadConfig()

	// Configure logging from application config
	logging.SetConfigFromAppConfig(
		cfg.Logging.Level,
		cfg.Logging.Encoding,
		cfg.Logging.Development,
	)
	app := fx.New(
		fx.Supply(cfg),
		fx.Supply(logging.DefaultLogger().Desugar()),

		// fx.Provide(config.NewConfig),
		fx.Provide(auth.NewAuthMiddleware),
		fx.Provide(auth.APITokenMiddleware),
		fx.Provide(auth.NewMultiAuthMiddleware),
		fx.Provide(auth.NewIdentityProvider),
		fx.Provide(resource.NewHandler),

		// fx.Provide(NewBot),
		fx.Provide(database.NewDatabase),
		fx.Provide(chRepo.NewChoreRepository),
		fx.Provide(chore.NewHandler),
		fx.Provide(uRepo.NewUserRepository),
		fx.Provide(user.NewDeletionService),
		fx.Provide(user.NewHandler),
		fx.Provide(cRepo.NewCircleRepository),
		fx.Provide(circle.NewHandler),

		// Device management:
		fx.Provide(dRepo.NewDeviceRepository),
		fx.Provide(device.NewHandler),

		fx.Provide(nRepo.NewNotificationRepository),
		fx.Provide(nps.NewNotificationPlanner),

		// add notifier
		fx.Provide(pushover.NewPushover),
		fx.Provide(telegram.NewTelegramNotifier),
		fx.Provide(discord.NewDiscordNotifier),
		fx.Provide(notifier.NewNotifier),
		fx.Provide(events.NewEventsProducer),
		fx.Provide(fcm.NewFCMNotifier),

		// Rate limiter
		fx.Provide(utils.NewRateLimiter),

		// add email sender:
		fx.Provide(email.NewEmailSender),

		// MFA services
		fx.Provide(mfa.NewService),
		fx.Provide(mfa.NewCleanupService),

		// Auth services
		fx.Provide(auth.NewTokenService),
		fx.Provide(auth.NewCleanupService),

		fx.Provide(apple.NewAppleService),

		// add handlers also
		fx.Provide(newServer),
		fx.Provide(notifier.NewScheduler),

		// things
		fx.Provide(tRepo.NewThingRepository),

		// points
		fx.Provide(pRepo.NewPointsRepository),
		fx.Provide(spRepo.NewSubTasksRepository),

		// Labels:
		fx.Provide(lRepo.NewLabelRepository),
		fx.Provide(label.NewHandler),

		// Rewards:
		fx.Provide(rRepo.NewRewardRepository),
		fx.Provide(reward.NewHandler),
		fx.Provide(reward.NewAPI),

		// Projects:
		fx.Provide(pjRepo.NewProjectRepository),
		fx.Provide(project.NewHandler),
		fx.Provide(project.NewAPI),

		// Filters:
		fx.Provide(fRepo.NewFilterRepository),
		fx.Provide(filter.NewHandler),

		fx.Provide(thing.NewAPI),
		fx.Provide(thing.NewHandler),

		// External Only:
		fx.Provide(sService.NewStripeService,
			sRepo.NewStripeDB,
			sRepo.NewRevenueCatDB,
			sRepo.NewSubscriptionDB,
		),
		fx.Provide(payment.NewHandler),
		fx.Provide(payment.NewWebhook),
		fx.Provide(chore.NewAPI),

		// Frontend
		fx.Provide(frontend.NewHandler),

		// Docs
		fx.Provide(docs.NewHandler),

		// storage :
		// is storage local or remote?
		// fx.Provide(storage.NewLocalStorage),
		// fx.Provide(storage.NewURLSignerLocal),
		fx.Provide(storage.NewS3Storage),
		fx.Provide(storage.NewURLSignerS3),

		fx.Provide(storage.NewHandler),
		fx.Provide(storageRepo.NewStorageRepository),

		// backup service
		// fx.Provide(backup.NewService),
		// fx.Provide(backup.NewHandler),

		// Real-time service and components
		fx.Provide(realtime.NewRealTimeService),
		fx.Provide(realtime.NewAuthMiddleware),

		// MCP server

		// fx.Invoke(RunApp),
		fx.Invoke(
			chore.Routes,
			chore.APIs,
			user.Routes,
			circle.Routes,
			device.Routes,
			thing.Routes,
			thing.APIs,
			label.Routes,
			project.Routes,
			project.APIs,
			filter.Routes,
			reward.Routes,
			reward.APIs,

			storage.Routes,
			frontend.Routes,
			docs.Routes,
			resource.Routes,
			// backup.Routes,

			realtime.Routes, // (router, rts, authMiddleware, pollingHandler)

			func(r *gin.Engine) {},
		),
	)

	if err := app.Err(); err != nil {
		log.Fatal(err)
	}

	app.Run()

}

func newServer(lc fx.Lifecycle, cfg *config.Config, db *gorm.DB, notifier *notifier.Scheduler, eventProducer *events.EventsProducer, mfaCleanup *mfa.CleanupService, authCleanup *auth.CleanupService, rts *realtime.RealTimeService) *gin.Engine {
	// Set Gin mode based on logging configuration
	if cfg.Logging.Development || strings.ToLower(cfg.Logging.Level) == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// log when http request is made:

	r := gin.New()
	r.Use(gin.Recovery())

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	corsConfig := cors.DefaultConfig()
	allowOrigins := make(map[string]bool)
	for _, origin := range cfg.Server.CorsAllowOrigins {
		allowOrigins[origin] = true
	}
	corsConfig.AllowOriginFunc = func(origin string) bool {
		return allowOrigins[origin]
	}

	corsConfig.AllowCredentials = true
	// Add all headers that browsers commonly send
	corsConfig.AddAllowHeaders(
		"Authorization",
		"secretkey",
		"Cache-Control",
		"Content-Type",
		"Accept",
		"Sec-Ch-Ua",
		"Sec-Ch-Ua-Mobile",
		"Sec-Ch-Ua-Platform",
		"User-Agent",
		"Referer",
		"X-Impersonate-User-ID",
		"refresh_token",
	)
	// Expose headers that the frontend might need
	corsConfig.AddExposeHeaders("Content-Type")
	r.Use(cors.New(corsConfig))

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if cfg.Database.Migration {
				// Backup SQLite database before migrations
				if cfg.Database.Type == "sqlite" {
					if err := backupSQLiteDatabase(cfg); err != nil {
						logging.DefaultLogger().Warnf("failed to backup SQLite database: %v", err)
						// Don't block startup on backup failure
					}
				}

				database.Migration(db)
				migrations.Run(context.Background(), db)
				err := database.MigrationScripts(db, cfg)
				if err != nil {
					panic(err)
				}
			}
			notifier.Start(context.Background())
			eventProducer.Start(context.Background())
			mfaCleanup.Start(context.Background())
			authCleanup.Start(context.Background())

			// Start real-time service
			if err := rts.Start(ctx); err != nil {
				log.Printf("Failed to start real-time service: %v", err)
			}

			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("listen: %s\n", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			// Stop real-time service first with timeout
			done := make(chan error, 1)
			go func() {
				done <- rts.Stop()
			}()

			select {
			case err := <-done:
				if err != nil {
					log.Printf("Failed to stop real-time service: %v", err)
				}
			case <-time.After(5 * time.Second):
				log.Printf("Real-time service shutdown timeout, forcing shutdown")
			}

			mfaCleanup.Stop()
			authCleanup.Stop()

			// Shutdown HTTP server with timeout
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Printf("Server shutdown timeout: %s", err)
				// Force close
				srv.Close()
			}
			return nil
		},
	})

	// Simple health-check endpoint
	r.GET("/api/v1/health", func(c *gin.Context) {
		c.Status(http.StatusOK) // 200 - OK if server started
	})

	return r
}
