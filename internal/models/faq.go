package models

import "time"

type FAQ struct {
	ID         int64     `json:"id"`
	Question   string    `json:"question"`
	Answer     string    `json:"answer"`
	IsActive   string    `json:"is_active"`
	SortOrder  int       `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}