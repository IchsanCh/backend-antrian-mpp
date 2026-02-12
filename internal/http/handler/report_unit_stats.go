package handler

import (
	"backend-antrian/internal/config"
	"time"

	"github.com/gofiber/fiber/v2"
)

// GetUnitVisitorStatistics - Endpoint untuk data visualisasi laporan unit
func GetUnitVisitorStatistics(c *fiber.Ctx) error {
	// Get unit_id from JWT token
	unitID, ok := c.Locals("unit_id").(int64)
	if !ok {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Unit ID not found in token",
		})
	}

	// Parse query parameters
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	// Validasi input
	if startDate == "" || endDate == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Parameter start_date dan end_date wajib diisi",
		})
	}

	// Validasi format tanggal (YYYY-MM-DD)
	_, err1 := time.Parse("2006-01-02", startDate)
	_, err2 := time.Parse("2006-01-02", endDate)
	if err1 != nil || err2 != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Format tanggal harus YYYY-MM-DD (contoh: 2026-02-01)",
		})
	}

	// Validasi tanggal akhir >= tanggal mulai
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	if end.Before(start) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Tanggal akhir harus lebih besar atau sama dengan tanggal mulai",
		})
	}

	// ===========================
	// 1. SUMMARY DATA
	// ===========================
	
	// Total Visitors untuk unit ini
	var totalVisitors int
	queryTotalVisitors := `
		SELECT COUNT(qt.id)
		FROM queue_tickets qt
		JOIN services s ON s.id = qt.service_id
		WHERE qt.unit_id = ?
			AND DATE(qt.created_at) BETWEEN ? AND ?
	`
	err := config.DB.QueryRow(queryTotalVisitors, unitID, startDate, endDate).Scan(&totalVisitors)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil total kunjungan",
		})
	}

	// Total Layanan aktif yang punya queue dalam range
	var totalLayanan int
	queryTotalLayanan := `
		SELECT COUNT(DISTINCT qt.service_id)
		FROM queue_tickets qt
		INNER JOIN services s ON qt.service_id = s.id
		WHERE qt.unit_id = ?
		AND DATE(qt.created_at) BETWEEN ? AND ?
	`
	err = config.DB.QueryRow(queryTotalLayanan, unitID, startDate, endDate).Scan(&totalLayanan)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil total layanan",
		})
	}

	// ===========================
	// 2. DAILY VISITORS
	// ===========================
	type DailyVisitor struct {
		Date  string `json:"date"`
		Total int    `json:"total"`
	}

	queryDaily := `
		SELECT 
			t.date,
			COUNT(*) AS total
		FROM (
			SELECT DATE(qt.created_at) AS date
			FROM queue_tickets qt
			WHERE qt.unit_id = ?
			AND DATE(qt.created_at) BETWEEN ? AND ?
		) t
		GROUP BY t.date
		ORDER BY t.date ASC
	`

	rows, err := config.DB.Query(queryDaily, unitID, startDate, endDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data harian: " + err.Error(),
		})
	}
	defer rows.Close()

	dailyVisitors := []DailyVisitor{}
	for rows.Next() {
		var dv DailyVisitor
		if err := rows.Scan(&dv.Date, &dv.Total); err != nil {
			continue
		}
		dailyVisitors = append(dailyVisitors, dv)
	}

	// ===========================
	// 3. LAYANAN DATA (Services dari unit ini)
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
		AND DATE(qt.created_at) BETWEEN ? AND ?
		GROUP BY qt.service_id, s.nama_service
		HAVING total > 0
		ORDER BY total DESC
	`

	rows, err = config.DB.Query(queryLayanan, unitID, startDate, endDate)
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
				"total_layanan":  totalLayanan,
			},
			"daily_visitors": dailyVisitors,
			"layanan_data":   layananData,
		},
	})
}