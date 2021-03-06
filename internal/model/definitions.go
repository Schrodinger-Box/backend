package model

import (
	"time"

	"gorm.io/gorm"
)

// Standard time object for Gorm-managed tables
// CreatedAt and UpdatedAt will never be empty
type DBTime struct {
	CreatedAt time.Time      `jsonapi:"attr,created_at,iso8601"`
	UpdatedAt time.Time      `jsonapi:"attr,updated_at,iso8601"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Fields shared by both Token and User
type IdentityFields struct {
	NUSID    string `jsonapi:"attr,nusid,omitempty"`
	Email    string `jsonapi:"attr,email,omitempty"`
	Fullname string `jsonapi:"attr,fullname,omitempty"`
}
