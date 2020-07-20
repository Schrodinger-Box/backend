package external

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"schrodinger-box/internal/model"
	"strings"
	"time"
)

// this file contains general functions for notification system to work

var timeOffset = map[string]time.Duration{
	"1day":  24 * time.Hour,
	"4hr":   4 * time.Hour,
	"30min": 30 * time.Minute,
}

func NotificationCron(db *gorm.DB) {
	// generate notifications from batches
	var batches []*model.NotificationBatch
	db.Where("generate_time < ?", time.Now()).Find(&batches)
	for _, batch := range batches {
		link := strings.Split(*batch.LinkID, "-")
		switch link[0] {
		case "Event":
			event := &model.Event{}
			if err := db.Preload("EventSignups").
				Preload("EventSignups.User").
				Preload("EventSignups.User.Subscription").
				First(event, link[1]).Error; err != nil {
				fmt.Fprintf(gin.DefaultWriter, "[Schrodinger's Box] Cannot find resource - %s", err.Error())
				continue
			}
			tx := db.Begin()
			errorOccurred := false
			for _, signup := range event.EventSignups {
				var sendTime time.Time
				var action string
				switch link[2] {
				case "1day", "4hr", "30min":
					action = "EventReminder"
					sendTime = event.TimeBegin.Add(-timeOffset[link[2]])
				default:
					continue
					// TODO: do nothing for other actions
				}
				text := batch.GenText(map[string]string{
					"fullname": signup.User.Fullname,
					"nickname": *signup.User.Nickname,
					"email":    signup.User.Email,
					"nusid":    signup.User.NUSID,
				})
				// we have the text and now create notification objects for all enabled mediums
				if err := signup.User.CreateNotificationAll(tx, action, text, sendTime, batch.ID); err != nil {
					fmt.Fprintf(gin.DefaultWriter, "[Schrodinger's Box] Cannot create notification - %s", err.Error())
					errorOccurred = true
					break
				}
			}
			if errorOccurred {
				tx.Rollback()
				continue
			}
			if err := batch.Generated(tx); err != nil {
				fmt.Fprintf(gin.DefaultWriter, "[Schrodinger's Box] Cannot set batch status as generated - %s", err.Error())
				tx.Rollback()
				continue
			}
			tx.Commit()
		default:
			fmt.Fprintf(gin.DefaultWriter, "[Schrodinger's Box] Unknown LinkID resource type - %s", *batch.LinkID)
			continue
		}
	}
}
