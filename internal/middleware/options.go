package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/spf13/viper"
)

func OptionsMiddleware(ctx *gin.Context) {
	if ctx.Request.Method != "OPTIONS" {
		ctx.Next()
	} else {
		ctx.Header("Access-Control-Allow-Origin", viper.GetString("cors.origin"))
		ctx.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "authorization, origin, content-type, accept, x-token-id, x-token-secret")
		ctx.Header("Allow", "HEAD,GET,POST,PUT,PATCH,DELETE,OPTIONS")
		ctx.Header("Content-Type", jsonapi.MediaType)
		ctx.AbortWithStatus(http.StatusOK)
	}
}
