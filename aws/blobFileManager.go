package aws

import (
	"context"
	"net/url"
)

type BlobMetaData struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

type BlobFileManager interface {
	Upload(ctx context.Context, metaData *BlobMetaData, dataToUpload []byte) error
	Download(ctx context.Context, metaData *BlobMetaData) ([]byte, error)
	PresignedURL(ctx context.Context, metaData *BlobMetaData) (*url.URL, error)
}
