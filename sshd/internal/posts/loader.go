package posts

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Page represents a standalone content page (e.g., about.md)
type Page struct {
	Title   string `yaml:"title"`
	Content string
}

// LoadPage reads a markdown file with optional YAML frontmatter
func LoadPage(path string) (Page, error) {
	var page Page
	data, err := os.ReadFile(path)
	if err != nil {
		return page, fmt.Errorf("reading %s: %w", path, err)
	}

	parts := bytes.SplitN(data, []byte("---"), 3)
	if len(parts) < 3 {
		// No frontmatter — treat entire file as content
		page.Content = strings.TrimSpace(string(data))
		return page, nil
	}

	page.Content = strings.TrimSpace(string(parts[2]))
	if err := yaml.Unmarshal(parts[1], &page); err != nil {
		return page, fmt.Errorf("parsing %s: %w", path, err)
	}

	return page, nil
}

func LoadPosts(dir string) ([]Post, error) {
	posts := []Post{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return posts, fmt.Errorf("reading %s: %w", dir, err)
	}

	var lastErr error
	for _, path := range entries {
		if path.IsDir() || filepath.Ext(path.Name()) != ".md" {
			continue
		}
		post, err := parsePost(filepath.Join(dir, path.Name()))
		if err != nil {
			lastErr = err
			continue
		}
		if !post.Draft {
			posts = append(posts, post)
		}
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	return posts, lastErr
}

func parsePost(path string) (Post, error) {
	post := Post{}
	data, err := os.ReadFile(path)
	if err != nil {
		return post, fmt.Errorf("reading %s: %w", path, err)
	}

	parts := bytes.SplitN(data, []byte("---"), 3)
	if len(parts) < 3 {
		return post, fmt.Errorf("invalid post: %s", path)
	}

	post.Content = strings.TrimSpace(string(parts[2]))

	err = yaml.Unmarshal(parts[1], &post)
	if err != nil {
		return post, fmt.Errorf("parsing %s: %w", path, err)
	}

	name := strings.TrimSuffix(filepath.Base(path), ".md")
	if parts := strings.SplitN(name, "_", 2); len(parts) == 2 {
		post.Slug = parts[1]
	}

	return post, nil
}
