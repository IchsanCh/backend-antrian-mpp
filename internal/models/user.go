package models

import (
	"database/sql"
	"time"
)

/*
|--------------------------------------------------------------------------
| DATABASE MODEL (INTERNAL)
|--------------------------------------------------------------------------
| Dipakai untuk query ke DB
*/
type User struct {
	ID        int64
	Nama      string
	Email     string
	Password  string
	Role      string
	IsBanned  string
	UnitID    sql.NullInt64
	CreatedAt time.Time
	UpdatedAt time.Time
}

/*
|--------------------------------------------------------------------------
| REQUEST
|--------------------------------------------------------------------------
*/
type LoginRequest struct {
	Email          string `json:"email" validate:"required"`
	Password       string `json:"password" validate:"required"`
	RecaptchaToken string `json:"recaptcha_token"`
}

type UpdateUserRequest struct {
	Nama     string         `json:"nama" validate:"omitempty,max=255"`
	Email    string         `json:"email" validate:"omitempty,email,max=255"`
	Password string         `json:"password" validate:"omitempty,min=6"`
	Role     string         `json:"role" validate:"omitempty,oneof=super_user unit"`
	IsBanned string         `json:"is_banned" validate:"omitempty,oneof=y n"`
	UnitID   sql.NullInt64  `json:"unit_id"`
}

/*
|--------------------------------------------------------------------------
| RESPONSE DTO
|--------------------------------------------------------------------------
| Dipakai untuk API response
*/
type UserResponse struct {
	ID       int64  `json:"id"`
	Nama     string `json:"nama"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	UnitID   *int64 `json:"unit_id,omitempty"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// UserDetailResponse - untuk response list user dengan info unit
type UserDetailResponse struct {
	ID        int64     `json:"id"`
	Nama      string    `json:"nama"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsBanned  string    `json:"is_banned"`
	UnitID    *int64    `json:"unit_id"`
	UnitName  string    `json:"unit_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

/*
|--------------------------------------------------------------------------
| MAPPER
|--------------------------------------------------------------------------
| Convert User (DB) -> UserResponse (API)
*/
func ToUserResponse(u User) UserResponse {
	var unitID *int64

	if u.UnitID.Valid {
		unitID = &u.UnitID.Int64
	}

	return UserResponse{
		ID:     u.ID,
		Nama:   u.Nama,
		Email:  u.Email,
		Role:   u.Role,
		UnitID: unitID,
	}
}

// ToUserDetailResponse - untuk list user dengan info lengkap
func ToUserDetailResponse(u User, unitName string) UserDetailResponse {
	var unitID *int64

	if u.UnitID.Valid {
		unitID = &u.UnitID.Int64
	}

	return UserDetailResponse{
		ID:        u.ID,
		Nama:      u.Nama,
		Email:     u.Email,
		Role:      u.Role,
		IsBanned:  u.IsBanned,
		UnitID:    unitID,
		UnitName:  unitName,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}