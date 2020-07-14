package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/jsonapi"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"schrodinger-box/internal/misc"
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
	Images       []*File     `jsonapi:"relation,images,omitempty" gorm:"polymorphic:Link"`

	OrganizerID  *uint          `gorm:"not null"`
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
			"related": misc.APIAbsolutePath("/user/" + fmt.Sprint(*event.OrganizerID)),
		}
	}
	return nil
}

func (event *Event) BeforeSave(tx *gorm.DB) error {
	// Marshal Location object into LocationJSON
	jsonByteSlice, err := json.Marshal(event.Location)
	jsonString := string(jsonByteSlice)
	event.LocationJSON = &jsonString
	return errors.WithStack(err)
}

func (event *Event) AfterSave(tx *gorm.DB) error {
	return event.AfterFind(tx)
}

func (event *Event) AfterFind(tx *gorm.DB) error {
	// Unmarshal LocationJSON into Location object
	err := json.Unmarshal([]byte(*event.LocationJSON), &event.Location)
	return errors.WithStack(err)
}

func (event *Event) AfterDelete(tx *gorm.DB) error {
	// delete all linked event signup records
	var eventSignups []*EventSignup
	if err := tx.Model(event).Association("EventSignups").Find(&eventSignups); err != nil {
		return err
	}
	if len(eventSignups) == 0 {
		return nil
	} else {
		return tx.Delete(&eventSignups).Error
	}
}

func (event *Event) LoadSignups(db *gorm.DB) error {
	return db.Model(event).Preload("User").Association("EventSignups").Find(&event.EventSignups)
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
	Event   *Event `jsonapi:"relation,event,omitempty" gorm:"PRELOAD:false"`
	UserID  *uint  `gorm:"not null"`
	User    *User  `jsonapi:"relation,user,omitempty" gorm:"PRELOAD:false"`

	// Status codes:
	// - created   : signup record is initially created
	// - attended  : this user's attendance is recorded by the event organizer
	// - reviewed  : this user has left his/her review to the event
	// - withdrawn : this user withdrawn his/her signup record to the event
	Status      *string `jsonapi:"attr,status" gorm:"not null;default:'created'"`
	ReviewScore *uint   `jsonapi:"attr,review_score,omitempty"`
	ReviewText  *string `jsonapi:"attr,review_text,omitempty"`

	DBTime
}

func (signup *EventSignup) AfterDelete(tx *gorm.DB) error {
	if *signup.Status == "created" {
		// mark the signup record as withdrawn if it is deleted before user attend the event
		return tx.Model(signup).Update("status", "withdrawn").Error
	} else {
		return nil
	}
}
