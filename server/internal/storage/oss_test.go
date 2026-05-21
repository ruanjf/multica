package storage

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOSSStorageKeyFromURL_VirtualHostedStyle(t *testing.T) {
	o := &OSSStorage{
		bucket: "test-bucket",
		region: "cn-hangzhou",
	}

	rawURL := "https://test-bucket.oss-cn-hangzhou.aliyuncs.com/uploads/abc/file.png"
	if got := o.KeyFromURL(rawURL); got != "uploads/abc/file.png" {
		t.Fatalf("KeyFromURL(%q) = %q, want %q", rawURL, got, "uploads/abc/file.png")
	}
}

func TestOSSStorageKeyFromURL_CDNDomain(t *testing.T) {
	o := &OSSStorage{
		bucket:    "test-bucket",
		region:    "cn-hangzhou",
		cdnDomain: "cdn.example.com",
	}

	rawURL := "https://cdn.example.com/uploads/abc/file.png"
	if got := o.KeyFromURL(rawURL); got != "uploads/abc/file.png" {
		t.Fatalf("KeyFromURL(%q) = %q, want %q", rawURL, got, "uploads/abc/file.png")
	}
}

func TestOSSStorageKeyFromURL_CustomEndpoint(t *testing.T) {
	o := &OSSStorage{
		bucket:      "test-bucket",
		region:      "cn-hangzhou",
		endpointURL: "http://oss-internal.example.com",
	}

	rawURL := "http://oss-internal.example.com/test-bucket/uploads/abc/file.png"
	if got := o.KeyFromURL(rawURL); got != "uploads/abc/file.png" {
		t.Fatalf("KeyFromURL(%q) = %q, want %q", rawURL, got, "uploads/abc/file.png")
	}
}

func TestOSSStorageKeyFromURL_CustomEndpointTrailingSlash(t *testing.T) {
	o := &OSSStorage{
		bucket:      "test-bucket",
		region:      "cn-hangzhou",
		endpointURL: "http://oss-internal.example.com/",
	}

	rawURL := "http://oss-internal.example.com/test-bucket/uploads/abc/file.png"
	if got := o.KeyFromURL(rawURL); got != "uploads/abc/file.png" {
		t.Fatalf("KeyFromURL(%q) = %q, want %q", rawURL, got, "uploads/abc/file.png")
	}
}

func TestOSSStorageKeyFromURL_StaticDomain(t *testing.T) {
	o := &OSSStorage{
		bucket:       "test-bucket",
		region:       "cn-hangzhou",
		staticDomain: "static.example.com",
	}

	rawURL := "https://static.example.com/workspaces/ws-uuid/file.png"
	if got := o.KeyFromURL(rawURL); got != "workspaces/ws-uuid/file.png" {
		t.Fatalf("KeyFromURL(%q) = %q, want %q", rawURL, got, "workspaces/ws-uuid/file.png")
	}
}

// staticDomain must take priority over endpointURL and cdnDomain.
func TestOSSStorageKeyFromURL_StaticDomainPriorityOverEndpoint(t *testing.T) {
	o := &OSSStorage{
		bucket:       "test-bucket",
		region:       "cn-hangzhou",
		staticDomain: "static.example.com",
		cdnDomain:    "cdn.example.com",
		endpointURL:  "http://oss-internal.example.com",
	}

	rawURL := "https://static.example.com/workspaces/ws-uuid/file.png"
	if got := o.KeyFromURL(rawURL); got != "workspaces/ws-uuid/file.png" {
		t.Fatalf("KeyFromURL(%q) = %q, want %q (staticDomain should have highest priority)", rawURL, got, "workspaces/ws-uuid/file.png")
	}
}

