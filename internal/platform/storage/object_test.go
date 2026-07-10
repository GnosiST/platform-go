package storage

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type fakeS3ObjectClient struct {
	objects map[string]fakeS3Object
}

type fakeS3Object struct {
	body        string
	contentType string
}

func TestLocalObjectStoreSavesOpensAndDeletesObject(t *testing.T) {
	baseDir := t.TempDir()
	now := time.Date(2026, 7, 5, 10, 20, 30, 40, time.UTC)
	store := NewLocalObjectStore(LocalObjectStoreOptions{
		BaseDir:       baseDir,
		PublicBaseURL: "/assets",
		Now:           func() time.Time { return now },
	})

	metadata, err := store.Save(context.Background(), ObjectSaveInput{
		FileName:    "demo file.txt",
		ContentType: "text/plain",
		Reader:      strings.NewReader("hello platform"),
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if metadata.Driver != "local" || metadata.Key == "" || metadata.SizeBytes != int64(len("hello platform")) {
		t.Fatalf("metadata = %+v, want local key and size", metadata)
	}
	if !strings.Contains(metadata.Key, "demo_file.txt") {
		t.Fatalf("metadata key = %q, want sanitized file name", metadata.Key)
	}
	if metadata.URL != "/assets/"+metadata.Key {
		t.Fatalf("metadata URL = %q, want /assets/%s", metadata.URL, metadata.Key)
	}

	body, err := store.Open(context.Background(), metadata.Key)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	content, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(content) != "hello platform" {
		t.Fatalf("content = %q", string(content))
	}

	if err := store.Delete(context.Background(), metadata.Key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Open(context.Background(), metadata.Key); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("Open(deleted) error = %v, want ErrObjectNotFound", err)
	}
}

func TestLocalObjectStoreKeepsTraversalKeysInsideBaseDir(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: baseDir})

	path := store.pathForKey("../../secret.txt")
	expected := filepath.Join(baseDir, "secret.txt")
	if path != expected {
		t.Fatalf("pathForKey traversal = %q, want %q", path, expected)
	}
}

func TestNewObjectStoreRejectsUnsupportedDrivers(t *testing.T) {
	_, err := NewObjectStore(ObjectStoreConfig{Driver: "ftp"})
	if !errors.Is(err, ErrUnsupportedObjectStoreDriver) {
		t.Fatalf("NewObjectStore(ftp) error = %v, want unsupported driver", err)
	}
}

func TestNewObjectStoreRejectsS3WithoutBucket(t *testing.T) {
	_, err := NewObjectStore(ObjectStoreConfig{Driver: "s3"})
	if !errors.Is(err, ErrInvalidObjectStoreConfig) {
		t.Fatalf("NewObjectStore(s3 without bucket) error = %v, want invalid config", err)
	}
}

func TestNewObjectStoreRejectsS3PartialCredentials(t *testing.T) {
	_, err := NewObjectStore(ObjectStoreConfig{
		Driver: "s3",
		S3: S3ObjectStoreConfig{
			Bucket:    "platform",
			AccessKey: "access",
		},
	})
	if !errors.Is(err, ErrInvalidObjectStoreConfig) {
		t.Fatalf("NewObjectStore(s3 partial credentials) error = %v, want invalid config", err)
	}
}

func TestS3ObjectStoreSavesOpensAndDeletesObject(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 20, 30, 40, time.UTC)
	client := &fakeS3ObjectClient{objects: map[string]fakeS3Object{}}
	store, err := newS3ObjectStoreWithClient(client, "platform-bucket", "tenant/platform", func() time.Time { return now })
	if err != nil {
		t.Fatalf("newS3ObjectStoreWithClient() error = %v", err)
	}

	metadata, err := store.Save(context.Background(), ObjectSaveInput{
		FileName:    "demo file.txt",
		ContentType: "text/plain",
		Reader:      strings.NewReader("hello s3"),
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if metadata.Driver != "s3" || metadata.SizeBytes != int64(len("hello s3")) {
		t.Fatalf("metadata = %+v, want s3 metadata with size", metadata)
	}
	if !strings.HasPrefix(metadata.Key, "tenant/platform/2026/07/05/") || !strings.HasSuffix(metadata.Key, "/demo_file.txt") {
		t.Fatalf("metadata key = %q, want prefix and sanitized file name", metadata.Key)
	}
	if metadata.Path != "s3://platform-bucket/"+metadata.Key {
		t.Fatalf("metadata path = %q, want bucket URI", metadata.Path)
	}
	if metadata.URL != "" {
		t.Fatalf("metadata URL = %q, want empty URL until public URL policy exists", metadata.URL)
	}
	if client.objects[metadata.Key].contentType != "text/plain" {
		t.Fatalf("stored content type = %q, want text/plain", client.objects[metadata.Key].contentType)
	}

	body, err := store.Open(context.Background(), metadata.Key)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	content, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(content) != "hello s3" {
		t.Fatalf("content = %q", string(content))
	}

	if err := store.Delete(context.Background(), metadata.Key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Open(context.Background(), metadata.Key); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("Open(deleted) error = %v, want ErrObjectNotFound", err)
	}
}

func (client *fakeS3ObjectClient) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	client.objects[aws.ToString(input.Key)] = fakeS3Object{
		body:        string(body),
		contentType: aws.ToString(input.ContentType),
	}
	return &s3.PutObjectOutput{}, nil
}

func (client *fakeS3ObjectClient) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	object, ok := client.objects[aws.ToString(input.Key)]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "NoSuchKey", Message: "missing"}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(object.body))}, nil
}

func (client *fakeS3ObjectClient) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	delete(client.objects, aws.ToString(input.Key))
	return &s3.DeleteObjectOutput{}, nil
}
