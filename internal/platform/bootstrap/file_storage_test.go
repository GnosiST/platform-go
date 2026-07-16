package bootstrap

import (
	"errors"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestFileStorageFromConfigUsesLocalStoreByDefault(t *testing.T) {
	store, err := FileStorageFromConfig(config.Config{})
	if err != nil {
		t.Fatalf("FileStorageFromConfig() error = %v", err)
	}
	if store == nil {
		t.Fatalf("FileStorageFromConfig() store is nil")
	}
}

func TestFileStorageFromConfigRejectsUnsupportedDriver(t *testing.T) {
	_, err := FileStorageFromConfig(config.Config{FileStorageDriver: "ftp"})
	if !errors.Is(err, storage.ErrUnsupportedObjectStoreDriver) {
		t.Fatalf("FileStorageFromConfig(ftp) error = %v, want unsupported driver", err)
	}
}

func TestFileStorageFromConfigRejectsS3WithoutBucket(t *testing.T) {
	_, err := FileStorageFromConfig(config.Config{FileStorageDriver: "s3"})
	if !errors.Is(err, storage.ErrInvalidObjectStoreConfig) {
		t.Fatalf("FileStorageFromConfig(s3 without bucket) error = %v, want invalid config", err)
	}
}

func TestFileStorageFromConfigBuildsS3Store(t *testing.T) {
	store, err := FileStorageFromConfig(config.Config{
		FileStorageDriver:                 "s3",
		FileStorageS3Region:               "us-east-1",
		FileStorageS3Bucket:               "platform",
		FileStorageS3AccessKey:            "access",
		FileStorageS3SecretKey:            "secret",
		FileStorageS3Prefix:               "tenant/platform",
		FileStorageS3PathStyle:            true,
		FileStorageS3ServerSideEncryption: "AES256",
	})
	if err != nil {
		t.Fatalf("FileStorageFromConfig(s3) error = %v", err)
	}
	if store == nil {
		t.Fatalf("FileStorageFromConfig(s3) store is nil")
	}
}
