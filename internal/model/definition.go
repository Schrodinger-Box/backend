package model

import "time"

// Standard time object for Gorm-managed tables
type DBTime struct {
    CreatedAt       time.Time   `jsonapi:"attr,created_at,omitempty"`
    UpdatedAt       time.Time   `jsonapi:"attr,updated_at,omitempty"`
    DeletedAt       *time.Time  `jsonapi:"attr,deleted_at,omitempty" sql:"index"`
}