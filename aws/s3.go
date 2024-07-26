package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"os"
)

func UploadFile(ctx context.Context, bucket, key, filename string) error {
	c, err := NewClient()
	if err != nil {
		return err
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	} else {
		defer file.Close()
		input := &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   file,
		}

		if _, err = c.s3.PutObject(ctx, input); err != nil {
			return err
		}
	}

	return nil
}
