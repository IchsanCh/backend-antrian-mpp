package handler

import (
	"backend-antrian/internal/config"
	"github.com/gofiber/fiber/v2"
)

func GetGlobalQueue(c *fiber.Ctx) error {
	val, _ := config.Redis.Get(config.Ctx, "queue:global").Int64()

	return c.JSON(fiber.Map{
		"current_queue": val,
	})
}
