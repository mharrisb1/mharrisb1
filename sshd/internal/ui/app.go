package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mharrisb1/sshd/internal/posts"
)

// ViewMode represents the current view state
type ViewMode int

const (
	ViewModeList ViewMode = iota
	ViewModeReader
	ViewModeSearch
)

// Model holds all application state
type Model struct {
	posts  []posts.Post
	mode   ViewMode
	cursor int

	// Reader view
	viewport    viewport.Model
	currentPost *posts.Post

	// Search view
	searchInput   textinput.Model
	searchResults []posts.Post
	searchCursor  int

	// Markdown renderer (created once, reused)
	mdRenderer *glamour.TermRenderer

	// Terminal dimensions
	width  int
	height int
}

// NewModel creates a new Model with the given posts
func NewModel(p []posts.Post) Model {
	// Create markdown renderer once (expensive operation)
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(120),
	)

	// Create search input
	ti := textinput.New()
	ti.CharLimit = 50

	return Model{
		posts:       p,
		mode:        ViewModeList,
		cursor:      0,
		width:       80,
		height:      24,
		mdRenderer:  renderer,
		searchInput: ti,
	}
}

// WithDimensions returns a new Model with the given terminal dimensions
func (m Model) WithDimensions(width, height int) Model {
	m.width = width
	m.height = height
	return m
}

// Init implements bubbletea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle global keys and window resize
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update viewport size when window resizes
		m.viewport.Width = msg.Width - 10
		m.viewport.Height = msg.Height - 12
	}

	// Handle mode-specific updates
	switch m.mode {
	case ViewModeList:
		return m.updateList(msg)
	case ViewModeReader:
		return m.updateReader(msg)
	case ViewModeSearch:
		return m.updateSearch(msg)
	}

	return m, cmd
}

// updateList handles input in list view
func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.posts)-1 {
				m.cursor++
			}

		case "enter":
			// Open the selected post
			m.currentPost = &m.posts[m.cursor]
			m.mode = ViewModeReader

			// Render markdown and set viewport content
			content, err := m.renderMarkdown(m.currentPost.Content)
			if err != nil {
				content = m.currentPost.Content // fallback to raw content
			}
			m.viewport = viewport.New(m.width-10, m.height-12)
			m.viewport.SetContent(content)

		case "/":
			// Enter search mode
			m.mode = ViewModeSearch
			m.searchInput.Reset()
			m.searchInput.Focus()
			m.searchResults = m.posts // Start with all posts
			m.searchCursor = 0
			return m, textinput.Blink
		}
	}
	return m, nil
}

// updateReader handles input in reader view
func (m Model) updateReader(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			// Return to list view (q doesn't quit from reader)
			m.mode = ViewModeList
			m.currentPost = nil
			return m, nil
		}
	}

	// Pass other messages to viewport for scrolling
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// updateSearch handles input in search view
func (m Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Cancel search, return to list
			m.mode = ViewModeList
			m.searchInput.Reset()
			return m, nil

		case "up", "ctrl+p":
			if m.searchCursor > 0 {
				m.searchCursor--
			}
			return m, nil

		case "down", "ctrl+n":
			if m.searchCursor < len(m.searchResults)-1 {
				m.searchCursor++
			}
			return m, nil

		case "enter":
			// Open selected search result
			if len(m.searchResults) > 0 {
				m.currentPost = &m.searchResults[m.searchCursor]
				m.mode = ViewModeReader

				content, err := m.renderMarkdown(m.currentPost.Content)
				if err != nil {
					content = m.currentPost.Content
				}
				m.viewport = viewport.New(m.width-10, m.height-12)
				m.viewport.SetContent(content)
			}
			return m, nil
		}
	}

	// Update text input and filter results
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.searchResults = m.filterPosts(m.searchInput.Value())
	m.searchCursor = 0 // Reset cursor when results change

	return m, cmd
}

// filterPosts returns posts matching the search query
func (m Model) filterPosts(query string) []posts.Post {
	if query == "" {
		return m.posts
	}

	query = strings.ToLower(query)
	var results []posts.Post

	for _, post := range m.posts {
		// Search in title, summary, tags, and content
		if strings.Contains(strings.ToLower(post.Title), query) ||
			strings.Contains(strings.ToLower(post.Summary), query) ||
			strings.Contains(strings.ToLower(post.TagString()), query) ||
			strings.Contains(strings.ToLower(post.Content), query) {
			results = append(results, post)
		}
	}

	return results
}

