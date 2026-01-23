package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"database/sql"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// GetUserByID - Ambil user berdasarkan ID
func GetUserByID(c *fiber.Ctx) error {
	id := c.Params("id")

	var user models.User
	var unitName string
	query := `
		SELECT u.id, u.nama, u.email, u.role, u.is_banned, u.unit_id, 
		       COALESCE(un.nama_unit, '') as unit_name, u.created_at, u.updated_at 
		FROM users u
		LEFT JOIN units un ON u.unit_id = un.id
		WHERE u.id = ?
	`

	err := config.DB.QueryRow(query, id).Scan(
		&user.ID,
		&user.Nama,
		&user.Email,
		&user.Role,
		&user.IsBanned,
		&user.UnitID,
		&unitName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User tidak ditemukan",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data user",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    models.ToUserDetailResponse(user, unitName),
	})
}

// GetAllUsers - Ambil semua user
func GetAllUsers(c *fiber.Ctx) error {
	isBanned := c.Query("is_banned")
	search := c.Query("search")

	query := `
		SELECT u.id, u.nama, u.email, u.role, u.is_banned, u.unit_id, 
		       COALESCE(un.nama_unit, '') as unit_name, u.created_at, u.updated_at 
		FROM users u
		LEFT JOIN units un ON u.unit_id = un.id
		WHERE 1=1
	`
	args := []interface{}{}

	if isBanned != "" {
		query += " AND u.is_banned = ?"
		args = append(args, isBanned)
	}

	if search != "" {
		search = "%" + strings.TrimSpace(search) + "%"
		query += " AND (u.email LIKE ? OR u.nama LIKE ?)"
		args = append(args, search, search)
	}

	query += " ORDER BY u.created_at DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data user",
		})
	}
	defer rows.Close()

	users := []models.UserDetailResponse{}
	for rows.Next() {
		var user models.User
		var unitName string
		err := rows.Scan(
			&user.ID,
			&user.Nama,
			&user.Email,
			&user.Role,
			&user.IsBanned,
			&user.UnitID,
			&unitName,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			continue
		}
		users = append(users, models.ToUserDetailResponse(user, unitName))
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    users,
	})
}

// GetAllUsersPagination - Ambil semua user dengan pagination
func GetAllUsersPagination(c *fiber.Ctx) error {
	isBanned := c.Query("is_banned")
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
	countQuery := "SELECT COUNT(*) FROM users WHERE 1=1"
	countArgs := []interface{}{}

	if isBanned != "" {
		countQuery += " AND is_banned = ?"
		countArgs = append(countArgs, isBanned)
	}

	if search != "" {
		search = "%" + strings.TrimSpace(search) + "%"
		countQuery += " AND (email LIKE ? OR nama LIKE ?)"
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
	query := `
		SELECT u.id, u.nama, u.email, u.role, u.is_banned, u.unit_id, 
		       COALESCE(un.nama_unit, '') as unit_name, u.created_at, u.updated_at 
		FROM users u
		LEFT JOIN units un ON u.unit_id = un.id
		WHERE 1=1
	`
	args := []interface{}{}

	if isBanned != "" {
		query += " AND u.is_banned = ?"
		args = append(args, isBanned)
	}

	if search != "" {
		query += " AND (u.email LIKE ? OR u.nama LIKE ?)"
		args = append(args, search, search)
	}

	query += " ORDER BY u.created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data user",
		})
	}
	defer rows.Close()

	users := []models.UserDetailResponse{}
	for rows.Next() {
		var user models.User
		var unitName string
		err := rows.Scan(
			&user.ID,
			&user.Nama,
			&user.Email,
			&user.Role,
			&user.IsBanned,
			&user.UnitID,
			&unitName,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			continue
		}
		users = append(users, models.ToUserDetailResponse(user, unitName))
	}

	// Hitung total pages
	totalPages := (totalData + limit - 1) / limit

	return c.JSON(fiber.Map{
		"success": true,
		"data":    users,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total_data":  totalData,
			"total_pages": totalPages,
		},
	})
}

