package api

import (
    "log"
    "net/http"

    "github.com/Schrodinger-Box/openid-go"
    "github.com/gin-gonic/gin"
    "github.com/google/jsonapi"
    "github.com/google/uuid"
    "github.com/jinzhu/gorm"
    "github.com/spf13/viper"

    "schrodinger-box/internal/model"
)

// This file contains methods handling authentication and authorization


// This method receives Basic HTTP authentication and return a token when credentials are valid
func CreateToken(ctx *gin.Context) {
    openid.SetSregFields(map[string]bool {
        "email":    false,
        "fullname": false,
    })
    if url, err := openid.RedirectURL("https://openid.nus.edu.sg",
        viper.GetString("domain") + "/callback/openid",
        viper.GetString("domain"),
        viper.GetBool("openid.associationMode"),
        viper.GetBool("openid.doubleVerification")); err == nil {
        db := ctx.MustGet("DB").(*gorm.DB)
        secret := uuid.New().String()
        token := model.Token{
            Secret: &secret,
            AuthURL: url,
        }
        db.Save(&token)
        ctx.Writer.WriteHeader(http.StatusCreated)
        if err := jsonapi.MarshalPayload(ctx.Writer, &token); err != nil {
            http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
        }
    } else {
        log.Print(err)
        // TODO error handling
    }
}