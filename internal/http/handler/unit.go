package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"backend-antrian/internal/realtime"
	"database/sql"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// GetAllUnits - Ambil semua unit
func GetAllUnits(c *fiber.Ctx) error {
	isActive := c.Query("is_active")

	query := "SELECT id, code, nama_unit, is_active, main_display, audio_file, created_at, updated_at FROM units WHERE 1=1"
	args := []interface{}{}

	if isActive != "" {
		query += " AND is_active = ?"
		args = append(args, isActive)
	}

	query += " ORDER BY nama_unit ASC"

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
			&unit.MainDisplay,
			&unit.AudioFile,
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

// GetAllUnitsPagination - Ambil semua unit dengan pagination
func GetAllUnitsPagination(c *fiber.Ctx) error {
	isActive := c.Query("is_active")
	search := c.Query("search")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	// Validasi pagination
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Query untuk hitung total data
	countQuery := "SELECT COUNT(*) FROM units WHERE 1=1"
	countArgs := []interface{}{}

	if isActive != "" {
		countQuery += " AND is_active = ?"
		countArgs = append(countArgs, isActive)
	}

	if search != "" {
		search = "%" + strings.TrimSpace(search) + "%"
		countQuery += " AND (code LIKE ? OR nama_unit LIKE ?)"
		countArgs = append(countArgs, search, search)
	}

	var totalData int
	err := config.DB.QueryRow(countQuery, countArgs...).Scan(&totalData)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menghitung total data",
		})
	}

	// Query untuk ambil data dengan pagination
	query := "SELECT id, code, nama_unit, is_active, main_display, audio_file, created_at, updated_at FROM units WHERE 1=1"
	args := []interface{}{}

	if isActive != "" {
		query += " AND is_active = ?"
		args = append(args, isActive)
	}

	if search != "" {
		query += " AND (code LIKE ? OR nama_unit LIKE ?)"
		args = append(args, search, search)
	}

	query += " ORDER BY nama_unit ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

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
			&unit.MainDisplay,
			&unit.AudioFile,
			&unit.CreatedAt,
			&unit.UpdatedAt,
		)
		if err != nil {
			continue
		}
		units = append(units, unit)
	}

	// Hitung total pages
	totalPages := (totalData + limit - 1) / limit

	return c.JSON(fiber.Map{
		"success": true,
		"data":    units,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total_data":  totalData,
			"total_pages": totalPages,
		},
	})
}

// GetUnitByID - Ambil unit berdasarkan ID
func GetUnitByID(c *fiber.Ctx) error {
	id := c.Params("id")

	var unit models.Unit
	query := "SELECT id, code, nama_unit, is_active, main_display, audio_file, created_at, updated_at FROM units WHERE id = ?"

	err := config.DB.QueryRow(query, id).Scan(
		&unit.ID,
		&unit.Code,
		&unit.NamaUnit,
		&unit.IsActive,
		&unit.MainDisplay,
		&unit.AudioFile,
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
	var req struct {
		Code        string  `json:"code"`
		NamaUnit    string  `json:"nama_unit"`
		IsActive    string  `json:"is_active"`
		MainDisplay string  `json:"main_display"`
		AudioFile   *string `json:"audio_file"`
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
	
	// Normalisasi code
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))

	// Validasi: hanya huruf A-Z, panjang 1–10
	re := regexp.MustCompile(`^[A-Z]{1,10}$`)
	if !re.MatchString(req.Code) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Code unit harus 1–10 huruf dan tanpa angka atau karakter khusus",
		})
	}

	// Set default is_active jika kosong
	if req.IsActive == "" {
		req.IsActive = "y"
	}
	// Set default main_display jika kosong
	if req.MainDisplay == "" {
		req.MainDisplay = "active"
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
	query := "INSERT INTO units (code, nama_unit, is_active, main_display, audio_file) VALUES (?, ?, ?, ?, ?)"
	result, err := config.DB.Exec(query, req.Code, req.NamaUnit, req.IsActive, req.MainDisplay, req.AudioFile)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membuat unit",
		})
	}

	id, _ := result.LastInsertId()

	// Ambil data yang baru dibuat
	var unit models.Unit
	config.DB.QueryRow(
		"SELECT id, code, nama_unit, is_active, main_display, audio_file, created_at, updated_at FROM units WHERE id = ?",
		id,
	).Scan(&unit.ID, &unit.Code, &unit.NamaUnit, &unit.IsActive, &unit.MainDisplay, &unit.AudioFile, &unit.CreatedAt, &unit.UpdatedAt)
	
	broadcastUnitsUpdate()

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Unit berhasil dibuat",
		"data":    unit,
	})
}

