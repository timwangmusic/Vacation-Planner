package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var client *s3.Client

func init() {

}

func uploadFile(ctx context.Context, bucket string, filename string, content []byte) error {

}
