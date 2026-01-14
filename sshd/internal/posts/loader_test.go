package posts

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helper: creates a markdown post file in the given directory
func createTestPost(t *testing.T, dir, filename, content string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}
}

func TestParsePost(t *testing.T) {
	// Arrange: create a temp directory and test post
	dir := t.TempDir()
	content := `---
title: "Test Post"
date: 2025-06-15
tags: [go, testing]
summary: "A test post"
draft: false
---

# Hello

This is test content.`

	createTestPost(t, dir, "2025-06-15_test-post.md", content)

	// Act: parse the post
	post, err := parsePost(filepath.Join(dir, "2025-06-15_test-post.md"))

	// Assert: verify the result
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if post.Title != "Test Post" {
		t.Errorf("expected title 'Test Post', got '%s'", post.Title)
	}
	if post.Slug != "test-post" {
		t.Errorf("expected slug 'test-post', got '%s'", post.Slug)
	}
	if len(post.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(post.Tags))
	}
	expectedDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	if !post.Date.Equal(expectedDate) {
		t.Errorf("expected date %v, got %v", expectedDate, post.Date)
	}
}
