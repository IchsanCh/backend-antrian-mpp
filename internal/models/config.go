package models

type Config struct {
	ID       int64  `json:"id"`
	JamBuka  string `json:"jam_buka"`  // format: "HH:MM:SS"
	JamTutup string `json:"jam_tutup"` // format: "HH:MM:SS"
}

type CreateConfigRequest struct {
	JamBuka  string `json:"jam_buka" validate:"required"`
	JamTutup string `json:"jam_tutup" validate:"required"`
}

type UpdateConfigRequest struct {
	JamBuka  string `json:"jam_buka" validate:"required"`
	JamTutup string `json:"jam_tutup" validate:"required"`
}