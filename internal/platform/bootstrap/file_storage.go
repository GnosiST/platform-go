package bootstrap

import (
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func FileStorageFromConfig(cfg config.Config) (storage.ObjectStore, error) {
	return storage.NewObjectStore(storage.ObjectStoreConfig{
		Driver:       cfg.FileStorageDriver,
		LocalBaseDir: cfg.FileStorageLocalDir,
		S3: storage.S3ObjectStoreConfig{
			Endpoint:             cfg.FileStorageS3Endpoint,
			Region:               cfg.FileStorageS3Region,
			Bucket:               cfg.FileStorageS3Bucket,
			AccessKey:            cfg.FileStorageS3AccessKey,
			SecretKey:            cfg.FileStorageS3SecretKey,
			Prefix:               cfg.FileStorageS3Prefix,
			ForcePathStyle:       cfg.FileStorageS3PathStyle,
			ServerSideEncryption: cfg.FileStorageS3ServerSideEncryption,
			KMSKeyID:             cfg.FileStorageS3KMSKeyID,
		},
	})
}
