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

// Tab represents the active top-level tab
type Tab int

const (
	TabAbout Tab = iota
	TabPosts
)

// Model holds all application state
type Model struct {
	posts     []posts.Post
	mode      ViewMode
	activeTab Tab
	cursor    int

	// Reader view
	viewport    viewport.Model
	currentPost *posts.Post

	// About view
	aboutContent  string
	aboutViewport viewport.Model
	aboutReady    bool

	// Search view
	searchInput   textinput.Model
	searchResults []posts.Post
	searchCursor  int

	// Terminal dimensions
	width  int
	height int
}

// NewModel creates a new Model with the given posts and about page content
func NewModel(p []posts.Post, aboutContent string) Model {
	// Create search input
	ti := textinput.New()
	ti.CharLimit = 50

	return Model{
		posts:        p,
		mode:         ViewModeList,
		activeTab:    TabAbout,
		cursor:       0,
		aboutContent: aboutContent,
		width:        80,
		height:       24,
		searchInput:  ti,
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
		// Update viewport sizes when window resizes
		m.viewport.Width = m.contentWidth(maxReaderWidth)
		m.viewport.Height = m.viewportHeight(7)
		m.aboutViewport.Width = m.contentWidth(maxListWidth)
		m.aboutViewport.Height = m.viewportHeight(6)
	}

	// Route based on active tab
	if m.activeTab == TabAbout && m.mode == ViewModeList {
		return m.updateAbout(msg)
	}

	// Posts tab: route by view mode
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

// updateAbout handles input in the About tab
func (m Model) updateAbout(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Initialize viewport on first display
	if !m.aboutReady {
		m.aboutViewport = viewport.New(m.contentWidth(maxListWidth), m.viewportHeight(6))
		content, err := renderMarkdown(m.aboutContent, m.contentWidth(maxListWidth))
		if err != nil {
			content = m.aboutContent
		}
		m.aboutViewport.SetContent(content)
		m.aboutReady = true
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "right", "2":
			m.activeTab = TabPosts
			return m, nil
		}
	}

	m.aboutViewport, cmd = m.aboutViewport.Update(msg)
	return m, cmd
}

// updateList handles input in list view
func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit

		case "left", "1":
			m.activeTab = TabAbout
			return m, nil

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
			content, err := renderMarkdown(m.currentPost.Content, m.contentWidth(maxReaderWidth))
			if err != nil {
				content = m.currentPost.Content // fallback to raw content
			}
			m.viewport = viewport.New(m.contentWidth(maxReaderWidth), m.viewportHeight(7))
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

				content, err := renderMarkdown(m.currentPost.Content, m.contentWidth(maxReaderWidth))
				if err != nil {
					content = m.currentPost.Content
				}
				m.viewport = viewport.New(m.contentWidth(maxReaderWidth), m.viewportHeight(7))
				m.viewport.SetContent(content)
			}
			return m, nil
		}
	}

	// Update text input and filter results
	prev := m.searchInput.Value()
	m.searchInput, cmd = m.searchInput.Update(msg)
	if m.searchInput.Value() != prev {
		m.searchResults = m.filterPosts(m.searchInput.Value())
		m.searchCursor = 0
	}

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

// renderMarkdown converts markdown to styled terminal output at the given wrap width
func renderMarkdown(content string, wrapWidth int) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(wrapWidth),
	)
	if err != nil {
		return content, err
	}
	return renderer.Render(content)
}

// Layout constants
const (
	maxListWidth   = 81
	maxReaderWidth = 120
	maxHeight      = 48
	boxHOverhead   = 6 // border(2) + padding(4)
	boxVOverhead   = 4 // border(2) + padding(2)
)

// contentWidth returns the usable content width inside the box, capped at max
func (m Model) contentWidth(max int) int {
	w := m.width - boxHOverhead
	if w > max {
		w = max
	}
	if w < 20 {
		w = 20
	}
	return w
}

// boxHeight returns the effective terminal height, capped at maxHeight
func (m Model) boxHeight() int {
	h := m.height
	if h > maxHeight {
		h = maxHeight
	}
	return h
}

// viewportHeight returns the available height for a viewport given lines used by header/footer
func (m Model) viewportHeight(nonViewportLines int) int {
	h := m.boxHeight() - boxVOverhead - nonViewportLines
	if h < 1 {
		h = 1
	}
	return h
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

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("242"))
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

// renderTabBar renders the tab bar with the active tab highlighted
func (m Model) renderTabBar() string {
	about := "[1: About]"
	posts := "[2: Posts]"
	if m.activeTab == TabAbout {
		about = activeTabStyle.Render(about)
		posts = inactiveTabStyle.Render(posts)
	} else {
		about = inactiveTabStyle.Render(about)
		posts = activeTabStyle.Render(posts)
	}
	return about + "  " + posts
}

// View implements tea.Model
func (m Model) View() string {
	if m.activeTab == TabAbout && m.mode == ViewModeList {
		return m.viewAbout()
	}

	switch m.mode {
	case ViewModeReader:
		return m.viewReader()
	case ViewModeSearch:
		return m.viewSearch()
	default:
		return m.viewList()
	}
}

// viewAbout renders the about page
func (m Model) viewAbout() string {
	var content strings.Builder
	contentWidth := m.contentWidth(maxListWidth)

	// Header: tab bar + divider
	content.WriteString(m.renderTabBar() + "\n")
	content.WriteString(divider(contentWidth) + "\n\n")

	// About content in viewport
	content.WriteString(m.aboutViewport.View())
	content.WriteString("\n\n")

	// Footer
	content.WriteString(divider(contentWidth) + "\n")
	scrollPercent := fmt.Sprintf("%3.f%%", m.aboutViewport.ScrollPercent()*100)
	content.WriteString(footerStyle.Render("↑/↓: scroll  →/2: posts  q: quit  " + scrollPercent))

	box := boxStyle.Render(content.String())

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}

// viewList renders the post list view
func (m Model) viewList() string {
	var content strings.Builder
	contentWidth := m.contentWidth(maxListWidth)

	// Calculate available height for posts
	// Box has: border (2) + padding (2) = 4 lines overhead
	// Header: title (1) + divider (1) + blank (1) = 3 lines
	// Footer: divider (1) + footer text (1) = 2 lines
	// Each post: title (1) + meta (1) + summary (1) + blank (1) = 4 lines
	boxOverhead := 4
	headerLines := 3
	footerLines := 2
	linesPerPost := 4

	availableHeight := m.boxHeight() - boxOverhead - headerLines - footerLines
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
	content.WriteString(m.renderTabBar() + "\n")
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
	content.WriteString(footerStyle.Render("↑/↓: navigate  enter: read  /: search  ←/1: about  q: quit"))

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
	contentWidth := m.contentWidth(maxReaderWidth)

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
	contentWidth := m.contentWidth(maxListWidth)

	// Calculate available height for search results
	// Box has: border (2) + padding (2) = 4 lines overhead
	// Header: search input (1) + divider (1) + blank (1) = 3 lines
	// Footer: divider (1) + footer text (1) = 2 lines
	// Each result: title (1) + meta (1) + blank (1) = 3 lines
	boxOverhead := 4
	headerLines := 3
	footerLines := 2
	linesPerResult := 3

	availableHeight := m.boxHeight() - boxOverhead - headerLines - footerLines
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
