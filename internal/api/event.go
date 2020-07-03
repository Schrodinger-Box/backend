package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/mitchellh/mapstructure"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	event.OrganizerID = &user.ID
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
	if err := db.Preload(clause.Associations).First(event, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, http.StatusNotFound, "event does not exist")
		return
	} else if err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	ctx.Status(http.StatusOK)
	if err := jsonapi.MarshalPayload(ctx.Writer, event); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
}

func EventDelete(ctx *gin.Context) {
	var user *model.User
	if userInterface, exists := ctx.Get("User"); !exists {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you have to be a registered user to delete signup record")
		return
	} else {
		user = userInterface.(*model.User)
	}
	id := ctx.Param("id")
	event := &model.Event{}
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.Preload(clause.Associations).First(event, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, http.StatusNotFound, "event does not exist")
	} else if err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else if *event.OrganizerID != user.ID {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you can only delete event organized by your own")
	} else if err := db.Delete(&event).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else {
		ctx.Status(http.StatusNoContent)
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
	} else if eventSignup.Event == nil || eventSignup.Event.ID <= 0 {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "invalid event ID")
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
	} else if *eventSignup.UserID != user.ID {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you can only delete your own signup record")
	} else if err := db.Delete(&eventSignup).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

/*
 * Handlers for returning multiple events (sorting, pagination)
 */

func EventsGet(ctx *gin.Context) {
	// as per JSON:API specification v1.0, -id means sorting by id in descending order
	sortQuery := ctx.DefaultQuery("sort", "-id")
	offset, _ := strconv.Atoi(ctx.DefaultQuery("offset", "0"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "5"))

	// convert JSON:API sort query to SQL sorting syntax
	sortSlice := strings.Split(sortQuery, ",")
	for k, v := range sortSlice {
		if string(v[0]) == "-" {
			sortSlice[k] = v[1:] + " desc"
		} else {
			sortSlice[k] = v + " asc"
		}
	}
	sort := strings.Join(sortSlice, ",")

	db := ctx.MustGet("DB").(*gorm.DB)
	var events []*model.Event

	if err := db.Preload(clause.Associations).Limit(size).Offset(offset).Order(sort).Find(&events).Error; err == nil {
		ctx.Status(http.StatusOK)
		var jsonString strings.Builder
		var jsonData map[string]interface{}
		var count int64
		var next, prev *string
		if err := jsonapi.MarshalPayload(&jsonString, events); err == nil {
			json.Unmarshal([]byte(jsonString.String()), &jsonData)
			url := misc.APIAbsolutePath("/events") + "?sort=" + sortQuery + "&size=" + strconv.Itoa(size) + "&offset="
			db.Model(&events).Count(&count)
			firstString := url + "0"
			lastString := url + url + strconv.Itoa(int(count)/size*size)
			if offset+size >= int(count) {
				// already at last page
				next = nil
			} else {
				nextString := url + strconv.Itoa(offset+size)
				next = &nextString
			}
			if offset-size < 0 {
				// already at first page
				prev = nil
			} else {
				prevString := url + strconv.Itoa(offset-size)
				prev = &prevString
			}
			jsonData["links"] = map[string]*string{
				jsonapi.KeyFirstPage:    &firstString,
				jsonapi.KeyLastPage:     &lastString,
				jsonapi.KeyNextPage:     next,
				jsonapi.KeyPreviousPage: prev,
			}
			json.NewEncoder(ctx.Writer).Encode(jsonData)
		} else {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		}
	} else {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	}
}
