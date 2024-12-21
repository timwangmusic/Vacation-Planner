package planner

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weihesdlegend/Vacation-planner/POI"
	awsinternal "github.com/weihesdlegend/Vacation-planner/aws"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const (
	locationImagePromptTemplate = "generate an image for this location on a sunny day: %s, %s, %s"
)

// provision an image for the given location in the form of URL.
// If there are images in the blob storage (query database to find out), randomly selects one;
// otherwise generates one and upload to the blob storage
func (p *MyPlanner) getLocationImage(ctx *gin.Context) {
	c, err := awsinternal.NewClient()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	location := POI.Location{}
	if err = ctx.ShouldBindJSON(&location); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	rc := p.RedisClient
	var photos []string
	if photos, err = rc.GetLocationPhotoIDs(ctx, location); err != nil {
		iowrappers.Logger.Debugf("failed to get photos for location %+v: %v", location, err)
	}

	testBucket := p.BlobBucket

	if err == nil && len(photos) > 0 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		selected := photos[r.Intn(len(photos))]
		url, err := c.PresignedURL(ctx, &awsinternal.BlobMetaData{
			Bucket: testBucket,
			Key:    strings.Join([]string{locationToBlobKey(location), selected}, "/"),
		})
		if err != nil {
			iowrappers.Logger.Debugf("failed to get location photos: %v", err)
		} else {
			ctx.JSON(http.StatusOK, gin.H{"photo": url.String()})
			return
		}
	}

	prompt := fmt.Sprintf(locationImagePromptTemplate, location.City, location.AdminAreaLevelOne, location.Country)

	imgBytes, err := imageGeneration(ctx, prompt)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	photoId := uuid.New().String() + ".png"

	if err = c.Upload(ctx, &awsinternal.BlobMetaData{
		Bucket: testBucket,
		Key: strings.Join([]string{location.Country, location.AdminAreaLevelOne,
			location.City, photoId}, "/"),
	}, imgBytes); err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
	}

	if err = p.RedisClient.SaveLocationPhotoID(ctx, location, photoId); err != nil {
		iowrappers.Logger.Debugf("failed to save location photo: %v", err)
	}
}
