package models

import "time"

type Unit struct {
	ID        	 int64     `json:"id"`
	Code      	 string    `json:"code"`
	NamaUnit  	 string    `json:"nama_unit"`
	IsActive  	 string    `json:"is_active"`
	MainDisplay  string    `json:"main_display"`
	CreatedAt 	 time.Time `json:"created_at"`
	UpdatedAt 	 time.Time `json:"updated_at"`
}

type CreateUnitRequest struct {
	Code     	string `json:"code" validate:"required,max=10"`
	NamaUnit 	string `json:"nama_unit" validate:"required,max=255"`
	IsActive 	string `json:"is_active" validate:"omitempty,oneof=y n"`
	MainDisplay string `json:"main_display" validate:"omitempty,oneof=active inactive"`
}

type UpdateUnitRequest struct {
	Code     string `json:"code" validate:"omitempty,max=10"`
	NamaUnit string `json:"nama_unit" validate:"omitempty,max=255"`
	IsActive string `json:"is_active" validate:"omitempty,oneof=y n"`
	MainDisplay string `json:"main_display" validate:"omitempty,oneof=active inactive"`
}