// UpdateUnit - Update unit berdasarkan ID
func UpdateUnit(c *fiber.Ctx) error {
	id := c.Params("id")

	var req struct {
		Code        string  `json:"code"`
		NamaUnit    string  `json:"nama_unit"`
		IsActive    string  `json:"is_active"`
		MainDisplay string  `json:"main_display"`
		AudioFile   *string `json:"audio_file"`
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
		req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
		re := regexp.MustCompile(`^[A-Z]{1,10}$`)
		if !re.MatchString(req.Code) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Code unit harus 1–10 huruf dan tanpa angka atau karakter khusus",
			})
		}
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
	
	if req.MainDisplay != "" {
		updates = append(updates, "main_display = ?")
		args = append(args, req.MainDisplay)
	}

	// AudioFile bisa di-set null jika dikirim sebagai empty string
	if req.AudioFile != nil {
		updates = append(updates, "audio_file = ?")
		if *req.AudioFile == "" {
			args = append(args, nil)
		} else {
			args = append(args, *req.AudioFile)
		}
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
		"SELECT id, code, nama_unit, is_active, main_display, audio_file, created_at, updated_at FROM units WHERE id = ?",
		id,
	).Scan(&unit.ID, &unit.Code, &unit.NamaUnit, &unit.IsActive, &unit.MainDisplay, &unit.AudioFile, &unit.CreatedAt, &unit.UpdatedAt)
	
	broadcastUnitsUpdate()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Unit berhasil diupdate",
		"data":    unit,
	})
}

// DeleteUnit - Hapus unit (soft delete)
func DeleteUnit(c *fiber.Ctx) error {
	id := c.Params("id")

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
	
	broadcastUnitsUpdate()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Unit berhasil dihapus",
	})
}

// HardDeleteUnit - Hapus unit permanent
func HardDeleteUnit(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID unit tidak valid",
		})
	}

	// Langsung hapus, biar database yang handle foreign key constraint
	result, err := config.DB.Exec("DELETE FROM units WHERE id = ?", id)
	if err != nil {
		// Cek apakah error karena foreign key constraint
		if strings.Contains(err.Error(), "foreign key constraint") || 
		   strings.Contains(err.Error(), "FOREIGN KEY") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Unit tidak dapat dihapus karena masih digunakan oleh data lain",
			})
		}
		
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
	
	broadcastUnitsUpdate()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Unit berhasil dihapus permanent",
	})
}

func broadcastUnitsUpdate() {
	rows, err := config.DB.Query(`
		SELECT id, code, nama_unit, is_active, main_display, audio_file, created_at, updated_at
		FROM units
		ORDER BY nama_unit ASC
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	units := []models.Unit{}
	for rows.Next() {
		var unit models.Unit
		if err := rows.Scan(
			&unit.ID,
			&unit.Code,
			&unit.NamaUnit,
			&unit.IsActive,
			&unit.MainDisplay,
			&unit.AudioFile,
			&unit.CreatedAt,
			&unit.UpdatedAt,
		); err == nil {
			units = append(units, unit)
		}
	}

	payload, _ := json.Marshal(fiber.Map{
		"type": "units_updated",
		"data": units,
	})

	realtime.Units.Broadcast <- payload
}