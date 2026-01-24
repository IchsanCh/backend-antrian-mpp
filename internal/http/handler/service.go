package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// GetAllServices - Ambil semua service (untuk super_user bisa lihat semua, untuk unit hanya miliknya)
func GetAllServices(c *fiber.Ctx) error {
	isActive := c.Query("is_active")

	query := `
		SELECT 
			id, unit_id, nama_service, code, loket, limits_queue, 
			is_active, created_at, updated_at
		FROM services
		WHERE 1=1
	`
	args := []interface{}{}

	// Filter aktif / tidak aktif (opsional)
	if isActive != "" {
		query += " AND is_active = ?"
		args = append(args, isActive)
	}

	query += " ORDER BY created_at DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data service",
		})
	}
	defer rows.Close()

	services := []models.Service{}
	for rows.Next() {
		var service models.Service
		if err := rows.Scan(
			&service.ID,
			&service.UnitID,
			&service.NamaService,
			&service.Code,
			&service.Loket,
			&service.LimitsQueue,
			&service.IsActive,
			&service.CreatedAt,
			&service.UpdatedAt,
		); err != nil {
			continue
		}
		services = append(services, service)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    services,
	})
}

func GetServicesByUnitID(c *fiber.Ctx) error {
	unitID := c.Query("unit_id")
	isActive := c.Query("is_active")

	// unit_id wajib
	if unitID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unit_id wajib diisi",
		})
	}

	query := `
		SELECT 
			id, unit_id, nama_service, code, loket, limits_queue,
			is_active, created_at, updated_at
		FROM services
		WHERE unit_id = ?
	`
	args := []interface{}{unitID}

	// filter aktif / nonaktif (opsional)
	if isActive != "" {
		query += " AND is_active = ?"
		args = append(args, isActive)
	}

	query += " ORDER BY created_at ASC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data service",
		})
	}
	defer rows.Close()

	services := []models.Service{}
	for rows.Next() {
		var service models.Service
		if err := rows.Scan(
			&service.ID,
			&service.UnitID,
			&service.NamaService,
			&service.Code,
			&service.Loket,
			&service.LimitsQueue,
			&service.IsActive,
			&service.CreatedAt,
			&service.UpdatedAt,
		); err != nil {
			continue
		}
		services = append(services, service)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    services,
	})
}



// GetAllServicesPagination - Ambil semua service dengan pagination (filter by user's unit_id)
func GetAllServicesPagination(c *fiber.Ctx) error {
	claims := c.Locals("claims").(*config.JWTClaims)
	isActive := c.Query("is_active")
	search := c.Query("search")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	// Validasi: user harus punya unit_id (role unit)
	if claims.UnitID == nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "User tidak memiliki unit",
		})
	}

	// Validasi pagination
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Query untuk hitung total data - SELALU filter by unit_id user yang login
	countQuery := "SELECT COUNT(*) FROM services WHERE unit_id = ?"
	countArgs := []interface{}{*claims.UnitID}

	if isActive != "" {
		countQuery += " AND is_active = ?"
		countArgs = append(countArgs, isActive)
	}

	if search != "" {
		search = "%" + strings.TrimSpace(search) + "%"
		countQuery += " AND (code LIKE ? OR nama_service LIKE ? OR loket LIKE ?)"
		countArgs = append(countArgs, search, search, search)
	}

	var totalData int
	err := config.DB.QueryRow(countQuery, countArgs...).Scan(&totalData)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menghitung total data",
		})
	}

	// Query untuk ambil data dengan pagination - SELALU filter by unit_id user yang login
	query := "SELECT id, unit_id, nama_service, code, loket, limits_queue, is_active, created_at, updated_at FROM services WHERE unit_id = ?"
	args := []interface{}{*claims.UnitID}

	if isActive != "" {
		query += " AND is_active = ?"
		args = append(args, isActive)
	}

	if search != "" {
		query += " AND (code LIKE ? OR nama_service LIKE ? OR loket LIKE ?)"
		args = append(args, search, search, search)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data service",
		})
	}
	defer rows.Close()

	services := []models.Service{}
	for rows.Next() {
		var service models.Service
		err := rows.Scan(
			&service.ID,
			&service.UnitID,
			&service.NamaService,
			&service.Code,
			&service.Loket,
			&service.LimitsQueue,
			&service.IsActive,
			&service.CreatedAt,
			&service.UpdatedAt,
		)
		if err != nil {
			continue
		}
		services = append(services, service)
	}

	// Hitung total pages
	totalPages := (totalData + limit - 1) / limit

	return c.JSON(fiber.Map{
		"success": true,
		"data":    services,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total_data":  totalData,
			"total_pages": totalPages,
		},
	})
}

