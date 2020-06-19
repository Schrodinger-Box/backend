package api

import (
	"errors"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/mitchellh/mapstructure"
	"gorm.io/gorm"

	"schrodinger-box/internal/misc"
	"schrodinger-box/internal/model"
)

/*
 * Handlers for /event actions : fetch & creation of event resources
 */
func EventCreate(ctx *gin.Context) {
	var user *model.User
	if userInterface, exists := ctx.Get("User"); !exists {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you have to be a registered user to create event")
		return
	} else {
		user = userInterface.(*model.User)
	}
	event := &model.Event{}
	physicalLocation := &model.PhysicalLocation{}
	onlineLocation := &model.OnlineLocation{}
	if err := jsonapi.UnmarshalPayload(ctx.Request.Body, event); err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot unmarshal JSON of request: "+err.Error())
		return
	} else if event.Title == nil ||
		event.TimeBegin == nil ||
		event.TimeEnd == nil ||
		event.Type == nil ||
		reflect.ValueOf(event.Location).IsNil() {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "not all fields required are provided")
		return
	} else if eventType, exists := event.Location.(map[string]interface{})["type"]; !exists || (eventType != "physical" && eventType != "online") {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "illegal event type")
		return
	} else if eventType == "physical" &&
		(mapstructure.Decode(event.Location, physicalLocation) != nil ||
			physicalLocation.Address == "" ||
			physicalLocation.ZipCode == "") {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "illegal physical location")
		return
	} else if eventType == "online" &&
		(mapstructure.Decode(event.Location, onlineLocation) != nil ||
			onlineLocation.Platform == "" ||
			onlineLocation.Link == "") {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "illegal online location")
		return
	}
	event.OrganizerID = user.ID
	if err := event.SaveLocation(); err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot marshal location object into JSON")
		return
	}
	event.Organizer = user
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.Save(event).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	ctx.Status(http.StatusCreated)
	if err := jsonapi.MarshalPayload(ctx.Writer, event); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
}

func EventGet(ctx *gin.Context) {
	id := ctx.Param("id")
	event := &model.Event{}
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.Preload("Organizer").First(event, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, http.StatusNotFound, "event does not exist")
		return
	} else if err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	if err := event.LoadLocation(); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, "unable to decode event location: "+err.Error())
		return
	}
	ctx.Status(http.StatusOK)
	if err := jsonapi.MarshalPayload(ctx.Writer, event); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
}

/*
 * Handlers for /event/signup actions : event signup & withdrawal
 */

func EventSignupCreate(ctx *gin.Context) {
	var user *model.User
	if userInterface, exists := ctx.Get("User"); !exists {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you have to be a registered user to create signup record")
		return
	} else {
		user = userInterface.(*model.User)
	}
	eventSignup := &model.EventSignup{}
	if err := jsonapi.UnmarshalPayload(ctx.Request.Body, eventSignup); err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot unmarshal JSON of request")
		return
	}
	db := ctx.MustGet("DB").(*gorm.DB)
	event := model.Event{}
	if err := db.Where(eventSignup.Event).First(&event).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, http.StatusNotFound, "specified event cannot be found")
		return
	} else if err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	event.LoadLocation()
	eventSignup.EventID = &event.ID
	eventSignup.Event = &event
	eventSignup.UserID = &user.ID
	eventSignup.User = user
	if err := db.Save(eventSignup).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else {
		ctx.Status(http.StatusCreated)
		if err := jsonapi.MarshalPayload(ctx.Writer, eventSignup); err != nil {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		}
	}
}

func EventSignupDelete(ctx *gin.Context) {
	var user *model.User
	if userInterface, exists := ctx.Get("User"); !exists {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you have to be a registered user to delete signup record")
		return
	} else {
		user = userInterface.(*model.User)
	}
	id := ctx.Param("id")
	eventSignup := &model.EventSignup{}
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.Preload("User").First(eventSignup, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, http.StatusNotFound, "event signup record does not exist")
	} else if err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else if eventSignup.User.ID != user.ID {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you can only delete your own signup record")
	} else if err := db.Delete(&eventSignup).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else {
		ctx.Status(http.StatusNoContent)
	}
}
