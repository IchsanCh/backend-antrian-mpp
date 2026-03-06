package models

import "time"

// DayName mapping untuk display
var DayName = map[int]string{
	0: "Minggu",
	1: "Senin",
	2: "Selasa",
	3: "Rabu",
	4: "Kamis",
	5: "Jumat",
	6: "Sabtu",
}

type UnitSchedule struct {
	ID        int64     `json:"id"`
	UnitID    int64     `json:"unit_id"`
	DayOfWeek int       `json:"day_of_week"` // 0=Minggu, 1=Senin, ..., 6=Sabtu
	DayName   string    `json:"day_name"`
	JamBuka   string    `json:"jam_buka"`   // format: "HH:MM:SS"
	JamTutup  string    `json:"jam_tutup"`  // format: "HH:MM:SS"
	IsActive  string    `json:"is_active"`  // y=buka, n=tutup/libur
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpsertScheduleItem struct {
	DayOfWeek int    `json:"day_of_week" validate:"min=0,max=6"`
	JamBuka   string `json:"jam_buka"`
	JamTutup  string `json:"jam_tutup"`
	IsActive  string `json:"is_active" validate:"oneof=y n"`
}

type UpsertUnitSchedulesRequest struct {
	Schedules []UpsertScheduleItem `json:"schedules" validate:"required,min=1"`
}
