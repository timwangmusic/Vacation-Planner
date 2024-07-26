package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
