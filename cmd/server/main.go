package main

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/http/handler"
	"backend-antrian/internal/http/middleware"
	"backend-antrian/internal/realtime"
	"log"
	"os"
	"runtime"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	app := fiber.New(fiber.Config{
		Prefork:       false,
		CaseSensitive: true,
		StrictRouting: true,
	})
	
	config.LoadEnv()
	config.InitRedis()
	config.InitDB()
	defer config.CloseDB()

	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE",
		ExposeHeaders: "Content-Disposition, Content-Type, Content-Length",
	}))

	// Static files untuk audio
	app.Static("/audio", "./public/audio")

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Antrian API jalan",
		})
	})
	
	// Auth
	app.Post("/san/login", handler.Login)

	// Public endpoints (no auth)
	app.Get("/api/units", handler.GetAllUnits)
	app.Get("/api/units/paginate", handler.GetAllUnitsPagination)
	app.Get("/api/units/:id", handler.GetUnitByID)
	app.Get("/api/config", handler.GetConfig)
	app.Get("/api/services/unit", handler.GetServicesByUnitID)
	app.Get("/api/services/unit/status", handler.GetServicesByUnitIDWithStatus)
	app.Get("/api/audio", handler.GetAllAudios)
	
	// Queue public endpoints
	app.Get("/api/queue/display", handler.GetQueueDisplay)
	

	// WebSocket endpoints (public)
	app.Get("/ws/units", websocket.New(handler.UnitsWS))
	app.Get("/ws/queue", websocket.New(handler.QueueWebSocket))

	// Base API (semua wajib login)
	api := app.Group("/api", middleware.JWTAuth())

	// Auth
	api.Post("/logout", handler.Logout)

	// SUPER ADMIN ROUTES 
	// Users
	api.Get("/users/paginate", middleware.RoleAuth("super_user"), handler.GetAllUsersPagination)
	api.Get("/users", middleware.RoleAuth("super_user"), handler.GetAllUsers)
	api.Get("/users/:id", middleware.RoleAuth("super_user"), handler.GetUserByID)
	api.Post("/users", middleware.RoleAuth("super_user"), handler.CreateUser)
	api.Put("/users/:id", middleware.RoleAuth("super_user"), handler.UpdateUser)
	api.Delete("/users/:id/permanent", middleware.RoleAuth("super_user"), handler.HardDeleteUser)

	// Queue
	api.Post("/queue/take", middleware.RoleAuth("super_user"), handler.TakeQueue)
	
	//Audio
	api.Post("/audio", middleware.RoleAuth("super_user"), handler.CreateAudio)
	api.Delete("/audio/:id", middleware.RoleAuth("super_user"), handler.DeleteAudio)

	// Units  
	api.Post("/units", middleware.RoleAuth("super_user"), handler.CreateUnit)
	api.Put("/units/:id", middleware.RoleAuth("super_user"), handler.UpdateUnit)
	api.Delete("/units/:id", middleware.RoleAuth("super_user"), handler.DeleteUnit)
	api.Delete("/units/:id/permanent", middleware.RoleAuth("super_user"), handler.HardDeleteUser)

	// Config
	api.Post("/config", middleware.RoleAuth("super_user"), handler.CreateConfig)
	api.Put("/config", middleware.RoleAuth("super_user"), handler.UpdateConfig)
	
	//backup db
	api.Get("/backup/database", middleware.RoleAuth("super_user"), handler.ExportDatabase)
	
	//reports
	api.Get("/reports/visitors/export", middleware.RoleAuth("super_user"), handler.ExportVisitorReport)
	api.Get("/reports/visitors/statistics", middleware.RoleAuth("super_user"), handler.GetVisitorStatistics)
	
	// UNIT ROLE ROUTES  
	// Services
	api.Get("/services", middleware.RoleAuth("unit"), handler.GetAllServices)
	api.Get("/services/paginate", middleware.RoleAuth("unit"), handler.GetAllServicesPagination)
	api.Get("/services/:id", middleware.RoleAuth("unit"), handler.GetServiceByID)
	api.Post("/services", middleware.RoleAuth("unit"), handler.CreateService)
	api.Put("/services/:id", middleware.RoleAuth("unit"), handler.UpdateService)
	api.Delete("/services/:id", middleware.RoleAuth("unit"), handler.DeleteService)
	api.Delete("/services/:id/permanent", middleware.RoleAuth("unit"), handler.HardDeleteService)

	// Queue management (unit role)
	api.Post("/queue/call-next", middleware.RoleAuth("unit"), handler.CallNextQueue)
	api.Post("/queue/skip-and-next", middleware.RoleAuth("unit"), handler.SkipAndNext)
	api.Post("/queue/update-status", middleware.RoleAuth("unit"), handler.UpdateQueueStatus)
	api.Post("/queue/recall/:id", middleware.RoleAuth("unit"), handler.RecallQueue)
	
	//Reports
	api.Get("/reports/unit/visitors/export", middleware.RoleAuth("unit"), handler.ExportUnitVisitorReport)
	api.Get("/reports/unit/visitors/statistics", middleware.RoleAuth("unit"), handler.GetUnitVisitorStatistics)

	// Dashboard
	api.Get("/dashboard/unit/statistics", middleware.RoleAuth("unit"), handler.GetUnitDashboardStatistics)

	// Background tasks
	go realtime.RunUnitsBroadcaster()

	addr := os.Getenv("APP_HOST") + ":" + os.Getenv("APP_PORT")
	log.Println("Server jalan di", addr)
	log.Fatal(app.Listen(addr))
}