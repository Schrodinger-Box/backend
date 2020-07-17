package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

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
	images := event.Images
	event.Images = nil
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.Save(event).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	// link images to this event
	eventString := "events"
	for _, image := range images {
		if image.ID <= 0 {
			misc.ReturnStandardError(ctx, http.StatusBadRequest, "invalid image file ID")
		} else if err := db.Where(image).Find(image).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			misc.ReturnStandardError(ctx, http.StatusNotFound, "image specified not found")
		} else if err != nil {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		} else if *image.Status != "active" || *image.Type != "images" {
			misc.ReturnStandardError(ctx, http.StatusBadRequest, "image specified is not active or is not an image")
		} else if image.LinkType != nil {
			misc.ReturnStandardError(ctx, http.StatusBadRequest, "image has been linked to some other resource object")
		} else if err := db.Model(&image).Updates(model.File{LinkID: &event.ID, LinkType: &eventString}).Error; err != nil {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		} else {
			continue
		}
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
	event.LoadSignups(db)
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
	} else {
		// release file linkages
		for _, image := range event.Images {
			if err := db.Model(image).Updates(map[string]interface{}{"link_type": nil, "link_id": nil}).Error; err != nil {
				misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
				return
			}
		}
		if err := db.Delete(&event).Error; err != nil {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		} else {
			ctx.Status(http.StatusNoContent)
		}
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
	if *event.OrganizerID == user.ID {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "you cannot signup events organized by yourself")
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

func EventSignupUpdate(ctx *gin.Context) {
	// there are two situations for an event signup record to be updated
	// 1. the Organizer marks the user as attended (user.ID == signup.Event.OrganizerID)
	// 2. the Participant leaves review to the event (user.ID == signup.UserID)
	var user *model.User
	if userInterface, exists := ctx.Get("User"); !exists {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you have to be a registered user to update signup record")
		return
	} else {
		user = userInterface.(*model.User)
	}
	signupRequest := &model.EventSignup{}
	if err := jsonapi.UnmarshalPayload(ctx.Request.Body, signupRequest); err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot unmarshal JSON of request")
		return
	} else if signupRequest.ID <= 0 {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "invalid event_signup ID")
		return
	}
	db := ctx.MustGet("DB").(*gorm.DB)
	signup := &model.EventSignup{}
	reviewedString := "reviewed"
	if err := db.Preload(clause.Associations).Find(&signup, signupRequest.ID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, http.StatusNotFound, "specified event_signup cannot be found")
	} else if user.ID == *signup.UserID {
		if *signup.Status == "created" {
			misc.ReturnStandardError(ctx, http.StatusForbidden, "you cannot leave review before you attend the event")
		} else if *signup.Status == "reviewed" {
			misc.ReturnStandardError(ctx, http.StatusForbidden, "you have already reviewed this event before")
		} else if signupRequest.ReviewScore == nil || signupRequest.ReviewText == nil {
			misc.ReturnStandardError(ctx, http.StatusBadRequest, "you must provide both score and text comment")
		} else if err := db.Model(signup).Updates(model.EventSignup{ReviewText: signupRequest.ReviewText, ReviewScore: signupRequest.ReviewScore, Status: &reviewedString}).Error; err != nil {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		} else {
			ctx.Status(http.StatusOK)
			if err := jsonapi.MarshalPayload(ctx.Writer, signup); err != nil {
				misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
			}
		}
	} else if user.ID == *signup.Event.OrganizerID {
		if *signup.Status != "created" {
			misc.ReturnStandardError(ctx, http.StatusForbidden, "the user's attendance has been marked")
		} else if err := db.Model(signup).Update("status", "attended").Error; err != nil {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		} else {
			ctx.Status(http.StatusOK)
			if err := jsonapi.MarshalPayload(ctx.Writer, signup); err != nil {
				misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
			}
		}
	} else {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you are neither event organizer nor participant of this signup record")
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
	if err := db.First(eventSignup, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
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
	// keys allowed for filter requests
	filterKeys := map[string]struct{}{
		"organizer_id": {},
		"type":         {},
		"time_begin":   {},
		"time_end":     {},
	}
	// as per JSON:API specification v1.0, -id means sorting by id in descending order
	sortQuery := ctx.DefaultQuery("sort", "-id")
	// very important: page starts from 0
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	filterArray := ctx.QueryArray("filter")

	// set size of each page fixed at 10
	size := 12
	// calculate offset based on size and page
	offset := page * size

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

	var events []*model.Event
	var count int64

	db := ctx.MustGet("DB").(*gorm.DB)
	tx := db
	// build query transaction
	for _, filter := range filterArray {
		filterSlice := strings.Split(filter, ",")
		if _, ok := filterKeys[filterSlice[0]]; !ok {
			misc.ReturnStandardError(ctx, http.StatusBadRequest, fmt.Sprintf("invalid filter key: '%s'", filterSlice[0]))
			return
		} else if len(filterSlice) == 2 {
			tx = tx.Where(fmt.Sprintf("%s = ?", filterSlice[0]), filterSlice[1])
		} else if len(filterSlice) == 3 {
			tx = tx.Where(fmt.Sprintf("%s %s ?", filterSlice[0], filterSlice[1]), filterSlice[2])
		} else {
			misc.ReturnStandardError(ctx, http.StatusBadRequest, fmt.Sprintf("invalid filter format: '%s'", filter))
			return
		}
	}
	dbCtx, _ := context.WithTimeout(context.Background(), time.Second)
	tx.WithContext(dbCtx).Model(events).Count(&count)
	totalPages := int(count) / size
	if int(count)%size != 0 {
		totalPages++
	}
	if totalPages != 0 && (page > totalPages-1 || page < 0) {
		// trying to access a page that does not exist
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "page requested does not exist")
		return
	}
	if err := tx.Preload(clause.Associations).Offset(offset).Order(sort).Limit(size).Find(&events).Error; err == nil {
		for _, event := range events {
			event.LoadSignups(db)
		}
		var jsonString strings.Builder
		var jsonData map[string]interface{}
		var next, prev *string

		ctx.Status(http.StatusOK)
		if err := jsonapi.MarshalPayload(&jsonString, events); err == nil {
			json.Unmarshal([]byte(jsonString.String()), &jsonData)
			url := misc.APIAbsolutePath("/events") + "?sort=" + sortQuery + "&page="
			firstString := url + "0"
			var lastString string
			if totalPages == 0 {
				lastString = url + "0"
			} else {
				lastString = url + strconv.Itoa(totalPages-1)
			}
			if page == totalPages-1 || totalPages == 0 {
				// already at last page
				next = nil
			} else {
				nextString := url + strconv.Itoa(page+1)
				next = &nextString
			}
			if page == 0 {
				// already at first page
				prev = nil
			} else {
				prevString := url + strconv.Itoa(page-1)
				prev = &prevString
			}
			jsonData["links"] = map[string]*string{
				jsonapi.KeyFirstPage:    &firstString,
				jsonapi.KeyLastPage:     &lastString,
				jsonapi.KeyNextPage:     next,
				jsonapi.KeyPreviousPage: prev,
			}
			jsonData["meta"] = map[string]int{
				"total_pages":    totalPages,
				"current_page":   page,
				"max_page_size":  size,
				"this_page_size": len(events),
			}
			json.NewEncoder(ctx.Writer).Encode(jsonData)
		} else {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		}
	} else {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	}
}
