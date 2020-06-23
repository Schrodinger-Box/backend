package middleware

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func DatabaseMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set("DB", db)
		ctx.Next()
	}
}
