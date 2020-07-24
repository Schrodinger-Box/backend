package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"gorm.io/gorm"

	"schrodinger-box/internal/external"
	"schrodinger-box/internal/misc"
	"schrodinger-box/internal/model"
)

func UserGetSelf(ctx *gin.Context) {
	userInterface, exists := ctx.Get("User")
	if !exists {
		// User has not been created, return 404 to tell client to create user
		misc.ReturnStandardError(ctx, 404, "user has not been created")
		return
	}
	user := userInterface.(*model.User)
	user.LoadSignups(ctx.MustGet("DB").(*gorm.DB))
	ctx.Status(http.StatusOK)
	if err := jsonapi.MarshalPayload(ctx.Writer, user); err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func UserCreate(ctx *gin.Context) {
	token := ctx.MustGet("Token").(*model.Token)
	if _, exists := ctx.Get("User"); exists {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "a user linked to this NUSNET ID has been created before")
		return
	}
	userRequest := &model.User{}
	if err := jsonapi.UnmarshalPayload(ctx.Request.Body, userRequest); err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot unmarshal JSON of request: "+err.Error())
		return
	} else if userRequest.Nickname == nil || userRequest.Type == nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "nickname and type MUST be provided")
		return
	}
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.Where("nickname = ?", userRequest.Nickname).First(&model.User{}).Error; err == nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "nickname has been taken by someone else")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	// We only take the nickname and type of the request object
	// TODO: we need some permission check here (regarding type)
	user := &model.User{
		Nickname: userRequest.Nickname,
		Type:     userRequest.Type,
	}
	user.NUSID = token.NUSID
	user.Email = token.Email
	user.Fullname = token.Fullname
	if err := db.Save(user).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	// create default preference table for user
	if err := db.Save(&model.NotificationSubscription{UserID: &user.ID}).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	ctx.Status(http.StatusCreated)
	if err := jsonapi.MarshalPayload(ctx.Writer, user); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
}

func UserGet(ctx *gin.Context) {
	// TODO: we need some permission/privacy settings check here
	id := ctx.Param("id")
	user := &model.User{}
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.First(user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			misc.ReturnStandardError(ctx, http.StatusNotFound, "user does not exist")
			return
		} else {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
			return
		}
	}
	user.LoadSignups(db)
	ctx.Status(http.StatusOK)
	if err := jsonapi.MarshalPayload(ctx.Writer, user); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
}

func UserUpdate(ctx *gin.Context) {
	userRequest := &model.User{}
	if err := jsonapi.UnmarshalPayload(ctx.Request.Body, userRequest); err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot unmarshal JSON of request")
		return
	}
	user := &model.User{}
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.First(user, userRequest.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			misc.ReturnStandardError(ctx, http.StatusNotFound, "user does not exist")
			return
		} else {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
			return
		}
	}
	// TODO: we need some better permission check here
	token := ctx.MustGet("Token").(*model.Token)
	if token.NUSID != user.NUSID {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you can only update your own data")
		return
	}
	// For instance, only nickname field is allowed to be updated
	if err := db.Model(user).Select([]string{"nickname"}).Updates(userRequest).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	// No attributes provided by the server side
	ctx.Status(http.StatusOK)
	if err := jsonapi.MarshalPayload(ctx.Writer, user); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
}

func UserDelete(ctx *gin.Context) {
	var user *model.User
	if userInterface, exists := ctx.Get("User"); !exists {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you have to be a registered user to terminate yourself")
		return
	} else {
		user = userInterface.(*model.User)
	}
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.Delete(&user).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

func UserSMSBind(ctx *gin.Context) {
	number := ctx.Param("number")
	if number[0] != '+' || number == "" {
		// invalid number
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "invalid number provided")
		return
	}
	user, exist := ctx.Get("User")
	if !exist {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you must be a registered user to perform this action")
		return
	}
	var result map[string]map[string]string
	if data, err := ctx.GetRawData(); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	} else if err := json.Unmarshal(data, &result); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	db := ctx.MustGet("DB").(*gorm.DB)
	if _, exist := result["meta"]; exist {
		subscription := &model.NotificationSubscription{}
		sms := &model.SMSVerification{}
		if err := db.Where("user_id = ?", user.(*model.User).ID).FirstOrInit(subscription).Error; err != nil {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		} else if subscription.SMSNumber != nil {
			misc.ReturnStandardError(ctx, http.StatusForbidden, "there is already a number bound to your account")
		} else if err := db.Where("sms_number = ?", number).FirstOrInit(sms).Error; err != nil {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		} else if sms.Status != nil && *sms.Status == "locked" {
			misc.ReturnStandardError(ctx, http.StatusForbidden, "this number has been bound to someone else")
		} else if token, exist := result["meta"]["verification_code"]; exist {
			// verification code provided, try to verify if it is correct
			if sms.Token == nil || *sms.Token != token {
				misc.ReturnStandardError(ctx, http.StatusForbidden, "invalid verification code")
			} else {
				subscription.SMSNumber = &number
				subscription.UserID = &user.(*model.User).ID
				locked := "locked"
				sms.Status = &locked
				if err := db.Save(subscription).Error; err != nil {
					misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
				} else if err := db.Save(sms).Error; err != nil {
					misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
				} else {
					ctx.Status(http.StatusNoContent)
				}
			}
		} else {
			// verification code is not provided, try to generate a new one for this number
			rand.Seed(time.Now().UnixNano())
			// token is not provided, generate a new one for the number now
			token := rand.Intn(999999)
			text := fmt.Sprintf("Your verification code for Schrodinger's Box is [%d]", token)
			if err := external.SMSSend(number, text); err != nil {
				misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
			} else {
				sms.SMSNumber = &number
				tokenString := strconv.Itoa(token)
				sms.Token = &tokenString
				if err := db.Save(sms).Error; err != nil {
					misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
				} else {
					ctx.Status(http.StatusNoContent)
				}
			}
		}
	} else {
		// invalid input
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "meta object is missing")
	}
}

func UserSMSUnbind(ctx *gin.Context) {
	number := ctx.Param("number")
	if number[0] != '+' || number == "" {
		// invalid number
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "invalid number provided")
		return
	}
	user, exist := ctx.Get("User")
	if !exist {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you must be a registered user to perform this action")
		return
	}
	db := ctx.MustGet("DB").(*gorm.DB)
	subscription := &model.NotificationSubscription{}
	if err := db.Where("sms_number = ?", number).First(subscription).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, http.StatusNotFound, "this number is not bound to any user account")
	} else if err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else if *subscription.UserID != user.(*model.User).ID {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "this number is not bound to your account")
	} else if err := db.Model(subscription).Updates(map[string]interface{}{"sms_number": nil}).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else if err := db.Model(&model.SMSVerification{}).Where("sms_number = ?", number).Update("status", "released").Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else {
		ctx.Status(http.StatusNoContent)
	}
}
