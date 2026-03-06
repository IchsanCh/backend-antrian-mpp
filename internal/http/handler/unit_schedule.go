package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"database/sql"
	"regexp"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

var timeRegex = regexp.MustCompile(`^([0-1][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$`)

// GetUnitSchedules - Ambil semua jadwal untuk satu unit (7 hari)
func GetUnitSchedules(c *fiber.Ctx) error {
	unitID := c.Params("id")

	// Validasi unit ada
	var unitName string
	err := config.DB.QueryRow("SELECT nama_unit FROM units WHERE id = ?", unitID).Scan(&unitName)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Unit tidak ditemukan",
		})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal memvalidasi unit",
		})
	}

	rows, err := config.DB.Query(`
		SELECT id, unit_id, day_of_week, jam_buka, jam_tutup, is_active, created_at, updated_at
		FROM unit_schedules
		WHERE unit_id = ?
		ORDER BY day_of_week ASC
	`, unitID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mengambil jadwal unit",
		})
	}
	defer rows.Close()

	schedules := []models.UnitSchedule{}
	for rows.Next() {
		var s models.UnitSchedule
		if err := rows.Scan(
			&s.ID, &s.UnitID, &s.DayOfWeek,
			&s.JamBuka, &s.JamTutup, &s.IsActive,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			continue
		}
		s.DayName = models.DayName[s.DayOfWeek]
		schedules = append(schedules, s)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"unit_id":   unitID,
			"unit_name": unitName,
			"schedules": schedules,
		},
	})
}

// UpsertUnitSchedules - Simpan jadwal unit (insert atau update per hari)
// Menerima array semua 7 hari sekaligus
func UpsertUnitSchedules(c *fiber.Ctx) error {
	unitID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "ID unit tidak valid",
		})
	}

	// Validasi unit ada
	var exists int
	err = config.DB.QueryRow("SELECT COUNT(*) FROM units WHERE id = ?", unitID).Scan(&exists)
	if err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Unit tidak ditemukan",
		})
	}

	var req models.UpsertUnitSchedulesRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if len(req.Schedules) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Schedules tidak boleh kosong",
		})
	}

	// Validasi setiap item jadwal
	for i, s := range req.Schedules {
		if s.DayOfWeek < 0 || s.DayOfWeek > 6 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "day_of_week harus antara 0 (Minggu) sampai 6 (Sabtu)",
			})
		}
		if s.IsActive != "y" && s.IsActive != "n" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "is_active harus 'y' atau 'n'",
			})
		}
		// Jika hari aktif, jam wajib diisi dan valid
		if s.IsActive == "y" {
			if s.JamBuka == "" || s.JamTutup == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   "Jam buka dan jam tutup wajib diisi untuk hari yang aktif",
				})
			}
			if !timeRegex.MatchString(s.JamBuka) || !timeRegex.MatchString(s.JamTutup) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   "Format waktu harus HH:MM:SS (contoh: 08:00:00)",
				})
			}
			_ = i
		}
	}

	// Upsert setiap jadwal menggunakan INSERT ... ON DUPLICATE KEY UPDATE
	for _, s := range req.Schedules {
		jamBuka := s.JamBuka
		jamTutup := s.JamTutup

		// Jika tutup/libur, set default supaya NOT NULL constraint terpenuhi
		if s.IsActive == "n" {
			if jamBuka == "" {
				jamBuka = "00:00:00"
			}
			if jamTutup == "" {
				jamTutup = "00:00:00"
			}
		}

		_, err := config.DB.Exec(`
			INSERT INTO unit_schedules (unit_id, day_of_week, jam_buka, jam_tutup, is_active)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				jam_buka   = VALUES(jam_buka),
				jam_tutup  = VALUES(jam_tutup),
				is_active  = VALUES(is_active),
				updated_at = NOW()
		`, unitID, s.DayOfWeek, jamBuka, jamTutup, s.IsActive)

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Gagal menyimpan jadwal",
			})
		}
	}

	// Broadcast update ke WS karena jadwal berubah
	BroadcastUnitsStatus()

	// Ambil data terbaru
	rows, err := config.DB.Query(`
		SELECT id, unit_id, day_of_week, jam_buka, jam_tutup, is_active, created_at, updated_at
		FROM unit_schedules
		WHERE unit_id = ?
		ORDER BY day_of_week ASC
	`, unitID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mengambil jadwal terbaru",
		})
	}
	defer rows.Close()

	schedules := []models.UnitSchedule{}
	for rows.Next() {
		var s models.UnitSchedule
		if err := rows.Scan(
			&s.ID, &s.UnitID, &s.DayOfWeek,
			&s.JamBuka, &s.JamTutup, &s.IsActive,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			continue
		}
		s.DayName = models.DayName[s.DayOfWeek]
		schedules = append(schedules, s)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Jadwal berhasil disimpan",
		"data":    schedules,
	})
}

// DeleteUnitSchedule - Hapus jadwal satu hari untuk unit tertentu
func DeleteUnitSchedule(c *fiber.Ctx) error {
	unitID := c.Params("id")
	scheduleID := c.Params("schedule_id")

	// Pastikan schedule milik unit yang benar
	var ownerUnitID int64
	err := config.DB.QueryRow(
		"SELECT unit_id FROM unit_schedules WHERE id = ?", scheduleID,
	).Scan(&ownerUnitID)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Jadwal tidak ditemukan",
		})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal memvalidasi jadwal",
		})
	}

	parsedUnitID, _ := strconv.ParseInt(unitID, 10, 64)
	if ownerUnitID != parsedUnitID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "Jadwal tidak ditemukan di unit ini",
		})
	}

	_, err = config.DB.Exec("DELETE FROM unit_schedules WHERE id = ?", scheduleID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal menghapus jadwal",
		})
	}

	// Broadcast update ke WS
	BroadcastUnitsStatus()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Jadwal berhasil dihapus",
	})
}
