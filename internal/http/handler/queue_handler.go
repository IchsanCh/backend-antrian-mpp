package handler

import (
	"backend-antrian/internal/config"
	"database/sql"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// CallNextQueueRequest - Request untuk panggil antrian berikutnya
type CallNextQueueRequest struct {
	ServiceID int64 `json:"service_id"`
}

// UpdateQueueStatusRequest - Request untuk update status antrian
type UpdateQueueStatusRequest struct {
	TicketID int64  `json:"ticket_id"`
	Status   string `json:"status"` // done, skipped
}

// CallNextQueue - Endpoint untuk panggil antrian berikutnya
func CallNextQueue(c *fiber.Ctx) error {
	var req CallNextQueueRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Ambil user_id dan unit_id dari JWT context
	userID := c.Locals("user_id").(int64)
	userUnitID, ok := c.Locals("unit_id").(int64)
	if !ok {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "User tidak memiliki unit",
		})
	}

	// Validasi: Cek apakah service_id milik unit user
	var serviceUnitID int64
	err := config.DB.QueryRow("SELECT unit_id FROM services WHERE id = ?", req.ServiceID).
		Scan(&serviceUnitID)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Service tidak ditemukan",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal validasi service",
		})
	}

	if serviceUnitID != userUnitID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "Anda tidak memiliki akses ke service ini",
		})
	}

	// STEP 1: CEK ANTRIAN WAITING BERIKUTNYA DULU
	var nextTicketID int64
	var nextTicketCode string
	var nextUnitID int64

	queryNext := `
		SELECT id, ticket_code, unit_id 
		FROM queue_tickets 
		WHERE service_id = ? 
		AND status = 'waiting'
		AND DATE(created_at) = CURDATE()
		ORDER BY created_at ASC 
		LIMIT 1
	`

	err = config.DB.QueryRow(queryNext, req.ServiceID).
		Scan(&nextTicketID, &nextTicketCode, &nextUnitID)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Tidak ada antrian yang menunggu",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mengambil antrian",
		})
	}

	// STEP 2: UPDATE CURRENT TICKET (CALLED) JADI DONE (jika ada)
	var currentTicketID int64
	err = config.DB.QueryRow(`
		SELECT id FROM queue_tickets 
		WHERE service_id = ? AND status = 'called'
		LIMIT 1
	`, req.ServiceID).Scan(&currentTicketID)

	if err == nil {
		// Ada ticket yang sedang called, update jadi done
		_, err = config.DB.Exec(`
			UPDATE queue_tickets 
			SET status = 'done', updated_at = NOW()
			WHERE id = ?
		`, currentTicketID)

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Gagal mengupdate status antrian saat ini",
			})
		}

		// Insert transaction log untuk finish
		_, _ = config.DB.Exec(`
			INSERT INTO queue_transactions 
			(ticket_id, event, actor_user_id, created_at, updated_at) 
			VALUES (?, 'finish', ?, NOW(), NOW())
		`, currentTicketID, userID)
	}

	// STEP 3: PANGGIL ANTRIAN BERIKUTNYA
	_, err = config.DB.Exec(`
		UPDATE queue_tickets 
		SET status = 'called', 
		    last_called_at = NOW(),
		    user_id = ?,
		    updated_at = NOW()
		WHERE id = ?
	`, userID, nextTicketID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal memanggil antrian",
		})
	}

	// Insert transaction log untuk call
	_, err = config.DB.Exec(`
		INSERT INTO queue_transactions 
		(ticket_id, event, actor_user_id, created_at, updated_at) 
		VALUES (?, 'call', ?, NOW(), NOW())
	`, nextTicketID, userID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mencatat transaksi",
		})
	}

	// Broadcast update via WebSocket
	BroadcastQueueUpdate()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Antrian berhasil dipanggil",
		"data": fiber.Map{
			"ticket_id":   nextTicketID,
			"ticket_code": nextTicketCode,
			"unit_id":     nextUnitID,
			"service_id":  req.ServiceID,
		},
	})
}

