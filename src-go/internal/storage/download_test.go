package storage_test

import (
	"context"
	"testing"
)

func TestDownload(t *testing.T) {
	ctx := context.Background()
	client, _ := memClient(t)

	const key = "content/user-1/item-1.html"
	const body = "<html><body>Hello World</body></html>"

	// Upload first
	if err := client.UploadContent(ctx, key, body); err != nil {
		t.Fatalf("UploadContent error: %v", err)
	}

	// Download
	data, err := client.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	if string(data) != body {
		t.Errorf("Download content mismatch: got %q, want %q", string(data), body)
	}
}

func TestDownloadNotFound(t *testing.T) {
	ctx := context.Background()
	client, _ := memClient(t)

	_, err := client.Download(ctx, "nonexistent/key")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}
