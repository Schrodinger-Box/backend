package model

import (
	"crypto/md5"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/jsonapi"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"schrodinger-box/internal/misc"
)

type User struct {
	ID       uint    `jsonapi:"primary,user" gorm:"primarykey"`
	Nickname *string `jsonapi:"attr,nickname" gorm:"not null"`
	Type     *string `jsonapi:"attr,type" gorm:"not null"`

	IdentityFields
	EmailMD5     string                    `jsonapi:"attr,email_md5" gorm:"-"`
	EventSignups []*EventSignup            `jsonapi:"relation,event_signups,omitempty"`
	Subscription *NotificationSubscription `jsonapi:"-"`

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

func (user *User) CreateImmediateNotificationAll(db *gorm.DB, action string, text string) error {
	enabledServices := viper.GetStringSlice("external.enable")
	if user.Subscription != nil {
		subscription := reflect.ValueOf(*user.Subscription)
		for _, enabledService := range enabledServices {
			if subscribed := subscription.FieldByName(strings.Title(enabledService) + action).Interface().(*bool); !*subscribed {
				// skip if not subscribed
				continue
			}
			var target string
			switch enabledService {
			case "telegram":
				if chatId := user.Subscription.TelegramChatID; chatId == nil {
					// skip if chatId is empty
					continue
				} else {
					target = strconv.Itoa(int(*chatId))
				}
			}
			now := time.Now()
			notification := Notification{
				UserID:   &user.ID,
				Text:     &text,
				Target:   &target,
				SendTime: &now,
				Medium:   &enabledService,
			}
			if err := db.Save(&notification).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
