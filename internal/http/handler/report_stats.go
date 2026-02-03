package handler

import (
	"backend-antrian/internal/config"
	"time"

	"github.com/gofiber/fiber/v2"
)

// GetVisitorStatistics - Endpoint untuk data visualisasi laporan
func GetVisitorStatistics(c *fiber.Ctx) error {
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

	// Hitung jumlah hari
	diffDays := int(end.Sub(start).Hours()/24) + 1

	// ===========================
	// 1. SUMMARY DATA
	// ===========================
	var totalVisitors int
	queryTotalVisitors := `
		SELECT COUNT(qt.id)
		FROM queue_tickets qt
		INNER JOIN units u ON qt.unit_id = u.id
		WHERE u.is_active = 'y'
		AND DATE(qt.created_at) BETWEEN ? AND ?
	`
	err := config.DB.QueryRow(queryTotalVisitors, startDate, endDate).Scan(&totalVisitors)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil total kunjungan",
		})
	}

	// Total Instansi (unit aktif yang punya queue dalam range)
	var totalInstansi int
	queryTotalInstansi := `
		SELECT COUNT(DISTINCT qt.unit_id)
		FROM queue_tickets qt
		INNER JOIN units u ON qt.unit_id = u.id
		WHERE u.is_active = 'y'
		AND DATE(qt.created_at) BETWEEN ? AND ?
	`
	err = config.DB.QueryRow(queryTotalInstansi, startDate, endDate).Scan(&totalInstansi)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil total instansi",
		})
	}

	// Total Layanan (service yang punya queue dalam range)
	var totalLayanan int
	queryTotalLayanan := `
		SELECT COUNT(DISTINCT qt.service_id)
		FROM queue_tickets qt
		INNER JOIN units u ON qt.unit_id = u.id
		WHERE u.is_active = 'y'
		AND DATE(qt.created_at) BETWEEN ? AND ?
	`
	err = config.DB.QueryRow(queryTotalLayanan, startDate, endDate).Scan(&totalLayanan)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil total layanan",
		})
	}

	// Rata-rata per hari
	avgPerDay := 0
	if diffDays > 0 {
		avgPerDay = totalVisitors / diffDays
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
			JOIN units u ON qt.unit_id = u.id
			WHERE u.is_active = 'y'
			AND DATE(qt.created_at) BETWEEN ? AND ?
		) t
		GROUP BY t.date
		ORDER BY t.date ASC;
	`

	rows, err := config.DB.Query(queryDaily, startDate, endDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data harian" + err.Error(),
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
	// 3. INSTANSI DATA
	// ===========================
	type InstansiData struct {
		Nama  string `json:"nama"`
		Total int    `json:"total"`
	}

	queryInstansi := `
		SELECT 
			u.nama_unit as nama,
			COUNT(qt.id) as total
		FROM queue_tickets qt
		INNER JOIN units u ON qt.unit_id = u.id
		WHERE u.is_active = 'y'
		AND DATE(qt.created_at) BETWEEN ? AND ?
		GROUP BY qt.unit_id, u.nama_unit
		HAVING total > 0
		ORDER BY total DESC
	`

	rows, err = config.DB.Query(queryInstansi, startDate, endDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal mengambil data instansi",
		})
	}
	defer rows.Close()

	instansiData := []InstansiData{}
	for rows.Next() {
		var id InstansiData
		if err := rows.Scan(&id.Nama, &id.Total); err != nil {
			continue
		}
		instansiData = append(instansiData, id)
	}

	// ===========================
	// 4. LAYANAN DATA
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
		INNER JOIN units u ON qt.unit_id = u.id
		INNER JOIN services s ON qt.service_id = s.id
		WHERE u.is_active = 'y'
		AND DATE(qt.created_at) BETWEEN ? AND ?
		GROUP BY qt.service_id, s.nama_service
		HAVING total > 0
		ORDER BY total DESC
	`

	rows, err = config.DB.Query(queryLayanan, startDate, endDate)
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
				"total_instansi": totalInstansi,
				"total_layanan":  totalLayanan,
				"avg_per_day":    avgPerDay,
			},
			"daily_visitors": dailyVisitors,
			"instansi_data":  instansiData,
			"layanan_data":   layananData,
		},
	})
}