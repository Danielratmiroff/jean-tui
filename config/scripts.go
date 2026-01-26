package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ScriptConfig represents the jean.json configuration file
type ScriptConfig struct {
	Scripts   map[string]string `json:"scripts"`
	CopyPaths []string          `json:"copyPaths"` // Paths to copy from base repo to worktrees
}

// LoadScripts loads the jean.json file from a repository path
// Returns an empty ScriptConfig if the file doesn't exist
func LoadScripts(repoPath string) (*ScriptConfig, error) {
	configPath := filepath.Join(repoPath, "jean.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		// If file doesn't exist, return empty config (not an error)
		if os.IsNotExist(err) {
			return &ScriptConfig{
				Scripts: make(map[string]string),
			}, nil
		}
		return nil, err
	}

	var config ScriptConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config.Scripts == nil {
		config.Scripts = make(map[string]string)
	}

	return &config, nil
}

// GetScript returns the command for a named script
func (s *ScriptConfig) GetScript(name string) string {
	if s == nil || s.Scripts == nil {
		return ""
	}
	return s.Scripts[name]
}

// GetScriptNames returns a sorted list of script names
func (s *ScriptConfig) GetScriptNames() []string {
	if s == nil || s.Scripts == nil {
		return []string{}
	}

	names := make([]string, 0, len(s.Scripts))
	for name := range s.Scripts {
		names = append(names, name)
	}
	return names
}

// HasScripts returns true if there are any scripts configured
func (s *ScriptConfig) HasScripts() bool {
	if s == nil || s.Scripts == nil {
		return false
	}
	return len(s.Scripts) > 0
}

// GetCopyPaths returns the paths to copy from base repo to worktrees
// Defaults to [".claude"] if not configured (for Claude Code settings)
func (s *ScriptConfig) GetCopyPaths() []string {
	if s == nil || len(s.CopyPaths) == 0 {
		// Default: copy .claude directory (for Claude Code settings)
		return []string{".claude"}
	}
	return s.CopyPaths
}
