package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/helper"
	"backend-antrian/internal/models"
	"backend-antrian/internal/realtime"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

func UnitsWS(c *websocket.Conn) {
	realtime.Units.Register <- c
	defer func() {
		realtime.Units.Unregister <- c
	}()

	var cfg models.Config
	err := config.DB.QueryRow(`
		SELECT jam_buka, jam_tutup, text_marque
		FROM configs
		LIMIT 1
	`).Scan(&cfg.JamBuka, &cfg.JamTutup, &cfg.TextMarque)

	if err != nil {
		_ = c.WriteJSON(fiber.Map{
			"type":    "error",
			"message": "Konfigurasi antrian belum diatur",
		})
		return
	}

	if !helper.IsQueueOpen(cfg.JamBuka, cfg.JamTutup) {
		_ = c.WriteJSON(fiber.Map{
			"type":      "status",
			"queue":     "closed",
			"message":   "Antrian belum dibuka",
			"jam_buka":  cfg.JamBuka,
			"jam_tutup": cfg.JamTutup,
		})
	}

	// listen client
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}
}

