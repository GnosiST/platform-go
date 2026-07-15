package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	ErrObjectNotFound               = errors.New("file object not found")
	ErrObjectSaveFailed             = errors.New("file object save failed")
	ErrObjectOpenFailed             = errors.New("file object open failed")
	ErrObjectDeleteFailed           = errors.New("file object delete failed")
	ErrUnsupportedObjectStoreDriver = errors.New("unsupported object store driver")
	ErrInvalidObjectStoreConfig     = errors.New("invalid object store config")
	ErrUnsafeObjectPath             = errors.New("unsafe file object path")
)

type objectKeyGenerator func() (string, error)

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
	SizeBytes int64
}

type ObjectStoreConfig struct {
	Driver       string
	LocalBaseDir string
	S3           S3ObjectStoreConfig
}

type S3ObjectStoreConfig struct {
	Endpoint             string
	Region               string
	Bucket               string
	AccessKey            string
	SecretKey            string
	Prefix               string
	ForcePathStyle       bool
	ServerSideEncryption string
	KMSKeyID             string
}

type LocalObjectStoreOptions struct {
	BaseDir string
}

type LocalObjectStore struct {
	baseDir      string
	root         *os.Root
	rootInfo     os.FileInfo
	initErr      error
	keyGenerator objectKeyGenerator
}

func NewObjectStore(config ObjectStoreConfig) (ObjectStore, error) {
	driver := strings.ToLower(strings.TrimSpace(config.Driver))
	switch driver {
	case "", "local":
		store := NewLocalObjectStore(LocalObjectStoreOptions{BaseDir: config.LocalBaseDir})
		if store.initErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidObjectStoreConfig, store.initErr)
		}
		return store, nil
	case "s3":
		return NewS3ObjectStore(context.Background(), config.S3)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedObjectStoreDriver, driver)
	}
}

func NewLocalObjectStore(options LocalObjectStoreOptions) LocalObjectStore {
	baseDir := resolveLocalObjectBaseDir(options.BaseDir)
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return LocalObjectStore{baseDir: baseDir, initErr: err, keyGenerator: newOpaqueObjectKey}
	}
	canonicalDir, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return LocalObjectStore{baseDir: baseDir, initErr: err, keyGenerator: newOpaqueObjectKey}
	}
	canonicalDir, err = filepath.Abs(canonicalDir)
	if err != nil {
		return LocalObjectStore{baseDir: canonicalDir, initErr: err, keyGenerator: newOpaqueObjectKey}
	}
	info, err := os.Lstat(canonicalDir)
	if err != nil {
		return LocalObjectStore{baseDir: canonicalDir, initErr: err, keyGenerator: newOpaqueObjectKey}
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return LocalObjectStore{baseDir: canonicalDir, initErr: fmt.Errorf("%w: local object root must be a real directory", ErrUnsafeObjectPath), keyGenerator: newOpaqueObjectKey}
	}
	if err := os.Chmod(canonicalDir, 0o700); err != nil {
		return LocalObjectStore{baseDir: canonicalDir, initErr: err, keyGenerator: newOpaqueObjectKey}
	}
	root, err := os.OpenRoot(canonicalDir)
	return LocalObjectStore{
		baseDir:      canonicalDir,
		root:         root,
		rootInfo:     info,
		initErr:      err,
		keyGenerator: newOpaqueObjectKey,
	}
}

func (store LocalObjectStore) Save(_ context.Context, input ObjectSaveInput) (ObjectMetadata, error) {
	if input.Reader == nil {
		return ObjectMetadata{}, ErrObjectSaveFailed
	}
	if err := store.validateRoot(); err != nil {
		return ObjectMetadata{}, normalizeObjectOperationError(err, ErrObjectSaveFailed)
	}
	key, err := store.keyGenerator()
	if err != nil {
		return ObjectMetadata{}, ErrObjectSaveFailed
	}
	key, err = safeObjectKey(key)
	if err != nil {
		return ObjectMetadata{}, normalizeObjectOperationError(err, ErrObjectSaveFailed)
	}
	if err := store.ensurePrivateObjectDir(path.Dir(key)); err != nil {
		return ObjectMetadata{}, normalizeObjectOperationError(err, ErrObjectSaveFailed)
	}
	if info, err := store.root.Lstat(key); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return ObjectMetadata{}, fmt.Errorf("%w: object target must not be a symlink", ErrUnsafeObjectPath)
		}
		return ObjectMetadata{}, ErrObjectSaveFailed
	} else if !errors.Is(err, os.ErrNotExist) {
		return ObjectMetadata{}, ErrObjectSaveFailed
	}
	file, err := store.root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return ObjectMetadata{}, ErrObjectSaveFailed
	}
	size, copyErr := io.Copy(file, input.Reader)
	closeErr := file.Close()
	if copyErr != nil {
		_ = store.root.Remove(key)
		return ObjectMetadata{}, ErrObjectSaveFailed
	}
	if closeErr != nil {
		_ = store.root.Remove(key)
		return ObjectMetadata{}, ErrObjectSaveFailed
	}
	return ObjectMetadata{
		Driver:    "local",
		Key:       key,
		SizeBytes: size,
	}, nil
}

