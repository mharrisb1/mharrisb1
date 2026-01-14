package server

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/mharrisb1/sshd/internal/posts"
	"github.com/mharrisb1/sshd/internal/ui"
)

// NewSSHServer creates a new SSH server configured to serve the blog TUI
// No auth handlers = NoClientAuth is automatically enabled (public access)
func NewSSHServer(loadedPosts []posts.Post, host string, port int) (*ssh.Server, error) {
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", host, port)),
		wish.WithHostKeyPath(".ssh/blog_host_key"),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler(loadedPosts)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating SSH server: %w", err)
	}

	return s, nil
}

// teaHandler returns a function that creates a new TUI for each SSH session
func teaHandler(loadedPosts []posts.Post) func(ssh.Session) (tea.Model, []tea.ProgramOption) {
	return func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		// Get terminal size from SSH session
		pty, _, _ := s.Pty()

		// Create a fresh model for this session
		m := ui.NewModel(loadedPosts)

		// Update model with actual terminal dimensions
		m = m.WithDimensions(pty.Window.Width, pty.Window.Height)

		return m, []tea.ProgramOption{
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(), // Enable mouse support
		}
	}
}
