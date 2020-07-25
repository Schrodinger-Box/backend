package external

import (
	"errors"
	"fmt"
	"strconv"
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
		// check if subscribed to provide different help info and steps
		subscription := &model.NotificationSubscription{}
		if err := db.Where("telegram_chat_id = ?", chatId).First(subscription).Error; err != nil {
			// this telegram account has not subscribed to anyone
			subscription = nil
		}
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "help", "start":
				msg.Text = "Welcome to Schrodinger's Box Telegram bot!\n" +
					"You can:\n"
				if subscription == nil {
					msg.Text += "type /subscribe to subscribe to notifications from Schrodinger's Box;\n"
				} else {
					msg.Text += "type /unsub to unsubscribe to ALL notifications from Schrodinger's Box;\n"
					msg.Text += "type /adjust to adjust what type of messages you want to subscribe;\n"
					msg.Text += "type /check to check who you subscribed to;\n"
				}
				msg.Text += "type /help to show this message again."
			case "subscribe":
				if subscription == nil {
					msg.Text = "Please enter your Token ID. This can be obtained from the website."
					actionCache[chatId] = "token_id"
				} else {
					msg.Text = "You have already subscribed to a user. " +
						"You have to unsubscribe from it before you can make new subscription!"
				}
			case "unsub":
				if subscription == nil {
					msg.Text = "You have not subscribed to anyone!"
				} else {
					if err := db.Model(subscription).Updates(map[string]interface{}{"telegram_chat_id": nil}).Error; err != nil {
						msg.Text = "Something wrong occurred at the server side. Maybe try this again later?"
					} else {
						msg.Text = "Successfully unsubscribed. Hope we can get your subscription again in the future."
					}
				}
			case "adjust":
				if subscription == nil {
					msg.Text = "You have not subscribed to anyone!"
				} else {
					msg.Text = getSubscriptionText(subscription)
					actionCache[chatId] = "adjust"
				}
			case "check":
				if subscription == nil {
					msg.Text = "You have not subscribed to anyone!"
				} else {
					user := &model.User{}
					if err := db.First(user, subscription.UserID).Error; err != nil {
						msg.Text = "Error occurred when retrieving user information"
					} else {
						msg.Text = fmt.Sprintf("You have subscribed to user %s (uid=%d).", *user.Nickname, user.ID)
					}
				}
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
							subscription = &model.NotificationSubscription{}
							if err := db.Where("user_id = ?", user.ID).FirstOrInit(subscription).Error; err != nil {
								// something wrong other than record not found occurred, not changing action cache for this case
								msg.Text = "Something wrong happens when subscribing you to this user.\n" +
									"Please try to enter Token Secret a while later."
							} else if subscription.TelegramChatID != nil {
								msg.Text = "Someone has subscribed to this user's Telegram notifications!"
								actionCache[chatId] = ""
							} else {
								subscription.UserID = &user.ID
								subscription.TelegramChatID = &chatId
								if err := db.Save(subscription).Error; err != nil {
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
				case "adjust":
					if update.Message.Text == "done" {
						actionCache[chatId] = ""
						msg.Text = "OK. You are now at the main menu again.\nType /help to list commands available."
					} else {
						var err error
						switch update.Message.Text {
						case "1":
							err = db.Model(subscription).Update("telegram_event_reminder", !*subscription.TelegramEventReminder).Error
						case "2":
							err = db.Model(subscription).Update("telegram_event_suggestion", !*subscription.TelegramEventSuggestion).Error
						case "3":
							err = db.Model(subscription).Update("telegram_user_login", !*subscription.TelegramUserLogin).Error
						default:
							err = errors.New("invalid index entered")
						}
						if err == nil {
							msg.Text = "Successfully toggled setting.\n\n"
						} else {
							msg.Text = "Something wrong occurred: " + err.Error() + "\n\n"
						}
						msg.Text += getSubscriptionText(subscription)
						msg.Text += "\nYou can enter 'done' to go back to main menu."
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
	// sending scheduled notifications
	var notifications []*model.Notification
	db.Where("send_time < ?", time.Now()).Where("medium = ?", "telegram").Find(&notifications)
	for _, notification := range notifications {
		chatId, _ := strconv.Atoi(*notification.Target)
		if _, err := TelegramSend(bot, int64(chatId), *notification.Text); err != nil {
			fmt.Fprintf(gin.DefaultWriter, "[Schrodinger's Box] Cannot send message - %s", err.Error())
			continue
		}
		notification.Sent(db)
	}
}

func TelegramSend(bot *tgbotapi.BotAPI, chatId int64, message string) (tgbotapi.Message, error) {
	return bot.Send(tgbotapi.NewMessage(chatId, message))
}

func getSubscriptionText(subscription *model.NotificationSubscription) string {
	text := "Your current subscription setting is:\n"
	if *subscription.TelegramEventReminder {
		text += "1. Event Reminder - ON\n"
	} else {
		text += "1. Event Reminder - OFF\n"
	}
	if *subscription.TelegramEventSuggestion {
		text += "2. Event Suggestion - ON\n"
	} else {
		text += "2. Event Suggestion - OFF\n"
	}
	if *subscription.TelegramUserLogin {
		text += "3. New Login Notification - ON\n"
	} else {
		text += "3. New Login Notification - OFF\n"
	}
	text += "\nPlease enter the index to toggle each item. "
	return text
}
