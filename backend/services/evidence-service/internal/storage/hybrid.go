package storage

import (
	"context"
	"fmt"
	"io"
)

// HybridProvider keeps historical object locations authoritative. Primary only
// determines the location of NEW sessions; it never rewrites a File Version.
type HybridProvider struct {
	primary Provider
	local   Provider
	oss     Provider
	plane   LocalDataPlane
}

func NewHybridProvider(primary string, local, oss Provider) (*HybridProvider, error) {
	if local == nil || oss == nil || local.Name() != "local" || oss.Name() != "aliyun-oss" {
		return nil, fmt.Errorf("hybrid storage requires local and aliyun-oss providers")
	}
	plane, ok := local.(LocalDataPlane)
	if !ok || bucketName(oss) == "" {
		return nil, fmt.Errorf("hybrid storage requires a local data plane and an OSS bucket")
	}
	p := &HybridProvider{local: local, oss: oss, plane: plane}
	switch primary {
	case "local":
		p.primary = local
	case "aliyun-oss":
		p.primary = oss
	default:
		return nil, fmt.Errorf("hybrid primary must be local or aliyun-oss")
	}
	return p, nil
}

func (p *HybridProvider) Name() string        { return p.primary.Name() }
func (p *HybridProvider) StorageMode() string { return "hybrid" }
func (p *HybridProvider) BucketName() string  { return bucketName(p.primary) }
func (p *HybridProvider) Ready(ctx context.Context) error {
	if err := p.local.Ready(ctx); err != nil {
		return fmt.Errorf("local storage unavailable: %w", err)
	}
	if err := p.oss.Ready(ctx); err != nil {
		return fmt.Errorf("OSS storage unavailable: %w", err)
	}
	return nil
}

func (p *HybridProvider) route(ref ObjectRef) (Provider, error) {
	switch ref.Provider {
	case "local":
		if ref.Bucket == "" {
			return p.local, nil
		}
	case "aliyun-oss":
		if ref.Bucket == bucketName(p.oss) {
			return p.oss, nil
		}
	}
	return nil, ErrInvalidAuthorization
}

func (p *HybridProvider) AuthorizeUpload(ctx context.Context, intent UploadIntent) (UploadAuthorization, error) {
	provider, err := p.route(intent.Object)
	if err != nil {
		return UploadAuthorization{}, err
	}
	return provider.AuthorizeUpload(ctx, intent)
}
func (p *HybridProvider) Open(ctx context.Context, ref ObjectRef) (ObjectReader, error) {
	provider, err := p.route(ref)
	if err != nil {
		return ObjectReader{}, err
	}
	return provider.Open(ctx, ref)
}
func (p *HybridProvider) AuthorizeAccess(ctx context.Context, intent AccessIntent) (AccessGrant, error) {
	provider, err := p.route(intent.Object)
	if err != nil {
		return AccessGrant{}, err
	}
	return provider.AuthorizeAccess(ctx, intent)
}
func (p *HybridProvider) Delete(ctx context.Context, ref ObjectRef) error {
	provider, err := p.route(ref)
	if err != nil {
		return err
	}
	return provider.Delete(ctx, ref)
}

func (p *HybridProvider) localObject(ref ObjectRef) error {
	_, err := p.route(ref)
	if err != nil || ref.Provider != "local" {
		return ErrInvalidAuthorization
	}
	return nil
}
func (p *HybridProvider) VerifyUploadToken(token string, intent UploadIntent) error {
	if err := p.localObject(intent.Object); err != nil {
		return err
	}
	return p.plane.VerifyUploadToken(token, intent)
}
func (p *HybridProvider) Put(ctx context.Context, intent UploadIntent, reader io.Reader, length int64) (string, error) {
	if err := p.localObject(intent.Object); err != nil {
		return "", err
	}
	return p.plane.Put(ctx, intent, reader, length)
}
func (p *HybridProvider) VerifyAccessToken(token string, intent AccessIntent) error {
	if err := p.localObject(intent.Object); err != nil {
		return err
	}
	return p.plane.VerifyAccessToken(token, intent)
}
func bucketName(provider Provider) string {
	if named, ok := provider.(interface{ BucketName() string }); ok {
		return named.BucketName()
	}
	return ""
}
