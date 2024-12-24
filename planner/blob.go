package planner

import (
	"errors"
	"fmt"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/weihesdlegend/Vacation-planner/POI"
	awsinternal "github.com/weihesdlegend/Vacation-planner/aws"
	"github.com/weihesdlegend/Vacation-planner/user"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	locationImagePromptTemplate = "generate an image for this location on a sunny day: %s, %s, %s"
	UseGeneratedImagesThreshold = 5
)

// provision an image for the given location in the form of URL.
// If there are images in the blob storage (query database to find out), randomly selects one;
// otherwise generates one and upload to the blob storage
func (p *MyPlanner) getLocationPhoto(ctx *gin.Context, c *awsinternal.Client, location *POI.Location) (string, error) {
	rc := p.RedisClient
	blobBucket := p.BlobBucket

	prompt := fmt.Sprintf(locationImagePromptTemplate, location.City, location.AdminAreaLevelOne, location.Country)
	locationRedisKey := fmt.Sprintf("photos:%s:%s:%s", location.Country, location.AdminAreaLevelOne, location.City)

	var resultURL *url.URL
	err := rc.Get().Watch(ctx, func(tx *redis.Tx) error {
		photos, err := rc.GetLocationPhotoIDs(ctx, location)
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}

		if len(photos) >= UseGeneratedImagesThreshold {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			selected := photos[r.Intn(len(photos))]
			blobKey := strings.Join([]string{locationToBlobKey(location), selected}, "/")
			resultURL, err = c.PresignedURL(ctx, &awsinternal.BlobMetaData{
				Bucket: blobBucket,
				Key:    blobKey,
			})
			if err != nil {
				return fmt.Errorf("failed to get location photos with key %s: %v", blobKey, err)
			}
			return nil
		}

		imgBytes, err := imageGeneration(ctx, prompt)
		if err != nil {
			return err
		}

		photoId := uuid.New().String() + ".png"
		blobKey := strings.Join([]string{location.Country, location.AdminAreaLevelOne,
			location.City, photoId}, "/")

		if err = c.Upload(ctx, &awsinternal.BlobMetaData{
			Bucket: blobBucket,
			Key:    blobKey,
		}, imgBytes); err != nil {
			return err
		}

		resultURL, err = c.PresignedURL(ctx, &awsinternal.BlobMetaData{
			Bucket: blobBucket,
			Key:    blobKey,
		})
		if err != nil {
			return err
		}

		pipe := tx.Pipeline()
		pipe.SAdd(ctx, locationRedisKey, photoId)
		_, err = pipe.Exec(ctx)
		if err != nil {
			return err
		}

		return nil
	}, locationRedisKey)

	if err != nil {
		return "", err
	}

	return resultURL.String(), nil
}

// handler for location image endpoint
func (p *MyPlanner) getLocationImage(ctx *gin.Context) {
	requestId := requestid.Get(ctx)
	ctx.Set(requestIdKey, requestId)

	if _, err := p.UserAuthentication(ctx, user.LevelRegular); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err})
		return
	}

	c, err := awsinternal.NewClient()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	location := &POI.Location{}
	if err = ctx.ShouldBindJSON(location); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
	location.Normalize()

	var u string
	if u, err = p.getLocationPhoto(ctx, c, location); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	ctx.JSON(http.StatusOK, gin.H{"photo": u})
}
