package external

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"schrodinger-box/internal/model"
	"strings"
	"time"
)

func SmsCron(db *gorm.DB) {
	// sending scheduled sms
	var notifications []*model.Notification
	db.Where("send_time < ?", time.Now()).Where("medium = ?", "sms").Find(&notifications)
	for _, notification := range notifications {
		if err := SMSSend(*notification.Target, *notification.Text); err != nil {
			fmt.Fprintf(gin.DefaultWriter, "[Schrodinger's Box] Cannot send sms - %s", err.Error())
			continue
		}
		notification.Sent(db)
	}
}

func SMSSend(to string, text string) error {
	sid := viper.GetString("external.sms.sid")
	token := viper.GetString("external.sms.token")
	urlString := "https://api.twilio.com/2010-04-01/Accounts/" + sid + "/Messages.json"

	msgData := url.Values{}
	msgData.Set("To", to)
	msgData.Set("From", viper.GetString("external.sms.from"))
	msgData.Set("Body", text)
	msgDataReader := *strings.NewReader(msgData.Encode())

	client := http.Client{}
	req, _ := http.NewRequest("POST", urlString, &msgDataReader)
	req.SetBasicAuth(sid, token)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	result, err := client.Do(req)
	if err == nil && (result.StatusCode >= 300 || result.StatusCode < 200) {
		err = errors.New("twilio returned a non-200 response")
	}
	return err
}
