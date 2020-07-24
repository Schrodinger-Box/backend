package model

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

/*
 * For sending notifications, a batch will be first generated with a unique LinkID.
 * After GenerateTime, notification objects will be generated for all users subscribed to a specific medium.
 */
type Notification struct {
	ID      uint  `gorm:"primarykey"`
	UserID  *uint `gorm:"not null"`
	User    *User
	BatchID *uint
	Batch   *NotificationBatch
	Medium  *string `gorm:"not null"`
	// target is medium-specific, it is chatId for telegram, email address for email and HP number for SMS
	Target   *string    `gorm:"not null"`
	Text     *string    `gorm:"not null"`
	SendTime *time.Time `gorm:"not null"`
	// Status codes:
	// - created   : scheduled but have not sent yet
	// - sent      : notification has been sent
	// - cancelled : action of sending was cancelled before message being sent out
	Status *string `gorm:"not null;default:'created'"`

	DBTime
}

type NotificationBatch struct {
	ID uint `gorm:"primary"`
	// Link ID format:
	// Event-123-1day
	//   |    |    ┕--------- action
	//   |    ┕-------------- related resource ID
	//   ┕------------------- related resource type (currently only Event)
	// Link ID should be unique to avoid sending duplicated messages of the same action
	// since we are using soft delete, checking of duplication LinkID is done manually
	LinkID *string `gorm:"not null"`
	// Template tags:
	// :nickname: , :fullname: , :email: , :nusid: , :event_title: , :time_begin: , :time_end:
	// (custom tags) :c1: , :c2: , :c3: , :c4: , :c5:
	Template *string `gorm:"not null"`
	// notification will be generated after this generate time
	GenerateTime *time.Time `gorm:"not null"`
	// Status codes:
	// - created   : scheduled but have not generate notification messages yet
	// - generated : notification has been generated
	// - cancelled : action of generating was cancelled before message being sent out
	Status *string `gorm:"not null;default:'created'"`

	DBTime
}

type NotificationSubscription struct {
	ID     uint  `gorm:"primary"`
	UserID *uint `gorm:"not null"`

	TelegramChatID *int64
	// EventReminder - reminder of event participation, sent out 1 day, 4 hrs, 30 mins before event start
	// EventSuggestion - suggestion on events that might interest a user
	// EventUpdate - reminder of event details change, such as cancellation of event or change of location/time
	// UserLogin - notification for a new login activity
	TelegramEventReminder   *bool `gorm:"not null;default:1"`
	TelegramEventSuggestion *bool `gorm:"not null;default:1"`
	TelegramEventUpdate     *bool `gorm:"not null;default:1"`
	TelegramUserLogin       *bool `gorm:"not null;default:1"`

	EmailEventReminder   *bool `gorm:"not null;default:1"`
	EmailEventSuggestion *bool `gorm:"not null;default:1"`
	EmailEventUpdate     *bool `gorm:"not null;default:1"`
	EmailUserLogin       *bool `gorm:"not null;default:1"`

	SMSNumber          *string
	SMSEventReminder   *bool `gorm:"not null;default:1"`
	SMSEventSuggestion *bool `gorm:"not null;default:1"`
	SMSEventUpdate     *bool `gorm:"not null;default:1"`
	SMSUserLogin       *bool `gorm:"not null;default:1"`

	DBTime
}

type SMSVerification struct {
	ID        uint    `gorm:"primary"`
	SMSNumber *string `gorm:"not null;unique"`
	Token     *string `gorm:"not null"`

	// Status codes:
	// - locked   : some user has bound their account to this number
	// - released : this number is not bound to any user account
	Status *string `gorm:"default:'released'"`
	DBTime
}

var ServicePrefix = map[string]string{
	"email": "Email",
	"sms":   "SMS",
}

// marks a notification record as 'sent'
func (notification *Notification) Sent(db *gorm.DB) error {
	return db.Model(notification).Update("status", "sent").Error
}

// marks a notification record as 'cancelled'
func (notification *Notification) Cancelled(db *gorm.DB) error {
	return db.Model(notification).Update("status", "cancelled").Error
}

// marks a notification 'deleted' after it is sent or cancelled
func (notification *Notification) AfterUpdate(tx *gorm.DB) error {
	return tx.Delete(notification).Error
}

// marks a notification record as 'generated'
func (batch *NotificationBatch) Generated(db *gorm.DB) error {
	return db.Model(batch).Update("status", "generated").Error
}

// marks a notification record as 'cancelled'
func (batch *NotificationBatch) Cancelled(db *gorm.DB) error {
	return db.Model(batch).Update("status", "cancelled").Error
}

// marks a notification batch 'deleted' after it is generated or cancelled
func (batch *NotificationBatch) AfterUpdate(tx *gorm.DB) error {
	return tx.Delete(batch).Error
}

// build link ID and insert record to database
// flags[0] - whether cancel before create; returns error if duplicate LinkID found and this is set to false; default false
func (batch *NotificationBatch) Create(db *gorm.DB, link interface{}, action string) error {
	linkID := reflect.TypeOf(link).String() +
		strconv.Itoa(int(reflect.ValueOf(link).FieldByName("ID").Uint())) +
		action
	dupNotification := &Notification{}
	if err := db.Model(Notification{}).Where("link_id", linkID).First(dupNotification).Error; err == nil {
		return errors.New("duplicate LinkID is found")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		// something other than record not found occurred
		return err
	}
	batch.LinkID = &linkID
	return db.Save(batch).Error
}

func (batch *NotificationBatch) GenText(replacements map[string]string) string {
	val := *batch.Template
	for k, v := range replacements {
		val = strings.Replace(val, ":"+k+":", v, -1)
	}
	return val
}
