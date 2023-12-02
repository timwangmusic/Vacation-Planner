package iowrappers

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
	"googlemaps.github.io/maps"
)

const ValidPrefix = "https://lh3.googleusercontent.com/places/"

var isHtmlAnchor = func(node *html.Node) bool {
	return node.Type == html.ElementNode && node.Data == "a"
}

var isValidPhotoUrl = func(url string) bool {
	return strings.HasPrefix(url, ValidPrefix)
}

type PhotoURL string

type PhotoClient interface {
	GetPhotoURL(context.Context, string, string) (PhotoURL, error)
}

type MapsPhotoClient struct {
	redisClient *RedisClient
	mapsClient  *MapsClient
	apiKey      string
	apiBaseURL  string
}

type PhotoHttpClient struct {
	client     *http.Client
	apiKey     string
	apiBaseURL string
}

// CreatePhotoClient is a factory method for PhotoClient
func CreatePhotoClient(apiKey string, baseURL string, enableMapPhotoClient bool, placeDetailsFields []string, redisClient *RedisClient) (PhotoClient, error) {
	if enableMapPhotoClient {
		mapsClient := CreateMapsClient(apiKey)
		mapsClient.SetDetailedSearchFields(placeDetailsFields)
		return &MapsPhotoClient{mapsClient: mapsClient, apiKey: apiKey, apiBaseURL: baseURL, redisClient: redisClient}, nil
	}
	return &PhotoHttpClient{
		// turn off http auto-direct
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}, apiKey: apiKey, apiBaseURL: baseURL}, nil
}

func (photoClient *PhotoHttpClient) GetPhotoURL(ctx context.Context, photoRef string, s string) (PhotoURL, error) {
	var photoURL PhotoURL
	var reqURL = fmt.Sprintf(photoClient.apiBaseURL, photoRef, photoClient.apiKey)
	res, err := photoClient.client.Get(reqURL)
	if err != nil {
		return photoURL, err
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 302 {
		Logger.Warnf("status code should be 302, but is %d", res.StatusCode)
	}
	if err != nil {
		return photoURL, err
	}

	photoURL, err = parseHTML(body, isHtmlAnchor, isValidPhotoUrl)
	if err != nil {
		return "", err
	}
	return photoURL, nil
}

func parseHTML(htmlBody []byte, judger func(*html.Node) bool, validator func(string) bool) (PhotoURL, error) {
	// Use http package parse htmlBody
	var photoURL PhotoURL
	if len(htmlBody) == 0 {
		return photoURL, nil
	}

	doc, err := html.Parse(strings.NewReader(string(htmlBody)))
	if err != nil {
		Logger.Fatal(err)
		return photoURL, err
	}
	url, found := dfs(doc, judger, validator)
	if !found {
		return photoURL, errors.New("no URL is found in HTML body")
	}
	photoURL = PhotoURL(url)

	return photoURL, nil
}

func dfs(node *html.Node, judger func(*html.Node) bool, validator func(string) bool) (string, bool) {
	if judger(node) {
		for _, a := range node.Attr {
			if (a.Key != "href") || (!validator(a.Val)) {
				continue
			}
			return a.Val, true
		}
	}
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		url, found := dfs(c, judger, validator)
		if found {
			return url, true
		}
	}
	return "", false
}

func (c *MapsPhotoClient) GetPhotoURL(ctx context.Context, photoRef string, placeId string) (PhotoURL, error) {
	var photoURL PhotoURL
	resp, err := c.mapsClient.client.PlacePhoto(ctx, &maps.PlacePhotoRequest{
		PhotoReference: photoRef,
		MaxWidth:       400,
	})
	if err != nil {
		return photoURL, err
	}

	Logger.Debugf("photo response content type is: %s", resp.ContentType)
	var img image.Image
	if resp.ContentType == "image/png" {
		img, err = png.Decode(resp.Data)
		if err != nil {
			return "", err
		}
	} else {
		img, err = resp.Image()
		// content type maybe text/html, need to update place photo reference
		if err != nil {
			var r maps.PlaceDetailsResult
			r, err = c.mapsClient.PlaceDetailedSearch(ctx, placeId, c.mapsClient.DetailedSearchFields)
			if err != nil {
				return photoURL, err
			}

			data := make(map[string]interface{})
			if len(r.Photos) > 0 {
				data["photo"] = POI.PlacePhoto{Reference: r.Photos[0].PhotoReference, Width: 400}
			}
			err = c.redisClient.UpdatePlace(ctx, placeId, data)
			if err != nil {
				return photoURL, err
			}
			// try once again after the photo reference is updated
			resp, err = c.mapsClient.client.PlacePhoto(ctx, &maps.PlacePhotoRequest{
				PhotoReference: r.Photos[0].PhotoReference,
				MaxWidth:       400,
			})
			if err != nil {
				return photoURL, err
			}
			if resp.ContentType == "image/png" {
				img, err = png.Decode(resp.Data)
			} else {
				img, err = resp.Image()
			}
		}
	}

	// encode img to base64
	buffer := new(bytes.Buffer)
	if err = jpeg.Encode(buffer, img, nil); err != nil {
		return photoURL, err
	}
	data := base64.StdEncoding.EncodeToString(buffer.Bytes())
	return PhotoURL(data), nil
}
