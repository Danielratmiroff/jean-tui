# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is jean?

A TUI for managing Git worktrees with integrated wezterm tab management. Built in Go using Bubble Tea framework. Provides Claude CLI sessions per worktree via wezterm tabs.

## Development Commands

```bash
go run main.go                    # Run locally
go run main.go -path /some/repo   # Test with different repo
go build -o jean                  # Build binary
go mod tidy                       # Update dependencies
```

## Architecture

Bubble Tea MVC pattern with async operations via Tea commands:

- **tui/model.go** - State (`Model` struct), message types, Tea command functions
- **tui/update.go** - Event handling, keybindings in `handleMainInput()`, modal handlers
- **tui/view.go** - UI rendering, modal renderers, help bar
- **git/worktree.go** - `Manager` struct wrapping all git worktree operations
- **session/session.go** - Session name sanitization utilities
- **config/config.go** - Per-repo settings in `~/.config/jean/config.json`
- **github/pr.go** - PR operations via `gh` CLI

### Key Patterns

**Async flow**: Keybinding → Tea command → typed message (e.g., `worktreeCreatedMsg`) → Update handler → View render

**Modals**: Defined as `modalType` enum in model.go. Each modal has: state fields in `Model`, handler in update.go, renderer in view.go

**Shell integration**: App writes to `JEAN_SWITCH_FILE` env var; shell wrappers read it to perform `cd` and wezterm tab operations

## Adding Features

**New keybinding**: Add case in `handleMainInput()` (update.go), create message type + command func (model.go), handle message in `Update()`

**New modal**: Add `modalType` constant, state fields to `Model`, keybinding to open, handler function, renderer function

**New git operation**: Add method to `git.Manager`, wrap in Tea command, define result message type

## Release Process

1. Update `CliVersion` in `internal/version/version.go`
2. Commit: `git commit -m "chore: bump version to X.Y.Z"`
3. Create draft release: `gh release create vX.Y.Z --draft --title "vX.Y.Z" --notes "..."`

## Module

Import path: `github.com/coollabsio/jean-tui`
