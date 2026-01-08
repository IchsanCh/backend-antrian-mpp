package handler

import (
	"fmt"

	"backend-antrian/internal/config"
	"github.com/gofiber/fiber/v2"
)

func TakeQueue(c *fiber.Ctx) error {
	type Request struct {
		UnitID    int `json:"unit_id"`
		ServiceID int `json:"service_id"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}

	serviceKey := fmt.Sprintf("queue:unit:%d:service:%d", req.UnitID, req.ServiceID)
	unitKey := fmt.Sprintf("queue:unit:%d:global", req.UnitID)
	globalKey := "queue:global"

	serviceNo, _ := config.Redis.Incr(config.Ctx, serviceKey).Result()
	unitNo, _ := config.Redis.Incr(config.Ctx, unitKey).Result()
	globalNo, _ := config.Redis.Incr(config.Ctx, globalKey).Result()

	return c.JSON(fiber.Map{
		"unit_id":        req.UnitID,
		"service_id":     req.ServiceID,
		"service_number": serviceNo,
		"unit_number":    unitNo,
		"global_number":  globalNo,
	})
}
