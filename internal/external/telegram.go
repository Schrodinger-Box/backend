package external

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"gorm.io/gorm"

	"schrodinger-box/internal/model"
)

// this file contains everything regarding telegram bot integration

// this function handles updates received from the bot API
func TelegramLoop(db *gorm.DB, bot *tgbotapi.BotAPI) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, _ := bot.GetUpdatesChan(u)

	actionCache := make(map[int64]string)
	dataCache := make(map[int64]interface{})
	for update := range updates {
		// we only receive command / replies through private chats
		if update.Message == nil || !update.Message.Chat.IsPrivate() {
			continue
		}
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		chatId := update.Message.Chat.ID
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "help":
				msg.Text = "Welcome to Schrodinger's Box Telegram bot!\n" +
					"You can:\n" +
					"type /subscribe to subscribe to notifications from Schrodinger's Box;\n" +
					"type /help to show this message again."
			case "subscribe":
				msg.Text = "Please enter your Token ID. This can be obtained from the website."
				actionCache[chatId] = "token_id"
			}
		} else {
			if val, ok := actionCache[chatId]; ok && val != "" {
				switch val {
				case "token_id":
					if i, err := strconv.Atoi(update.Message.Text); err != nil {
						msg.Text = "We can't understand your reply. Is it a number?"
					} else {
						dataCache[chatId] = i
						msg.Text = "Nice! Now enter your Token Secret."
						actionCache[chatId] = "token_secret"
					}
				case "token_secret":
					tokenId := dataCache[chatId].(int)
					tokenSecret := update.Message.Text
					token := model.Token{
						ID:     uint(tokenId),
						Secret: &tokenSecret,
					}
					if err := db.Where(&token).First(&token).Error; errors.Is(err, gorm.ErrRecordNotFound) {
						msg.Text = "Oops, we cannot authenticate you. Please make sure you have entered correct Token information.\n" +
							"Now, please enter your token ID again."
						actionCache[chatId] = "token_id"
					} else if err != nil {
						msg.Text = "Something wrong happens when searching for your token in our database.\n" +
							"Please try to enter Token Secret a while later."
						// not changing action cache for this case
					} else if *token.Status != "active" {
						msg.Text = "Your token is inactive. Please obtain a new one from the website.\n" +
							"Enter your new Token ID after that."
						actionCache[chatId] = "token_id"
					} else {
						user := model.User{}
						user.NUSID = token.NUSID
						if err := db.Where(&user).First(&user).Error; err != nil {
							msg.Text = "Something wrong happens when searching for your user in our database.\n" +
								"Please try to enter Token Secret a while later."
							// not changing action cache for this case
						} else {
							subscription := model.NotificationSubscription{}
							if err := db.Where("user_id = ?", user.ID).FirstOrInit(&subscription).Error; err != nil {
								// something wrong other than record not found occurred, not changing action cache for this case
								msg.Text = "Something wrong happens when subscribing you to this user.\n" +
									"Please try to enter Token Secret a while later."
							} else if subscription.TelegramChatID != nil {
								msg.Text = "Someone has subscribed to this user's Telegram notifications!"
								actionCache[chatId] = ""
							} else {
								subscription.UserID = &user.ID
								subscription.TelegramChatID = &chatId
								if err := db.Save(&subscription).Error; err != nil {
									msg.Text = "Something wrong happens when subscribing you to this user.\n" +
										"Please try to enter Token Secret a while later."
									// not changing action cache for this case
								} else {
									msg.Text = "You have successfully subscribed to notifications for user " + *user.Nickname
									actionCache[chatId] = ""
								}
							}
						}
					}
				}
			} else {
				msg.Text = "Sorry we don't understand what you need :(\n" +
					"Maybe you can type /help for more information."
			}
		}

		bot.Send(msg)
	}
}

func TelegramCron(db *gorm.DB, bot *tgbotapi.BotAPI) {
	fmt.Printf("Cron executed")
	// generate notifications from batches
	var batches []*model.NotificationBatch
	db.Where("generate_time < ?", time.Now()).Where("medium = ?", "telegram").Find(&batches)
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
			timeOffset := map[string]time.Duration{
				"1day":  24 * time.Hour,
				"4hr":   4 * time.Hour,
				"30min": 30 * time.Minute,
			}
			tx := db.Begin()
			errorOccurred := false
			for _, signup := range event.EventSignups {
				if signup.User.Subscription == nil || signup.User.Subscription.TelegramChatID == nil {
					continue
				}
				// permission check
				var sendTime time.Time
				switch link[2] {
				case "1day":
				case "4hr":
				case "30min":
					if *signup.User.Subscription.TelegramEventReminder == false {
						continue
					}
					sendTime = event.TimeBegin.Add(-timeOffset[link[2]])
				default:
					continue
					// TODO: do nothing for other actions
				}
				target := strconv.Itoa(int(*signup.User.Subscription.TelegramChatID))
				text := batch.GenText(map[string]string{
					"fullname": signup.User.Fullname,
					"nickname": *signup.User.Nickname,
					"email":    signup.User.Email,
					"nusid":    signup.User.NUSID,
				})
				notification := model.Notification{
					UserID:   signup.UserID,
					BatchID:  &batch.ID,
					Medium:   batch.Medium,
					Target:   &target,
					Text:     &text,
					SendTime: &sendTime,
				}
				if err := tx.Save(&notification).Error; err != nil {
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

	// sending scheduled notifications
	var notifications []*model.Notification
	db.Where("send_time < ?", time.Now()).Where("medium = ?", "telegram").Find(&notifications)
	for _, notification := range notifications {
		chatId, _ := strconv.Atoi(*notification.Target)
		if _, err := sendMessage(bot, int64(chatId), *notification.Text); err != nil {
			fmt.Fprintf(gin.DefaultWriter, "[Schrodinger's Box] Cannot send message - %s", err.Error())
			continue
		}
		notification.Sent(db)
	}
}

func sendMessage(bot *tgbotapi.BotAPI, chatId int64, message string) (tgbotapi.Message, error) {
	return bot.Send(tgbotapi.NewMessage(chatId, message))
}