func TestOSSStorageUploadedURL(t *testing.T) {
	const key = "uploads/abc/file.png"

	cases := []struct {
		name         string
		bucket       string
		region       string
		cdnDomain    string
		endpointURL  string
		staticDomain string
		want         string
	}{
		{
			name:   "default virtual hosted style",
			bucket: "test-bucket",
			region: "cn-hangzhou",
			want:   "https://test-bucket.oss-cn-hangzhou.aliyuncs.com/uploads/abc/file.png",
		},
		{
			name:      "cdn domain wins",
			bucket:    "test-bucket",
			region:    "cn-hangzhou",
			cdnDomain: "cdn.example.com",
			want:      "https://cdn.example.com/uploads/abc/file.png",
		},
		{
			name:        "custom endpoint",
			bucket:      "test-bucket",
			region:      "cn-hangzhou",
			endpointURL: "http://oss-internal.example.com",
			want:        "http://oss-internal.example.com/test-bucket/uploads/abc/file.png",
		},
		{
			name:        "cdn wins over endpoint",
			bucket:      "test-bucket",
			region:      "cn-hangzhou",
			cdnDomain:   "cdn.example.com",
			endpointURL: "http://oss-internal.example.com",
			want:        "https://cdn.example.com/uploads/abc/file.png",
		},
		{
			name:        "endpoint trailing slash stripped",
			bucket:      "test-bucket",
			region:      "cn-hangzhou",
			endpointURL: "http://oss-internal.example.com/",
			want:        "http://oss-internal.example.com/test-bucket/uploads/abc/file.png",
		},
		{
			name:         "static domain wins over cdn and endpoint",
			bucket:       "test-bucket",
			region:       "cn-hangzhou",
			staticDomain: "static.example.com",
			cdnDomain:    "cdn.example.com",
			endpointURL:  "http://oss-internal.example.com",
			want:         "https://static.example.com/uploads/abc/file.png",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &OSSStorage{
				bucket:       tc.bucket,
				region:       tc.region,
				cdnDomain:    tc.cdnDomain,
				endpointURL:  tc.endpointURL,
				staticDomain: tc.staticDomain,
			}
			if got := o.uploadedURL(key); got != tc.want {
				t.Fatalf("uploadedURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSignCDNURL_Format(t *testing.T) {
	o := &OSSStorage{
		bucket:     "test-bucket",
		region:     "cn-hangzhou",
		cdnDomain:  "cdn.example.com",
		cdnAuthKey: "testkey123",
	}
	const objKey = "uploads/ws/file.png"
	expiry := 30 * time.Minute

	signed := o.signCDNURL(objKey, expiry)

	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("signCDNURL returned unparseable URL: %v", err)
	}
	if parsed.Host != "cdn.example.com" {
		t.Errorf("host = %q, want cdn.example.com", parsed.Host)
	}
	if parsed.Path != "/"+objKey {
		t.Errorf("path = %q, want /%s", parsed.Path, objKey)
	}
	authKey := parsed.Query().Get("auth_key")
	if authKey == "" {
		t.Fatal("auth_key query param missing")
	}
	parts := strings.SplitN(authKey, "-", 4)
	if len(parts) != 4 {
		t.Fatalf("auth_key %q: expected 4 dash-separated parts, got %d", authKey, len(parts))
	}
	ts, randPart, uid, hash := parts[0], parts[1], parts[2], parts[3]
	if randPart != "0" || uid != "0" {
		t.Errorf("rand=%q uid=%q: want both '0'", randPart, uid)
	}
	// Re-derive the expected MD5 and compare.
	plain := fmt.Sprintf("/%s-%s-%s-%s-%s", objKey, ts, randPart, uid, o.cdnAuthKey)
	want := fmt.Sprintf("%x", md5.Sum([]byte(plain)))
	if hash != want {
		t.Errorf("hash = %q, want %q (plain: %q)", hash, want, plain)
	}
}

func TestSignCDNURL_ExpiryEmbedded(t *testing.T) {
	o := &OSSStorage{
		bucket:     "b",
		region:     "cn-shenzhen",
		cdnDomain:  "cdn.example.com",
		cdnAuthKey: "secret",
	}
	before := time.Now()
	signed := o.signCDNURL("k", 10*time.Minute)
	after := time.Now()

	parsed, _ := url.Parse(signed)
	parts := strings.SplitN(parsed.Query().Get("auth_key"), "-", 4)
	var ts int64
	fmt.Sscanf(parts[0], "%d", &ts)

	if ts < before.Add(10*time.Minute).Unix() || ts > after.Add(10*time.Minute).Unix() {
		t.Errorf("timestamp %d not in expected range [%d, %d]", ts, before.Add(10*time.Minute).Unix(), after.Add(10*time.Minute).Unix())
	}
}