// GetServiceByID - Ambil service berdasarkan ID
func GetServiceByID(c *fiber.Ctx) error {
	claims := c.Locals("claims").(*config.JWTClaims)
	id := c.Params("id")

	var service models.Service
	query := "SELECT id, unit_id, nama_service, code, loket, limits_queue, is_active, created_at, updated_at FROM services WHERE id = ?"
	args := []interface{}{id}

	// Jika role unit, pastikan service milik unit tersebut
	if claims.Role == "unit" && claims.UnitID != nil {
		query += " AND unit_id = ?"
		args = append(args, *claims.UnitID)
	}

	err := config.DB.QueryRow(query, args...).Scan(
		&service.ID,
		&service.UnitID,
		&service.NamaService,
		&service.Code,
		&service.Loket,
		&service.LimitsQueue,
		&service.IsActive,
		&service.CreatedAt,
		&service.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Service tidak ditemukan",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data service",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    service,
	})
}

// CreateService - Buat service baru (hanya untuk role unit)
func CreateService(c *fiber.Ctx) error {
	claims := c.Locals("claims").(*config.JWTClaims)

	// Validasi: hanya role unit yang bisa create service
	if claims.Role != "unit" || claims.UnitID == nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Hanya user unit yang bisa membuat service",
		})
	}

	var req struct {
		NamaService string `json:"nama_service"`
		Code        string `json:"code"`
		Loket       string `json:"loket"`
		LimitsQueue int    `json:"limits_queue"`
		IsActive    string `json:"is_active"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validasi input service
	if req.NamaService == "" || req.Code == "" || req.Loket == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Nama service, code, dan loket wajib diisi",
		})
	}

	// Normalisasi code
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))

	// Validasi: hanya huruf A-Z, panjang 3-10
	re := regexp.MustCompile(`^[A-Z]{3,10}$`)
	if !re.MatchString(req.Code) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Code service harus 3–10 huruf dan tanpa angka atau karakter khusus",
		})
	}

	// Set default is_active jika kosong
	if req.IsActive == "" {
		req.IsActive = "y"
	}

	// Cek apakah code sudah ada
	var count int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM services WHERE code = ?", req.Code).Scan(&count)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal validasi code",
		})
	}

	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Code service sudah digunakan",
		})
	}

	// Insert ke database (unit_id dari JWT token)
	query := "INSERT INTO services (unit_id, nama_service, code, loket, limits_queue, is_active) VALUES (?, ?, ?, ?, ?, ?)"
	result, err := config.DB.Exec(query, *claims.UnitID, req.NamaService, req.Code, req.Loket, req.LimitsQueue, req.IsActive)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membuat service",
		})
	}

	id, _ := result.LastInsertId()

	// Ambil data yang baru dibuat
	var service models.Service
	config.DB.QueryRow(
		"SELECT id, unit_id, nama_service, code, loket, limits_queue, is_active, created_at, updated_at FROM services WHERE id = ?",
		id,
	).Scan(&service.ID, &service.UnitID, &service.NamaService, &service.Code, &service.Loket, &service.LimitsQueue, &service.IsActive, &service.CreatedAt, &service.UpdatedAt)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Service berhasil dibuat",
		"data":    service,
	})
}

// UpdateService - Update service berdasarkan ID (hanya untuk role unit)
func UpdateService(c *fiber.Ctx) error {
	claims := c.Locals("claims").(*config.JWTClaims)
	id := c.Params("id")

	// Validasi: hanya role unit yang bisa update service
	if claims.Role != "unit" || claims.UnitID == nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Hanya user unit yang bisa mengupdate service",
		})
	}

	var req struct {
		NamaService string `json:"nama_service"`
		Code        string `json:"code"`
		Loket       string `json:"loket"`
		LimitsQueue *int   `json:"limits_queue"`
		IsActive    string `json:"is_active"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Cek apakah service ada dan milik unit ini
	var exists int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM services WHERE id = ? AND unit_id = ?", id, *claims.UnitID).Scan(&exists)
	if err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Service tidak ditemukan atau bukan milik unit Anda",
		})
	}

	// Build dynamic update query
	query := "UPDATE services SET "
	args := []interface{}{}
	updates := []string{}

	if req.Code != "" {
		req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
		re := regexp.MustCompile(`^[A-Z]{3,10}$`)
		if !re.MatchString(req.Code) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Code service harus 3–10 huruf dan tanpa angka atau karakter khusus",
			})
		}
		var count int
		config.DB.QueryRow("SELECT COUNT(*) FROM services WHERE code = ? AND id != ?", req.Code, id).Scan(&count)
		if count > 0 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Code service sudah digunakan",
			})
		}
		updates = append(updates, "code = ?")
		args = append(args, req.Code)
	}

	if req.NamaService != "" {
		updates = append(updates, "nama_service = ?")
		args = append(args, req.NamaService)
	}

	if req.Loket != "" {
		updates = append(updates, "loket = ?")
		args = append(args, req.Loket)
	}

	if req.LimitsQueue != nil {
		updates = append(updates, "limits_queue = ?")
		args = append(args, *req.LimitsQueue)
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
	query += " WHERE id = ? AND unit_id = ?"
	args = append(args, id, *claims.UnitID)

	_, err = config.DB.Exec(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengupdate service",
		})
	}

	var service models.Service
	config.DB.QueryRow(
		"SELECT id, unit_id, nama_service, code, loket, limits_queue, is_active, created_at, updated_at FROM services WHERE id = ?",
		id,
	).Scan(&service.ID, &service.UnitID, &service.NamaService, &service.Code, &service.Loket, &service.LimitsQueue, &service.IsActive, &service.CreatedAt, &service.UpdatedAt)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Service berhasil diupdate",
		"data":    service,
	})
}

