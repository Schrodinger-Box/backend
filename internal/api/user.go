package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/jinzhu/gorm"

	"schrodinger-box/internal/misc"
	"schrodinger-box/internal/model"
)

func UserGetSelf(ctx *gin.Context) {
	user, exists := ctx.Get("User")
	if !exists {
		// User has not been created, return 404 to tell client to create user
		misc.ReturnStandardError(ctx, 404, "user has not been created")
		return
	}
	ctx.Writer.WriteHeader(http.StatusOK)
	if err := jsonapi.MarshalPayload(ctx.Writer, user.(*model.User)); err != nil {
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
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot unmarshal JSON of request")
		return
	} else if userRequest.Nickname == nil || userRequest.Type == nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "nickname and type MUST be provided")
		return
	}
	db := ctx.MustGet("DB").(*gorm.DB)
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
	ctx.Writer.WriteHeader(http.StatusCreated)
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
		if gorm.IsRecordNotFoundError(err) {
			misc.ReturnStandardError(ctx, http.StatusNotFound, "user does not exist")
			return
		} else {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
			return
		}
	}
	ctx.Writer.WriteHeader(http.StatusOK)
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
		if gorm.IsRecordNotFoundError(err) {
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
	ctx.Writer.WriteHeader(http.StatusOK)
	if err := jsonapi.MarshalPayload(ctx.Writer, user); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
}
