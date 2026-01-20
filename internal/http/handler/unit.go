package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// GetAllUnits - Ambil semua unit
func GetAllUnits(c *fiber.Ctx) error {
	isActive := c.Query("is_active")

	query := "SELECT id, code, nama_unit, is_active, created_at, updated_at FROM units WHERE 1=1"
	args := []interface{}{}

	if isActive != "" {
		query += " AND is_active = ?"
		args = append(args, isActive)
	}

	query += " ORDER BY created_at DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data unit",
		})
	}
	defer rows.Close()

	units := []models.Unit{}
	for rows.Next() {
		var unit models.Unit
		err := rows.Scan(
			&unit.ID,
			&unit.Code,
			&unit.NamaUnit,
			&unit.IsActive,
			&unit.CreatedAt,
			&unit.UpdatedAt,
		)
		if err != nil {
			continue
		}
		units = append(units, unit)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    units,
	})
}

// GetUnitByID - Ambil unit berdasarkan ID
func GetUnitByID(c *fiber.Ctx) error {
	id := c.Params("id")

	var unit models.Unit
	query := "SELECT id, code, nama_unit, is_active, created_at, updated_at FROM units WHERE id = ?"

	err := config.DB.QueryRow(query, id).Scan(
		&unit.ID,
		&unit.Code,
		&unit.NamaUnit,
		&unit.IsActive,
		&unit.CreatedAt,
		&unit.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Unit tidak ditemukan",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data unit",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    unit,
	})
}

// CreateUnit - Buat unit baru
func CreateUnit(c *fiber.Ctx) error {
	// Role sudah divalidasi di middleware, langsung parse request
	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		NamaUnit string `json:"nama_unit"`
		IsActive string `json:"is_active"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validasi input unit
	if req.Code == "" || req.NamaUnit == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Code dan nama unit wajib diisi",
		})
	}

	// Set default is_active jika kosong
	if req.IsActive == "" {
		req.IsActive = "y"
	}

	// Cek apakah code sudah ada
	var count int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM units WHERE code = ?", req.Code).Scan(&count)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal validasi code",
		})
	}

	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Code unit sudah digunakan",
		})
	}

	// Insert ke database
	query := "INSERT INTO units (code, nama_unit, is_active) VALUES (?, ?, ?)"
	result, err := config.DB.Exec(query, req.Code, req.NamaUnit, req.IsActive)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membuat unit",
		})
	}

	id, _ := result.LastInsertId()

	// Ambil data yang baru dibuat
	var unit models.Unit
	config.DB.QueryRow(
		"SELECT id, code, nama_unit, is_active, created_at, updated_at FROM units WHERE id = ?",
		id,
	).Scan(&unit.ID, &unit.Code, &unit.NamaUnit, &unit.IsActive, &unit.CreatedAt, &unit.UpdatedAt)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Unit berhasil dibuat",
		"data":    unit,
	})
}

// UpdateUnit - Update unit berdasarkan ID
func UpdateUnit(c *fiber.Ctx) error {
	id := c.Params("id")

	// Role sudah divalidasi di middleware
	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		NamaUnit string `json:"nama_unit"`
		IsActive string `json:"is_active"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Cek apakah unit ada
	var exists int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM units WHERE id = ?", id).Scan(&exists)
	if err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Unit tidak ditemukan",
		})
	}

	// Build dynamic update query
	query := "UPDATE units SET "
	args := []interface{}{}
	updates := []string{}

	if req.Code != "" {
		var count int
		config.DB.QueryRow("SELECT COUNT(*) FROM units WHERE code = ? AND id != ?", req.Code, id).Scan(&count)
		if count > 0 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Code unit sudah digunakan",
			})
		}
		updates = append(updates, "code = ?")
		args = append(args, req.Code)
	}

	if req.NamaUnit != "" {
		updates = append(updates, "nama_unit = ?")
		args = append(args, req.NamaUnit)
	}

	if req.IsActive != "" {
		updates = append(updates, "is_active = ?")
		args = append(args, req.IsActive)
	}

	if len(updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Tidak ada data yang diupdate",
		})
	}

	for i, update := range updates {
		if i > 0 {
			query += ", "
		}
		query += update
	}
	query += " WHERE id = ?"
	args = append(args, id)

	_, err = config.DB.Exec(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengupdate unit",
		})
	}

	var unit models.Unit
	config.DB.QueryRow(
		"SELECT id, code, nama_unit, is_active, created_at, updated_at FROM units WHERE id = ?",
		id,
	).Scan(&unit.ID, &unit.Code, &unit.NamaUnit, &unit.IsActive, &unit.CreatedAt, &unit.UpdatedAt)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Unit berhasil diupdate",
		"data":    unit,
	})
}

// DeleteUnit - Hapus unit (soft delete)
func DeleteUnit(c *fiber.Ctx) error {
	id := c.Params("id")

	// Role sudah divalidasi di middleware
	var req struct {
		Email string `json:"email"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Cek apakah unit ada
	var exists int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM units WHERE id = ?", id).Scan(&exists)
	if err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Unit tidak ditemukan",
		})
	}

	// Soft delete
	_, err = config.DB.Exec("UPDATE units SET is_active = 'n' WHERE id = ?", id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menghapus unit",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Unit berhasil dihapus",
	})
}

// HardDeleteUnit - Hapus unit permanent
func HardDeleteUnit(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)

	// Role sudah divalidasi di middleware
	var req struct {
		Email string `json:"email"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	result, err := config.DB.Exec("DELETE FROM units WHERE id = ?", id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menghapus unit",
		})
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Unit tidak ditemukan",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Unit berhasil dihapus permanent",
	})
}