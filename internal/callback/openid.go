package callback

import (
	"net/http"
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
	domain := viper.Get("domain")
	if err == nil {
		token := model.Token{}
		if err := db.First(&token, tokenId).Error; err == nil {
			active := "active"
			token.Status = &active
			idSlice := strings.Split(id, "/")
			token.NUSID = idSlice[len(idSlice)-1]
			token.Email = ctx.Query("openid.sreg.email")
			token.Fullname = ctx.Query("openid.sreg.fullname")
			if err := db.Save(&token).Error; err == nil {
				ctx.HTML(http.StatusOK, "callback.tmpl", gin.H{
					"domain": domain,
					"name":   token.Fullname,
				})
			} else {
				ctx.HTML(http.StatusInternalServerError, "error.tmpl", gin.H{
					"domain": domain,
					"error":  "Unable to save authentication information to database:\n" + err.Error(),
				})
			}
		} else {
			ctx.HTML(http.StatusInternalServerError, "error.tmpl", gin.H{
				"domain": domain,
				"error":  "Unable to retrieve token from database:\n" + err.Error(),
			})
		}
	} else {
		ctx.HTML(http.StatusInternalServerError, "error.tmpl", gin.H{
			"domain": domain,
			"error":  "Unable to verify your authentication callback:\n" + err.Error(),
		})
	}
}
