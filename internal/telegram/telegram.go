package telegram

import (
	"errors"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"gorm.io/gorm"

	"schrodinger-box/internal/model"
)

// this file contains everything regarding telegram bot integration

// this mask determines what kind of message will be sent to the user
// by default, user will receive all notifications upon subscription
const (
	MaskEventJoined = 0b0000000000000001
	MaskDebug       = 0b1000000000000000
	MaskAll         = 0b0111111111111111
)

// this function handles updates received from the bot API
func Loop(db *gorm.DB, bot *tgbotapi.BotAPI) {
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
							subscription := model.TelegramSubscription{
								UserID: &user.ID,
								ChatID: &chatId,
							}
							if err := db.Where(&subscription).First(&subscription).Error; err == nil {
								msg.Text = "You have already subscribed to this user's notifications"
								actionCache[chatId] = ""
							} else {
								subscription.Mask = MaskAll
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

func Cron(db *gorm.DB, bot *tgbotapi.BotAPI) {
	// TODO: do some scheduled jobs (for example, sending reminders to users)
	var subscriptions []model.TelegramSubscription
	db.Find(&subscriptions)
	for _, subscription := range subscriptions {
		if subscription.Mask&MaskEventJoined != 0 {
			// this user subscribed to notifications from event joined, do something
		}
		if subscription.Mask&MaskDebug != 0 {
			sendMessage(bot, *subscription.ChatID, "Debug - Cron is running")
		}
	}
	// this sends a debug information to all users with debug flag enabled

}

func sendMessage(bot *tgbotapi.BotAPI, chatId int64, message string) (tgbotapi.Message, error) {
	return bot.Send(tgbotapi.NewMessage(chatId, message))
}
