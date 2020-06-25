package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/spf13/viper"
)

func APIMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Content-Type", jsonapi.MediaType)
		ctx.Header("Access-Control-Allow-Origin", viper.GetString("cors.origin"))
	}
}
