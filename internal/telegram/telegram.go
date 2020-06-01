package telegram

import (
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/spf13/viper"
)

// this file contains everything regarding telegram bot integration

// this function handles updates received from the bot API
func Loop() {
	bot, err := tgbotapi.NewBotAPI(viper.GetString("api.telegram.key"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = false

	debugPrint("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// TODO: do something with the update message
		// this prototype bot replies whatever text received by it
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
		bot.Send(msg)
		debugPrint("[%s] %s <- %s", update.Message.From.UserName, update.Message.Text, msg.Text)
	}
}

func Cron() {
	// TODO: do some scheduled jobs (for example, sending reminders to users)
	debugPrint("%s", "Cron job executed")
}

// this function prints a line of debug information to the default IO writer
// debugging status and DefaultWriter are inherited from gin
func debugPrint(format string, values ...interface{}) {
	if gin.IsDebugging() {
		if !strings.HasSuffix(format, "\n") {
			format += "\n"
		}
		fmt.Fprintf(gin.DefaultWriter, "[Telegram API] "+format, values...)
	}
}
