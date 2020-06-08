package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Schrodinger-Box/openid-go"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"schrodinger-box/internal/misc"
	"schrodinger-box/internal/model"
)

// This file contains methods handling authentication and authorization

// This method receives Basic HTTP authentication and return a token when credentials are valid
func CreateToken(ctx *gin.Context) {
	db := ctx.MustGet("DB").(*gorm.DB)
	secret := uuid.New().String()
	token := model.Token{
		Secret: &secret,
	}
	if db.Save(&token).Error == nil {
		openid.SetSregFields(map[string]bool{
			"email":    false,
			"fullname": false,
		})
		if url, err := openid.RedirectURL("https://openid.nus.edu.sg",
			viper.GetString("domain")+"/callback/openid/"+fmt.Sprint(token.ID),
			viper.GetString("domain"),
			viper.GetBool("openid.associationMode"),
			viper.GetBool("openid.doubleVerification")); err == nil {
			token.AuthURL = url
			ctx.Writer.WriteHeader(http.StatusCreated)
			if err := jsonapi.MarshalPayload(ctx.Writer, &token); err != nil {
				misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
			}
		} else {
			log.Print(err)
			// TODO error handling
		}
	} else {
		// TODO error handling (db)
	}
}
