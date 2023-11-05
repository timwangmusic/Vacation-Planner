package iowrappers

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image/jpeg"
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
	GetPhotoURL(context.Context, string) PhotoURL
}

// Use Google Map API
type MapsPhotoClient struct {
	maps_client *maps.Client
	apiKey      string
	apiBaseURL  string
}

type PhotoHttpClient struct {
	client     *http.Client
	apiKey     string
	apiBaseURL string
}

// CreatePhotoClient is a factory method for PhotoClient
func CreatePhotoClient(apiKey string, baseURL string, enableMapPhotoClient bool) PhotoClient {
	if enableMapPhotoClient {
		mapsClient, err := maps.NewClient(maps.WithAPIKey(apiKey))
		if err != nil {
			Logger.Fatal(err)
		}
		return &MapsPhotoClient{maps_client: mapsClient, apiKey: apiKey, apiBaseURL: baseURL}
	}
	return &PhotoHttpClient{
		// turn off http auto-direct
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}, apiKey: apiKey, apiBaseURL: baseURL}
}

func (photoClient *PhotoHttpClient) GetPhotoURL(ctx context.Context, photoRef string) PhotoURL {
	var photoURL PhotoURL
	var reqURL = fmt.Sprintf(photoClient.apiBaseURL, photoRef, photoClient.apiKey)
	res, err := photoClient.client.Get(reqURL)
	if err != nil {
		Logger.Fatal(err)
		return photoURL
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 302 {
		Logger.Warnf("status code should be 302, but is %d", res.StatusCode)
	}
	if err != nil {
		Logger.Fatal(err)
		return photoURL
	}

	photoURL, err = parseHTML(body, isHtmlAnchor, isValidPhotoUrl)
	if err != nil {
		Logger.Warn("Err Msg: ", err.Error())
		return ""
	}
	return photoURL
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

// TOOD(rwangsc18): add a unit test for this function
func (photoClient *MapsPhotoClient) GetPhotoURL(ctx context.Context, photoRef string) PhotoURL {
	// get PlacePhotoResponse
	resp, err := photoClient.maps_client.PlacePhoto(ctx, &maps.PlacePhotoRequest{
		PhotoReference: photoRef,
		MaxWidth:       400,
	})
	if err != nil {
		Logger.Error(err)
		// TODO(rwangsc18): replace with a default image
		return PhotoURL("fake_url")
	}

	// get image data
	image, err := resp.Image()
	if err != nil {
		Logger.Error(err)
		return PhotoURL("fake_url")
	}

	// encode image to base64
	buffer := new(bytes.Buffer)
	if err := jpeg.Encode(buffer, image, nil); err != nil {
		Logger.Error("unable to encode image.")
		return PhotoURL("fake_url")
	}
	data := base64.StdEncoding.EncodeToString(buffer.Bytes())
	return PhotoURL(data)
}
