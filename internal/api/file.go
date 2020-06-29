package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"schrodinger-box/internal/misc"
	"schrodinger-box/internal/model"
)

var ReadSASExpiresAt *time.Time
var ReadSASQueryParam string
var SharedKeyCredential *azblob.SharedKeyCredential

// validity is 1 hour and 30 second gap is given to avoid problems of network delay
const SASValidTime = 1 * time.Hour
const SASValidAllowance = 30 * time.Second

const ImageContainer = "images"
const DefaultType = "image"

var AllowedExtension = map[string]struct{}{
	"jpg":  {},
	"jpeg": {},
	"png":  {},
	"webp": {},
	"tiff": {},
	"bmp":  {},
}

// generate SAS for user to upload image to Azure Blob Storage
func FileCreate(ctx *gin.Context) {
	var user *model.User
	if userInterface, exists := ctx.Get("User"); !exists {
		// User has not been created, we only allow existing user to visit images stored
		misc.ReturnStandardError(ctx, 403, "you will have to be a registered user to do this")
		return
	} else {
		user = userInterface.(*model.User)
	}
	file := &model.File{}
	if err := jsonapi.UnmarshalPayload(ctx.Request.Body, file); err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot unmarshal JSON of request")
		return
	} else if file.Filename == nil || file.Type == nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "filename and type must be provided")
		return
	}
	// get file extension
	fileNameSlice := strings.Split(*file.Filename, ".")
	extension := fileNameSlice[len(fileNameSlice)-1]
	if _, ok := AllowedExtension[extension]; !ok {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "file extension is not accepted")
		return
	}
	// standardize filename to uuid + extension
	newFilename := uuid.New().String() + "." + extension
	file.Filename = &newFilename
	file.UploaderID = &user.ID
	file.Uploader = user

	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.Save(file).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot save file record to database")
		return
	}
	expiresAt := time.Now().UTC().Add(SASValidTime)
	qp, err := getSASQueryParam(expiresAt, ImageContainer, *file.Filename, azblob.BlobSASPermissions{Add: true}.String())
	if err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot save file record to database")
		return
	}
	file.QueryParam = &qp
	expiresAt = expiresAt.Add(-SASValidAllowance)
	file.QueryParamExpiresAt = &expiresAt
	ctx.Status(http.StatusCreated)
	if err := jsonapi.MarshalPayload(ctx.Writer, file); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
}

// generate SAS for user to load images from Azure Blob Storage
func FilesGet(ctx *gin.Context) {
	if _, exists := ctx.Get("User"); !exists {
		// User has not been created, we only allow existing user to visit images stored
		misc.ReturnStandardError(ctx, 403, "you will have to be a registered user to do this")
		return
	}
	if ReadSASExpiresAt == nil || time.Now().After(*ReadSASExpiresAt) {
		// SAS token expires or have not been created at all, we need to generate a new one
		newExpiresAt := time.Now().UTC().Add(SASValidTime)
		qp, err := getSASQueryParam(newExpiresAt, ImageContainer, "",
			azblob.ContainerSASPermissions{Read: true}.String())
		if err != nil {
			misc.ReturnStandardError(ctx, 500, err.Error())
			return
		}
		ReadSASQueryParam = qp
		newExpiresAt = newExpiresAt.Add(-SASValidAllowance)
		ReadSASExpiresAt = &newExpiresAt
	}
	ctx.String(http.StatusOK, "{\"meta\":{\"qp\": \""+ReadSASQueryParam+"\", \"qp_expires_at\": \""+ReadSASExpiresAt.Format(time.RFC3339)+"\"}}")
}

func getSASQueryParam(expireTime time.Time, container string, blob string, permissions string) (string, error) {
	var credential *azblob.SharedKeyCredential
	var err error
	if SharedKeyCredential == nil {
		credential, err = loadCredential()
		if err != nil {
			return "", err
		}
	} else {
		credential = SharedKeyCredential
	}
	qp, err := azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,
		ExpiryTime:    expireTime,
		ContainerName: container,
		BlobName:      blob,
		Permissions:   permissions,
	}.NewSASQueryParameters(credential)
	return fmt.Sprintf("%s", qp.Encode()), err
}

func loadCredential() (*azblob.SharedKeyCredential, error) {
	credential, err := azblob.NewSharedKeyCredential(
		viper.GetString("azure.accountName"),
		viper.GetString("azure.accountKey"))
	if err == nil {
		SharedKeyCredential = credential
	}
	return credential, err
}
