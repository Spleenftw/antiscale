package models

import (
	"time"
)

// Node represents a device in the WireGuard mesh network.
type Node struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UserID           uint      `json:"user_id"` // Links to GitHub User
	Hostname         string    `gorm:"size:255;not null" json:"hostname"`
	MagicName        string    `gorm:"size:255;uniqueIndex" json:"magic_name"`
	PublicKey        string    `gorm:"size:64;uniqueIndex;not null" json:"public_key"`
	PrivateIP        string    `gorm:"size:15;uniqueIndex;not null" json:"private_ip"`
	Endpoint         string    `gorm:"size:255" json:"endpoint"`
	IsExit           bool      `gorm:"default:false" json:"is_exit"`
	AdvertisedRoutes string    `gorm:"type:text" json:"advertised_routes"`
	ApprovedRoutes   string    `gorm:"type:text" json:"approved_routes"`
	AcceptRoutes     bool      `gorm:"default:false" json:"accept_routes"`
	Status           string    `gorm:"default:'pending'" json:"status"` // pending, approved, blocked
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
