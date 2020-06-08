package callback

import (
	"log"
	"strings"

	"github.com/Schrodinger-Box/gormid"
	"github.com/Schrodinger-Box/openid-go"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"schrodinger-box/internal/model"
)

func HandleOpenidCallback(ctx *gin.Context) {
	tokenId := ctx.Param("tokenId")
	db := ctx.MustGet("DB").(*gorm.DB)
	gormStore := gormid.CreateNewStore(db)
	fullUrl := viper.GetString("domain") + ctx.Request.URL.String()
	id, err := openid.Verify(
		fullUrl,
		gormStore.DiscoveryCache, gormStore.NonceStore)
	if err == nil {
		token := model.Token{}
		if db.First(&token, tokenId).Error == nil {
			active := "active"
			token.Status = &active
			idSlice := strings.Split(id, "/")
			token.NUSID = idSlice[len(idSlice)-1]
			token.Email = ctx.Query("openid.sreg.email")
			token.Fullname = ctx.Query("openid.sreg.fullname")
			if db.Save(&token).Error == nil {
				// TODO render HTML document to save the new token into localStorage
			} else {
				// TODO database saving error handling
			}
		} else {
			// TODO database error handling
		}
	} else {
		log.Println("OpenID callback verification error")
		log.Print(err)
	}
}
