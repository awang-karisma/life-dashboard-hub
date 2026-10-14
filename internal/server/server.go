package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"golang.org/x/time/rate"
	"github.com/gofiber/fiber/v3/middleware/recover"
	_ "github.com/owen282000/life-dashboard-hub/docs"
	"github.com/owen282000/life-dashboard-hub/internal/auth"
	"github.com/owen282000/life-dashboard-hub/internal/config"
	"github.com/owen282000/life-dashboard-hub/internal/database"
	"github.com/owen282000/life-dashboard-hub/internal/handlers"
	"github.com/owen282000/life-dashboard-hub/internal/middleware"
	"github.com/owen282000/life-dashboard-hub/internal/resolver"
)

type Server struct {
	App            *fiber.App
	Cfg            *config.Config
	DB             *database.DB
	SessionService *auth.SessionService
	OIDCClient     *auth.OIDCClient
	Resolver       *resolver.PackageResolver
	Bus            *handlers.IngestionBus
}

func New(cfg *config.Config, db *database.DB) *Server {
	app := fiber.New(fiber.Config{
		BodyLimit: int(cfg.MaxBodyBytes),
	})

	// Global middlewares
	app.Use(recover.New())
	app.Use(middleware.Logger())
	app.Use(middleware.SecurityHeaders())
	app.Use(cors.New(cors.Config{
		AllowCredentials: true,
		AllowOriginsFunc: func(origin string) bool {
			return true
		},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Signature"},
	}))

	sessionService := auth.NewSessionService(db)
	oidcClient := auth.NewOIDCClient(context.Background(), cfg)

	bus := handlers.NewIngestionBus()
	s := &Server{
		App:            app,
		Cfg:            cfg,
		DB:             db,
		SessionService: sessionService,
		OIDCClient:     oidcClient,
		Resolver:       resolver.NewPackageResolver(db),
		Bus:            bus,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Swagger UI
	s.App.Get("/swagger/*", swaggo.HandlerDefault)

	// Health check
	s.App.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "time": time.Now().UTC()})
	})

	// Route groups
	// Rate limiters
	authLimiter := middleware.NewRateLimiter(rate.Every(6*time.Second), 5) // ~10/min
	webhookLimiter := middleware.NewRateLimiter(rate.Limit(1000.0/60.0), 50) // ~1000/min

	// Route groups
	api := s.App.Group("/api/v1")
	webhooks := s.App.Group("/webhooks/v1", webhookLimiter.Handler())

	// Webhook handler
	webhookHandler := handlers.NewWebhookHandler(s.DB, s.Cfg.MaxBodyBytes, s.Cfg.TZ, s.Bus)
	webhooks.Post("/devices/:deviceToken", webhookHandler.HandleWebhook)

	// Auth handlers
	authHandler := handlers.NewAuthHandler(s.DB, s.SessionService, s.OIDCClient, s.Cfg.PublicURL)
	authGroup := api.Group("/auth")
	authGroup.Get("/info", authHandler.GetAuthInfo)
	authGroup.Post("/register", authLimiter.Handler(), authHandler.Register)
	authGroup.Post("/login", authLimiter.Handler(), authHandler.Login)
	authGroup.Post("/logout", authHandler.Logout)
	authGroup.Get("/oidc/login", authHandler.OIDCLogin)
	authGroup.Get("/oidc/callback", authHandler.OIDCCallback)
	// Protected API routes
	protected := api.Group("", middleware.RequireAuth(s.SessionService))

	// Device handlers
	devicesHandler := handlers.NewDevicesHandler(s.DB, s.Cfg.PublicURL)
	devicesGroup := protected.Group("/devices")
	devicesGroup.Post("", devicesHandler.CreateDevice)
	devicesGroup.Get("", devicesHandler.ListDevices)
	devicesGroup.Get("/:id", devicesHandler.GetDevice)
	devicesGroup.Patch("/:id", devicesHandler.UpdateDevice)
	devicesGroup.Delete("/:id", devicesHandler.DeleteDevice)
	devicesGroup.Post("/:id/revoke", devicesHandler.RevokeDevice)
	devicesGroup.Post("/:id/regenerate", devicesHandler.RegenerateDeviceCredentials)

	// User profile handlers
	// Reports handlers
	reportsHandler := handlers.NewReportsHandler(s.DB, s.Resolver)
	reportsGroup := protected.Group("/reports")
	reportsGroup.Get("/screen-time/daily", reportsHandler.GetScreenTimeDaily)
	reportsGroup.Get("/screen-time/apps", reportsHandler.GetScreenTimeApps)
	reportsGroup.Post("/screen-time/aliases", reportsHandler.SetAppAlias)
	reportsGroup.Get("/health/daily", reportsHandler.GetHealthDailyTotals)
	reportsGroup.Get("/health/trends", reportsHandler.GetHealthTrends)
	reportsGroup.Get("/health/sleep", reportsHandler.GetSleep)
	reportsGroup.Get("/sync-status", reportsHandler.GetSyncStatus)

	// Data export and deletion handlers
	dataGroup := protected.Group("/data")
	dataGroup.Get("/export", reportsHandler.ExportData)
	dataGroup.Delete("", reportsHandler.DeleteData)
	// Families handlers
	familiesHandler := handlers.NewFamiliesHandler(s.DB, reportsHandler)
	familiesGroup := protected.Group("/families")
	familiesGroup.Post("", familiesHandler.CreateFamily)
	familiesGroup.Get("", familiesHandler.ListFamilies)
	familiesGroup.Get("/:id", familiesHandler.GetFamily)
	familiesGroup.Patch("/:id", familiesHandler.UpdateFamily)
	familiesGroup.Delete("/:id", familiesHandler.DeleteFamily)
	familiesGroup.Get("/:id/settings", familiesHandler.GetFamilySettings)
	familiesGroup.Patch("/:id/settings", familiesHandler.UpdateFamilySettings)
	familiesGroup.Post("/:id/invitations", familiesHandler.InviteMember)
	familiesGroup.Post("/:id/children", familiesHandler.CreateChildAccount)
	familiesGroup.Patch("/:familyId/members/:userId/sharing", familiesHandler.UpdateSharing)
	familiesGroup.Post("/:familyId/leave", familiesHandler.LeaveFamily)
	familiesGroup.Delete("/:familyId/members/:userId", familiesHandler.RemoveMember)
	familiesGroup.Get("/:familyId/members/:userId/reports/screen-time/daily", familiesHandler.GetMemberScreenTimeDaily)
	familiesGroup.Get("/:familyId/members/:userId/reports/screen-time/apps", familiesHandler.GetMemberScreenTimeApps)
	familiesGroup.Get("/:familyId/members/:userId/reports/health/daily", familiesHandler.GetMemberHealthDaily)

	protected.Post("/invitations/:token/accept", familiesHandler.AcceptInvitation)

	usersHandler := handlers.NewUsersHandler(s.DB)
	protected.Get("/users/me", usersHandler.GetCurrentUser)
	protected.Patch("/users/me", usersHandler.UpdateCurrentUser)

	// Events handler (SSE)
	eventsHandler := handlers.NewEventsHandler(s.Bus)
	protected.Get("/events/ingestion", eventsHandler.StreamIngestion)
	// Mount frontend if embedded (registered last so API routes take precedence)
	MountFrontend(s.App)
}

func (s *Server) Start(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		slog.Info("Starting server", "addr", s.Cfg.ListenAddr)
		if err := s.App.Listen(s.Cfg.ListenAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.App.ShutdownWithContext(shutdownCtx)
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}
}
