package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// GetAllFAQs - Public endpoint untuk ambil semua FAQ aktif
func GetAllFAQs(c *fiber.Ctx) error {
	query := `
		SELECT id, question, answer, is_active, sort_order, created_at, updated_at 
		FROM faqs 
		WHERE is_active = 'y' 
		ORDER BY sort_order ASC, created_at ASC
	`

	rows, err := config.DB.Query(query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data FAQ",
		})
	}
	defer rows.Close()

	faqs := []models.FAQ{}
	for rows.Next() {
		var faq models.FAQ
		err := rows.Scan(
			&faq.ID,
			&faq.Question,
			&faq.Answer,
			&faq.IsActive,
			&faq.SortOrder,
			&faq.CreatedAt,
			&faq.UpdatedAt,
		)
		if err != nil {
			continue
		}
		faqs = append(faqs, faq)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    faqs,
	})
}

// GetAllFAQsPagination - Admin endpoint untuk ambil semua FAQ dengan pagination
func GetAllFAQsPagination(c *fiber.Ctx) error {
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
	countQuery := "SELECT COUNT(*) FROM faqs WHERE 1=1"
	countArgs := []interface{}{}

	if isActive != "" {
		countQuery += " AND is_active = ?"
		countArgs = append(countArgs, isActive)
	}

	if search != "" {
		search = "%" + strings.TrimSpace(search) + "%"
		countQuery += " AND (question LIKE ? OR answer LIKE ?)"
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
	query := "SELECT id, question, answer, is_active, sort_order, created_at, updated_at FROM faqs WHERE 1=1"
	args := []interface{}{}

	if isActive != "" {
		query += " AND is_active = ?"
		args = append(args, isActive)
	}

	if search != "" {
		query += " AND (question LIKE ? OR answer LIKE ?)"
		args = append(args, search, search)
	}

	query += " ORDER BY sort_order ASC, created_at ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data FAQ",
		})
	}
	defer rows.Close()

	faqs := []models.FAQ{}
	for rows.Next() {
		var faq models.FAQ
		err := rows.Scan(
			&faq.ID,
			&faq.Question,
			&faq.Answer,
			&faq.IsActive,
			&faq.SortOrder,
			&faq.CreatedAt,
			&faq.UpdatedAt,
		)
		if err != nil {
			continue
		}
		faqs = append(faqs, faq)
	}

	// Hitung total pages
	totalPages := (totalData + limit - 1) / limit

	return c.JSON(fiber.Map{
		"success": true,
		"data":    faqs,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total_data":  totalData,
			"total_pages": totalPages,
		},
	})
}

// GetFAQByID - Ambil FAQ berdasarkan ID
func GetFAQByID(c *fiber.Ctx) error {
	id := c.Params("id")

	var faq models.FAQ
	query := "SELECT id, question, answer, is_active, sort_order, created_at, updated_at FROM faqs WHERE id = ?"

	err := config.DB.QueryRow(query, id).Scan(
		&faq.ID,
		&faq.Question,
		&faq.Answer,
		&faq.IsActive,
		&faq.SortOrder,
		&faq.CreatedAt,
		&faq.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "FAQ tidak ditemukan",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data FAQ",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    faq,
	})
}

// CreateFAQ - Buat FAQ baru
func CreateFAQ(c *fiber.Ctx) error {
	var req struct {
		Question  string `json:"question"`
		Answer    string `json:"answer"`
		IsActive  string `json:"is_active"`
		SortOrder int    `json:"sort_order"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validasi input
	req.Question = strings.TrimSpace(req.Question)
	req.Answer = strings.TrimSpace(req.Answer)

	if req.Question == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Question wajib diisi",
		})
	}

	if req.Answer == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Answer wajib diisi",
		})
	}

	// Validasi panjang karakter
	if len(req.Question) > 255 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Question maksimal 255 karakter",
		})
	}

	if len(req.Answer) > 3000 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Answer maksimal 3000 karakter",
		})
	}

	// Set default is_active jika kosong
	if req.IsActive == "" {
		req.IsActive = "y"
	}

	// Set default sort_order jika 0
	if req.SortOrder == 0 {
		req.SortOrder = 1
	}

	// Insert ke database
	query := "INSERT INTO faqs (question, answer, is_active, sort_order) VALUES (?, ?, ?, ?)"
	result, err := config.DB.Exec(query, req.Question, req.Answer, req.IsActive, req.SortOrder)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membuat FAQ",
		})
	}

	id, _ := result.LastInsertId()

	// Ambil data yang baru dibuat
	var faq models.FAQ
	config.DB.QueryRow(
		"SELECT id, question, answer, is_active, sort_order, created_at, updated_at FROM faqs WHERE id = ?",
		id,
	).Scan(&faq.ID, &faq.Question, &faq.Answer, &faq.IsActive, &faq.SortOrder, &faq.CreatedAt, &faq.UpdatedAt)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "FAQ berhasil dibuat",
		"data":    faq,
	})
}

// UpdateFAQ - Update FAQ berdasarkan ID
func UpdateFAQ(c *fiber.Ctx) error {
	id := c.Params("id")

	var req struct {
		Question  string `json:"question"`
		Answer    string `json:"answer"`
		IsActive  string `json:"is_active"`
		SortOrder *int   `json:"sort_order"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Cek apakah FAQ ada
	var exists int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM faqs WHERE id = ?", id).Scan(&exists)
	if err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "FAQ tidak ditemukan",
		})
	}

	// Build dynamic update query
	query := "UPDATE faqs SET "
	args := []interface{}{}
	updates := []string{}

	if req.Question != "" {
		req.Question = strings.TrimSpace(req.Question)
		if len(req.Question) > 255 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Question maksimal 255 karakter",
			})
		}
		updates = append(updates, "question = ?")
		args = append(args, req.Question)
	}

	if req.Answer != "" {
		req.Answer = strings.TrimSpace(req.Answer)
		if len(req.Answer) > 3000 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Answer maksimal 3000 karakter",
			})
		}
		updates = append(updates, "answer = ?")
		args = append(args, req.Answer)
	}

	if req.IsActive != "" {
		updates = append(updates, "is_active = ?")
		args = append(args, req.IsActive)
	}

	if req.SortOrder != nil {
		updates = append(updates, "sort_order = ?")
		args = append(args, *req.SortOrder)
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
			"error": "Gagal mengupdate FAQ",
		})
	}

	var faq models.FAQ
	config.DB.QueryRow(
		"SELECT id, question, answer, is_active, sort_order, created_at, updated_at FROM faqs WHERE id = ?",
		id,
	).Scan(&faq.ID, &faq.Question, &faq.Answer, &faq.IsActive, &faq.SortOrder, &faq.CreatedAt, &faq.UpdatedAt)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "FAQ berhasil diupdate",
		"data":    faq,
	})
}

// HardDeleteFAQ - Hapus FAQ permanent
func HardDeleteFAQ(c *fiber.Ctx) error {
	id := c.Params("id")

	result, err := config.DB.Exec("DELETE FROM faqs WHERE id = ?", id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menghapus FAQ",
		})
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "FAQ tidak ditemukan",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "FAQ berhasil dihapus permanent",
	})
}