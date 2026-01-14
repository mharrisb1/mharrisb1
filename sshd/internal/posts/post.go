package posts

import (
	"strings"
	"time"
)

type Post struct {
	// Frontmatter
	Title   string    `yaml:"title"`
	Date    time.Time `yaml:"date"`
	Tags    []string  `yaml:"tags"`
	Summary string    `yaml:"summary"`
	Draft   bool      `yaml:"draft"`

	// Derived
	Slug    string
	Content string
}

func (p Post) ReadTime() int {
	words := len(strings.Fields(p.Content))
	return (words + 199) / 200
}

func (p Post) FormattedDate() string {
	return p.Date.Format("Jan 02, 2006")
}

func (p Post) TagString() string {
	return strings.Join(p.Tags, ", ")
}
