package middleware

import (
	"backend-antrian/internal/config"
	"database/sql"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
)

// EmailRoleAuth - Middleware untuk validasi role berdasarkan email dari request body
func EmailRoleAuth(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Simpan body untuk bisa dibaca ulang
		bodyBytes := c.Body()

		// Parse email dari body
		var reqBody struct {
			Email string `json:"email"`
		}

		if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		if reqBody.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Email wajib diisi",
			})
		}

		// Check role dan status user dari database
		var role, isBanned string
		err := config.DB.QueryRow(
			"SELECT role, is_banned FROM users WHERE email = ?",
			reqBody.Email,
		).Scan(&role, &isBanned)

		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User tidak ditemukan",
			})
		}

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Gagal validasi user",
			})
		}

		// Check apakah user dibanned
		if isBanned == "y" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "User dibanned, tidak dapat melakukan operasi ini",
			})
		}

		// Check apakah role sesuai
		roleValid := false
		for _, allowedRole := range allowedRoles {
			if role == allowedRole {
				roleValid = true
				break
			}
		}

		if !roleValid {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Anda tidak memiliki akses ke resource ini",
			})
		}

		// Simpan user info ke context
		c.Locals("validated_email", reqBody.Email)
		c.Locals("validated_role", role)

		return c.Next()
	}
}