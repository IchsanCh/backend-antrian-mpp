package main

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/http/handler"
	"backend-antrian/internal/http/middleware"
	"backend-antrian/internal/realtime"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	fiberRecover "github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	app := fiber.New(fiber.Config{
		BodyLimit:     50 * 1024 * 1024,
		Prefork:       false,
		CaseSensitive: true,
		StrictRouting: true,
		ReadTimeout:   30 * time.Second,
		WriteTimeout:  30 * time.Second,
		IdleTimeout:   120 * time.Second,
		// EnableTrustedProxyCheck: true,
		// TrustedProxies: []string{
		// 	"172.18.0.0/16",
		// },
		// ProxyHeader: fiber.HeaderXForwardedFor,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			log.Printf("[ERROR] %s %s - %v", c.Method(), c.Path(), err)
			return c.Status(code).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		},
	})

	config.LoadEnv()
	config.InitDB()
	defer config.CloseDB()

	// Recover middleware 
	app.Use(fiberRecover.New(fiberRecover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			log.Printf("[PANIC] %s %s - %v\n%s", c.Method(), c.Path(), e, debug.Stack())
		},
	}))

	app.Use(cors.New(cors.Config{
		AllowOrigins:  "*",
		AllowHeaders:  "Origin, Content-Type, Accept, Authorization",
		AllowMethods:  "GET, POST, PUT, DELETE, OPTIONS",
		ExposeHeaders: "Content-Disposition, Content-Type, Content-Length",
		AllowCredentials: false,
	}))
	//   app.Use(cors.New(cors.Config{
    //             AllowOrigins:  "https://sandigi.lotusaja.com",
    //             AllowHeaders:  "Origin, Content-Type, Accept, Authorization",
    //             AllowMethods:  "GET, POST, PUT, DELETE, OPTIONS",
    //             ExposeHeaders: "Content-Disposition, Content-Type, Content-Length",
    //             AllowCredentials: true,
    //     }))
	// Rate limiting untuk WebSocket
	app.Use("/ws/*", limiter.New(limiter.Config{
		Max:        1000,
		Expiration: 1 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			log.Printf("[RATE_LIMIT] IP: %s", c.IP())
			return c.Status(429).JSON(fiber.Map{
				"error": "Too many connections",
			})
		},
	}))
	
	app.Static("/", "./public")
	app.Use("/audio", filesystem.New(filesystem.Config{
		Root:   http.Dir("./public/audio"),
		Browse: false,
	}))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Antrian API jalan",
			"version": "1.0.0",
			"status":  "healthy",
		})
	})

	// Auth
	app.Post("/san/login", handler.Login)

	// Public endpoints
	app.Get("/api/units", handler.GetAllUnits)
	app.Get("/api/units/paginate", handler.GetAllUnitsPagination)
	app.Get("/api/units/:id", handler.GetUnitByID)
	app.Get("/api/config", handler.GetConfig)
	app.Get("/api/services/unit", handler.GetServicesByUnitID)
	app.Get("/api/services/unit/status", handler.GetServicesByUnitIDWithStatus)
	app.Get("/api/audio", handler.GetAllAudios)
	app.Get("/api/queue/display", handler.GetQueueDisplay)
	app.Get("/api/faqs", handler.GetAllFAQs)

	// WebSocket endpoints (public)
	app.Get("/ws/units", websocket.New(handler.UnitsWS))
	app.Get("/ws/queue", websocket.New(handler.QueueWebSocket))

	// Protected API
	api := app.Group("/api", middleware.JWTAuth())
	api.Post("/logout", handler.Logout)

	// SUPER ADMIN ROUTES
	api.Get("/users/paginate", middleware.RoleAuth("super_user"), handler.GetAllUsersPagination)
	api.Get("/users", middleware.RoleAuth("super_user"), handler.GetAllUsers)
	api.Get("/users/:id", middleware.RoleAuth("super_user"), handler.GetUserByID)
	api.Post("/users", middleware.RoleAuth("super_user"), handler.CreateUser)
	api.Put("/users/:id", middleware.RoleAuth("super_user"), handler.UpdateUser)
	api.Delete("/users/:id/permanent", middleware.RoleAuth("super_user"), handler.HardDeleteUser)

	api.Post("/queue/take", middleware.RoleAuth("super_user"), handler.TakeQueue)
	api.Post("/audio", middleware.RoleAuth("super_user"), handler.CreateAudio)
	api.Delete("/audio/:id", middleware.RoleAuth("super_user"), handler.DeleteAudio)

	api.Post("/units", middleware.RoleAuth("super_user"), handler.CreateUnit)
	api.Put("/units/:id", middleware.RoleAuth("super_user"), handler.UpdateUnit)
	api.Delete("/units/:id", middleware.RoleAuth("super_user"), handler.DeleteUnit)
	api.Delete("/units/:id/permanent", middleware.RoleAuth("super_user"), handler.HardDeleteUnit)

	api.Post("/config", middleware.RoleAuth("super_user"), handler.CreateConfig)
	api.Put("/config", middleware.RoleAuth("super_user"), handler.UpdateConfig)
	api.Get("/backup/database", middleware.RoleAuth("super_user"), handler.ExportDatabase)
	api.Get("/reports/visitors/export", middleware.RoleAuth("super_user"), handler.ExportVisitorReport)
	api.Get("/reports/visitors/statistics", middleware.RoleAuth("super_user"), handler.GetVisitorStatistics)

	api.Get("/faqs/paginate", middleware.RoleAuth("super_user"), handler.GetAllFAQsPagination)
	api.Get("/faqs/:id", middleware.RoleAuth("super_user"), handler.GetFAQByID)
	api.Post("/faqs", middleware.RoleAuth("super_user"), handler.CreateFAQ)
	api.Put("/faqs/:id", middleware.RoleAuth("super_user"), handler.UpdateFAQ)
	api.Delete("/faqs/:id", middleware.RoleAuth("super_user"), handler.HardDeleteFAQ)

	// UNIT ROLE ROUTES
	api.Get("/services", middleware.RoleAuth("unit"), handler.GetAllServices)
	api.Get("/services/paginate", middleware.RoleAuth("unit"), handler.GetAllServicesPagination)
	api.Get("/services/:id", middleware.RoleAuth("unit"), handler.GetServiceByID)
	api.Post("/services", middleware.RoleAuth("unit"), handler.CreateService)
	api.Put("/services/:id", middleware.RoleAuth("unit"), handler.UpdateService)
	api.Delete("/services/:id", middleware.RoleAuth("unit"), handler.DeleteService)
	api.Delete("/services/:id/permanent", middleware.RoleAuth("unit"), handler.HardDeleteService)

	api.Post("/queue/call-next", middleware.RoleAuth("unit"), handler.CallNextQueue)
	api.Post("/queue/skip-and-next", middleware.RoleAuth("unit"), handler.SkipAndNext)
	api.Post("/queue/update-status", middleware.RoleAuth("unit"), handler.UpdateQueueStatus)
	api.Post("/queue/recall/:id", middleware.RoleAuth("unit"), handler.RecallQueue)
	api.Get("/reports/unit/visitors/export", middleware.RoleAuth("unit"), handler.ExportUnitVisitorReport)
	api.Get("/reports/unit/visitors/statistics", middleware.RoleAuth("unit"), handler.GetUnitVisitorStatistics)
	api.Get("/dashboard/unit/statistics", middleware.RoleAuth("unit"), handler.GetUnitDashboardStatistics)

	// Background tasks
	go realtime.RunUnitsBroadcaster()

	addr := os.Getenv("APP_HOST") + ":" + os.Getenv("APP_PORT")
	log.Printf("Server starting on %s", addr)
	
	// Graceful error handling
	if err := app.Listen(addr); err != nil {
		log.Printf("Server failed to start: %v", err)
		os.Exit(1)
	}
}