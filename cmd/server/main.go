package main

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/http/handler"
	"backend-antrian/internal/http/middleware"
	"log"
	"os"
	"runtime"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
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
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Antrian API jalan",
		})
	})
	
	app.Post("/san/login", handler.Login)
	app.Get("/api/units", handler.GetAllUnits)
	app.Get("/api/units/paginate", handler.GetAllUnitsPagination)
	app.Get("/api/units/:id", handler.GetUnitByID)
	app.Get("/api/config", handler.GetConfig)
	app.Get("/api/services/unit", handler.GetServicesByUnitID)

	// Base API (semua wajib login)
	api := app.Group("/api", middleware.JWTAuth())

	// Auth
	api.Post("/logout", handler.Logout)
	// Queue endpoints (accessible by both roles)
	api.Post("/queue/take", handler.TakeQueue)
	api.Get("/queue/unit/:unitId/service/:serviceId", handler.GetServiceQueue)
	api.Get("/queue/global", handler.GetGlobalQueue)

	// ===== SUPER ADMIN ROUTES =====
	// Users
	api.Get("/users/paginate", middleware.RoleAuth("super_user"), handler.GetAllUsersPagination)
	api.Get("/users", middleware.RoleAuth("super_user"), handler.GetAllUsers)
	api.Get("/users/:id", middleware.RoleAuth("super_user"), handler.GetUserByID)
	api.Post("/users", middleware.RoleAuth("super_user"), handler.CreateUser)
	api.Put("/users/:id", middleware.RoleAuth("super_user"), handler.UpdateUser)
	api.Delete("/users/:id/permanent", middleware.RoleAuth("super_user"), handler.HardDeleteUser)

	// Units  
	api.Post("/units", middleware.RoleAuth("super_user"), handler.CreateUnit)
	api.Put("/units/:id", middleware.RoleAuth("super_user"), handler.UpdateUnit)
	api.Delete("/units/:id", middleware.RoleAuth("super_user"), handler.DeleteUnit)
	api.Delete("/units/:id/permanent", middleware.RoleAuth("super_user"), handler.HardDeleteUser)

	// Config
	api.Post("/config", middleware.RoleAuth("super_user"), handler.CreateConfig)
	api.Put("/config", middleware.RoleAuth("super_user"), handler.UpdateConfig)

	// ===== UNIT ROLE ROUTES  =====
	// Services
	api.Get("/services", middleware.RoleAuth("unit"), handler.GetAllServices)
	api.Get("/services/paginate", middleware.RoleAuth("unit"), handler.GetAllServicesPagination)
	api.Get("/services/:id", middleware.RoleAuth("unit"), handler.GetServiceByID)
	api.Post("/services", middleware.RoleAuth("unit"), handler.CreateService)
	api.Put("/services/:id", middleware.RoleAuth("unit"), handler.UpdateService)
	api.Delete("/services/:id", middleware.RoleAuth("unit"), handler.DeleteService)
	api.Delete("/services/:id/permanent", middleware.RoleAuth("unit"), handler.HardDeleteService)

	addr := os.Getenv("APP_HOST") + ":" + os.Getenv("APP_PORT")
	log.Println("Server jalan di", addr)
	log.Fatal(app.Listen(addr))
}