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

	api := app.Group("/api", middleware.JWTAuth())
	api.Post("/logout", handler.Logout)
	

	// CRUD yang butuh super_user
	api.Post("/units", middleware.EmailRoleAuth("super_user"), handler.CreateUnit)
	api.Put("/units/:id", middleware.EmailRoleAuth("super_user"), handler.UpdateUnit)
	api.Delete("/units/:id", middleware.EmailRoleAuth("super_user"), handler.DeleteUnit)
	api.Delete("/units/:id/permanent", middleware.EmailRoleAuth("super_user"), handler.HardDeleteUnit)
	api.Post("/config", middleware.EmailRoleAuth("super_user"), handler.CreateConfig) 
	api.Put("/config", middleware.EmailRoleAuth("super_user"), handler.UpdateConfig)    


	api.Post("/queue/take", handler.TakeQueue)
	api.Get("/queue/unit/:unitId/service/:serviceId", handler.GetServiceQueue)
	api.Get("/queue/global", handler.GetGlobalQueue)

	addr := os.Getenv("APP_HOST") + ":" + os.Getenv("APP_PORT")
	log.Println("Server jalan di", addr)
	log.Fatal(app.Listen(addr))
}