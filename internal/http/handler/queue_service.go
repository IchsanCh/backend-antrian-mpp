package handler

import (
	"fmt"

	"backend-antrian/internal/config"
	"github.com/gofiber/fiber/v2"
)

func GetServiceQueue(c *fiber.Ctx) error {
	unitID := c.Params("unitId")
	serviceID := c.Params("serviceId")

	key := fmt.Sprintf("queue:unit:%s:service:%s", unitID, serviceID)
	val, _ := config.Redis.Get(config.Ctx, key).Int64()

	return c.JSON(fiber.Map{
		"current_queue": val,
	})
}

