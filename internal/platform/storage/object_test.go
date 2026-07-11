package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: baseDir})

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
	if !regexp.MustCompile(`^objects/[a-f0-9]{64}$`).MatchString(metadata.Key) || strings.Contains(metadata.Key, "demo") {
		t.Fatalf("metadata key = %q, want opaque object key", metadata.Key)
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

func TestLocalObjectStoreKeyIgnoresCrossPlatformAndControlCharacterFilenames(t *testing.T) {
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: t.TempDir()})

	metadata, err := store.Save(context.Background(), ObjectSaveInput{
		FileName: "..\\..\\unsafe\x00\n report.txt",
		Reader:   strings.NewReader("private"),
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !regexp.MustCompile(`^objects/[a-f0-9]{64}$`).MatchString(metadata.Key) || strings.Contains(metadata.Key, "unsafe") {
		t.Fatalf("opaque key = %q", metadata.Key)
	}
}

func TestLocalObjectStoreSameNameAndTimeDoNotCollide(t *testing.T) {
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: t.TempDir()})
	first, err := store.Save(context.Background(), ObjectSaveInput{FileName: "same.txt", Reader: strings.NewReader("first")})
	if err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	second, err := store.Save(context.Background(), ObjectSaveInput{FileName: "same.txt", Reader: strings.NewReader("second")})
	if err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	if first.Key == second.Key || strings.Contains(first.Key, "same") || strings.Contains(second.Key, "same") {
		t.Fatalf("same-name keys = %q/%q, want distinct opaque keys", first.Key, second.Key)
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

func TestLocalObjectStoreCanonicalizesConfiguredSymlinkRoot(t *testing.T) {
	parent := t.TempDir()
	canonicalDir := filepath.Join(parent, "canonical")
	if err := os.Mkdir(canonicalDir, 0o700); err != nil {
		t.Fatalf("Mkdir(canonical) error = %v", err)
	}
	configuredDir := filepath.Join(parent, "configured")
	if err := os.Symlink(canonicalDir, configuredDir); err != nil {
		t.Fatalf("Symlink(configured root) error = %v", err)
	}
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: configuredDir})
	wantCanonical, err := filepath.EvalSymlinks(canonicalDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(canonical) error = %v", err)
	}
	if store.baseDir != wantCanonical {
		t.Fatalf("store baseDir = %q, want canonical %q", store.baseDir, wantCanonical)
	}
	if _, err := store.Save(context.Background(), ObjectSaveInput{FileName: "private.txt", Reader: strings.NewReader("private")}); err != nil {
		t.Fatalf("Save() through canonicalized root error = %v", err)
	}
}

func TestLocalObjectStoreRejectsReplacedCanonicalRoot(t *testing.T) {
	parent := t.TempDir()
	baseDir := filepath.Join(parent, "objects")
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: baseDir})
	stored, err := store.Save(context.Background(), ObjectSaveInput{FileName: "private.txt", Reader: strings.NewReader("private")})
	if err != nil {
		t.Fatalf("initial Save() error = %v", err)
	}
	originalDir := filepath.Join(parent, "objects-original")
	if err := os.Rename(baseDir, originalDir); err != nil {
		t.Fatalf("Rename(root) error = %v", err)
	}
	outsideDir := t.TempDir()
	if err := os.Symlink(outsideDir, baseDir); err != nil {
		t.Fatalf("Symlink(replacement root) error = %v", err)
	}
	if _, err := store.Save(context.Background(), ObjectSaveInput{FileName: "new.txt", Reader: strings.NewReader("new")}); !errors.Is(err, ErrUnsafeObjectPath) {
		t.Fatalf("Save() after root replacement error = %v, want unsafe path", err)
	}
	if _, err := store.Open(context.Background(), stored.Key); !errors.Is(err, ErrUnsafeObjectPath) {
		t.Fatalf("Open() after root replacement error = %v, want unsafe path", err)
	}
	if err := store.Delete(context.Background(), stored.Key); !errors.Is(err, ErrUnsafeObjectPath) {
		t.Fatalf("Delete() after root replacement error = %v, want unsafe path", err)
	}
}

