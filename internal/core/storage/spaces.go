package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var client *s3.Client

func InitSpaces() error {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(os.Getenv("DO_SPACE_REGION")),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				os.Getenv("DO_SPACE_KEY"),
				os.Getenv("DO_SPACE_SECRET"),
				"",
			),
		),
	)
	if err != nil {
		return err
	}

	client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(os.Getenv("DO_SPACE_ENDPOINT"))
	})

	return nil
}

func UploadFile(key string, data []byte, contentType string) (string, error) {

	_, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(os.Getenv("DO_SPACE_NAME")),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		ACL:         "types.ObjectCannedACLPublicRead",
	})

	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/%s",
		os.Getenv("DO_SPACE_PUBLIC_BASE"),
		key,
	)

	return url, nil
}