// DeleteService - Hapus service (soft delete)
func DeleteService(c *fiber.Ctx) error {
	claims := c.Locals("claims").(*config.JWTClaims)
	id := c.Params("id")

	// Validasi: hanya role unit yang bisa delete service
	if claims.Role != "unit" || claims.UnitID == nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Hanya user unit yang bisa menghapus service",
		})
	}

	// Cek apakah service ada dan milik unit ini
	var exists int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM services WHERE id = ? AND unit_id = ?", id, *claims.UnitID).Scan(&exists)
	if err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Service tidak ditemukan atau bukan milik unit Anda",
		})
	}

	// Soft delete
	_, err = config.DB.Exec("UPDATE services SET is_active = 'n' WHERE id = ? AND unit_id = ?", id, *claims.UnitID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menghapus service",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Service berhasil dihapus",
	})
}

// HardDeleteService - Hapus service permanent
func HardDeleteService(c *fiber.Ctx) error {
	claims := c.Locals("claims").(*config.JWTClaims)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)

	// Validasi: hanya role unit yang bisa hard delete service
	if claims.Role != "unit" || claims.UnitID == nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Hanya user unit yang bisa menghapus permanent service",
		})
	}

	result, err := config.DB.Exec("DELETE FROM services WHERE id = ? AND unit_id = ?", id, *claims.UnitID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menghapus service",
		})
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Service tidak ditemukan atau bukan milik unit Anda",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Service berhasil dihapus permanent",
	})
}