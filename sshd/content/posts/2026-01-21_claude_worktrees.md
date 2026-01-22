---
title: "Claude Code worktrees"
date: 2026-01-21
tags: [llm, claude, git, worktree, bash]
summary: "A simple script to bootstrap Claude Code agent deployment for a given Linear ticket"
draft: false
---

We're playing around internally with how to better use Claude Code to power through a lot of the backlog of medium and below priority tickets in Linear. I'm still not comfortable giving Claude Code full reign and I do not want it touching high impact tickets or greenfield features, but I've found it useful for the giant backlog of low priority, low impact tickets (see my related [impact-effort analysis framework](https://github.com/mharrisb1/iea)).

Here is a script I wrote today that:

- [x] Fetches a Linear issue given an identifier using the [GraphQL API](https://linear.app/developers/graphql)
- [x] Uses those details to create a [git worktree](https://git-scm.com/docs/git-worktree)
- [x] Navigates to that worktree and installs dependencies (this is just for the frontend so `pnpm i` is sufficient)
- [x] Creates new Claude Code interactive session with Linear issue details as the intial prompt

```bash
#!/bin/bash

set -e

# Get script directory for reliable sourcing
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/include/logging.sh"

issue_identifier=""
branch_name=""
directory=""
prompt=""

print_help() {
  echo " "
  echo "Create a new worktree and Claude Code agent session for a Linear issue"
  echo " "
  echo "Usage: ./scripts/worktree.sh -i <issue-id> [-p <prompt>] [-b <branch>] [-d <directory>]"
  echo " "
  echo "Options:"
  echo " "
  echo "  -i, --issue        Linear issue identifier (e.g. DEF-1234) [required]"
  echo "  -b, --branch       Branch name override. Defaults to Linear's suggested branch name"
  echo "  -d, --directory    Directory override. Defaults to ../wax-<issue-id>"
  echo "  -p, --prompt       Additional prompting for the Claude agent"
  echo " "
  echo "  -h, --help         Show this help message"
  echo "  -q, --quiet        Hide info and debug level logs"
  echo "  -v, --verbose      Include debug level logs"
  echo " "
  echo "Environment:"
  echo "  LINEAR_API_KEY     Required. Your Linear API key"
  echo " "
  echo "Examples:"
  echo "  ./scripts/worktree.sh -i DEF-1234"
  echo "  ./scripts/worktree.sh -i DEF-1234 -p 'Focus on the frontend components'"
  echo "  ./scripts/worktree.sh -i DEF-1234 -b my-custom-branch -d ~/projects/wax-feature"
  echo " "
}

log_set_level "INFO"

while test $# -gt 0; do
  case "$1" in
  -h | --help)
    print_help
    exit 0
    ;;
  -v | --verbose)
    log_set_level "DEBUG"
    shift
    ;;
  -q | --quiet)
    log_set_level "WARN"
    shift
    ;;
  -i | --issue)
    issue_identifier="$2"
    shift 2
    ;;
  -b | --branch)
    branch_name="$2"
    shift 2
    ;;
  -d | --directory)
    directory="$2"
    shift 2
    ;;
  -p | --prompt)
    prompt="$2"
    shift 2
    ;;
  *)
    log_error "Unknown option: $1"
    print_help
    exit 1
    ;;
  esac
done

# ============================================
# Validation
# ============================================
if [ -z "$issue_identifier" ]; then
  log_error "Issue identifier is required. Use -i or --issue"
  print_help
  exit 1
fi

if [ -z "$LINEAR_API_KEY" ]; then
  log_error "LINEAR_API_KEY environment variable is not set"
  exit 1
fi

# Check for required tools
for cmd in curl jq git claude; do
  if ! command -v "$cmd" &> /dev/null; then
    log_error "Required command not found: $cmd"
    exit 1
  fi
done

# ============================================
# Fetch Linear Issue Data
# ============================================
log_info "Fetching issue data for ${issue_identifier}..."

linear_response=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: $LINEAR_API_KEY" \
  --data "{
    \"query\": \"query (\$id: String!) { issue(id: \$id) { id identifier branchName title description } }\",
    \"variables\": { \"id\": \"$issue_identifier\" }
  }" \
  https://api.linear.app/graphql)

# Parse the response
issue_uuid=$(echo "$linear_response" | jq -r '.data.issue.id')
identifier=$(echo "$linear_response" | jq -r '.data.issue.identifier')
title=$(echo "$linear_response" | jq -r '.data.issue.title')
description=$(echo "$linear_response" | jq -r '.data.issue.description // empty')

# Use Linear's branch name if not overridden
if [ -z "$branch_name" ]; then
  branch_name=$(echo "$linear_response" | jq -r '.data.issue.branchName')
fi

# Validate the response
if [ "$issue_uuid" = "null" ] || [ -z "$issue_uuid" ]; then
  log_error "Could not fetch issue data. Response:"
  echo "$linear_response" | jq .
  exit 1
fi

log_info "Found: ${identifier} - ${title}"
log_debug "Branch: ${branch_name}"

# ============================================
# Create Worktree
# ============================================
if [ -z "$directory" ]; then
  current_dir=$(basename "$(pwd)")
  parent_dir=$(dirname "$(pwd)")
  identifier_lower=$(echo "$identifier" | tr '[:upper:]' '[:lower:]')
  directory="${parent_dir}/${current_dir}-${identifier_lower}"
fi

log_info "Creating worktree at ${directory}..."

if git worktree list | grep -q "$directory"; then
  log_warn "Worktree already exists at ${directory}"
else
  if git show-ref --verify --quiet "refs/heads/${branch_name}"; then
    log_info "Branch ${branch_name} exists, checking out"
    git worktree add "$directory" "$branch_name"
  else
    log_info "Creating new branch: ${branch_name}"
    git worktree add -b "$branch_name" "$directory"
  fi
fi

# ============================================
# Mark Issue as In Progress
# ============================================
log_info "Marking issue as In Progress..."

IN_PROGRESS_STATE_ID="6f864b19-71da-45e9-bbe1-b2d693516c15"

update_response=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: $LINEAR_API_KEY" \
  --data "{
    \"query\": \"mutation (\$id: String!, \$stateId: String!) { issueUpdate(id: \$id, input: { stateId: \$stateId }) { success } }\",
    \"variables\": { \"id\": \"$issue_uuid\", \"stateId\": \"$IN_PROGRESS_STATE_ID\" }
  }" \
  https://api.linear.app/graphql)

update_success=$(echo "$update_response" | jq -r '.data.issueUpdate.success')
if [ "$update_success" = "true" ]; then
  log_info "Issue marked as In Progress"
else
  log_warn "Could not update issue status"
  log_debug "$update_response"
fi

# ============================================
# Build Prompt and Start Claude
# ============================================
claude_prompt="Linear Issue: ${identifier} - ${title}

${description}"

if [ -n "$prompt" ]; then
  claude_prompt="${claude_prompt}

Additional instructions: ${prompt}"
fi

# ============================================
# Install Dependencies
# ============================================
log_info "Installing dependencies in ${directory}..."
cd "$directory" || exit 1
pnpm i

# ============================================
# Start Claude Code
# ============================================
log_info "Starting Claude Code..."
log_debug "Prompt: ${claude_prompt}"

exec claude "$claude_prompt"
```

I'll iterate on this for a bit and maybe move into another programming language but this is a good start.

I also played around with parallelization having multiple worktrees + agents going on at once using this script. Seems to work fine but we need a way to have separate local deployments with some URL scheme that would play nicely with CORS. Otherwise, they agents can implement separately but I'm still testing one at a time.
