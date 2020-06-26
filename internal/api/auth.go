package api

import (
	"errors"
	"fmt"
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
func TokenCreate(ctx *gin.Context) {
	db := ctx.MustGet("DB").(*gorm.DB)
	secret := uuid.New().String()
	token := model.Token{
		Secret: &secret,
	}
	if err := db.Save(&token).Error; err == nil {
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
			ctx.Status(http.StatusCreated)
			if err := jsonapi.MarshalPayload(ctx.Writer, &token); err != nil {
				misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
			}
		} else {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		}
	} else {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	}
}

func TokenGet(ctx *gin.Context) {
	token := model.Token{}
	if err := ctx.ShouldBindHeader(&token); err != nil {
		misc.ReturnStandardError(ctx, 401, "token missing")
		return
	}
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.Where(&token).First(&token).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, 401, "token information invalid")
		return
	} else if err != nil {
		misc.ReturnStandardError(ctx, 500, err.Error())
		return
	}
	ctx.Status(http.StatusOK)
	if err := jsonapi.MarshalPayload(ctx.Writer, &token); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	}
}
