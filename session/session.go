package session

import (
	"regexp"
	"strings"
	"time"
)

const sessionPrefix = "jean-"

// Session represents a terminal session (kept for backwards compatibility)
type Session struct {
	Name         string
	Branch       string
	Path         string // Working directory of the session
	Active       bool
	Windows      int
	LastActivity time.Time
}

// Manager handles session operations
type Manager struct{}

// NewManager creates a new session manager
func NewManager() *Manager {
	return &Manager{}
}

// SanitizeBranchName sanitizes a branch name for use as a git branch (without prefix)
// This is useful when accepting user input for branch names
func (m *Manager) SanitizeBranchName(branch string) string {
	// Replace invalid characters with hyphens
	reg := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
	sanitized := reg.ReplaceAllString(branch, "-")

	// Remove consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	sanitized = reg.ReplaceAllString(sanitized, "-")

	// Trim hyphens from start/end
	sanitized = strings.Trim(sanitized, "-")

	return sanitized
}

// SanitizeName sanitizes a repo name and branch name for use as a session name
// Format: jean-<repo>-<branch>
func (m *Manager) SanitizeName(repoName, branch string) string {
	// Sanitize both repo name and branch name
	sanitizedRepo := m.SanitizeBranchName(repoName)
	sanitizedBranch := m.SanitizeBranchName(branch)

	// Combine with repo name for uniqueness across repositories
	if sanitizedRepo != "" {
		return sessionPrefix + sanitizedRepo + "-" + sanitizedBranch
	}
	// Fallback if repo name is empty (shouldn't happen)
	return sessionPrefix + sanitizedBranch
}

// List returns an empty list (sessions are now managed via wezterm tabs)
func (m *Manager) List(repoPath string) ([]Session, error) {
	return []Session{}, nil
}

// Kill is a no-op (sessions are now managed via wezterm tabs)
func (m *Manager) Kill(sessionName string) error {
	return nil
}

// RenameSession is a no-op (sessions are now managed via wezterm tabs)
func (m *Manager) RenameSession(oldName, newName string) error {
	return nil
}