// renderMarkdown converts markdown to styled terminal output
func (m Model) renderMarkdown(content string) (string, error) {
	if m.mdRenderer == nil {
		return content, nil // fallback to raw content
	}
	return m.mdRenderer.Render(content)
}

// Styles
var (
	accentColor = lipgloss.Color("212")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	selectedStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(1, 2)
)

// divider creates a horizontal line of the given width
func divider(width int) string {
	return dividerStyle.Render(strings.Repeat("─", width))
}

// View implements tea.Model
func (m Model) View() string {
	switch m.mode {
	case ViewModeReader:
		return m.viewReader()
	case ViewModeSearch:
		return m.viewSearch()
	default:
		return m.viewList()
	}
}

// viewList renders the post list view
func (m Model) viewList() string {
	var content strings.Builder
	contentWidth := 120

	// Header
	content.WriteString(titleStyle.Render("Michael Harris - Blog") + "\n")
	content.WriteString(divider(contentWidth) + "\n\n")

	// Posts
	for i, post := range m.posts {
		// Cursor indicator
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		// Title
		var title string
		if i == m.cursor {
			title = selectedStyle.Render(post.Title)
		} else {
			title = normalStyle.Render(post.Title)
		}

		content.WriteString(cursor + title + "\n")

		// Date and tags
		content.WriteString("  " + dimStyle.Render(post.FormattedDate()+" • "+post.TagString()) + "\n")

		// Summary
		content.WriteString("  " + dimStyle.Render(post.Summary) + "\n")

		content.WriteString("\n")
	}

	// Footer
	content.WriteString(divider(contentWidth) + "\n")
	content.WriteString(footerStyle.Render("↑/↓: navigate  enter: read  /: search  q: quit"))

	// Wrap content in a styled box
	box := boxStyle.Render(content.String())

	// Center the box in the terminal
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}

// viewReader renders the post reader view
func (m Model) viewReader() string {
	if m.currentPost == nil {
		return ""
	}

	var content strings.Builder
	contentWidth := 120

	// Header with post title
	content.WriteString(titleStyle.Render(m.currentPost.Title) + "\n")
	content.WriteString(dimStyle.Render(fmt.Sprintf("%s • %d min read",
		m.currentPost.FormattedDate(),
		m.currentPost.ReadTime())) + "\n")
	content.WriteString(divider(contentWidth) + "\n\n")

	// Post content (rendered markdown in viewport)
	content.WriteString(m.viewport.View())
	content.WriteString("\n\n")

	// Footer
	content.WriteString(divider(contentWidth) + "\n")
	scrollPercent := fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100)
	content.WriteString(footerStyle.Render("↑/↓: scroll  esc: back  q: quit  " + scrollPercent))

	// Wrap content in a styled box
	box := boxStyle.Render(content.String())

	// Center the box in the terminal
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}

// viewSearch renders the search view
func (m Model) viewSearch() string {
	var content strings.Builder
	contentWidth := 120

	// Search input header
	content.WriteString(titleStyle.Render("Search: ") + m.searchInput.View() + "\n")
	content.WriteString(divider(contentWidth) + "\n\n")

	// Search results
	if len(m.searchResults) == 0 {
		content.WriteString(dimStyle.Render("  No posts found") + "\n")
	} else {
		for i, post := range m.searchResults {
			// Cursor indicator
			cursor := "  "
			if i == m.searchCursor {
				cursor = "> "
			}

			// Title
			var title string
			if i == m.searchCursor {
				title = selectedStyle.Render(post.Title)
			} else {
				title = normalStyle.Render(post.Title)
			}

			content.WriteString(cursor + title + "\n")

			// Date and tags
			content.WriteString("  " + dimStyle.Render(post.FormattedDate()+" • "+post.TagString()) + "\n")

			content.WriteString("\n")
		}
	}

	// Footer
	content.WriteString(divider(contentWidth) + "\n")
	content.WriteString(footerStyle.Render("↑/↓: navigate  enter: read  esc: cancel"))

	// Wrap content in a styled box
	box := boxStyle.Render(content.String())

	// Center the box in the terminal
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}
