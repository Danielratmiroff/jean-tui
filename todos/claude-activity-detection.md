# Claude Activity Detection

## Overview

This document outlines the research and implementation plan for detecting when Claude CLI sessions are actively thinking/working vs idle in gcool.

## Problem Statement

We want to show visual indicators in the gcool TUI to let users know when their Claude sessions are:
- **Actively thinking/processing** - Working on a task
- **Idle** - Waiting for user input
- **Not running** - No active session

## Research Findings

### Claude CLI Behavior

**Process Characteristics:**
- Runs as a Node.js process
- Process name appears as `claude` in process list
- No PID files, lock files, or status files created
- No socket/IPC files for external communication

**Directory Structure:**
- Session UUIDs stored in `~/.claude/session-env/` (but directories are empty)
- Projects stored in `~/.claude/projects/` with path-based naming
- History tracked in `~/.claude/history.jsonl`

**Hooks System (Key Discovery):**
Claude CLI has a hooks system in `~/.claude/settings.json` supporting lifecycle events:
- `"Stop"` hook - Fires when Claude finishes processing/thinking
- `"Notification"` hook - Fires when Claude is waiting for user input
- `"Start"` hook - Fires when Claude begins processing
- Hooks can execute arbitrary shell commands

### Detection Approaches Evaluated

#### 1. Hook-Based Detection ✅ RECOMMENDED
**Mechanism:** Extend Claude CLI's hook system in `~/.claude/settings.json`

**How it works:**
- Add hooks that write status to files when Claude state changes
- gcool reads these status files to determine current state

**Pros:**
- Clean, event-driven architecture
- Accurate state detection
- Low overhead (event-driven, not polling-heavy)

**Cons:**
- Requires modifying user's Claude settings
- Need to manage hook installation/removal

**Implementation:**
```json
{
  "hooks": {
    "Start": "echo 'thinking' > /tmp/gcool-claude-$CLAUDE_SESSION_ID.state",
    "Stop": "echo 'idle' > /tmp/gcool-claude-$CLAUDE_SESSION_ID.state",
    "Notification": "echo 'idle' > /tmp/gcool-claude-$CLAUDE_SESSION_ID.state"
  }
}
```

#### 2. tmux Pane Capture + Pattern Matching
**Mechanism:** Use `tmux capture-pane -p` to read terminal output

**How it works:**
- Periodically capture pane content
- Look for Claude CLI prompt patterns (thinking indicators, cursor prompts)

**Pros:**
- No Claude config modification needed
- Direct observation of terminal state

**Cons:**
- Fragile (depends on output format)
- CPU-intensive polling
- May miss rapid state changes
- Sensitive to Claude CLI UI changes

**Implementation:**
```bash
tmux capture-pane -t gcool-<branch> -p | tail -20
# Look for patterns like "Thinking...", spinners, or idle prompts
```

#### 3. Process CPU Monitoring
**Mechanism:** Monitor Claude process CPU usage via `ps`

**How it works:**
```bash
tmux list-panes -t gcool-<branch> -F "#{pane_pid}"
ps -p <PID> -o %cpu
```

**Pros:**
- Simple implementation
- No Claude config modification

