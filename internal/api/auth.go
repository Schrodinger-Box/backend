package api

import (
    "log"
    "net/http"

    "github.com/Schrodinger-Box/openid-go"
    "github.com/gin-gonic/gin"
    "github.com/google/jsonapi"
)

// This file contains methods handling authentication and authorization

// This method receives Basic HTTP authentication and return a token when credentials are valid
func CreateToken(ctx *gin.Context) {
    // TODO: do something to handle the authentication request
    type SampleToken struct {
        ID          uint        `jsonapi:"primary,token" gorm:"primary_key"`
        OpenIDUrl   string      `jsonapi:"attr,openid_url"`
        // some other properties
    }

    sampleToken := SampleToken{ID: 123}
    // TODO: sample -> nusnet id
    openid.SetSregFields(map[string]bool {
        "email":    false,
        "fullname": false,
    })
    if url, err := openid.RedirectURL("https://openid.nus.edu.sg",
        "http://localhost:8080/callback/openid",
        "http://localhost:8080/"); err == nil {
        sampleToken.OpenIDUrl = url
    } else {
        log.Print(err)
        // TODO error handling
    }

    if err := jsonapi.MarshalPayload(ctx.Writer, &sampleToken); err != nil {
        http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
    }
}