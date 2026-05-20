package storage

import "testing"

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

func TestOSSStorageUploadedURL(t *testing.T) {
	const key = "uploads/abc/file.png"

	cases := []struct {
		name        string
		bucket      string
		region      string
		cdnDomain   string
		endpointURL string
		want        string
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &OSSStorage{
				bucket:      tc.bucket,
				region:      tc.region,
				cdnDomain:   tc.cdnDomain,
				endpointURL: tc.endpointURL,
			}
			if got := o.uploadedURL(key); got != tc.want {
				t.Fatalf("uploadedURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
