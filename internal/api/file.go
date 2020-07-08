package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
const DefaultType = "images"

var AllowedExtensions = map[string]struct{}{
	"jpg":  {},
	"jpeg": {},
	"png":  {},
	"webp": {},
	"tiff": {},
	"bmp":  {},
}
var FileTypes = map[string]string{
	"images": "images",
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
	} else if _, ok := FileTypes[*file.Type]; !ok {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "type '"+*file.Type+"' is not accepted")
		return
	}
	// get file extension
	fileNameSlice := strings.Split(*file.Filename, ".")
	extension := fileNameSlice[len(fileNameSlice)-1]
	if _, ok := AllowedExtensions[extension]; !ok {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "file extension '"+extension+"'is not accepted")
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
	// we assign only create permission to this SAS (create permission does not allow updating resources)
	qp, err := getSASQueryParam(expiresAt, FileTypes[*file.Type], *file.Filename, azblob.BlobSASPermissions{Create: true}.String())
	if err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot save file record to database")
		return
	}
	file.QueryParam = qp
	file.QueryParamExpiresAt = expiresAt.Add(-SASValidAllowance)
	file.Endpoint = "https://" + viper.GetString("azure.accountName") + ".blob.core.windows.net/" + FileTypes[*file.Type] + "/" + newFilename
	ctx.Status(http.StatusCreated)
	if err := jsonapi.MarshalPayloadWithoutIncluded(ctx.Writer, file); err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
}

func FileDelete(ctx *gin.Context) {
	var user *model.User
	if userInterface, exists := ctx.Get("User"); !exists {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you have to be a registered user to delete file uploaded")
		return
	} else {
		user = userInterface.(*model.User)
	}
	id := ctx.Param("id")
	file := &model.File{}
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.First(file, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, http.StatusNotFound, "file record does not exist")
	} else if err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else if *file.UploaderID != user.ID {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you can only delete files uploaded by you")
	} else if *file.Status != "active" {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "file record is not active")
	} else if err := db.Delete(&file).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else {
		// delete file from Azure Blob Storage
		accountName := viper.GetString("azure.accountName")
		u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s",
			accountName, FileTypes[*file.Type], *file.Filename))
		if credential, err := azblob.NewSharedKeyCredential(accountName, viper.GetString("azure.accountKey")); err != nil {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
		} else {
			blobURL := azblob.NewBlobURL(*u, azblob.NewPipeline(credential, azblob.PipelineOptions{}))
			azCtx := context.Background()
			if _, err := blobURL.Delete(azCtx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{}); err != nil {
				misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
			} else {
				ctx.Status(http.StatusNoContent)
			}
		}
	}
}

func FileUpdate(ctx *gin.Context) {
	fileRequest := &model.File{}
	if err := jsonapi.UnmarshalPayload(ctx.Request.Body, fileRequest); err != nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "cannot unmarshal JSON of request")
		return
	} else if fileRequest.ID <= 0 || fileRequest.Status == nil {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "ID and status must be provided to update file record")
		return
	} else if *fileRequest.Status != "active" {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "you can only update file status to 'active'")
		return
	}
	var user *model.User
	if userInterface, exists := ctx.Get("User"); !exists {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you have to be a registered user to update file record")
		return
	} else {
		user = userInterface.(*model.User)
	}
	file := &model.File{}
	db := ctx.MustGet("DB").(*gorm.DB)
	if err := db.First(file, fileRequest.ID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		misc.ReturnStandardError(ctx, http.StatusNotFound, "file record does not exist")
	} else if err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else if *file.UploaderID != user.ID {
		misc.ReturnStandardError(ctx, http.StatusForbidden, "you can only update files uploaded by you")
	} else if err := db.Model(file).Update("status", *fileRequest.Status).Error; err != nil {
		misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
	} else {
		file.Uploader = user
		ctx.Status(http.StatusOK)
		if err := jsonapi.MarshalPayloadWithoutIncluded(ctx.Writer, file); err != nil {
			misc.ReturnStandardError(ctx, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

// generate SAS for user to load images from Azure Blob Storage
func FilesGet(ctx *gin.Context) {
	if _, exists := ctx.Get("User"); !exists {
		// User has not been created, we only allow existing user to visit images stored
		misc.ReturnStandardError(ctx, 403, "you will have to be a registered user to do this")
		return
	}
	fileType := ctx.DefaultQuery("type", DefaultType)
	if _, ok := FileTypes[fileType]; !ok {
		misc.ReturnStandardError(ctx, http.StatusBadRequest, "type '"+fileType+"' is not accepted")
		return
	}
	if ReadSASExpiresAt == nil || time.Now().After(*ReadSASExpiresAt) {
		// SAS token expires or have not been created at all, we need to generate a new one
		newExpiresAt := time.Now().UTC().Add(SASValidTime)
		qp, err := getSASQueryParam(newExpiresAt, FileTypes[fileType], "",
			azblob.ContainerSASPermissions{Read: true}.String())
		if err != nil {
			misc.ReturnStandardError(ctx, 500, err.Error())
			return
		}
		ReadSASQueryParam = qp
		newExpiresAt = newExpiresAt.Add(-SASValidAllowance)
		ReadSASExpiresAt = &newExpiresAt
	}
	data := map[string]map[string]string{
		"meta": {
			"qp":            ReadSASQueryParam,
			"qp_expires_at": ReadSASExpiresAt.Format(time.RFC3339),
			"endpoint":      "https://" + viper.GetString("azure.accountName") + ".blob.core.windows.net/" + FileTypes[fileType] + "/",
		},
	}
	ctx.JSON(http.StatusOK, data)
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
