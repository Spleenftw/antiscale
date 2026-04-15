package models

import "time"

// AuthKey represents a pre-shared key created by a User for authenticating devices.
type AuthKey struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `json:"user_id"`
	Key         string    `gorm:"uniqueIndex;not null" json:"key"` // e.g., "antskey-..."
	AutoApprove bool      `json:"auto_approve"` // Whether the node bypasses Pending state
	IsReusable  bool      `json:"is_reusable"` // False means it's consumed on first use
	IsUsed      bool      `gorm:"default:false" json:"is_used"`
	CreatedAt   time.Time `json:"created_at"`
}
