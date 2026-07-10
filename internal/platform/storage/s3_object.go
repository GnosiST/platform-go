package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type s3ObjectAPI interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type S3ObjectStore struct {
	client s3ObjectAPI
	bucket string
	prefix string
	now    func() time.Time
}

func NewS3ObjectStore(ctx context.Context, config S3ObjectStoreConfig) (S3ObjectStore, error) {
	if strings.TrimSpace(config.Bucket) == "" {
		return S3ObjectStore{}, fmt.Errorf("%w: s3 bucket is required", ErrInvalidObjectStoreConfig)
	}
	region := strings.TrimSpace(config.Region)
	if region == "" {
		region = "us-east-1"
	}
	accessKey := strings.TrimSpace(config.AccessKey)
	secretKey := strings.TrimSpace(config.SecretKey)
	if (accessKey == "") != (secretKey == "") {
		return S3ObjectStore{}, fmt.Errorf("%w: s3 access key and secret key must be configured together", ErrInvalidObjectStoreConfig)
	}
	loadOptions := []func(*awscfg.LoadOptions) error{awscfg.WithRegion(region)}
	if accessKey != "" {
		loadOptions = append(loadOptions, awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")))
	}
	awsConfig, err := awscfg.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return S3ObjectStore{}, err
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		if strings.TrimSpace(config.Endpoint) != "" {
			options.BaseEndpoint = aws.String(strings.TrimSpace(config.Endpoint))
		}
		options.UsePathStyle = config.ForcePathStyle
	})
	return newS3ObjectStoreWithClient(client, config.Bucket, config.Prefix, nil)
}

func newS3ObjectStoreWithClient(client s3ObjectAPI, bucket string, prefix string, now func() time.Time) (S3ObjectStore, error) {
	if client == nil {
		return S3ObjectStore{}, fmt.Errorf("%w: s3 client is required", ErrInvalidObjectStoreConfig)
	}
	if strings.TrimSpace(bucket) == "" {
		return S3ObjectStore{}, fmt.Errorf("%w: s3 bucket is required", ErrInvalidObjectStoreConfig)
	}
	if now == nil {
		now = time.Now
	}
	return S3ObjectStore{
		client: client,
		bucket: strings.TrimSpace(bucket),
		prefix: strings.Trim(strings.TrimSpace(prefix), "/"),
		now:    now,
	}, nil
}

func (store S3ObjectStore) Save(ctx context.Context, input ObjectSaveInput) (ObjectMetadata, error) {
	if input.Reader == nil {
		return ObjectMetadata{}, errors.New("object reader is required")
	}
	key := path.Join(store.prefix, store.timestampPrefix(), sanitizedObjectFileName(input.FileName))
	reader := &countingReader{reader: input.Reader}
	putInput := &s3.PutObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(key),
		Body:   reader,
	}
	if strings.TrimSpace(input.ContentType) != "" {
		putInput.ContentType = aws.String(strings.TrimSpace(input.ContentType))
	}
	if _, err := store.client.PutObject(ctx, putInput); err != nil {
		return ObjectMetadata{}, err
	}
	return ObjectMetadata{
		Driver:    "s3",
		Key:       key,
		Path:      fmt.Sprintf("s3://%s/%s", store.bucket, key),
		URL:       "",
		SizeBytes: reader.bytesRead,
	}, nil
}

func (store S3ObjectStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := store.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(cleanObjectKey(key)),
	})
	if isS3ObjectNotFound(err) {
		return nil, ErrObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}

func (store S3ObjectStore) Delete(ctx context.Context, key string) error {
	cleanKey := cleanObjectKey(key)
	if cleanKey == "" {
		return nil
	}
	_, err := store.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(cleanKey),
	})
	return err
}

func (store S3ObjectStore) timestampPrefix() string {
	now := store.now().UTC()
	return fmt.Sprintf("%04d/%02d/%02d/%d", now.Year(), now.Month(), now.Day(), now.UnixNano())
}

type countingReader struct {
	reader    io.Reader
	bytesRead int64
}

func (reader *countingReader) Read(p []byte) (int, error) {
	n, err := reader.reader.Read(p)
	reader.bytesRead += int64(n)
	return n, err
}

func isS3ObjectNotFound(err error) bool {
	if err == nil {
		return false
	}
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch apiError.ErrorCode() {
		case "NoSuchKey", "NotFound", "NoSuchBucket", "404":
			return true
		}
	}
	return false
}
