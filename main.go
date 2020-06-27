package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"schrodinger-box/internal/api"
	"schrodinger-box/internal/callback"
	"schrodinger-box/internal/middleware"
	"schrodinger-box/internal/model"
	"schrodinger-box/internal/telegram"
)

var startTime time.Time

func main() {
	startTime = time.Now()
	gin.ForceConsoleColor()

	viper.SetConfigName("schrodinger-box.yaml")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if home, err := os.UserHomeDir(); err == nil {
		viper.AddConfigPath(home)
	}
	viper.AddConfigPath("/etc")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	// set debug mode for gin
	if viper.GetBool("debug") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// load essential interfaces (Telegram bot API, database)
	// Telegram Bot API
	bot, err := tgbotapi.NewBotAPI(viper.GetString("external.telegram.key"))
	if err != nil {
		panic("Failed to connect to Telegram bot API: " + err.Error())
	} else {
		bot.Debug = viper.GetBool("debug")
		debugPrint("Authorized on account %s", bot.Self.UserName)
	}
	// database
	db, err := gorm.Open(mysql.Open(viper.GetString("database")), &gorm.Config{})
	if err != nil {
		panic("Fail to connect to DB: " + err.Error())
	} else {
		debugPrint("Database connected")
	}
	if viper.GetBool("debug") {
		db = db.Debug()
	}
	tables := []interface{}{
		model.Token{},
		model.User{},
		model.Event{},
		model.EventSignup{},
		model.TelegramSubscription{},
	}
	if err := db.AutoMigrate(tables...); err != nil {
		panic("Failed to migrate tables: " + err.Error())
	} else {
		debugPrint("Database migrated")
	}

	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.Use(gin.Recovery())
	router.Use(middleware.OptionsMiddleware)
	router.Use(middleware.DatabaseMiddleware(db))

	// router group dealing with all API calls from front end
	apiRouter := router.Group(viper.GetString("apiRoot"))
	apiRouter.Use(middleware.APIMiddleware)
	{
		apiRouter.GET("/uptime", uptime)

		apiRouter.POST("/token", api.TokenCreate)
		apiRouter.GET("/token", api.TokenGet)

		userRouter := apiRouter.Group("/user")
		userRouter.Use(middleware.TokenMiddleware())
		{
			userRouter.GET("", api.UserGetSelf)
			userRouter.POST("", api.UserCreate)
			userRouter.PATCH("", api.UserUpdate)
			userRouter.GET("/:id", api.UserGet)
		}

		eventRouter := apiRouter.Group("/event")
		eventRouter.Use(middleware.TokenMiddleware())
		{
			eventRouter.POST("", api.EventCreate)
			eventRouter.GET("/:id", api.EventGet)
			eventRouter.POST("/signup", api.EventSignupCreate)
			eventRouter.DELETE("/signup/:id", api.EventSignupDelete)
		}

		apiRouter.GET("/events", middleware.TokenMiddleware(), api.EventsGet)
	}

	callbackRouter := router.Group("/callback")
	{
		callbackRouter.GET("/openid/:tokenId", callback.HandleOpenidCallback)
	}

	router.GET("/print_token", middleware.TokenMiddleware(), printToken)

	c := cron.New()
	// telegram updates handler
	go telegram.Loop(db, bot)
	// telegram event scheduler
	c.AddFunc(viper.GetString("api.telegram.cron"), func() { telegram.Cron(db, bot) })

	c.Start()
	// listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
	router.Run()
}

func printToken(ctx *gin.Context) {
	_, exists := ctx.Get("User")
	if !exists {
		// User has not been created, return 404 to tell client to create user
		ctx.String(http.StatusBadRequest, "You have not yet create a Schrodinger's Box account.")
		return
	}
	token := ctx.MustGet("Token").(*model.Token)
	ctx.String(http.StatusOK, "Your Token ID is: %d;\nYour Token Secret is: %s", token.ID, *token.Secret)
}

func uptime(ctx *gin.Context) {
	ctx.String(http.StatusOK, "{\"meta\":{\"uptime\": \""+fmt.Sprintf("%s", time.Since(startTime))+"\"}}")
}

// this function prints a line of debug information to the default IO writer
// debugging status and DefaultWriter are inherited from gin
func debugPrint(format string, values ...interface{}) {
	if gin.IsDebugging() {
		if !strings.HasSuffix(format, "\n") {
			format += "\n"
		}
		fmt.Fprintf(gin.DefaultWriter, "[Schrodinger's Box] "+format, values...)
	}
}
