package iowrappers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
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
	GetPhotoURL(string) PhotoURL
}

// Use Google Map API
type MapsPhotoClient struct {
	apiKey     string
	apiBaseURL string
}

type PhotoHttpClient struct {
	client     *http.Client
	apiKey     string
	apiBaseURL string
}

// CreatePhotoClient is a factory method for PhotoClient
func CreatePhotoClient(apiKey string, baseURL string, enableMapPhotoClient bool) PhotoClient {
	if enableMapPhotoClient {
		return &MapsPhotoClient{apiKey: apiKey, apiBaseURL: baseURL}
	}
	return &PhotoHttpClient{
		// turn off http auto-direct
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}, apiKey: apiKey, apiBaseURL: baseURL}
}

func (photoClient *PhotoHttpClient) GetPhotoURL(photoRef string) PhotoURL {
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

// TODO(rwangsc18): add real implementation
func (photoClient *MapsPhotoClient) GetPhotoURL(photoRef string) PhotoURL {
	return PhotoURL(photoRef + "fake_url")
}