func (store LocalObjectStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if err := store.validateRoot(); err != nil {
		return nil, normalizeObjectOperationError(err, ErrObjectOpenFailed)
	}
	key, err := safeObjectKey(key)
	if err != nil {
		return nil, normalizeObjectOperationError(err, ErrObjectOpenFailed)
	}
	if err := store.validateObjectTarget(key); err != nil {
		return nil, normalizeObjectOperationError(err, ErrObjectOpenFailed)
	}
	file, err := store.root.Open(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrObjectNotFound
	}
	if err != nil {
		return nil, ErrObjectOpenFailed
	}
	return file, nil
}

func (store LocalObjectStore) Delete(_ context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	if err := store.validateRoot(); err != nil {
		return normalizeObjectOperationError(err, ErrObjectDeleteFailed)
	}
	key, err := safeObjectKey(key)
	if err != nil {
		return normalizeObjectOperationError(err, ErrObjectDeleteFailed)
	}
	if err := store.validateObjectTarget(key); err != nil {
		return normalizeObjectOperationError(err, ErrObjectDeleteFailed)
	}
	err = store.root.Remove(key)
	if errors.Is(err, os.ErrNotExist) {
		return ErrObjectNotFound
	}
	if err != nil {
		return ErrObjectDeleteFailed
	}
	return nil
}

func normalizeObjectOperationError(err error, operation error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrObjectNotFound):
		return ErrObjectNotFound
	case errors.Is(err, ErrUnsafeObjectPath):
		return ErrUnsafeObjectPath
	default:
		return operation
	}
}

func (store LocalObjectStore) pathForKey(key string) string {
	return filepath.Join(store.baseDir, filepath.FromSlash(cleanObjectKey(key)))
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

func (store LocalObjectStore) ensurePrivateObjectDir(relative string) error {
	if relative == "." || relative == "" {
		return nil
	}
	current := ""
	for _, component := range strings.Split(relative, "/") {
		current = path.Join(current, component)
		info, err := store.root.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err := store.root.Mkdir(current, 0o700); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("%w: object directory component must be a real directory", ErrUnsafeObjectPath)
		}
		if err := store.root.Chmod(current, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func (store LocalObjectStore) validateRoot() error {
	if store.initErr != nil {
		return store.initErr
	}
	info, err := os.Lstat(store.baseDir)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || !os.SameFile(store.rootInfo, info) {
		return fmt.Errorf("%w: local object root changed after initialization", ErrUnsafeObjectPath)
	}
	return nil
}

func (store LocalObjectStore) validateObjectTarget(key string) error {
	info, err := store.root.Lstat(key)
	if errors.Is(err, os.ErrNotExist) {
		return ErrObjectNotFound
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: object target must be a regular file", ErrUnsafeObjectPath)
	}
	return nil
}

func newOpaqueObjectKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return path.Join("objects", hex.EncodeToString(bytes)), nil
}

func safeObjectKey(key string) (string, error) {
	value := strings.TrimSpace(filepath.ToSlash(key))
	if value == "" || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("%w: invalid object key", ErrUnsafeObjectPath)
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%w: invalid object key", ErrUnsafeObjectPath)
	}
	return cleaned, nil
}

func cleanObjectKey(key string) string {
	return strings.TrimPrefix(path.Clean("/"+filepath.ToSlash(strings.TrimSpace(key))), "/")
}
