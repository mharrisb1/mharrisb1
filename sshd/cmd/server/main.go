package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mharrisb1/sshd/internal/posts"
	"github.com/mharrisb1/sshd/internal/server"
)

func main() {
	// Load posts from content directory
	loadedPosts, err := posts.LoadPosts("./content/posts")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading posts: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d posts\n", len(loadedPosts))

	// Create SSH server on port 2223 (use 22 in production)
	port := 2223
	host := "0.0.0.0"

	s, err := server.NewSSHServer(loadedPosts, host, port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	// Start server in background
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("Starting SSH server on %s:%d\n", host, port)
	fmt.Printf("Connect with: ssh -p %d localhost\n", port)

	go func() {
		if err := s.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	// Wait for interrupt signal
	<-done
	fmt.Println("\nShutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
	}

	fmt.Println("Server stopped")
}
