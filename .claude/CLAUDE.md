# Project Memory: mharrisb1.dev SSH Blog

## Project Overview

Personal website/blog for Michael Harris accessible via SSH (`ssh mharrisb1.dev`). Users connect via terminal and browse blog posts in an interactive TUI.

**Owner:** Michael Harris (mharrisb1)

## Tech Stack

- **Go 1.25.5** - Main language
- **Charmbracelet Wish** - SSH server framework
- **Bubbletea** - Terminal UI framework (Elm architecture)
- **Glamour** - Markdown rendering in terminal
- **Goldmark** - Markdown parsing
- **Bubbles** - UI components (viewport, textinput)
- **Lipgloss** - Terminal styling
- **go-yaml/v3** - YAML frontmatter parsing

### Content Format

Blog posts in `content/posts/` with naming: `YYYY-MM-DD_slug.md`

Example frontmatter:

```yaml
---
title: "Post Title"
date: 2025-12-31
tags: [meta, golang]
summary: "Brief description"
draft: false
---
Post content in markdown...
```

## Key Design Decisions

1. **Single Model with ViewMode enum** - Simpler state management vs multiple models
2. **Load posts once at startup** - Pass to each SSH session via middleware
3. **Port 22** (standard SSH) - Use systemd capabilities for non-root access
4. **Public access** - No authentication, like a public website
5. **Restart for new posts** - Simple deployment model

## Target Deployment

- **Platform:** GCP VM
- **DNS:** mharrisb1.dev → VM IP
- **Port:** 22 (standard SSH)
- **Service:** systemd with CAP_NET_BIND_SERVICE
- **Access:** `ssh mharrisb1.dev`
