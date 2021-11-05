package iowrappers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

type PhotoHttpClient struct {
	// connect to Google Photo API
	client     *http.Client
	apiKey     string
	apiBaseURL string
}

type PhotoURL string

var isHtmlAnchor = func(node *html.Node) bool {
	return (node.Type == html.ElementNode && node.Data == "a")
}

// CreatePhotoHttpClient is a factory method for PhotoClient
func CreatePhotoHttpClient(apiKey string, baseURL string) PhotoHttpClient {
	// turn off auto-direct
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return PhotoHttpClient{client: client, apiKey: apiKey, apiBaseURL: baseURL}
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

	photoURL, err = parseHTML(body, isHtmlAnchor, "href")
	if err != nil {
		Logger.Warn("Err Msg: ", err.Error())
	}
	return photoURL

}

func parseHTML(htmlBody []byte, judger func(*html.Node) bool, attr string) (PhotoURL, error) {
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
	url, found := dfs(doc, judger, attr)
	if !found {
		// Logger.Warn("No URL is found in HTML body!\n")
		Logger.Debugf("%s\n", htmlBody)
		return photoURL, errors.New("No URL is found in HTML body!")
	}
	photoURL = PhotoURL(url)

	return photoURL, nil
}

func dfs(node *html.Node, judger func(*html.Node) bool, attr string) (string, bool) {
	if judger(node) {
		for _, a := range node.Attr {
			if a.Key != attr {
				continue
			}
			return string(a.Val), true
		}
	}
	found := false
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		url, found := dfs(c, judger, attr)
		if found {
			return url, true
		}
	}
	return "", found
}
