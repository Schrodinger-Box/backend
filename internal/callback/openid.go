package callback

import (
    "log"

    "github.com/Schrodinger-Box/gormid"
    "github.com/Schrodinger-Box/openid-go"
    "github.com/gin-gonic/gin"
    "github.com/jinzhu/gorm"
    "github.com/spf13/viper"
)

func HandleOpenidCallback(ctx *gin.Context) {
    db := ctx.MustGet("DB").(*gorm.DB)
    gormStore := gormid.CreateNewStore(db)
    fullUrl := viper.GetString("domain") + ctx.Request.URL.String()
    id, err := openid.Verify(
        fullUrl,
        gormStore.DiscoveryCache, gormStore.NonceStore)
    if err == nil {
        p := make(map[string]string)
        p["user"] = id
        log.Println("OpenID callback verified, identity=" + id)
    } else {
        log.Println("OpenID callback verification error")
        log.Print(err)
    }
}