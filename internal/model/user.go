package model

import (
	"crypto/md5"
	"fmt"

	"github.com/google/jsonapi"
	"gorm.io/gorm"

	"schrodinger-box/internal/misc"
)

type User struct {
	ID       uint    `jsonapi:"primary,user" gorm:"primarykey"`
	Nickname *string `jsonapi:"attr,nickname" gorm:"unique;not null"`
	Type     *string `jsonapi:"attr,type" gorm:"not null"`

	IdentityFields
	EmailMD5     string         `jsonapi:"attr,email_md5" gorm:"-"`
	EventSignups []*EventSignup `jsonapi:"relation,event_signups,omitempty"`
	DBTime
}

func (user *User) JSONAPILinks() *jsonapi.Links {
	return &jsonapi.Links{
		"self": misc.APIAbsolutePath("/user/" + fmt.Sprint(user.ID)),
	}
}

func (user *User) LoadSignups(db *gorm.DB) error {
	// loading all events signed up by the user with event data side-loaded
	return db.Model(user).Preload("Event").Preload("Event.Organizer").Association("EventSignups").Find(&user.EventSignups)
}

func (user *User) AfterFind(tx *gorm.DB) error {
	user.EmailMD5 = fmt.Sprintf("%x", md5.Sum([]byte(user.Email)))
	return nil
}

func (user *User) AfterDelete(tx *gorm.DB) error {
	// delete all linked event signup records
	var eventSignups []*EventSignup
	if err := tx.Model(user).Association("EventSignups").Find(&eventSignups); err != nil {
		return err
	}
	return tx.Delete(&eventSignups).Error
}
