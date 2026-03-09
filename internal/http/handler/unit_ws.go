package handler

import (
	"backend-antrian/internal/config"
	"backend-antrian/internal/helper"
	"backend-antrian/internal/models"
	"backend-antrian/internal/realtime"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// UnitWithStatus - payload per unit yang dikirim via WS
type UnitWithStatus struct {
	ID          int64   `json:"id"`
	Code        string  `json:"code"`
	NamaUnit    string  `json:"nama_unit"`
	IsActive    string  `json:"is_active"`
	MainDisplay string  `json:"main_display"`
	AudioFile   *string `json:"audio_file"`
	// Status jadwal hari ini
	Queue       string `json:"queue"`         // "open" atau "closed"
	JamBuka     string `json:"jam_buka"`      // hanya diisi jika is_active='y'
	JamTutup    string `json:"jam_tutup"`     // hanya diisi jika is_active='y'
	HasSchedule bool   `json:"has_schedule"`  // false = tidak ada jadwal hari ini
	IsActiveDay bool   `json:"is_active_day"` // false = hari ini is_active='n' (libur)
}

func UnitsWS(c *websocket.Conn) {
	realtime.Units.Register <- c
	defer func() {
		realtime.Units.Unregister <- c
	}()

	// Kirim status awal semua unit saat client connect
	payload := buildUnitsStatusPayload()
	_ = c.WriteMessage(websocket.TextMessage, payload)

	// Listen client (untuk detect disconnect)
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}
}

// buildUnitsStatusPayload - bangun JSON payload semua unit dengan status jadwal
func buildUnitsStatusPayload() []byte {
	rows, err := config.DB.Query(`
		SELECT id, code, nama_unit, is_active, main_display, audio_file
		FROM units
		ORDER BY nama_unit ASC
	`)
	if err != nil {
		payload, _ := json.Marshal(fiber.Map{
			"type":  "units_status",
			"units": []UnitWithStatus{},
		})
		return payload
	}
	defer rows.Close()

	var units []UnitWithStatus
	for rows.Next() {
		var u models.Unit
		if err := rows.Scan(
			&u.ID, &u.Code, &u.NamaUnit, &u.IsActive, &u.MainDisplay, &u.AudioFile,
		); err != nil {
			continue
		}

		status := helper.IsUnitOpen(config.DB, u.ID)

		queueStr := "closed"
		if status.IsOpen {
			queueStr = "open"
		}

		units = append(units, UnitWithStatus{
			ID:          u.ID,
			Code:        u.Code,
			NamaUnit:    u.NamaUnit,
			IsActive:    u.IsActive,
			MainDisplay: u.MainDisplay,
			AudioFile:   u.AudioFile,
			Queue:       queueStr,
			JamBuka:     status.JamBuka,
			JamTutup:    status.JamTutup,
			HasSchedule: status.HasSchedule,
			IsActiveDay: status.IsActiveDay,
		})
	}

	if units == nil {
		units = []UnitWithStatus{}
	}

	payload, _ := json.Marshal(fiber.Map{
		"type":  "units_status",
		"units": units,
	})
	return payload
}

// BroadcastUnitsStatus - broadcast status semua unit ke semua WS client
// Dipanggil setiap kali ada perubahan unit atau jadwal
func BroadcastUnitsStatus() {
	payload := buildUnitsStatusPayload()
	realtime.Units.Broadcast <- payload
}