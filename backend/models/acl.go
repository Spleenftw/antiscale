package models

import "time"

// ACLPolicy holds the global JSON-based access control list for the mesh.
type ACLPolicy struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Policy    string    `gorm:"type:text" json:"policy"` // HuJSON/JSON storing groups and rules
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
