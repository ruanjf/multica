package storage

import (
	"context"
	"io"
	"time"
)

type Storage interface {
	Upload(ctx context.Context, key string, data []byte, contentType string, filename string) (string, error)
	Delete(ctx context.Context, key string)
	DeleteKeys(ctx context.Context, keys []string)
	KeyFromURL(rawURL string) string
	CdnDomain() string
	// GetReader streams an object back to the caller. Used by the attachment
	// preview proxy (GET /api/attachments/{id}/content) to bypass CloudFront
	// CORS and the inline/attachment Content-Disposition decision. Caller
	// must Close the returned reader.
	GetReader(ctx context.Context, key string) (io.ReadCloser, error)
}

// URLPresigner is implemented by storage backends that support time-limited
// signed URLs for private objects. When the handler detects this interface,
// it replaces DownloadURL with a presigned URL instead of the raw object URL.
// This is the OSS equivalent of CloudFront signed URLs.
type URLPresigner interface {
	PresignGetURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}
