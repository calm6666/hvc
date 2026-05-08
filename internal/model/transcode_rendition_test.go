package model

import "testing"

func TestBuildStableRenditionKeyStableAcrossRetry(t *testing.T) {
	key1 := BuildStableRenditionKey(1001, "1080p", 1920, 1080, 5000, 128, "h264", "aac")
	key2 := BuildStableRenditionKey(1001, "1080p", 1920, 1080, 5000, 128, "h264", "aac")
	if key1 == "" {
		t.Fatal("rendition key should not be empty")
	}
	if key1 != key2 {
		t.Fatalf("stable rendition key mismatch: %s != %s", key1, key2)
	}
}

func TestBuildStableRenditionKeyChangesBetweenRenditions(t *testing.T) {
	key1080p := BuildStableRenditionKey(1001, "1080p", 1920, 1080, 5000, 128, "h264", "aac")
	key720p := BuildStableRenditionKey(1001, "720p", 1280, 720, 2800, 128, "h264", "aac")
	if key1080p == key720p {
		t.Fatalf("different renditions should not share the same stable key: %s", key1080p)
	}
}
