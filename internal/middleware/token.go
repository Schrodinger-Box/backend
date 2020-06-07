package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"

	"schrodinger-box/internal/misc"
	"schrodinger-box/internal/model"
)

// This is a middleware checking whether token is present and valid
func TokenMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := model.Token{}
		if err := ctx.ShouldBindHeader(&token); err != nil {
			misc.ReturnStandardError(ctx, 401, "token missing")
			return
		}
		db := ctx.MustGet("DB").(*gorm.DB)
		if err := db.Where(&token).First(&token).Error; gorm.IsRecordNotFoundError(err) {
			misc.ReturnStandardError(ctx, 401, "token information invalid")
			return
		} else if err != nil {
			misc.ReturnStandardError(ctx, 500, err.Error())
			return
		}
		ctx.Set("Token", &token)
		// Get related user object as well
		if *token.Status == "active" {
			user := model.User{}
			user.NUSID = token.NUSID
			if err := db.Where(&user).First(&user).Error; err == nil {
				ctx.Set("User", &user)
			} else if !gorm.IsRecordNotFoundError(err) {
				// There is something wrong other than RecordNotFound (RNF means user has not been created)
				misc.ReturnStandardError(ctx, 500, err.Error())
				return
			}
		} else {
			misc.ReturnStandardError(ctx, http.StatusUnauthorized, "token is not active")
			return
		}
	}
}
