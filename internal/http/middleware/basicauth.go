package middleware

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
)

func BasicAuth() fiber.Handler {
	return basicauth.New(basicauth.Config{
		Users: map[string]string{
			os.Getenv("BASIC_AUTH_USER"): os.Getenv("BASIC_AUTH_PASS"),
		},
		Unauthorized: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		},
	})
}
