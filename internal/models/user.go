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
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
	RecaptchaToken  string `json:"recaptcha_token"`
}

/*
|--------------------------------------------------------------------------
| RESPONSE DTO
|--------------------------------------------------------------------------
| Dipakai untuk API response
*/
type UserResponse struct {
	ID        int64   `json:"id"`
	Nama      string  `json:"nama"`
	Email     string  `json:"email"`
	Role      string  `json:"role"`
	UnitID    *int64  `json:"unit_id,omitempty"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
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
		ID:        u.ID,
		Nama:      u.Nama,
		Email:     u.Email,
		Role:      u.Role,
		UnitID:    unitID,
	}
}
