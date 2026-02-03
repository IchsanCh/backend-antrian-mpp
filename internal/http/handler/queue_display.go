package handler

import (
	"backend-antrian/internal/config"
	"database/sql"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// DisplayQueueData - Struktur untuk display antrian
type DisplayQueueData struct {
	UnitID           int64   `json:"unit_id"`
	UnitName         string  `json:"unit_name"`
	ServiceID        int64   `json:"service_id"`
	ServiceName      string  `json:"service_name"`
	ServiceCode      string  `json:"service_code"`
	CurrentTicket    string  `json:"current_ticket"`
	CurrentLoket     string  `json:"current_loket"`
	TotalWaiting     int     `json:"total_waiting"`
	TotalCalledToday int     `json:"total_called_today"`
}

// GetQueueDisplay - Public endpoint untuk display antrian
func GetQueueDisplay(c *fiber.Ctx) error {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	// Query untuk ambil semua unit & service yang aktif dengan current queue
	query := `
		SELECT 
			u.id as unit_id,
			u.nama_unit,
			s.id as service_id,
			s.nama_service,
			s.code as service_code,
			s.loket,
			(
				SELECT qt.ticket_code 
				FROM queue_tickets qt 
				WHERE qt.unit_id = u.id 
				AND qt.service_id = s.id 
				AND qt.status = 'called'
				AND qt.created_at >= ? 
				AND qt.created_at < ?
				ORDER BY qt.last_called_at DESC 
				LIMIT 1
			) as current_ticket,
			(
				SELECT COUNT(*) 
				FROM queue_tickets qt 
				WHERE qt.unit_id = u.id 
				AND qt.service_id = s.id 
				AND qt.status = 'waiting'
				AND qt.created_at >= ? 
				AND qt.created_at < ?
			) as total_waiting,
			(
				SELECT COUNT(*) 
				FROM queue_tickets qt 
				WHERE qt.unit_id = u.id 
				AND qt.service_id = s.id 
				AND qt.status IN ('called', 'done')
				AND qt.created_at >= ? 
				AND qt.created_at < ?
			) as total_called_today
		FROM units u
		INNER JOIN services s ON s.unit_id = u.id
		WHERE u.is_active = 'y' 
		AND s.is_active = 'y'
		ORDER BY u.nama_unit, s.nama_service
	`

	rows, err := config.DB.Query(query, 
		startOfDay, endOfDay,  // current_ticket
		startOfDay, endOfDay,  // total_waiting
		startOfDay, endOfDay,  // total_called_today
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mengambil data display antrian",
		})
	}
	defer rows.Close()

	var displays []DisplayQueueData

	for rows.Next() {
		var display DisplayQueueData
		var currentTicket sql.NullString
		var loket string

		err := rows.Scan(
			&display.UnitID,
			&display.UnitName,
			&display.ServiceID,
			&display.ServiceName,
			&display.ServiceCode,
			&loket,
			&currentTicket,
			&display.TotalWaiting,
			&display.TotalCalledToday,
		)

		if err != nil {
			continue
		}

		// Set current ticket (default: CODE000)
		if currentTicket.Valid && currentTicket.String != "" {
			display.CurrentTicket = currentTicket.String
			display.CurrentLoket = loket
		} else {
			display.CurrentTicket = fmt.Sprintf("%s000", display.ServiceCode)
			display.CurrentLoket = ""
		}

		displays = append(displays, display)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    displays,
	})
}