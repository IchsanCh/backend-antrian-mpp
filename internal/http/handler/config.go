package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"database/sql"
	"regexp"

	"github.com/gofiber/fiber/v2"
)

func GetConfig(c *fiber.Ctx) error {
	var cfg models.Config
	query := "SELECT id, jam_buka, jam_tutup FROM configs LIMIT 1"

	err := config.DB.QueryRow(query).Scan(
		&cfg.ID,
		&cfg.JamBuka,
		&cfg.JamTutup,
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
	// Role sudah divalidasi di middleware (super_user only)
	var req struct {
		Email    string `json:"email"`
		JamBuka  string `json:"jam_buka"`
		JamTutup string `json:"jam_tutup"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validasi input
	if req.JamBuka == "" || req.JamTutup == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Jam buka dan jam tutup wajib diisi",
		})
	}

	// Validasi format waktu (HH:MM:SS)
	timeRegex := regexp.MustCompile(`^([0-1][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$`)
	if !timeRegex.MatchString(req.JamBuka) || !timeRegex.MatchString(req.JamTutup) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Format waktu harus HH:MM:SS (contoh: 08:00:00)",
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

	// Insert ke database
	query := "INSERT INTO configs (jam_buka, jam_tutup) VALUES (?, ?)"
	result, err := config.DB.Exec(query, req.JamBuka, req.JamTutup)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membuat konfigurasi",
		})
	}

	id, _ := result.LastInsertId()

	// Ambil data yang baru dibuat
	var cfg models.Config
	config.DB.QueryRow(
		"SELECT id, jam_buka, jam_tutup FROM configs WHERE id = ?",
		id,
	).Scan(&cfg.ID, &cfg.JamBuka, &cfg.JamTutup)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Konfigurasi berhasil dibuat",
		"data":    cfg,
	})
}

// UpdateConfig - Update konfigurasi yang sudah ada
func UpdateConfig(c *fiber.Ctx) error {
	// Role sudah divalidasi di middleware (super_user only)
	var req struct {
		Email    string `json:"email"`
		JamBuka  string `json:"jam_buka"`
		JamTutup string `json:"jam_tutup"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validasi input
	if req.JamBuka == "" || req.JamTutup == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Jam buka dan jam tutup wajib diisi",
		})
	}

	// Validasi format waktu (HH:MM:SS)
	timeRegex := regexp.MustCompile(`^([0-1][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$`)
	if !timeRegex.MatchString(req.JamBuka) || !timeRegex.MatchString(req.JamTutup) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Format waktu harus HH:MM:SS (contoh: 08:00:00)",
		})
	}

	// Cek apakah ada data yang bisa diupdate
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

	// Update data
	query := "UPDATE configs SET jam_buka = ?, jam_tutup = ? WHERE id = ?"
	_, err = config.DB.Exec(query, req.JamBuka, req.JamTutup, configID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengupdate konfigurasi",
		})
	}

	// Ambil data yang sudah diupdate
	var cfg models.Config
	config.DB.QueryRow(
		"SELECT id, jam_buka, jam_tutup FROM configs WHERE id = ?",
		configID,
	).Scan(&cfg.ID, &cfg.JamBuka, &cfg.JamTutup)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Konfigurasi berhasil diupdate",
		"data":    cfg,
	})
}