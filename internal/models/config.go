package models

type Config struct {
	ID         int64  `json:"id"`
	JamBuka    string `json:"jam_buka"`  // format: "HH:MM:SS"
	JamTutup   string `json:"jam_tutup"` // format: "HH:MM:SS"
	TextMarque string `json:"text_marque"`
}

type CreateConfigRequest struct {
	JamBuka    string `json:"jam_buka" validate:"required"`
	JamTutup   string `json:"jam_tutup" validate:"required"`
	TextMarque string `json:"text_marque" validate:"required"`
}

type UpdateConfigRequest struct {
	JamBuka    string `json:"jam_buka" validate:"required"`
	JamTutup   string `json:"jam_tutup" validate:"required"`
	TextMarque string `json:"text_marque" validate:"required"`
}