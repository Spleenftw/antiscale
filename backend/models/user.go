package models

import "time"

// User represents an Admin who logged in via GitHub SSO.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GithubID  int64     `gorm:"uniqueIndex" json:"github_id"`
	Username  string    `json:"username"`
	AvatarURL string    `json:"avatar_url"`
	Role      string    `gorm:"default:'admin'" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