// CreateUser - Buat user baru
func CreateUser(c *fiber.Ctx) error {
	// Role sudah divalidasi di middleware
	var req struct {
		Email     string         `json:"email"`
		Nama      string         `json:"nama"`
		UserEmail string         `json:"user_email"`
		Password  string         `json:"password"`
		Role      string         `json:"role"`
		IsBanned  string         `json:"is_banned"`
		UnitID    *int64 `json:"unit_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validasi input wajib
	if req.Nama == "" || req.UserEmail == "" || req.Password == "" || req.Role == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Nama, email, password, dan role wajib diisi",
		})
	}

	// Validasi email format
	if !strings.Contains(req.UserEmail, "@") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Format email tidak valid",
		})
	}

	// Validasi role
	if req.Role != "super_user" && req.Role != "unit" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Role harus 'super_user' atau 'unit'",
		})
	}

	// Validasi password minimal 6 karakter
	if len(req.Password) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password minimal 6 karakter",
		})
	}

	// Set default is_banned jika kosong
	if req.IsBanned == "" {
		req.IsBanned = "n"
	}

	// Validasi is_banned
	if req.IsBanned != "y" && req.IsBanned != "n" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "is_banned harus 'y' atau 'n'",
		})
	}

	// Cek apakah email sudah digunakan
	var count int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", req.UserEmail).Scan(&count)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal validasi email",
		})
	}

	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Email sudah digunakan",
		})
	}

	// Validasi unit_id jika role = unit
	var unitID sql.NullInt64

	if req.Role == "unit" {
		if req.UnitID == nil || *req.UnitID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "unit_id wajib diisi untuk role 'unit'",
			})
		}

		unitID = sql.NullInt64{
			Int64: *req.UnitID,
			Valid: true,
		}

		var unitExists int
		config.DB.QueryRow(
			"SELECT COUNT(*) FROM units WHERE id = ?", 
			unitID.Int64,
		).Scan(&unitExists)

		if unitExists == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Unit ID tidak ditemukan",
			})
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengenkripsi password",
		})
	}

	// Insert ke database
	query := "INSERT INTO users (nama, email, password, role, is_banned, unit_id) VALUES (?, ?, ?, ?, ?, ?)"
	result, err := config.DB.Exec(query, req.Nama, req.UserEmail, string(hashedPassword), req.Role, req.IsBanned, unitID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membuat user",
		})
	}

	id, _ := result.LastInsertId()

	// Ambil data yang baru dibuat dengan join
	var user models.User
	var unitName string
	querySelect := `
		SELECT u.id, u.nama, u.email, u.role, u.is_banned, u.unit_id, 
		       COALESCE(un.nama_unit, '') as unit_name, u.created_at, u.updated_at 
		FROM users u
		LEFT JOIN units un ON u.unit_id = un.id
		WHERE u.id = ?
	`
	config.DB.QueryRow(querySelect, id).Scan(
		&user.ID,
		&user.Nama,
		&user.Email,
		&user.Role,
		&user.IsBanned,
		&user.UnitID,
		&unitName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "User berhasil dibuat",
		"data":    models.ToUserDetailResponse(user, unitName),
	})
}

// UpdateUser - Update user berdasarkan ID
func UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")

	// Role sudah divalidasi di middleware
	var req struct {
		Email        string        `json:"email"`
		Nama         string        `json:"nama"`
		UserEmail    string        `json:"user_email"`
		Password     string        `json:"password"`
		Role         string        `json:"role"`
		IsBanned     string        `json:"is_banned"`
		UnitID    *int64  `json:"unit_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Cek apakah user ada
	var exists int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", id).Scan(&exists)
	if err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User tidak ditemukan",
		})
	}

	// Build dynamic update query
	query := "UPDATE users SET "
	args := []interface{}{}
	updates := []string{}

	if req.Nama != "" {
		updates = append(updates, "nama = ?")
		args = append(args, req.Nama)
	}

	if req.UserEmail != "" {
		// Validasi email format (basic)
		if !strings.Contains(req.UserEmail, "@") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Format email tidak valid",
			})
		}

		// Cek apakah email sudah digunakan user lain
		var count int
		config.DB.QueryRow("SELECT COUNT(*) FROM users WHERE email = ? AND id != ?", req.UserEmail, id).Scan(&count)
		if count > 0 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Email sudah digunakan",
			})
		}

		updates = append(updates, "email = ?")
		args = append(args, req.UserEmail)
	}

	if req.Password != "" {
		// Validasi minimal length password
		if len(req.Password) < 6 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Password minimal 6 karakter",
			})
		}
		// Hash password baru
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Gagal mengenkripsi password",
			})
		}
		updates = append(updates, "password = ?")
		args = append(args, string(hashedPassword))
	}

	if req.Role != "" {
		if req.Role != "super_user" && req.Role != "unit" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Role harus 'super_user' atau 'unit'",
			})
		}
		updates = append(updates, "role = ?")
		args = append(args, req.Role)
	}

	if req.IsBanned != "" {
		if req.IsBanned != "y" && req.IsBanned != "n" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "is_banned harus 'y' atau 'n'",
			})
		}
		updates = append(updates, "is_banned = ?")
		args = append(args, req.IsBanned)
	}

	// Handle UnitID (bisa null)
	// Cek apakah field unit_id ada di request body
	// Handle unit_id (optional)
	if req.UnitID != nil {
		var unitID sql.NullInt64

		if *req.UnitID == 0 {
			// explicit set NULL
			unitID = sql.NullInt64{
				Valid: false,
			}
		} else {
			// validasi unit_id
			var unitExists int
			config.DB.QueryRow(
				"SELECT COUNT(*) FROM units WHERE id = ?",
				*req.UnitID,
			).Scan(&unitExists)

			if unitExists == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Unit ID tidak ditemukan",
				})
			}

			unitID = sql.NullInt64{
				Int64: *req.UnitID,
				Valid: true,
			}
		}

		updates = append(updates, "unit_id = ?")
		args = append(args, unitID)
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
			"error": "Gagal mengupdate user",
		})
	}

	// Ambil data user yang sudah diupdate dengan join
	var user models.User
	var unitName string
	querySelect := `
		SELECT u.id, u.nama, u.email, u.role, u.is_banned, u.unit_id, 
		       COALESCE(un.nama_unit, '') as unit_name, u.created_at, u.updated_at 
		FROM users u
		LEFT JOIN units un ON u.unit_id = un.id
		WHERE u.id = ?
	`
	config.DB.QueryRow(querySelect, id).Scan(
		&user.ID,
		&user.Nama,
		&user.Email,
		&user.Role,
		&user.IsBanned,
		&user.UnitID,
		&unitName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User berhasil diupdate",
		"data":    models.ToUserDetailResponse(user, unitName),
	})
}

// HardDeleteUser - Hapus user permanent
func HardDeleteUser(c *fiber.Ctx) error {
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

	// Cek apakah user yang akan dihapus adalah super_user terakhir
	var superUserCount int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'super_user'").Scan(&superUserCount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal validasi user",
		})
	}

	// Cek role user yang akan dihapus
	var userRole string
	err = config.DB.QueryRow("SELECT role FROM users WHERE id = ?", id).Scan(&userRole)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User tidak ditemukan",
		})
	}

	// Jika super_user tinggal 1 dan yang dihapus adalah super_user, tolak
	if superUserCount == 1 && userRole == "super_user" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Tidak dapat menghapus super_user terakhir",
		})
	}

	result, err := config.DB.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal menghapus user",
		})
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User tidak ditemukan",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User berhasil dihapus permanent",
	})
}