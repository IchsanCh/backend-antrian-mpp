package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/models"
	"database/sql"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
)

// TakeQueueRequest - Request body untuk mengambil nomor antrian
type TakeQueueRequest struct {
	UnitID    int64 `json:"unit_id"`
	ServiceID int64 `json:"service_id"`
}

// TakeQueue - Endpoint untuk mengambil nomor antrian
func TakeQueue(c *fiber.Ctx) error {
	var req TakeQueueRequest
	userID := c.Locals("user_id").(int64)
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validasi input
	if req.UnitID == 0 || req.ServiceID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "unit_id dan service_id wajib diisi",
		})
	}

	// 1. Cek apakah unit aktif
	var unitActive string
	var unitName string
	err := config.DB.QueryRow(
		"SELECT is_active, nama_unit FROM units WHERE id = ?",
		req.UnitID,
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

	if unitActive != "y" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Unit %s sedang tidak aktif", unitName),
		})
	}

	// 2. Cek apakah service ada, aktif, dan sesuai dengan unit
	var serviceActive string
	var serviceName string
	var serviceCode string
	var limitsQueue int
	var serviceUnitID int64

	err = config.DB.QueryRow(
		"SELECT is_active, nama_service, code, limits_queue, unit_id FROM services WHERE id = ?",
		req.ServiceID,
	).Scan(&serviceActive, &serviceName, &serviceCode, &limitsQueue, &serviceUnitID)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Layanan tidak ditemukan",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal memvalidasi layanan",
		})
	}

	// Validasi service sesuai dengan unit
	if serviceUnitID != req.UnitID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Layanan tidak tersedia di unit ini",
		})
	}

	// Validasi service aktif
	if serviceActive != "y" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Layanan %s di unit %s sedang tutup", serviceName, unitName),
		})
	}

	// 3. Hitung jumlah antrian hari ini untuk service ini
	// PERBAIKAN: Gunakan DATE() di MySQL untuk handle timezone lebih baik
	var todayQueueCount int
	err = config.DB.QueryRow(`
		SELECT COUNT(*) 
		FROM queue_tickets 
		WHERE service_id = ? 
		AND unit_id = ? 
		AND DATE(created_at) = CURDATE()
	`, req.ServiceID, req.UnitID).Scan(&todayQueueCount)

	if err != nil {
		log.Printf("[TakeQueue] Error counting queue: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal menghitung antrian hari ini",
		})
	}

	// 4. Validasi limit queue (jika limits_queue > 0)
	if limitsQueue > 0 && todayQueueCount >= limitsQueue {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Kuota antrian hari ini untuk layanan %s sudah penuh (%d/%d)", serviceName, todayQueueCount, limitsQueue),
		})
	}

	// 5. Generate ticket code
	// Format: KODE_SERVICE + NOMOR_URUT (misal: KTP001, KTP002, dst)
	queueNumber := todayQueueCount + 1
	ticketCode := fmt.Sprintf("%s%03d", serviceCode, queueNumber)

	// 6. Insert ticket ke database
	query := `
		INSERT INTO queue_tickets 
		(ticket_code, unit_id, service_id, user_id, status, created_at, updated_at) 
		VALUES (?, ?, ?, ?, 'waiting', NOW(), NOW())
	`
	
	result, err := config.DB.Exec(query, ticketCode, req.UnitID, req.ServiceID, userID)
	if err != nil {
		log.Printf("[TakeQueue] Error inserting ticket: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal membuat nomor antrian",
		})
	}

	ticketID, _ := result.LastInsertId()
	// 7. Insert transaction log (event: take)
	_, err = config.DB.Exec(`
		INSERT INTO queue_transactions 
		(ticket_id, event, actor_user_id, created_at, updated_at) 
		VALUES (?, 'take', ?, NOW(), NOW())
	`, ticketID, userID)

	if err != nil {
		log.Printf("[TakeQueue] Error inserting transaction: %v", err)
		// Rollback ticket jika gagal insert transaction
		config.DB.Exec("DELETE FROM queue_tickets WHERE id = ?", ticketID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mencatat transaksi antrian",
		})
	}

	// 8. Ambil data ticket yang baru dibuat
	var ticket models.QueueTicket
	err = config.DB.QueryRow(`
		SELECT id, ticket_code, unit_id, service_id, user_id, status, 
		       last_called_at, created_at, updated_at 
		FROM queue_tickets 
		WHERE id = ?
	`, ticketID).Scan(
		&ticket.ID,
		&ticket.TicketCode,
		&ticket.UnitID,
		&ticket.ServiceID,
		&ticket.UserID,
		&ticket.Status,
		&ticket.LastCalledAt,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	if err != nil {
		log.Printf("[TakeQueue] Error fetching ticket: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mengambil data antrian",
		})
	}

	// Broadcast update ke WebSocket display
	BroadcastQueueUpdate()

	// 9. Return response dengan info tambahan
	remaining := 0
	if limitsQueue > 0 {
		remaining = limitsQueue - queueNumber
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Nomor antrian berhasil diambil",
		"data": fiber.Map{
			"ticket":        ticket,
			"unit_name":     unitName,
			"service_name":  serviceName,
			"queue_number":  queueNumber,
			"total_today":   queueNumber,
			"limit_queue":   limitsQueue,
			"remaining":     remaining,
		},
	})
}