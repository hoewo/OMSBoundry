package bootstrap

import (
	"fmt"

	"omsbizfoundry/evidence-service/internal/config"
	"omsbizfoundry/evidence-service/internal/storage"
)

func Provider(cfg config.Config) (storage.Provider, error) {
	switch cfg.StorageProvider {
	case config.StorageLocal:
		return storage.NewLocalProvider(cfg.LocalStorageRoot, cfg.PublicBaseURL, cfg.LocalSigningKey, cfg.AccessGrantTTL)
	case config.StorageAliyunOSS:
		return ossProvider(cfg)
	case config.StorageHybrid:
		local, err := storage.NewLocalProvider(cfg.LocalStorageRoot, cfg.PublicBaseURL, cfg.LocalSigningKey, cfg.AccessGrantTTL)
		if err != nil {
			return nil, err
		}
		oss, err := ossProvider(cfg)
		if err != nil {
			return nil, err
		}
		return storage.NewHybridProvider(string(cfg.PrimaryStorageProvider), local, oss)
	default:
		return nil, fmt.Errorf("unsupported storage provider %q", cfg.StorageProvider)
	}
}

func ossProvider(cfg config.Config) (*storage.AliyunOSSProvider, error) {
	return storage.NewAliyunOSSProvider(storage.AliyunOSSOptions{
		Region:          cfg.OSSRegion,
		BrowserEndpoint: cfg.OSSBrowserEndpoint,
		BrowserCName:    cfg.OSSBrowserCName,
		ServiceEndpoint: cfg.OSSServiceEndpoint,
		Bucket:          cfg.OSSBucket,
		RoleARN:         cfg.OSSRoleARN,
		STSEndpoint:     cfg.OSSSTSEndpoint,
		STSDuration:     cfg.OSSSTSDuration,
		AccessTTL:       cfg.AccessGrantTTL,
	})
}
