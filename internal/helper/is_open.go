package helper

import (
	"strings"
	"time"
)

func IsQueueOpen(jamBuka, jamTutup string) bool {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return false
	}

	now := time.Now().In(loc)

	// Database TIME format bisa HH:MM:SS atau HH:MM
	layout := "15:04:05"
	
	// Normalize format - tambahkan :00 jika cuma HH:MM
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

	// Set tanggal HARI INI
	openTime = time.Date(
		now.Year(), now.Month(), now.Day(),
		openTime.Hour(), openTime.Minute(), openTime.Second(),
		0, loc,
	)

	closeTime = time.Date(
		now.Year(), now.Month(), now.Day(),
		closeTime.Hour(), closeTime.Minute(), closeTime.Second(),
		0, loc,
	)

	// Handle case jam tutup melewati tengah malam
	// Contoh: buka 22:00, tutup 02:00
	if closeTime.Before(openTime) {
		// Jam tutup di hari berikutnya
		closeTime = closeTime.Add(24 * time.Hour)
		
		// Jika sekarang sebelum jam buka, berarti masih di periode kemarin
		if now.Before(openTime) {
			openTime = openTime.Add(-24 * time.Hour)
		}
	}

	return now.After(openTime) && now.Before(closeTime)
}