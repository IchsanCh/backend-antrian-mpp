package models

import "time"

type Service struct {
	ID          int64     `json:"id"`
	UnitID      int64     `json:"unit_id"`
	NamaService string    `json:"nama_service"`
	Code        string    `json:"code"`
	Loket       string    `json:"loket"`        
	LimitsQueue int       `json:"limits_queue"`
	IsActive    string    `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateServiceRequest struct {
	NamaService string `json:"nama_service" validate:"required,max=255"`
	Code        string `json:"code" validate:"required,max=10"`
	LimitsQueue int    `json:"limits_queue" validate:"min=0"`
	IsActive    string `json:"is_active" validate:"omitempty,oneof=y n"`
}

type UpdateServiceRequest struct {
	NamaService string `json:"nama_service" validate:"omitempty,max=255"`
	Code        string `json:"code" validate:"omitempty,max=10"`
	LimitsQueue *int   `json:"limits_queue" validate:"omitempty,min=0"`
	IsActive    string `json:"is_active" validate:"omitempty,oneof=y n"`
}