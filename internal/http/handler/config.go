package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"database/sql"

	"github.com/gofiber/fiber/v2"
)

func GetConfig(c *fiber.Ctx) error {
	var cfg models.Config
	query := "SELECT id, text_marque FROM configs LIMIT 1"

	err := config.DB.QueryRow(query).Scan(
		&cfg.ID,
		&cfg.TextMarque,
	)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Konfigurasi belum diatur",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data konfigurasi",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    cfg,
	})
}

// CreateConfig - Buat konfigurasi baru (hanya jika belum ada)
func CreateConfig(c *fiber.Ctx) error {
	var req struct {
		TextMarque string `json:"text_marque"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.TextMarque == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Text marque wajib diisi",
		})
	}

	// Cek apakah sudah ada data
	var count int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM configs").Scan(&count)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal validasi konfigurasi",
		})
	}

	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Konfigurasi sudah ada, gunakan update untuk mengubah",
		})
	}

	result, err := config.DB.Exec("INSERT INTO configs (text_marque) VALUES (?)", req.TextMarque)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membuat konfigurasi",
		})
	}

	id, _ := result.LastInsertId()

	var cfg models.Config
	config.DB.QueryRow("SELECT id, text_marque FROM configs WHERE id = ?", id).
		Scan(&cfg.ID, &cfg.TextMarque)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Konfigurasi berhasil dibuat",
		"data":    cfg,
	})
}

// UpdateConfig - Update konfigurasi yang sudah ada
func UpdateConfig(c *fiber.Ctx) error {
	var req struct {
		TextMarque string `json:"text_marque"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.TextMarque == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Text marque wajib diisi",
		})
	}

	var configID int64
	err := config.DB.QueryRow("SELECT id FROM configs LIMIT 1").Scan(&configID)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Konfigurasi belum dibuat, gunakan create terlebih dahulu",
		})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data konfigurasi",
		})
	}

	_, err = config.DB.Exec("UPDATE configs SET text_marque = ? WHERE id = ?", req.TextMarque, configID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengupdate konfigurasi",
		})
	}

	var cfg models.Config
	config.DB.QueryRow("SELECT id, text_marque FROM configs WHERE id = ?", configID).
		Scan(&cfg.ID, &cfg.TextMarque)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Konfigurasi berhasil diupdate",
		"data":    cfg,
	})
}