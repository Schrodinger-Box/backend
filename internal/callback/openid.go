package callback

import (
    "log"

    "github.com/Schrodinger-Box/openid-go"
    "github.com/gin-gonic/gin"
)

var nonceStore = openid.NewSimpleNonceStore()
var discoveryCache = openid.NewSimpleDiscoveryCache()

func HandleOpenidCallback(ctx *gin.Context) {
    fullUrl := "http://localhost:8080" + ctx.Request.URL.String()
    log.Print(fullUrl)
    id, err := openid.Verify(
        fullUrl,
        discoveryCache, nonceStore)
    if err == nil {
        p := make(map[string]string)
        p["user"] = id
        log.Print(id)
    } else {
        log.Println("WTF2")
        log.Print(err)
    }
}