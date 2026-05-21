package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type OSSStorage struct {
	client      *oss.Client
	bucket      string
	region      string
	cdnDomain   string
	endpointURL string
}

// NewOSSStorageFromEnv creates an OSSStorage from environment variables.
// Returns nil if OSS_BUCKET is not set.
//
// Environment variables:
//   - OSS_BUCKET (required)
//   - OSS_REGION (required, e.g. cn-hangzhou)
//   - ALIBABA_CLOUD_ACCESS_KEY_ID / ALIBABA_CLOUD_ACCESS_KEY_SECRET (optional; falls back to ECS RAM role)
//   - OSS_CDN_DOMAIN (optional)
//   - OSS_ENDPOINT (optional, custom endpoint for internal/VPC access)
func NewOSSStorageFromEnv() *OSSStorage {
	bucket := os.Getenv("OSS_BUCKET")
	if bucket == "" {
		slog.Info("OSS_BUCKET not set, OSS upload disabled")
		return nil
	}

	region := os.Getenv("OSS_REGION")
	if region == "" {
		slog.Warn("OSS_REGION not set, OSS upload disabled")
		return nil
	}

	cfg := oss.LoadDefaultConfig().WithRegion(region)

	accessKey := os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")
	secretKey := os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	if accessKey != "" && secretKey != "" {
		cfg = cfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey))
	} else {
		cfg = cfg.WithCredentialsProvider(credentials.NewEcsRoleCredentialsProvider())
	}

	endpointURL := os.Getenv("OSS_ENDPOINT")
	if endpointURL != "" {
		cfg = cfg.WithEndpoint(endpointURL)
	}

	cdnDomain := os.Getenv("OSS_CDN_DOMAIN")

	slog.Info("OSS storage initialized", "bucket", bucket, "region", region, "cdn_domain", cdnDomain, "endpoint", endpointURL)
	return &OSSStorage{
		client:      oss.NewClient(cfg),
		bucket:      bucket,
		region:      region,
		cdnDomain:   cdnDomain,
		endpointURL: endpointURL,
	}
}

func (o *OSSStorage) CdnDomain() string {
	return o.cdnDomain
}

// KeyFromURL extracts the OSS object key from a CDN or bucket URL.
func (o *OSSStorage) KeyFromURL(rawURL string) string {
	if o.endpointURL != "" {
		prefix := strings.TrimRight(o.endpointURL, "/") + "/" + o.bucket + "/"
		if strings.HasPrefix(rawURL, prefix) {
			return strings.TrimPrefix(rawURL, prefix)
		}
	}

	prefixes := make([]string, 0, 3)
	if o.cdnDomain != "" {
		prefixes = append(prefixes, "https://"+o.cdnDomain+"/")
	}
	// virtual-hosted-style: https://<bucket>.oss-<region>.aliyuncs.com/<key>
	prefixes = append(prefixes, "https://"+o.bucket+".oss-"+o.region+".aliyuncs.com/")

	for _, prefix := range prefixes {
		if strings.HasPrefix(rawURL, prefix) {
			return strings.TrimPrefix(rawURL, prefix)
		}
	}
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		return rawURL[i+1:]
	}
	return rawURL
}

// PresignGetURL generates a time-limited signed URL for direct object download.
// Implements storage.URLPresigner; used by the handler when CloudFront signing
// is not configured. The returned URL bypasses the CDN domain and points directly
// to the OSS bucket endpoint, signed with the configured credentials.
func (o *OSSStorage) PresignGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if key == "" {
		return "", fmt.Errorf("oss PresignGetURL: empty key")
	}
	result, err := o.client.Presign(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(o.bucket),
		Key:    oss.Ptr(key),
	}, oss.PresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("oss presign: %w", err)
	}
	return result.URL, nil
}

func (o *OSSStorage) GetReader(ctx context.Context, key string) (io.ReadCloser, error) {
	if key == "" {
		return nil, fmt.Errorf("oss GetReader: empty key")
	}
	out, err := o.client.GetObject(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(o.bucket),
		Key:    oss.Ptr(key),
	})
	if err != nil {
		return nil, fmt.Errorf("oss GetObject: %w", err)
	}
	return out.Body, nil
}

func (o *OSSStorage) Delete(ctx context.Context, key string) {
	if key == "" {
		return
	}
	_, err := o.client.DeleteObject(ctx, &oss.DeleteObjectRequest{
		Bucket: oss.Ptr(o.bucket),
		Key:    oss.Ptr(key),
	})
	if err != nil {
		slog.Error("oss DeleteObject failed", "key", key, "error", err)
	}
}

func (o *OSSStorage) DeleteKeys(ctx context.Context, keys []string) {
	for _, key := range keys {
		o.Delete(ctx, key)
	}
}

func (o *OSSStorage) Upload(ctx context.Context, key string, data []byte, contentType string, filename string) (string, error) {
	safe := sanitizeFilename(filename)
	disposition := "attachment"
	if isInlineContentType(contentType) {
		disposition = "inline"
	}
	_, err := o.client.PutObject(ctx, &oss.PutObjectRequest{
		Bucket:             oss.Ptr(o.bucket),
		Key:                oss.Ptr(key),
		Body:               bytes.NewReader(data),
		ContentType:        oss.Ptr(contentType),
		ContentDisposition: oss.Ptr(fmt.Sprintf(`%s; filename="%s"`, disposition, safe)),
		CacheControl:       oss.Ptr("max-age=432000,public"),
	})
	if err != nil {
		return "", fmt.Errorf("oss PutObject: %w", err)
	}
	return o.uploadedURL(key), nil
}

func (o *OSSStorage) uploadedURL(key string) string {
	if o.cdnDomain != "" {
		return fmt.Sprintf("https://%s/%s", o.cdnDomain, key)
	}
	if o.endpointURL != "" {
		return fmt.Sprintf("%s/%s/%s", strings.TrimRight(o.endpointURL, "/"), o.bucket, key)
	}
	return fmt.Sprintf("https://%s.oss-%s.aliyuncs.com/%s", o.bucket, o.region, key)
}
