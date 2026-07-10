package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrObjectNotFound               = errors.New("file object not found")
	ErrUnsupportedObjectStoreDriver = errors.New("unsupported object store driver")
	ErrInvalidObjectStoreConfig     = errors.New("invalid object store config")
)

type ObjectStore interface {
	Save(ctx context.Context, input ObjectSaveInput) (ObjectMetadata, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type ObjectSaveInput struct {
	FileName    string
	ContentType string
	Reader      io.Reader
}

type ObjectMetadata struct {
	Driver    string
	Key       string
	Path      string
	URL       string
	SizeBytes int64
}

type ObjectStoreConfig struct {
	Driver        string
	LocalBaseDir  string
	PublicBaseURL string
	S3            S3ObjectStoreConfig
}

type S3ObjectStoreConfig struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	Prefix         string
	ForcePathStyle bool
}

type LocalObjectStoreOptions struct {
	BaseDir       string
	PublicBaseURL string
	Now           func() time.Time
}

type LocalObjectStore struct {
	baseDir       string
	publicBaseURL string
	now           func() time.Time
}

func NewObjectStore(config ObjectStoreConfig) (ObjectStore, error) {
	driver := strings.ToLower(strings.TrimSpace(config.Driver))
	switch driver {
	case "", "local":
		return NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: config.LocalBaseDir, PublicBaseURL: config.PublicBaseURL}), nil
	case "s3":
		return NewS3ObjectStore(context.Background(), config.S3)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedObjectStoreDriver, driver)
	}
}

func NewLocalObjectStore(options LocalObjectStoreOptions) LocalObjectStore {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return LocalObjectStore{
		baseDir:       resolveLocalObjectBaseDir(options.BaseDir),
		publicBaseURL: normalizePublicBaseURL(options.PublicBaseURL),
		now:           now,
	}
}

func (store LocalObjectStore) Save(_ context.Context, input ObjectSaveInput) (ObjectMetadata, error) {
	if input.Reader == nil {
		return ObjectMetadata{}, errors.New("object reader is required")
	}
	name := sanitizedObjectFileName(input.FileName)
	key := path.Join(store.timestampPrefix(), name)
	targetPath := store.pathForKey(key)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return ObjectMetadata{}, err
	}
	file, err := os.Create(targetPath)
	if err != nil {
		return ObjectMetadata{}, err
	}
	size, copyErr := io.Copy(file, input.Reader)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(targetPath)
		return ObjectMetadata{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(targetPath)
		return ObjectMetadata{}, closeErr
	}
	return ObjectMetadata{
		Driver:    "local",
		Key:       key,
		Path:      targetPath,
		URL:       store.PublicURLForKey(key),
		SizeBytes: size,
	}, nil
}

func (store LocalObjectStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	file, err := os.Open(store.pathForKey(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrObjectNotFound
	}
	return file, err
}

func (store LocalObjectStore) Delete(_ context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	err := os.Remove(store.pathForKey(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (store LocalObjectStore) PublicURLForKey(key string) string {
	key = strings.TrimPrefix(cleanObjectKey(key), "/")
	if key == "" {
		return ""
	}
	if store.publicBaseURL == "" {
		return ""
	}
	return strings.TrimRight(store.publicBaseURL, "/") + "/" + key
}

func (store LocalObjectStore) pathForKey(key string) string {
	cleanKey := cleanObjectKey(key)
	return filepath.Join(store.baseDir, filepath.FromSlash(cleanKey))
}

func (store LocalObjectStore) timestampPrefix() string {
	now := store.now().UTC()
	return fmt.Sprintf("%04d/%02d/%02d/%d", now.Year(), now.Month(), now.Day(), now.UnixNano())
}

func resolveLocalObjectBaseDir(input string) string {
	baseDir := strings.TrimSpace(input)
	if baseDir == "" {
		baseDir = ".platform/uploads"
	}
	if filepath.IsAbs(baseDir) {
		return filepath.Clean(baseDir)
	}
	absolute, err := filepath.Abs(baseDir)
	if err != nil {
		return filepath.Clean(baseDir)
	}
	return absolute
}

func normalizePublicBaseURL(input string) string {
	value := strings.TrimSpace(input)
	if value == "" {
		return "/uploads"
	}
	return strings.TrimRight(value, "/")
}

func sanitizedObjectFileName(fileName string) string {
	name := filepath.Base(strings.TrimSpace(fileName))
	name = strings.ReplaceAll(name, " ", "_")
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "file"
	}
	return name
}

func cleanObjectKey(key string) string {
	return strings.TrimPrefix(path.Clean("/"+filepath.ToSlash(strings.TrimSpace(key))), "/")
}
