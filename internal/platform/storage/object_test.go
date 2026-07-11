package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type fakeS3ObjectClient struct {
	objects      map[string]fakeS3Object
	lastPutInput *s3.PutObjectInput
}

type fakeS3Object struct {
	body        string
	contentType string
}

func TestLocalObjectStoreSavesOpensAndDeletesObject(t *testing.T) {
	baseDir := t.TempDir()
	now := time.Date(2026, 7, 5, 10, 20, 30, 40, time.UTC)
	store := NewLocalObjectStore(LocalObjectStoreOptions{
		BaseDir: baseDir,
		Now:     func() time.Time { return now },
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
	info, err := os.Stat(store.pathForKey(metadata.Key))
	if err != nil {
		t.Fatalf("Stat(saved object) error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("saved object mode = %o, want 600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(store.pathForKey(metadata.Key)))
	if err != nil {
		t.Fatalf("Stat(object directory) error = %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("object directory mode = %o, want 700", dirInfo.Mode().Perm())
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

func TestLocalObjectStoreSanitizesCrossPlatformAndControlCharacterFilenames(t *testing.T) {
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: t.TempDir()})

	metadata, err := store.Save(context.Background(), ObjectSaveInput{
		FileName: "..\\..\\unsafe\x00\n report.txt",
		Reader:   strings.NewReader("private"),
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if strings.Contains(metadata.Key, "..") || strings.ContainsAny(metadata.Key, "\\\x00\n\r") || !strings.HasSuffix(metadata.Key, "/unsafe_report.txt") {
		t.Fatalf("sanitized key = %q", metadata.Key)
	}
}

func TestLocalObjectStoreDoesNotOverwriteCollidingObjectKey(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: t.TempDir(), Now: func() time.Time { return now }})
	first, err := store.Save(context.Background(), ObjectSaveInput{FileName: "same.txt", Reader: strings.NewReader("first")})
	if err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	if _, err := store.Save(context.Background(), ObjectSaveInput{FileName: "same.txt", Reader: strings.NewReader("second")}); err == nil {
		t.Fatal("second Save() succeeded, want collision error")
	}
	body, err := store.Open(context.Background(), first.Key)
	if err != nil {
		t.Fatalf("Open(first) error = %v", err)
	}
	content, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil || string(content) != "first" {
		t.Fatalf("first object content/error = %q/%v", string(content), err)
	}
}

func TestLocalObjectStoreRejectsSymlinkDirectoryEscape(t *testing.T) {
	baseDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.Symlink(outsideDir, filepath.Join(baseDir, "2026")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	store := NewLocalObjectStore(LocalObjectStoreOptions{
		BaseDir: baseDir,
		Now: func() time.Time {
			return time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
		},
	})

	if _, err := store.Save(context.Background(), ObjectSaveInput{FileName: "private.txt", Reader: strings.NewReader("private")}); err == nil {
		t.Fatal("Save() through symlink directory succeeded, want error")
	}
	if _, err := os.Stat(filepath.Join(outsideDir, "07", "12")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside directory was modified through symlink, Stat error = %v", err)
	}
}

func TestLocalObjectStoreLimitsFilenameComponent(t *testing.T) {
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: t.TempDir()})
	metadata, err := store.Save(context.Background(), ObjectSaveInput{
		FileName: strings.Repeat("a", 400) + ".txt",
		Reader:   strings.NewReader("private"),
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	name := filepath.Base(metadata.Key)
	if len(name) > 255 || !strings.HasSuffix(name, ".txt") {
		t.Fatalf("stored filename length/suffix = %d/%q", len(name), name)
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
	store, err := newS3ObjectStoreWithClient(client, S3ObjectStoreConfig{
		Bucket:               "platform-bucket",
		Prefix:               "tenant/platform",
		ServerSideEncryption: "AES256",
	}, func() time.Time { return now })
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

func TestS3ObjectStoreAppliesConfiguredServerSideEncryption(t *testing.T) {
	tests := []struct {
		name       string
		encryption string
		kmsKeyID   string
		want       types.ServerSideEncryption
	}{
		{name: "s3 managed", encryption: "AES256", want: types.ServerSideEncryptionAes256},
		{name: "kms", encryption: "aws:kms", kmsKeyID: "arn:aws:kms:us-east-1:123456789012:key/test", want: types.ServerSideEncryptionAwsKms},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeS3ObjectClient{objects: map[string]fakeS3Object{}}
			store, err := newS3ObjectStoreWithClient(client, S3ObjectStoreConfig{
				Bucket:               "platform",
				ServerSideEncryption: tt.encryption,
				KMSKeyID:             tt.kmsKeyID,
			}, time.Now)
			if err != nil {
				t.Fatalf("newS3ObjectStoreWithClient() error = %v", err)
			}
			if _, err := store.Save(context.Background(), ObjectSaveInput{FileName: "private.txt", ContentType: "text/plain", Reader: strings.NewReader("private")}); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			if client.lastPutInput == nil {
				t.Fatal("PutObject input was not captured")
			}
			if client.lastPutInput.ServerSideEncryption != tt.want {
				t.Fatalf("ServerSideEncryption = %q, want %q", client.lastPutInput.ServerSideEncryption, tt.want)
			}
			if aws.ToString(client.lastPutInput.SSEKMSKeyId) != tt.kmsKeyID {
				t.Fatalf("SSEKMSKeyId = %q, want %q", aws.ToString(client.lastPutInput.SSEKMSKeyId), tt.kmsKeyID)
			}
			if client.lastPutInput.ACL != "" {
				t.Fatalf("ACL = %q, want no public or explicit ACL", client.lastPutInput.ACL)
			}
		})
	}
}

func TestNewS3ObjectStoreRejectsInvalidEncryptionPolicy(t *testing.T) {
	tests := []S3ObjectStoreConfig{
		{Bucket: "platform"},
		{Bucket: "platform", ServerSideEncryption: "unknown"},
		{Bucket: "platform", ServerSideEncryption: "aws:kms"},
	}
	for _, config := range tests {
		client := &fakeS3ObjectClient{objects: map[string]fakeS3Object{}}
		if _, err := newS3ObjectStoreWithClient(client, config, time.Now); !errors.Is(err, ErrInvalidObjectStoreConfig) {
			t.Fatalf("newS3ObjectStoreWithClient(%+v) error = %v, want invalid config", config, err)
		}
	}
}

func (client *fakeS3ObjectClient) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	client.lastPutInput = input
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
