package helper

import (
	"database/sql"
	"strings"
	"time"
)

// UnitScheduleStatus hasil cek jadwal satu unit
type UnitScheduleStatus struct {
	IsOpen   bool
	JamBuka  string
	JamTutup string
	// HasSchedule false berarti tidak ada jadwal hari ini (libur/tidak diset)
	HasSchedule bool
}

// IsUnitOpen mengecek apakah unit tertentu sedang buka berdasarkan unit_schedules.
// Logika:
//  - Jika tidak ada baris jadwal untuk hari ini → tutup (HasSchedule=false)
//  - Jika ada tapi is_active='n' → tutup (libur)
//  - Jika ada, is_active='y', dan waktu sekarang dalam rentang jam_buka–jam_tutup → buka
func IsUnitOpen(db *sql.DB, unitID int64) UnitScheduleStatus {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return UnitScheduleStatus{}
	}

	now := time.Now().In(loc)
	// MySQL DAYOFWEEK: 1=Minggu..7=Sabtu, kita pakai 0=Minggu..6=Sabtu
	// time.Weekday(): 0=Minggu..6=Sabtu — sudah sama persis
	dayOfWeek := int(now.Weekday())

	var jamBuka, jamTutup, isActive string
	err = db.QueryRow(`
		SELECT jam_buka, jam_tutup, is_active
		FROM unit_schedules
		WHERE unit_id = ? AND day_of_week = ?
	`, unitID, dayOfWeek).Scan(&jamBuka, &jamTutup, &isActive)

	if err == sql.ErrNoRows {
		// Tidak ada jadwal hari ini
		return UnitScheduleStatus{IsOpen: false, HasSchedule: false}
	}
	if err != nil {
		return UnitScheduleStatus{IsOpen: false, HasSchedule: false}
	}

	if isActive != "y" {
		// Hari ini libur / tidak beroperasi
		return UnitScheduleStatus{
			IsOpen:      false,
			HasSchedule: true,
			JamBuka:     jamBuka,
			JamTutup:    jamTutup,
		}
	}

	// Cek rentang waktu
	isOpen := checkTimeRange(now, loc, jamBuka, jamTutup)

	return UnitScheduleStatus{
		IsOpen:      isOpen,
		HasSchedule: true,
		JamBuka:     jamBuka,
		JamTutup:    jamTutup,
	}
}

// checkTimeRange mengecek apakah waktu `now` berada dalam rentang jamBuka–jamTutup.
// Mendukung kasus jam tutup melewati tengah malam (misal 22:00–02:00).
func checkTimeRange(now time.Time, loc *time.Location, jamBuka, jamTutup string) bool {
	layout := "15:04:05"

	// Normalize HH:MM → HH:MM:SS
	if strings.Count(jamBuka, ":") == 1 {
		jamBuka += ":00"
	}
	if strings.Count(jamTutup, ":") == 1 {
		jamTutup += ":00"
	}

	openTime, err := time.ParseInLocation(layout, jamBuka, loc)
	if err != nil {
		return false
	}
	closeTime, err := time.ParseInLocation(layout, jamTutup, loc)
	if err != nil {
		return false
	}

	// Set ke tanggal hari ini
	openTime = time.Date(now.Year(), now.Month(), now.Day(),
		openTime.Hour(), openTime.Minute(), openTime.Second(), 0, loc)
	closeTime = time.Date(now.Year(), now.Month(), now.Day(),
		closeTime.Hour(), closeTime.Minute(), closeTime.Second(), 0, loc)

	// Handle melewati tengah malam
	if closeTime.Before(openTime) {
		closeTime = closeTime.Add(24 * time.Hour)
		if now.Before(openTime) {
			openTime = openTime.Add(-24 * time.Hour)
		}
	}

	return now.After(openTime) && now.Before(closeTime)
}