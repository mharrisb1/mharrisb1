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

// truncate shortens a string to maxLen, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
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
	contentWidth := 81

	// Calculate available height for posts
	// Box has: border (2) + padding (2) = 4 lines overhead
	// Header: title (1) + divider (1) + blank (1) = 3 lines
	// Footer: divider (1) + footer text (1) = 2 lines
	// Each post: title (1) + meta (1) + summary (1) + blank (1) = 4 lines
	boxOverhead := 4
	headerLines := 3
	footerLines := 2
	linesPerPost := 4

	availableHeight := m.height - boxOverhead - headerLines - footerLines
	if availableHeight < linesPerPost {
		availableHeight = linesPerPost // Show at least one post
	}

	maxVisiblePosts := availableHeight / linesPerPost
	if maxVisiblePosts < 1 {
		maxVisiblePosts = 1
	}

	// Calculate window of visible posts
	startIdx := 0
	endIdx := len(m.posts)

	if len(m.posts) > maxVisiblePosts {
		// Center the cursor in the visible window when possible
		halfWindow := maxVisiblePosts / 2
		startIdx = m.cursor - halfWindow
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxVisiblePosts
		if endIdx > len(m.posts) {
			endIdx = len(m.posts)
			startIdx = endIdx - maxVisiblePosts
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	// Header
	content.WriteString(titleStyle.Render("Michael Harris - Blog") + "\n")
	content.WriteString(divider(contentWidth) + "\n\n")

	// Show scroll indicator at top if there are hidden posts above
	if startIdx > 0 {
		content.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more above", startIdx)) + "\n\n")
	}

	// Posts (only visible window)
	// Max text width accounts for cursor/indent prefix (2 chars)
	maxTextWidth := contentWidth - 2

	for i := startIdx; i < endIdx; i++ {
		post := m.posts[i]

		// Cursor indicator
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		// Title (truncated)
		titleText := truncate(post.Title, maxTextWidth)
		var title string
		if i == m.cursor {
			title = selectedStyle.Render(titleText)
		} else {
			title = normalStyle.Render(titleText)
		}

		content.WriteString(cursor + title + "\n")

		// Date and tags (truncated)
		meta := truncate(post.FormattedDate()+" • "+post.TagString(), maxTextWidth)
		content.WriteString("  " + dimStyle.Render(meta) + "\n")

		// Summary (truncated)
		summary := truncate(post.Summary, maxTextWidth)
		content.WriteString("  " + dimStyle.Render(summary) + "\n")

		content.WriteString("\n")
	}

	// Show scroll indicator at bottom if there are hidden posts below
	if endIdx < len(m.posts) {
		content.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more below", len(m.posts)-endIdx)) + "\n")
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
	contentWidth := 81

	// Calculate available height for search results
	// Box has: border (2) + padding (2) = 4 lines overhead
	// Header: search input (1) + divider (1) + blank (1) = 3 lines
	// Footer: divider (1) + footer text (1) = 2 lines
	// Each result: title (1) + meta (1) + blank (1) = 3 lines
	boxOverhead := 4
	headerLines := 3
	footerLines := 2
	linesPerResult := 3

	availableHeight := m.height - boxOverhead - headerLines - footerLines
	if availableHeight < linesPerResult {
		availableHeight = linesPerResult
	}

	maxVisibleResults := availableHeight / linesPerResult
	if maxVisibleResults < 1 {
		maxVisibleResults = 1
	}

	// Calculate window of visible results
	startIdx := 0
	endIdx := len(m.searchResults)

	if len(m.searchResults) > maxVisibleResults {
		halfWindow := maxVisibleResults / 2
		startIdx = m.searchCursor - halfWindow
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxVisibleResults
		if endIdx > len(m.searchResults) {
			endIdx = len(m.searchResults)
			startIdx = endIdx - maxVisibleResults
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	// Search input header
	content.WriteString(titleStyle.Render("Search: ") + m.searchInput.View() + "\n")
	content.WriteString(divider(contentWidth) + "\n\n")

	// Search results
	// Max text width accounts for cursor/indent prefix (2 chars)
	maxTextWidth := contentWidth - 2

	if len(m.searchResults) == 0 {
		content.WriteString(dimStyle.Render("  No posts found") + "\n")
	} else {
		// Show scroll indicator at top if there are hidden results above
		if startIdx > 0 {
			content.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more above", startIdx)) + "\n\n")
		}

		for i := startIdx; i < endIdx; i++ {
			post := m.searchResults[i]

			// Cursor indicator
			cursor := "  "
			if i == m.searchCursor {
				cursor = "> "
			}

			// Title (truncated)
			titleText := truncate(post.Title, maxTextWidth)
			var title string
			if i == m.searchCursor {
				title = selectedStyle.Render(titleText)
			} else {
				title = normalStyle.Render(titleText)
			}

			content.WriteString(cursor + title + "\n")

			// Date and tags (truncated)
			meta := truncate(post.FormattedDate()+" • "+post.TagString(), maxTextWidth)
			content.WriteString("  " + dimStyle.Render(meta) + "\n")

			content.WriteString("\n")
		}

		// Show scroll indicator at bottom if there are hidden results below
		if endIdx < len(m.searchResults) {
			content.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more below", len(m.searchResults)-endIdx)) + "\n")
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
