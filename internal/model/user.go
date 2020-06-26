package model

import (
	"fmt"

	"github.com/google/jsonapi"
	"gorm.io/gorm"

	"schrodinger-box/internal/misc"
)

type User struct {
	ID       uint    `jsonapi:"primary,user" gorm:"primarykey"`
	Nickname *string `jsonapi:"attr,nickname" gorm:"unique,not null"`
	Type     *string `jsonapi:"attr,type" gorm:"not null"`

	IdentityFields
	EventSignups []*EventSignup `jsonapi:"relation,event_signups,omitempty"`
	DBTime
}

func (user *User) JSONAPILinks() *jsonapi.Links {
	return &jsonapi.Links{
		"self": misc.APIAbsolutePath("/user/" + fmt.Sprint(user.ID)),
	}
}

func (user *User) LoadSignups(db *gorm.DB) {
	// loading all events signed up by the user with event data side-loaded
	db.Model(user).Preload("Event").Preload("Event.Organizer").Association("EventSignups").Find(&user.EventSignups)
	for _, signup := range user.EventSignups {
		signup.Event.LoadLocation()
	}
}
