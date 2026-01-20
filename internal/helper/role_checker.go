package helper

import (
	"backend-antrian/internal/config"
	"database/sql"
	"errors"
)

var (
	ErrUserNotFound = errors.New("user tidak ditemukan")
	ErrUserBanned   = errors.New("user dibanned")
	ErrInvalidRole  = errors.New("role tidak sesuai")
)

func CheckUserRole(email string, allowedRoles ...string) error {
	var role, isBanned string

	query := "SELECT role, is_banned FROM users WHERE email = ?"
	err := config.DB.QueryRow(query, email).Scan(&role, &isBanned)

	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}

	if err != nil {
		return err
	}

	if isBanned == "y" {
		return ErrUserBanned
	}

	for _, allowedRole := range allowedRoles {
		if role == allowedRole {
			return nil
		}
	}

	return ErrInvalidRole
}

func CheckUserRoleByID(userID int64, allowedRoles ...string) error {
	var role, isBanned string

	query := "SELECT role, is_banned FROM users WHERE id = ?"
	err := config.DB.QueryRow(query, userID).Scan(&role, &isBanned)

	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}

	if err != nil {
		return err
	}

	if isBanned == "y" {
		return ErrUserBanned
	}

	for _, allowedRole := range allowedRoles {
		if role == allowedRole {
			return nil
		}
	}

	return ErrInvalidRole
}

func GetUserByEmail(email string) (map[string]interface{}, error) {
	var id, unitID sql.NullInt64
	var nama, emailUser, role, isBanned string

	query := "SELECT id, nama, email, role, is_banned, unit_id FROM users WHERE email = ?"
	err := config.DB.QueryRow(query, email).Scan(&id, &nama, &emailUser, &role, &isBanned, &unitID)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	user := map[string]interface{}{
		"id":        id.Int64,
		"nama":      nama,
		"email":     emailUser,
		"role":      role,
		"is_banned": isBanned,
	}

	if unitID.Valid {
		user["unit_id"] = unitID.Int64
	}

	return user, nil
}

func IsSuperUser(email string) bool {
	err := CheckUserRole(email, "super_user")
	return err == nil
}

func IsUnitUser(email string) bool {
	err := CheckUserRole(email, "unit")
	return err == nil
}