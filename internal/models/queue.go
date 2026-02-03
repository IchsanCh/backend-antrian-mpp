package models

import (
	"time"
)

type QueueTicket struct {
	ID           int64          `json:"id"`
	TicketCode   string         `json:"ticket_code"`
	UnitID       int64          `json:"unit_id"`
	ServiceID    int64          `json:"service_id"`
	UserID       *int64         `json:"user_id"`
	Status       string         `json:"status"` // waiting, called, done, skipped
	LastCalledAt *time.Time     `json:"last_called_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type QueueTransaction struct {
	ID          int64      `json:"id"`
	TicketID    int64      `json:"ticket_id"`
	Event       string     `json:"event"` // take, call, finish, skip, recall
	ActorUserID *int64     `json:"actor_user_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}