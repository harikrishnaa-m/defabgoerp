package storage

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func DeleteFile(key string) error {

	b := os.Getenv("DO_SPACE_NAME")

	_, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(b),
		Key:    aws.String(key),
	})

	return err
}
