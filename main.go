package main

import (
    "fmt"
    "os"

    "github.com/gin-gonic/gin"
    "github.com/robfig/cron/v3"
    "github.com/spf13/viper"

    "schrodinger-box/internal/api"
    "schrodinger-box/internal/callback"
    "schrodinger-box/internal/middleware"
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

    router := gin.Default()
    router.Use(middleware.DatabaseMiddleware(viper.GetString("database")))

    // router group dealing with all API calls from front end
    apiRouter := router.Group("/api")
    apiRouter.Use(middleware.APIMiddleware())
    {
        apiRouter.POST("token", api.CreateToken)
    }

    callbackRouter := router.Group("/callback")
    {
        callbackRouter.GET("/openid/:tokenId", callback.HandleOpenidCallback)
    }

    c := cron.New()

    // telegram updates handler
    go telegram.Loop()
    // telegram event scheduler
    c.AddFunc(viper.GetString("api.telegram.cron"), telegram.Cron)

    c.Start()
    // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
    router.Run()
}
