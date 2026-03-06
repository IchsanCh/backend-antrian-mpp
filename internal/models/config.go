package models

type Config struct {
	ID         int64  `json:"id"`
	TextMarque string `json:"text_marque"`
}

type CreateConfigRequest struct {
	TextMarque string `json:"text_marque" validate:"required"`
}

type UpdateConfigRequest struct {
	TextMarque string `json:"text_marque" validate:"required"`
}