func TestLocalObjectStoreRejectsSymlinkDirectoryComponent(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: baseDir})
	outsideDir := t.TempDir()
	if err := os.Symlink(outsideDir, filepath.Join(baseDir, "objects")); err != nil {
		t.Fatalf("Symlink(directory component) error = %v", err)
	}
	if _, err := store.Save(context.Background(), ObjectSaveInput{FileName: "private.txt", Reader: strings.NewReader("private")}); !errors.Is(err, ErrUnsafeObjectPath) {
		t.Fatalf("Save() through symlink directory error = %v, want unsafe path", err)
	}
	entries, err := os.ReadDir(outsideDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside directory entries/error = %v/%v, want empty", entries, err)
	}
}

func TestLocalObjectStoreRejectsSymlinkObjectForSaveOpenAndDelete(t *testing.T) {
	baseDir := t.TempDir()
	store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: baseDir})
	store.keyGenerator = func() (string, error) { return "objects/fixed", nil }
	if err := os.Mkdir(filepath.Join(baseDir, "objects"), 0o700); err != nil {
		t.Fatalf("Mkdir(objects) error = %v", err)
	}
	outsideFile := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o600); err != nil {
		t.Fatalf("WriteFile(outside) error = %v", err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(baseDir, "objects", "fixed")); err != nil {
		t.Fatalf("Symlink(object) error = %v", err)
	}
	if _, err := store.Save(context.Background(), ObjectSaveInput{FileName: "private.txt", Reader: strings.NewReader("private")}); !errors.Is(err, ErrUnsafeObjectPath) {
		t.Fatalf("Save() target symlink error = %v, want unsafe path", err)
	}
	if _, err := store.Open(context.Background(), "objects/fixed"); !errors.Is(err, ErrUnsafeObjectPath) {
		t.Fatalf("Open() target symlink error = %v, want unsafe path", err)
	}
	if err := store.Delete(context.Background(), "objects/fixed"); !errors.Is(err, ErrUnsafeObjectPath) {
		t.Fatalf("Delete() target symlink error = %v, want unsafe path", err)
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
	client := &fakeS3ObjectClient{objects: map[string]fakeS3Object{}}
	store, err := newS3ObjectStoreWithClient(client, S3ObjectStoreConfig{
		Bucket:               "platform-bucket",
		Prefix:               "tenant/platform",
		ServerSideEncryption: "AES256",
	}, nil)
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
	if !regexp.MustCompile(`^tenant/platform/objects/[a-f0-9]{64}$`).MatchString(metadata.Key) || strings.Contains(metadata.Key, "demo") {
		t.Fatalf("metadata key = %q, want prefixed opaque key", metadata.Key)
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

func TestS3ObjectStoreSameNameAndTimeDoNotCollide(t *testing.T) {
	client := &fakeS3ObjectClient{objects: map[string]fakeS3Object{}}
	store, err := newS3ObjectStoreWithClient(client, S3ObjectStoreConfig{Bucket: "platform", ServerSideEncryption: "AES256"}, nil)
	if err != nil {
		t.Fatalf("newS3ObjectStoreWithClient() error = %v", err)
	}
	first, err := store.Save(context.Background(), ObjectSaveInput{FileName: "same.txt", Reader: strings.NewReader("first")})
	if err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	second, err := store.Save(context.Background(), ObjectSaveInput{FileName: "same.txt", Reader: strings.NewReader("second")})
	if err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	if first.Key == second.Key || strings.Contains(first.Key, "same") || strings.Contains(second.Key, "same") {
		t.Fatalf("same-name S3 keys = %q/%q, want distinct opaque keys", first.Key, second.Key)
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
			}, nil)
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
		if _, err := newS3ObjectStoreWithClient(client, config, nil); !errors.Is(err, ErrInvalidObjectStoreConfig) {
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
