package handler

import (
	"backend-antrian/internal/config"
	"time"

	"github.com/gofiber/fiber/v2"
)

// GetUnitDashboardStatistics - Endpoint untuk dashboard unit (hari ini saja)
func GetUnitDashboardStatistics(c *fiber.Ctx) error {
	// Ambil unit_id dari JWT claims
	unitID := c.Locals("unit_id")
	if unitID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unit ID tidak ditemukan",
		})
	}

	// Get today's date
	today := time.Now().Format("2006-01-02")

	// ===========================
	// 1. SUMMARY DATA
	// ===========================

	// Total Kunjungan (semua status, hari ini, unit ini)
	var totalVisitors int
	queryTotalVisitors := `
		SELECT COUNT(qt.id)
		FROM queue_tickets qt
		WHERE qt.unit_id = ?
		AND DATE(qt.created_at) = ?
	`
	err := config.DB.QueryRow(queryTotalVisitors, unitID, today).Scan(&totalVisitors)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil total kunjungan",
		})
	}

	// Total Dilayani (status 'done' saja)
	var totalServed int
	queryTotalServed := `
		SELECT COUNT(qt.id)
		FROM queue_tickets qt
		WHERE qt.unit_id = ?
		AND DATE(qt.created_at) = ?
		AND qt.status = 'done'
	`
	err = config.DB.QueryRow(queryTotalServed, unitID, today).Scan(&totalServed)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil total dilayani",
		})
	}

	// Total Skip (status 'skipped' saja)
	var totalSkipped int
	queryTotalSkipped := `
		SELECT COUNT(qt.id)
		FROM queue_tickets qt
		WHERE qt.unit_id = ?
		AND DATE(qt.created_at) = ?
		AND qt.status = 'skipped'
	`
	err = config.DB.QueryRow(queryTotalSkipped, unitID, today).Scan(&totalSkipped)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil total skip",
		})
	}

	// ===========================
	// 2. LAYANAN DATA (Top layanan dari unit ini, hari ini)
	// ===========================
	type LayananData struct {
		Nama  string `json:"nama"`
		Total int    `json:"total"`
	}

	queryLayanan := `
		SELECT 
			s.nama_service as nama,
			COUNT(qt.id) as total
		FROM queue_tickets qt
		INNER JOIN services s ON qt.service_id = s.id
		WHERE qt.unit_id = ?
		AND DATE(qt.created_at) = ?
		GROUP BY qt.service_id, s.nama_service
		HAVING total > 0
		ORDER BY total DESC
	`

	rows, err := config.DB.Query(queryLayanan, unitID, today)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data layanan",
		})
	}
	defer rows.Close()

	layananData := []LayananData{}
	for rows.Next() {
		var ld LayananData
		if err := rows.Scan(&ld.Nama, &ld.Total); err != nil {
			continue
		}
		layananData = append(layananData, ld)
	}

	// ===========================
	// RESPONSE
	// ===========================
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"summary": fiber.Map{
				"total_visitors": totalVisitors,
				"total_served":   totalServed,
				"total_skipped":  totalSkipped,
			},
			"layanan_data": layananData,
		},
	})
}