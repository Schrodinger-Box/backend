package model

import (
	"fmt"

	"github.com/google/jsonapi"
	"github.com/spf13/viper"
)

type User struct {
	ID       uint    `jsonapi:"primary,user" gorm:"primarykey"`
	Nickname *string `jsonapi:"attr,nickname" gorm:"unique,not null"`
	Type     *string `jsonapi:"attr,type" gorm:"not null"`

	IdentityFields
	DBTime
}

func (user *User) JSONAPILinks() *jsonapi.Links {
	return &jsonapi.Links{
		"self": viper.GetString("domain") + "/api/user/" + fmt.Sprint(user.ID),
	}
}
