# Development Guide

This guide explains how to get the SSH blog running locally for development.

## Prerequisites

- **Go 1.21+** — [Install Go](https://go.dev/doc/install)
- **Git** — For cloning the repository

Verify Go is installed:

```bash
go version
```

## Setup

### 1. Clone the repository

```bash
git clone https://github.com/mharrisb1/sshd.git
cd sshd
```

### 2. Install dependencies

```bash
go mod download
```

### 3. Verify the build

```bash
go build ./...
```

## Running Locally

### Start the SSH server

```bash
go run ./cmd/server
```

You should see:

```
Loaded 3 posts
Starting SSH server on 0.0.0.0:2223
Connect with: ssh -p 2223 localhost
```

### Connect via SSH

In a separate terminal:

```bash
ssh -p 2223 localhost
```

**Note:** On first connection, you may be asked to accept the host key. Type `yes` to continue.

## Running the TUI directly (without SSH)

For faster iteration during UI development:

```bash
go run ./cmd/dev
```

This runs the TUI directly in your terminal without the SSH layer.

## Project Structure

```
sshd/
├── cmd/
│   ├── dev/           # Local TUI runner (no SSH)
│   │   └── main.go
│   └── server/        # SSH server entry point
│       └── main.go
├── content/
│   └── posts/         # Blog posts (markdown with YAML frontmatter)
│       └── *.md
├── internal/
│   ├── posts/         # Post loading and parsing
│   │   ├── post.go
│   │   ├── loader.go
│   │   └── loader_test.go
│   ├── server/        # SSH server configuration
│   │   └── ssh.go
│   └── ui/            # Bubbletea TUI
│       └── app.go
└── go.mod
```

## Adding Blog Posts

Create a new markdown file in `content/posts/` with the naming convention:

```
YYYY-MM-DD_slug.md
```

Example: `2025-01-15_my-new-post.md`

### Frontmatter format

```yaml
---
title: "My New Post"
date: 2025-01-15
tags: [go, ssh, tutorial]
summary: "A brief description of the post"
draft: false
---
# Post content here

Your markdown content...
```

**Note:** Posts with `draft: true` are excluded from the list.

## Running Tests

```bash
go test ./...
```

## Exposing via ngrok (for testing remotely)

To test from another machine or share a preview:

```bash
# Terminal 1: Start the server
go run ./cmd/server

# Terminal 2: Start ngrok tunnel
ngrok tcp 2223
```

Then connect using the ngrok URL:

```bash
ssh -p <PORT> <SUBDOMAIN>.tcp.ngrok.io
```

## Configuration

### Changing the port

Edit `cmd/server/main.go`:

```go
port := 2223  // Change this value
```

### Adjusting content width

Edit `internal/ui/app.go`:

- `WithWordWrap(120)` — Markdown text width
- `contentWidth := 120` — Divider line width

## Troubleshooting

### "Address already in use"

Another process is using the port. Either:

- Kill the existing process: `pkill -f "cmd/server"`
- Or change the port in `cmd/server/main.go`

### SSH connection refused

Make sure the server is running and check the port number matches.

### Host key verification failed

Clear the old host key:

```bash
ssh-keygen -R "[localhost]:2223"
```
