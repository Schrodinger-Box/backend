package model

import (
    "time"
)

// Standard time object for Gorm-managed tables
type DBTime struct {
    CreatedAt       time.Time   `jsonapi:"attr,created_at,omitempty"`
    UpdatedAt       time.Time   `jsonapi:"attr,updated_at,omitempty"`
    DeletedAt       *time.Time  `jsonapi:"attr,deleted_at,omitempty" sql:"index"`
}

// Fields shared by both Token and User
type IdentityFields struct {
    NUSID       string      `jsonapi:"attr,nusid,omitempty" gorm:"default=NULL"`
    Email       string      `jsonapi:"attr,email,omitempty" gorm:"default=NULL"`
    Fullname    string      `jsonapi:"attr,fullname,omitempty" gorm:"default=NULL"`
}