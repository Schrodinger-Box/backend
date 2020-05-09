package api

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/jsonapi"
)

// This file contains methods handling authentication and authorization

// This method receives Basic HTTP authentication and return a token when credentials are valid
func CreateToken(ctx *gin.Context) {
    // TODO: do something to handle the authentication request
    type SampleToken struct {
        ID          uint        `jsonapi:"primary,upstreams" gorm:"primary_key"`
        // some other properties
    }
    sampleToken := SampleToken{ID: 123}
    if err := jsonapi.MarshalPayload(ctx.Writer, &sampleToken); err != nil {
        http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
    }
}