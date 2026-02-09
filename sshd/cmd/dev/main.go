package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mharrisb1/sshd/internal/posts"
	"github.com/mharrisb1/sshd/internal/ui"
)

func main() {
	// Load posts from content directory
	loadedPosts, err := posts.LoadPosts("./content/posts")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading posts: %v\n", err)
		os.Exit(1)
	}

	if len(loadedPosts) == 0 {
		fmt.Println("No posts found in ./content/posts")
		os.Exit(1)
	}

	// Load about page
	aboutPage, err := posts.LoadPage("./content/about.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load about page: %v\n", err)
	}

	// Create and run the TUI
	p := tea.NewProgram(
		ui.NewModel(loadedPosts, aboutPage.Content),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
