package handler

import "github.com/gofiber/fiber/v2"

func Logout(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "Logout berhasil",
	})
}
