package callback

import (
	"crypto/md5"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"net/http"
	"reflect"
	"schrodinger-box/internal/model"
	"strings"
)

type UnsubQuery struct {
	// email or sms
	Medium string `form:"medium" binding:"required"`
	// a single email address, or HP number
	Address string `form:"address" binding:"required"`
	// verification hash
	Hash string `form:"hash" binding:"required"`
	// action list: multiple action is separated by a comma (",")
	Action string `form:"action" binding:"required"`
}

func HandleUnsub(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/html")
	query := &UnsubQuery{}
	if err := ctx.ShouldBindQuery(query); err != nil {
		ctx.String(http.StatusBadRequest, "Unable to parse request query - "+err.Error())
		return
	} else if query.Medium != "email" && query.Medium != "sms" {
		ctx.String(http.StatusBadRequest, "Not an acceptable medium - "+query.Medium)
		return
	} else if query.Hash !=
		fmt.Sprintf("%x", md5.Sum([]byte(query.Address+viper.GetString("external."+query.Medium+".unsubKey")))) {
		ctx.String(http.StatusUnauthorized, "Failed to verify your hash.")
		return
	}
	actions := strings.Split(query.Action, ",")
	db := ctx.MustGet("DB").(*gorm.DB)
	subscription := model.NotificationSubscription{}
	if query.Medium == "email" {
		user := model.User{}
		if err := db.Preload("Subscription").Where("email = ?", query.Address).First(&user).Error; err != nil {
			ctx.String(http.StatusInternalServerError, "Unable to fetch user object - "+err.Error())
			return
		} else if user.Subscription == nil {
			ctx.String(http.StatusNotFound, "No subscription has been found for this user")
			return
		} else {
			subscription = *user.Subscription
		}
	} else if err := db.Where("sms_number = ?", query.Address).First(&subscription).Error; err != nil {
		// query.Medium == "sms"
		ctx.String(http.StatusInternalServerError, "Unable to fetch subscription object - "+err.Error())
		return
	}
	// toggle subscription flags
	val := reflect.ValueOf(&subscription).Elem()
	for _, action := range actions {
		field := val.FieldByName(model.ServicePrefix[query.Medium] + action)
		if !field.IsValid() || !field.CanSet() || field.Kind() != reflect.Ptr || field.Elem().Kind() != reflect.Bool {
			// invalid field name
			continue
		}
		field.Elem().SetBool(false)
	}
	if err := db.Save(subscription).Error; err != nil {
		ctx.String(http.StatusInternalServerError, "No subscription has been found for this user")
	} else {
		ctx.String(http.StatusOK, "you have unsubscribed "+query.Medium+" messages for actions="+query.Action+
			".<br />You can now close this tab safely.")
	}
}
