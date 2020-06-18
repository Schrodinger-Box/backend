package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"

	"schrodinger-box/internal/api"
	"schrodinger-box/internal/callback"
	"schrodinger-box/internal/middleware"
	"schrodinger-box/internal/model"
	"schrodinger-box/internal/telegram"
)

func main() {
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

	connString := viper.GetString("database")
	router := gin.Default()
	router.Use(gin.Recovery())
	router.Use(middleware.DatabaseMiddleware(connString))

	// router group dealing with all API calls from front end
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.APIMiddleware())
	{
		apiRouter.POST("/token", api.CreateToken)
		userRouter := apiRouter.Group("/user")
		userRouter.Use(middleware.TokenMiddleware())
		{
			userRouter.GET("/", api.UserGetSelf)
			userRouter.POST("/", api.UserCreate)
			userRouter.PATCH("/", api.UserUpdate)
			userRouter.GET("/:id", api.UserGet)
		}
		eventRouter := apiRouter.Group("/event")
		eventRouter.Use(middleware.TokenMiddleware())
		{
			eventRouter.POST("/", api.EventCreate)
			eventRouter.GET("/:id", api.EventGet)
		}
	}

	callbackRouter := router.Group("/callback")
	{
		callbackRouter.GET("/openid/:tokenId", callback.HandleOpenidCallback)
	}

	router.GET("/print_token", middleware.TokenMiddleware(), printToken)
	router.GET("/ping", printPing)

	c := cron.New()

	// telegram updates handler
	go telegram.Loop(connString)
	// telegram event scheduler
	c.AddFunc(viper.GetString("api.telegram.cron"), func() { telegram.Cron(connString) })

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

func printPing(ctx *gin.Context) {
	ctx.String(http.StatusOK, "pong")
}
