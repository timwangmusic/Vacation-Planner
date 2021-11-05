package iowrappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func init() {
	_ = CreateLogger()
}

func TestCreatePhotoClient(t *testing.T) {
	tests := []struct {
		apiKey  string
		baseURL string
	}{
		{"abcd", "https://foos.com"},
		{"xyz", "https://www.google.com"},
	}

	for _, test := range tests {
		client := CreatePhotoHttpClient(test.apiKey, test.baseURL)
		assert.Equal(t, client.apiKey, test.apiKey)
		assert.Equal(t, client.apiBaseURL, test.baseURL)
	}
}

func TestDfsParseHtmlForUrl(t *testing.T) {
	var htmlOneLink string = `<HTML><HEAD><meta http-equiv="content-type" content="text/html;charset=utf-8">
	<TITLE>302 Moved</TITLE></HEAD><BODY>
	<H1>302 Moved</H1>
	The document has moved
	<A HREF="https://lh3.googleusercontent.com/places/AAcXr">here</A>.
	</BODY></HTML>`

	var htmlTwoLinks string = `<HTML><HEAD><meta http-equiv="content-type" content="text/html;charset=utf-8">
	<TITLE>Document</TITLE></HEAD><BODY>
	<H1>Document</H1>
	Test Data
	<A HREF="https://www.link1.com/">here</A>.
	<A HREF="https://www.link2.com/">here</A>.
	</BODY></HTML>`

	tests := []struct {
		HtmlBody string
		Attr     string
		Output   PhotoURL
	}{
		{htmlOneLink, "href", PhotoURL("https://lh3.googleusercontent.com/places/AAcXr")},
		{htmlTwoLinks, "href", PhotoURL("https://www.link1.com/")},
	}

	for _, test := range tests {
		url, err := parseHTML([]byte(test.HtmlBody), isHtmlAnchor, test.Attr)
		assert.Equal(t, url, test.Output)
		assert.Empty(t, err, nil)
	}

}

func TestDfsParseHtmlWithErr(t *testing.T) {
	var htmlOneLink string = `<HTML><HEAD><meta http-equiv="content-type" content="text/html;charset=utf-8">
	<TITLE>302 Moved</TITLE></HEAD><BODY>
	<H1>302 Moved</H1>
	The document has moved
	<A HREF="https://www.foos.com">here</A>.
	</BODY></HTML>`

	var htmlNoLink string = `<HTML><HEAD><meta http-equiv="content-type" content="text/html;charset=utf-8">
	<TITLE>Document</TITLE></HEAD><BODY>
	<H1>Document</H1>
	Test Data
	<p>here</p>.
	</BODY></HTML>`

	tests := []struct {
		HtmlBody string
		Attr     string
		Output   PhotoURL
		ErrMsg   string
	}{
		{htmlOneLink, "src", PhotoURL(""), "No URL is found in HTML body!"},
		{htmlNoLink, "href", PhotoURL(""), "No URL is found in HTML body!"},
	}

	for _, test := range tests {
		url, err := parseHTML([]byte(test.HtmlBody), isHtmlAnchor, test.Attr)
		assert.Equal(t, test.Output, url)
		assert.Error(t, err, test.ErrMsg)
	}
}
