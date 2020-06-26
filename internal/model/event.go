package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/jsonapi"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

/*
 * event base model - storing information of a certain event
 */
type Event struct {
	ID        uint       `jsonapi:"primary,event" gorm:"primarykey"`
	Title     *string    `jsonapi:"attr,title" gorm:"not null"`
	TimeBegin *time.Time `jsonapi:"attr,time_begin,iso8601" gorm:"not null"`
	TimeEnd   *time.Time `jsonapi:"attr,time_end,iso8601" gorm:"not null"`
	// This is either OnlineLocation or PhysicalLocation
	LocationJSON *string     `gorm:"not null"`
	Location     interface{} `jsonapi:"attr,location" gorm:"-"`
	Type         *string     `jsonapi:"attr,type" gorm:"not null"`

	OrganizerID  uint
	Organizer    *User          `jsonapi:"relation,organizer,omitempty"`
	EventSignups []*EventSignup `jsonapi:"relation,event_signups,omitempty"`

	DBTime
}

func (event *Event) JSONAPILinks() *jsonapi.Links {
	return &jsonapi.Links{
		"self": viper.GetString("domain") + "/api/event/" + fmt.Sprint(event.ID),
	}
}

func (event *Event) JSONAPIRelationshipLinks(relation string) *jsonapi.Links {
	if relation == "organizer" {
		return &jsonapi.Links{
			"related": viper.GetString("domain") + "/api/user/" + fmt.Sprint(event.OrganizerID),
		}
	}
	return nil
}

// Unmarshal LocationJSON into Location object
func (event *Event) LoadLocation() error {
	err := json.Unmarshal([]byte(*event.LocationJSON), &event.Location)
	return errors.WithStack(err)
}

// Marshal Location object into LocationJSON
func (event *Event) SaveLocation() error {
	jsonByteSlice, err := json.Marshal(event.Location)
	jsonString := string(jsonByteSlice)
	event.LocationJSON = &jsonString
	return errors.WithStack(err)
}

type OnlineLocation struct {
	// type = online
	Type     string `json:"type"`
	Platform string `json:"platform"`
	Link     string `json:"link"`
}

type PhysicalLocation struct {
	// type = physical
	// Building and Unit are optional fields
	Type     string `json:"type"`
	ZipCode  string `json:"zip_code" mapstructure:"zip_code"`
	Address  string `json:"address"`
	Building string `json:"building"`
	Unit     string `json:"unit"`
}

/*
 * Event signup model - store signup relation between an event and a user
 */
type EventSignup struct {
	ID      uint   `jsonapi:"primary,event_signup" gorm:"primarykey"`
	EventID *uint  `gorm:"not null"`
	Event   *Event `jsonapi:"relation,event,omitempty"`
	UserID  *uint  `gorm:"not null"`
	User    *User  `jsonapi:"relation,user,omitempty"`

	DBTime
}

func (event *Event) LoadSignups(db *gorm.DB) {
	// load all users who signed up this event with all their data side-loaded
	db.Model(event).Preload("User").Association("EventSignups").Find(&event.EventSignups)
}
