package model

import (
	"crypto/md5"
	"fmt"
	"net/mail"
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

// create notification for all enabled external service providers
func (user *User) CreateNotificationAll(db *gorm.DB, action string, text string, sendTime time.Time, batchID ...uint) error {
	enabledServices := viper.GetStringSlice("external.enable")
	// bLink := bitly.New(viper.GetString("external.bitly.key")).Links
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
			case "email":
				to := mail.Address{
					Name:    user.Fullname,
					Address: user.Email,
				}
				target = to.String()
				// insert unsub link to the end of the message for email
				baseUrl := viper.GetString("domain") + "/callback/unsub?medium=email&address=" + user.Email + "&hash=" +
					fmt.Sprintf("%x", md5.Sum([]byte(user.Email+viper.GetString("external.email.unsubKey")))) +
					"&action="
				// we do not use short links for emails
				unsubAll := baseUrl + "EventReminder,EventSuggestion,EventUpdate,UserLogin"
				unsubAction := baseUrl + action
				text += fmt.Sprintf("<br />(You may want to <a href=\"%s\"> unsub all</a> or just <a href=\"%s\"> unsub %s </a>)",
					unsubAll, unsubAction, action)
			case "sms":
				// TODO: SMS Implementation
				// Use short links for SMS: bLink.Shorten(...)
			}
			notification := Notification{
				UserID:   &user.ID,
				Text:     &text,
				Target:   &target,
				SendTime: &sendTime,
				Medium:   &enabledService,
			}
			if batchID != nil {
				notification.BatchID = &batchID[0]
			}
			if err := db.Save(&notification).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