// SkipAndNext - Endpoint untuk skip antrian saat ini dan panggil berikutnya
func SkipAndNext(c *fiber.Ctx) error {
	var req CallNextQueueRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Ambil user_id dan unit_id dari JWT context
	userID := c.Locals("user_id").(int64)
	userUnitID, ok := c.Locals("unit_id").(int64)
	if !ok {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "User tidak memiliki unit",
		})
	}

	// Validasi: Cek apakah service_id milik unit user
	var serviceUnitID int64
	err := config.DB.QueryRow("SELECT unit_id FROM services WHERE id = ?", req.ServiceID).
		Scan(&serviceUnitID)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Service tidak ditemukan",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal validasi service",
		})
	}

	if serviceUnitID != userUnitID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "Anda tidak memiliki akses ke service ini",
		})
	}

	// STEP 1: CEK ANTRIAN WAITING BERIKUTNYA DULU
	var nextTicketID int64
	var nextTicketCode string
	var nextUnitID int64

	queryNext := `
		SELECT id, ticket_code, unit_id 
		FROM queue_tickets 
		WHERE service_id = ? 
		AND status = 'waiting'
		AND DATE(created_at) = CURDATE()
		ORDER BY created_at ASC 
		LIMIT 1
	`

	err = config.DB.QueryRow(queryNext, req.ServiceID).
		Scan(&nextTicketID, &nextTicketCode, &nextUnitID)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Tidak ada antrian yang menunggu",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mengambil antrian",
		})
	}

	// STEP 2: UPDATE CURRENT TICKET (CALLED) JADI SKIPPED (jika ada)
	var currentTicketID int64
	err = config.DB.QueryRow(`
		SELECT id FROM queue_tickets 
		WHERE service_id = ? AND status = 'called'
		LIMIT 1
	`, req.ServiceID).Scan(&currentTicketID)

	if err == nil {
		// Ada ticket yang sedang called, update jadi skipped
		_, err = config.DB.Exec(`
			UPDATE queue_tickets 
			SET status = 'skipped', updated_at = NOW()
			WHERE id = ?
		`, currentTicketID)

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Gagal mengupdate status antrian saat ini",
			})
		}

		// Insert transaction log untuk skip
		_, _ = config.DB.Exec(`
			INSERT INTO queue_transactions 
			(ticket_id, event, actor_user_id, created_at, updated_at) 
			VALUES (?, 'skip', ?, NOW(), NOW())
		`, currentTicketID, userID)
	}

	// STEP 3: PANGGIL ANTRIAN BERIKUTNYA
	_, err = config.DB.Exec(`
		UPDATE queue_tickets 
		SET status = 'called', 
		    last_called_at = NOW(),
		    user_id = ?,
		    updated_at = NOW()
		WHERE id = ?
	`, userID, nextTicketID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal memanggil antrian",
		})
	}

	// Insert transaction log untuk call
	_, err = config.DB.Exec(`
		INSERT INTO queue_transactions 
		(ticket_id, event, actor_user_id, created_at, updated_at) 
		VALUES (?, 'call', ?, NOW(), NOW())
	`, nextTicketID, userID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mencatat transaksi",
		})
	}

	// Broadcast update via WebSocket
	BroadcastQueueUpdate()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Antrian di-skip dan antrian berikutnya berhasil dipanggil",
		"data": fiber.Map{
			"ticket_id":   nextTicketID,
			"ticket_code": nextTicketCode,
			"unit_id":     nextUnitID,
			"service_id":  req.ServiceID,
		},
	})
}

// UpdateQueueStatus - Endpoint untuk update status antrian (done/skip)
func UpdateQueueStatus(c *fiber.Ctx) error {
	var req UpdateQueueStatusRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validasi status
	if req.Status != "done" && req.Status != "skipped" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Status harus 'done' atau 'skipped'",
		})
	}

	// Ambil user_id dari JWT context
	userID := c.Locals("user_id").(int64)

	// Cek apakah ticket ada dan statusnya 'called'
	var currentStatus string
	err := config.DB.QueryRow("SELECT status FROM queue_tickets WHERE id = ?", req.TicketID).
		Scan(&currentStatus)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Ticket tidak ditemukan",
		})
	}

	if currentStatus != "called" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Ticket tidak bisa diubah. Status saat ini: %s", currentStatus),
		})
	}

	// Update status
	_, err = config.DB.Exec(`
		UPDATE queue_tickets 
		SET status = ?, updated_at = NOW()
		WHERE id = ?
	`, req.Status, req.TicketID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mengupdate status",
		})
	}

	// Insert transaction log
	event := "finish"
	if req.Status == "skipped" {
		event = "skip"
	}

	_, err = config.DB.Exec(`
		INSERT INTO queue_transactions 
		(ticket_id, event, actor_user_id, created_at, updated_at) 
		VALUES (?, ?, ?, NOW(), NOW())
	`, req.TicketID, event, userID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mencatat transaksi",
		})
	}

	// Broadcast update via WebSocket
	BroadcastQueueUpdate()

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Status berhasil diubah menjadi %s", req.Status),
	})
}

// RecallQueue - Endpoint untuk recall antrian (bisa dari status skipped, done, atau called)
func RecallQueue(c *fiber.Ctx) error {
	ticketID := c.Params("id")

	// Ambil user_id dan unit_id dari JWT context
	userID := c.Locals("user_id").(int64)
	userUnitID, ok := c.Locals("unit_id").(int64)
	if !ok {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "User tidak memiliki unit",
		})
	}

	// Cek apakah ticket ada dan validasi unit_id
	var currentStatus string
	var ticketUnitID int64
	err := config.DB.QueryRow(`
		SELECT qt.status, s.unit_id 
		FROM queue_tickets qt
		JOIN services s ON qt.service_id = s.id
		WHERE qt.id = ?
	`, ticketID).Scan(&currentStatus, &ticketUnitID)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Ticket tidak ditemukan",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mengambil data ticket",
		})
	}

	// Validasi: Cek apakah ticket milik unit user
	if ticketUnitID != userUnitID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "Anda tidak memiliki akses ke ticket ini",
		})
	}

	// Validasi: tidak bisa recall jika sudah waiting
	if currentStatus == "waiting" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Ticket sudah dalam status waiting",
		})
	}

	// Update status jadi waiting
	_, err = config.DB.Exec(`
		UPDATE queue_tickets
		SET status = 'waiting',
			updated_at = NOW()
		WHERE id = ?
	`, ticketID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal recall antrian",
		})
	}

	// Insert transaction log
	_, err = config.DB.Exec(`
		INSERT INTO queue_transactions 
		(ticket_id, event, actor_user_id, created_at, updated_at) 
		VALUES (?, 'recall', ?, NOW(), NOW())
	`, ticketID, userID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Gagal mencatat transaksi",
		})
	}

	// Broadcast update via WebSocket
	BroadcastQueueUpdate()

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Antrian berhasil di-recall dari status '%s' dan masuk ke antrian kembali", currentStatus),
	})
}