package handler

import (
	"backend-antrian/internal/config"
	"database/sql"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ServiceWithStatus - Struct untuk response service dengan status dan info kuota
type ServiceWithStatus struct {
	ID           int64     `json:"id"`
	UnitID       int64     `json:"unit_id"`
	NamaService  string    `json:"nama_service"`
	Code         string    `json:"code"`
	Loket        string    `json:"loket"`
	LimitsQueue  int       `json:"limits_queue"`
	IsActive     string    `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	
	// Info tambahan
	TodayQueueCount int    `json:"today_queue_count"`
	RemainingQuota  int    `json:"remaining_quota"`
	Status          string `json:"status"` // available, closed, quota_full
	StatusMessage   string `json:"status_message"`
}

// GetServicesByUnitIDWithStatus - Menampilkan semua layanan dari unit tertentu dengan status
func GetServicesByUnitIDWithStatus(c *fiber.Ctx) error {
	unitID := c.Query("unit_id")

	if unitID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "unit_id wajib diisi",
		})
	}

	// Cek apakah unit ada dan aktif
	var unitActive string
	var unitName string
	err := config.DB.QueryRow(
		"SELECT is_active, nama_unit FROM units WHERE id = ?",
		unitID,
	).Scan(&unitActive, &unitName)

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

	// Query semua services dari unit ini (TANPA filter is_active)
	// JOIN ke units untuk ambil nama_unit sebagai loket
	query := `
		SELECT 
			s.id, s.unit_id, s.nama_service, s.code, s.limits_queue,
			s.is_active, s.created_at, s.updated_at,
			u.nama_unit
		FROM services s
		JOIN units u ON s.unit_id = u.id
		WHERE s.unit_id = ?
		ORDER BY s.created_at ASC
	`

	rows, err := config.DB.Query(query, unitID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mengambil data layanan",
		})
	}
	defer rows.Close()

	// Hitung waktu untuk filter hari ini

	services := []ServiceWithStatus{}
	for rows.Next() {
		var service ServiceWithStatus
		var namaUnit string
		
		err := rows.Scan(
			&service.ID,
			&service.UnitID,
			&service.NamaService,
			&service.Code,
			&service.LimitsQueue,
			&service.IsActive,
			&service.CreatedAt,
			&service.UpdatedAt,
			&namaUnit,
		)
		if err != nil {
			continue
		}

		// Field Loket diisi dari nama_unit
		service.Loket = namaUnit

		// Hitung jumlah antrian hari ini untuk service ini
		var todayCount int
		err = config.DB.QueryRow(`
			SELECT COUNT(*) 
			FROM queue_tickets 
			WHERE service_id = ? 
			AND unit_id = ? 
			AND DATE(created_at) = CURDATE()
		`, service.ID, unitID).Scan(&todayCount)

		if err != nil {
			todayCount = 0
		}

		service.TodayQueueCount = todayCount

		// Tentukan status dan message
		if service.IsActive != "y" {
			service.Status = "closed"
			service.StatusMessage = fmt.Sprintf("Layanan %s sedang tutup", service.NamaService)
			service.RemainingQuota = 0
		} else if service.LimitsQueue > 0 && todayCount >= service.LimitsQueue {
			service.Status = "quota_full"
			service.StatusMessage = fmt.Sprintf("Kuota antrian hari ini sudah penuh (%d/%d)", todayCount, service.LimitsQueue)
			service.RemainingQuota = 0
		} else {
			service.Status = "available"
			service.StatusMessage = "Tersedia"
			if service.LimitsQueue > 0 {
				service.RemainingQuota = service.LimitsQueue - todayCount
			} else {
				service.RemainingQuota = -1 // -1 berarti unlimited
			}
		}

		services = append(services, service)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"unit_id":   unitID,
			"unit_name": unitName,
			"services":  services,
		},
	})
}