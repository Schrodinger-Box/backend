package external

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"schrodinger-box/internal/model"
	"time"
)

// this file contains all methods to handle email scheduled jobs and send emails

func EmailCron(db *gorm.DB) {
	// sending scheduled emails
	var notifications []*model.Notification
	db.Where("send_time < ?", time.Now()).Where("medium = ?", "email").Find(&notifications)
	for _, notification := range notifications {
		if err := emailSend(*notification.Target, *notification.Text); err != nil {
			fmt.Fprintf(gin.DefaultWriter, "[Schrodinger's Box] Cannot send email - %s", err.Error())
			continue
		}
		notification.Sent(db)
	}
}

// initiate a new email client and send a text to @to (an email address in RFC 5322 format)
func emailSend(toString string, messageString string) (err error) {
	var from, to *mail.Email
	if from, err = mail.ParseEmail(viper.GetString("external.email.from")); err != nil {
		return
	} else if to, err = mail.ParseEmail(toString); err != nil {
		return
	}
	message := mail.NewSingleEmail(from, "New Message from Schrodinger's Box", to, "You have a new message from Scherodinger's Box:", "<p>"+messageString+"</p>")
	client := sendgrid.NewSendClient(viper.GetString("external.email.key"))
	result, err := client.Send(message)
	if err == nil && result.StatusCode >= 300 {
		err = errors.New("sendgrid returned a non-200 response")
	}
	return err
}
