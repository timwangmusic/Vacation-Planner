package aws

import (
	"bytes"
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"net/url"
	"time"
)

const (
	SignedUrlValidDuration = 30 * time.Minute
)

type Client struct {
	s3 *s3.Client
}

func NewClient() (*Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	return &Client{
		s3: s3.NewFromConfig(cfg),
	}, nil
}

func (c *Client) Upload(ctx context.Context, metaData *BlobMetaData, dataToUpload []byte) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(metaData.Bucket),
		Key:    aws.String(metaData.Key),
		Body:   bytes.NewReader(dataToUpload),
	}

	_, err := c.s3.PutObject(ctx, input)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Download(ctx context.Context, metaData *BlobMetaData) ([]byte, error) {
	object, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(metaData.Bucket),
		Key:    aws.String(metaData.Key),
	})
	if err != nil {
		return nil, err
	}

	return io.ReadAll(object.Body)
}

func (c *Client) PresignedURL(ctx context.Context, metaData *BlobMetaData) (*url.URL, error) {
	_, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(metaData.Bucket),
		Key:    aws.String(metaData.Key),
	})

	if err != nil {
		var respErr *http.ResponseError
		if errors.As(err, &respErr) && respErr.Response.StatusCode == 404 {
			return nil, errors.New("object not found")
		}
		return nil, err
	}
	presignClient := s3.NewPresignClient(c.s3)

	expiresAt := time.Now().Add(SignedUrlValidDuration)
	req, err := presignClient.PresignGetObject(
		ctx, &s3.GetObjectInput{
			Bucket:          aws.String(metaData.Bucket),
			Key:             aws.String(metaData.Key),
			ResponseExpires: &expiresAt,
		})
	if err != nil {
		return nil, err
	}

	return url.Parse(req.URL)
}
