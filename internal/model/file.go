package model

import (
	"time"

	"github.com/google/jsonapi"
	"gorm.io/gorm"
)

type File struct {
	ID       uint    `jsonapi:"primary,file" gorm:"primarykey"`
	Filename *string `jsonapi:"attr,filename" gorm:"not null"`

	UploaderID *uint `gorm:"not null"`
	Uploader   *User `jsonapi:"relation,uploader"`

	// Status codes:
	// - created : newly created file record, have not uploaded yet
	// - active  : created and uploaded file
	// - destroyed : file deleted
	Status *string `jsonapi:"attr,status" gorm:"not null;default:'created'"`
	Type   *string `jsonapi:"attr,type" gorm:"not null"`

	// this is only returned when doing file.create and will not be logged in the database
	QueryParam          string    `jsonapi:"-" gorm:"-"`
	QueryParamExpiresAt time.Time `jsonapi:"-" gorm:"-"`
	Endpoint			string    `jsonapi:"-" gorm:"-"`

	DBTime
}

func (file *File) JSONAPIMeta() *jsonapi.Meta {
	if file.QueryParam != "" {
		return &jsonapi.Meta{
			"qp":            file.QueryParam,
			"qp_expires_at": file.QueryParamExpiresAt.Format(time.RFC3339),
			"endpoint":      file.Endpoint,
		}
	} else {
		return nil
	}
}

func (file *File) AfterDelete(tx *gorm.DB) error {
	return tx.Model(file).Update("status", "destroyed").Error
}