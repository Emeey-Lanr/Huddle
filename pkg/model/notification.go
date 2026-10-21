package model

import (
	"time"

	"github.com/google/uuid"
)

// Notification kinds and statuses.
const (
	NotificationOrderPaid = "order_paid"

	NotificationPending = "pending"
	NotificationSent    = "sent"
	NotificationFailed  = "failed"
)

// Notifications is an email to a seller, queued with the change it reports.
type Notifications struct {
	Id            uuid.UUID
	Kind          string
	OrderId       uuid.UUID
	Status        string
	Attempts      int
	LastError     string
	NextAttemptAt time.Time
	SentAt        *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