**Cons:**
- Unreliable (network I/O doesn't spike CPU)
- Cannot distinguish idle vs waiting on network
- Requires PID tracking

#### 4. tmux Pane Command Detection
**Mechanism:** Use `tmux list-panes -F "#{pane_current_command}"`

**How it works:**
- Check if command is "claude" vs "zsh"/"bash"

**Pros:**
- Simple and reliable for running vs not running
- Already available via existing tmux commands

**Cons:**
- Cannot distinguish thinking vs idle (both show "claude")
- Binary state only (running or not)

**Implementation:**
```bash
tmux list-panes -t gcool-<branch> -F "#{pane_current_command}"
# Returns: "claude" if running, "zsh"/"bash" if not
```

## Recommended Implementation

### Architecture: Hybrid Hook + Pane Detection

**Primary Method:** Hook-based detection (opt-in via settings)
- Add gcool-specific hooks to Claude settings
- Write state to `/tmp/gcool-claude-<session-name>.state`
- States: `thinking`, `idle`, `stopped`
- Timestamp each state change for staleness detection

**Fallback Method:** Basic pane command detection
- Use `tmux list-panes` to detect if Claude process is running
- Binary state: running or not running

### Implementation Plan

#### 1. Create Claude State Management Module

**File:** `session/claude.go` (NEW)

**Types:**
```go
type ClaudeState int

const (
    ClaudeNotRunning ClaudeState = iota
    ClaudeIdle
    ClaudeThinking
    ClaudeUnknown
)

type ClaudeManager struct {
    configPath string // ~/.claude/settings.json
}
```

**Methods:**
```go
// GetClaudeState reads state from /tmp/gcool-claude-<session>.state
func (m *ClaudeManager) GetClaudeState(sessionName string) (ClaudeState, error)

// InstallClaudeHooks adds hooks to ~/.claude/settings.json
func (m *ClaudeManager) InstallClaudeHooks() error

// UninstallClaudeHooks removes gcool hooks
func (m *ClaudeManager) UninstallClaudeHooks() error

// HasClaudeHooks checks if hooks are installed
func (m *ClaudeManager) HasClaudeHooks() (bool, error)

// UpdateClaudeHooks updates existing hooks (safe update)
func (m *ClaudeManager) UpdateClaudeHooks() error
```

**Hook Installation Strategy:**
- Use unique markers (similar to tmux config approach)
- Safe concurrent modification of settings.json
- Preserve existing hooks
- Add comments for identification:
  ```json
  {
    "hooks": {
      "# gcool-activity-detection-start": "",
      "Start": "echo \"thinking:$(date +%s)\" > /tmp/gcool-claude-${GCOOL_SESSION_NAME}.state",
      "Stop": "echo \"idle:$(date +%s)\" > /tmp/gcool-claude-${GCOOL_SESSION_NAME}.state",
      "Notification": "echo \"idle:$(date +%s)\" > /tmp/gcool-claude-${GCOOL_SESSION_NAME}.state",
      "# gcool-activity-detection-end": ""
    }
  }
  ```

#### 2. TUI Integration

**File:** `tui/model.go`

**Model Changes:**
```go
type Model struct {
    // ... existing fields
    claudeStates      map[string]ClaudeState  // session name -> state
    claudeManager     *claude.ClaudeManager
    lastClaudeCheck   time.Time
    claudeCheckInterval time.Duration  // default: 2 seconds
}
```

**New Message Types:**
```go
type claudeStateCheckedMsg struct {
    states map[string]ClaudeState
    err    error
}

type claudeTickMsg time.Time
```

**New Commands:**
```go
// scheduleClaudeCheck returns a command that checks Claude states periodically
func scheduleClaudeCheck(interval time.Duration) tea.Cmd {
    return tea.Every(interval, func(t time.Time) tea.Msg {
        return claudeTickMsg(t)
    })
}

// checkClaudeStates checks state of all Claude sessions
func (m Model) checkClaudeStates() tea.Cmd {
    return func() tea.Msg {
        states := make(map[string]ClaudeState)
        for _, wt := range m.worktrees {
            sessionName := session.SanitizeSessionName(wt.Branch)
            state, err := m.claudeManager.GetClaudeState(sessionName)
            if err != nil {
                states[sessionName] = ClaudeUnknown
            } else {
                states[sessionName] = state
            }
        }
        return claudeStateCheckedMsg{states: states, err: nil}
    }
}
```

**File:** `tui/update.go`

**Update Handler:**
```go
case claudeTickMsg:
    // Only check if enough time has passed
    if time.Since(m.lastClaudeCheck) >= m.claudeCheckInterval {
        m.lastClaudeCheck = time.Now()
        return m, m.checkClaudeStates()
    }
    return m, nil

case claudeStateCheckedMsg:
    if msg.err == nil {
        m.claudeStates = msg.states
    }
    return m, scheduleClaudeCheck(m.claudeCheckInterval)
```

**Initialize in Init():**
```go
func (m Model) Init() tea.Cmd {
    return tea.Batch(
        m.loadWorktrees(),
        scheduleClaudeCheck(m.claudeCheckInterval),
    )
}
```

#### 3. Visual Indicators

**File:** `tui/view.go`

**Worktree List Rendering:**
```go
func (m Model) renderMainView() string {
    // ... existing code
    for i, wt := range m.worktrees {
        // Get Claude state
        sessionName := session.SanitizeSessionName(wt.Branch)
        claudeState := m.claudeStates[sessionName]

        // Add state indicator
        stateIcon := m.getClaudeStateIcon(claudeState)

        line := fmt.Sprintf("%s %s %s", cursor, stateIcon, wt.Branch)
        // ... rest of rendering
    }
}

func (m Model) getClaudeStateIcon(state ClaudeState) string {
    switch state {
    case ClaudeThinking:
        return m.styles.thinkingIcon.Render("⚡") // or animated spinner
    case ClaudeIdle:
        return m.styles.idleIcon.Render("💤")
    case ClaudeNotRunning:
        return "  " // no icon
    default:
        return m.styles.unknownIcon.Render("?")
    }
}
```

**Details Panel:**
```go
func (m Model) renderDetailsPanel(wt *git.Worktree) string {
    // ... existing details

    sessionName := session.SanitizeSessionName(wt.Branch)
    claudeState := m.claudeStates[sessionName]

    var stateText string
    switch claudeState {
    case ClaudeThinking:
        stateText = m.styles.thinkingColor.Render("Claude is thinking...")
    case ClaudeIdle:
        stateText = m.styles.idleColor.Render("Claude is idle")
    case ClaudeNotRunning:
        stateText = m.styles.dimmed.Render("No Claude session")
    default:
        stateText = m.styles.dimmed.Render("Claude state unknown")
    }

    details += fmt.Sprintf("\nClaude: %s\n", stateText)
    // ... rest of details
}
```

**File:** `tui/styles.go`

**New Styles:**
```go
type Styles struct {
    // ... existing styles
    thinkingIcon   lipgloss.Style
    idleIcon       lipgloss.Style
    unknownIcon    lipgloss.Style
    thinkingColor  lipgloss.Style
    idleColor      lipgloss.Style
}

func DefaultStyles() Styles {
    return Styles{
        // ... existing styles
        thinkingIcon:  lipgloss.NewStyle().Foreground(lipgloss.Color("226")), // Yellow
        idleIcon:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")), // Dim gray
        unknownIcon:   lipgloss.NewStyle().Foreground(lipgloss.Color("243")), // Medium gray
        thinkingColor: lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true),
        idleColor:     lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
    }
}
```

#### 4. Settings Integration

**File:** `tui/update.go` (Settings Modal)

**Add New Setting:**
```go
const (
    settingEditor = iota
    settingBaseBranch
    settingTmuxConfig
    settingClaudeDetection  // NEW
)

func (m Model) handleSettingsModalInput(msg tea.KeyMsg) (Model, tea.Cmd) {
    switch msg.String() {
    case "down", "j":
        if m.settingsCursor < 3 { // Update max cursor
            m.settingsCursor++
        }
    // ... rest of navigation

    case "enter":
        switch m.settingsCursor {
        // ... existing cases
        case settingClaudeDetection:
            m.modal = claudeDetectionModal
        }
    }
}
```

**New Modal Type:**
```go
const (
    // ... existing modals
    claudeDetectionModal
)
```

**Modal Handler:**
```go
func (m Model) handleClaudeDetectionModalInput(msg tea.KeyMsg) (Model, tea.Cmd) {
    switch msg.String() {
    case "esc":
        m.modal = settingsModal
        return m, nil

    case "i": // Install hooks
        return m, m.installClaudeHooks()

    case "u": // Update hooks
        return m, m.updateClaudeHooks()

    case "r": // Remove hooks
        return m, m.removeClaudeHooks()
    }
    return m, nil
}
```

**Commands:**
```go
type claudeHooksInstalledMsg struct{ err error }
type claudeHooksUpdatedMsg struct{ err error }
type claudeHooksRemovedMsg struct{ err error }

func (m Model) installClaudeHooks() tea.Cmd {
    return func() tea.Msg {
        err := m.claudeManager.InstallClaudeHooks()
        return claudeHooksInstalledMsg{err: err}
    }
}

func (m Model) updateClaudeHooks() tea.Cmd {
    return func() tea.Msg {
        err := m.claudeManager.UpdateClaudeHooks()
        return claudeHooksUpdatedMsg{err: err}
    }
}

func (m Model) removeClaudeHooks() tea.Cmd {
    return func() tea.Msg {
        err := m.claudeManager.UninstallClaudeHooks()
        return claudeHooksRemovedMsg{err: err}
    }
}
```

**File:** `tui/view.go` (Settings Modal)

**Render Settings Menu:**
```go
func (m Model) renderSettingsModal() string {
    // ... existing settings
    options := []string{
        "Select Default Editor",
        "Change Base Branch",
        "Tmux Configuration",
        "Claude Activity Detection",  // NEW
    }
    // ... render with cursor
}
```

**Render Claude Detection Modal:**
```go
func (m Model) renderClaudeDetectionModal() string {
    hasHooks, _ := m.claudeManager.HasClaudeHooks()

    var status string
    if hasHooks {
        status = m.styles.successColor.Render("✓ Hooks installed")
    } else {
        status = m.styles.dimmed.Render("✗ Hooks not installed")
    }

    content := lipgloss.JoinVertical(lipgloss.Left,
        m.styles.modalTitle.Render("Claude Activity Detection"),
        "",
        status,
        "",
        "This feature adds hooks to ~/.claude/settings.json",
        "to detect when Claude is thinking or idle.",
        "",
        m.styles.dimmed.Render("Actions:"),
        "  i - Install hooks",
        "  u - Update hooks",
        "  r - Remove hooks",
        "  esc - Back",
    )

    return m.styles.modal.Render(content)
}
```

## State File Format

**Location:** `/tmp/gcool-claude-<session-name>.state`

**Format:** `<state>:<timestamp>`

**Examples:**
```
thinking:1698765432
idle:1698765450
```

**Staleness Detection:**
- If timestamp is > 30 seconds old, consider state `Unknown`
- Indicates hooks may not be working or session crashed

## Testing Strategy

### Manual Testing

1. **Install Hooks:**
   - Open gcool settings (press `s`)
   - Select "Claude Activity Detection"
   - Press `i` to install hooks
   - Verify hooks appear in `~/.claude/settings.json`

2. **Test Thinking State:**
   - Start Claude session in a worktree (press `enter`)
   - Give Claude a task (e.g., "implement a new feature")
   - Switch back to gcool TUI
   - Verify `⚡` icon appears next to the worktree

3. **Test Idle State:**
   - Wait for Claude to finish the task
   - Verify icon changes to `💤`

4. **Test Session Management:**
   - Detach from Claude session (`Ctrl+B`, `D`)
   - Verify state persists in TUI
   - Reattach to session
   - Verify state updates correctly

5. **Test Multiple Sessions:**
   - Start Claude in multiple worktrees
   - Verify each shows correct independent state

6. **Test Hook Removal:**
   - Remove hooks via settings
   - Verify hooks removed from `~/.claude/settings.json`
   - Verify no gcool markers left behind

### Automated Testing

**Unit Tests:**
```go
// session/claude_test.go
func TestGetClaudeState(t *testing.T)
func TestInstallClaudeHooks(t *testing.T)
func TestUninstallClaudeHooks(t *testing.T)
func TestHasClaudeHooks(t *testing.T)
```

**Integration Tests:**
- Create test Claude sessions
- Write state files manually
- Verify state detection
- Test hook installation/removal with temp config files

## Edge Cases & Considerations

### Multiple gcool Instances
- State files are session-specific (by name)
- Each instance reads its own worktrees' states
- No conflicts expected

### Hook Conflicts
- Check for existing hooks before installation
- Preserve user's existing hooks
- Use clear markers for gcool-managed sections
- Provide merge/update strategy

### Session Name Sanitization
- Must use same sanitization as tmux session names
- Ensure consistency: `session.SanitizeSessionName(branch)`

### State File Cleanup
- State files are in `/tmp`, cleaned on reboot
- Consider adding cleanup command in settings
- Clean up on worktree deletion

### Performance
- Check interval: 2 seconds (configurable)
- Only check worktrees that have sessions
- File reads are fast (< 1ms per file)
- No significant performance impact expected

### Hook Environment Variables
- Need to pass session name to hooks
- May need to set `GCOOL_SESSION_NAME` env var when starting Claude
- Alternative: Include session name in state file path based on PID

## Future Enhancements

### Animated Indicators
- Use Bubble Tea's tick mechanism for spinner animation
- Rotate through spinner frames: `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`

### State History
- Track state changes over time
- Show "Claude has been thinking for 5 minutes"
- Helpful for long-running tasks

### Notifications
- Desktop notifications when Claude finishes thinking
- Integrate with system notification system

### Dashboard View
- Overview of all Claude sessions and their states
- Quick navigation to thinking/idle sessions

### Metrics
- Track thinking time per worktree
- Identify slow operations
- Export metrics for analysis

## Files to Create/Modify

### New Files
- `session/claude.go` - Claude detection logic
- `session/claude_test.go` - Tests
- `docs/claude-activity-detection.md` - This document

### Modified Files
- `tui/model.go` - Add state tracking
- `tui/update.go` - Add periodic checks and handlers
- `tui/view.go` - Add visual indicators
- `tui/styles.go` - Add colors for states

## References

- Claude CLI hooks documentation (if available)
- gcool tmux config implementation (similar pattern)
- Bubble Tea tick/timer examples
- State file patterns in Unix/Linux
