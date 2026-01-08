package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func Login(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email dan password harus diisi",
		})
	}

	var user models.User
	query := `SELECT id, nama, email, password, role, is_banned, unit_id, service_id 
	          FROM users WHERE email = ?`
	err := config.DB.QueryRow(query, req.Email).Scan(
		&user.ID,
		&user.Nama,
		&user.Email,
		&user.Password,
		&user.Role,
		&user.IsBanned,
		&user.UnitID,
		&user.ServiceID,
	)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Email atau password salah",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Check if user is banned
	if user.IsBanned == "y" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Akun Anda telah diblokir",
		})
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Email atau password salah",
		})
	}

	// Handle nullable unit_id and service_id
	var unitID, serviceID *int64
	if user.UnitID.Valid {
		unitID = &user.UnitID.Int64
	}
	if user.ServiceID.Valid {
		serviceID = &user.ServiceID.Int64
	}

	// Generate JWT token
	token, err := config.GenerateToken(user.ID, user.Nama, user.Email, user.Role, unitID, serviceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	return c.JSON(models.LoginResponse{
		Token: token,
		User: models.UserResponse{
			ID:        user.ID,
			Nama:      user.Nama,
			Email:     user.Email,
			Role:      user.Role,
			UnitID:    user.UnitID,
			ServiceID: user.ServiceID,
		},
	})
}