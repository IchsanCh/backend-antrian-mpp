package models

import (
	"database/sql"
	"time"
)

type User struct {
	ID        int64          `json:"id"`
	Nama      string         `json:"nama"`
	Email     string         `json:"email"`
	Password  string         `json:"-"`
	Role      string         `json:"role"`
	IsBanned  string         `json:"is_banned"`
	UnitID    sql.NullInt64  `json:"unit_id"`
	ServiceID sql.NullInt64  `json:"service_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type UserResponse struct {
	ID        int64         `json:"id"`
	Nama      string        `json:"nama"`
	Email     string        `json:"email"`
	Role      string        `json:"role"`
	UnitID    sql.NullInt64 `json:"unit_id,omitempty"`
	ServiceID sql.NullInt64 `json:"service_id,omitempty"`
}