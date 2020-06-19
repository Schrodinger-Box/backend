package misc

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
)

func ReturnError(ctx *gin.Context, status int, title string, code string, detail string) {
	ctx.Status(status)
	if err := jsonapi.MarshalErrors(ctx.Writer, []*jsonapi.ErrorObject{{
		Title:  title,
		Code:   code,
		Status: strconv.Itoa(status),
		Detail: detail,
	}}); err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
	}
	ctx.Abort()
}

func ReturnStandardError(ctx *gin.Context, status int, detail string) {
	switch status {
	case http.StatusUnauthorized:
		ReturnError(ctx, status, "authentication token is missing or invalid", "error.unauthorized", detail)
	case http.StatusBadRequest:
		ReturnError(ctx, status, "errors occurred when processing request", "error.bad_request", detail)
	case http.StatusForbidden:
		ReturnError(ctx, status, "you are not authorized to access this resource in this way", "error.forbidden", detail)
	case http.StatusNotFound:
		ReturnError(ctx, status, "requested or related resources cannot be found", "error.not_found", detail)
	case http.StatusInternalServerError:
		ReturnError(ctx, status, "something unexpected happened at the server side", "error.internal", detail)
	}
}
