package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/google/jsonapi"
)

func GeneralMiddleware() gin.HandlerFunc {
    return func(ctx *gin.Context) {
        ctx.Header("Content-Type", jsonapi.MediaType)
    }
}