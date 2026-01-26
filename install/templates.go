package install

// BashZshWrapper is the wrapper function for bash and zsh shells
const BashZshWrapper = `# BEGIN JEAN INTEGRATION
# jean - Git Worktree TUI Manager shell wrapper
# Source this in your shell rc file to enable jean with directory switching

jean() {
    local debug_log="/tmp/jean-wrapper-debug.log"
    local debug_enabled=false

    # Check if debug logging is enabled in config
    if [ -f "$HOME/.config/jean/config.json" ]; then
        if grep -q '"debug_logging_enabled"\s*:\s*true' "$HOME/.config/jean/config.json"; then
            debug_enabled=true
        fi
    fi

    if [ "$debug_enabled" = "true" ]; then
        echo "DEBUG wrapper: jean function called with args: $@" >> "$debug_log"
    fi

    # Loop until user explicitly quits jean
    while true; do
        # Save current PATH to restore it later
        local saved_path="$PATH"

        # Create a temp file for communication
        local temp_file=$(mktemp)

        # Set environment variable so jean knows to write to file
        JEAN_SWITCH_FILE="$temp_file" command jean "$@"
        local exit_code=$?

        # Restore PATH if it got corrupted
        if [ -z "$PATH" ] || [ "$PATH" != "$saved_path" ]; then
            export PATH="$saved_path"
        fi

        # Check if switch info was written
        if [ -f "$temp_file" ] && [ -s "$temp_file" ]; then
            if [ "$debug_enabled" = "true" ]; then
                echo "DEBUG wrapper: switch file exists and has content" >> "$debug_log"
            fi
            # Read the switch info: path|branch|auto-claude|target-window|script-command|claude-session-name|is-claude-initialized
            local switch_info=$(cat "$temp_file")
            if [ "$debug_enabled" = "true" ]; then
                echo "DEBUG wrapper: switch_info=$switch_info" >> "$debug_log"
            fi
            # Only remove if it's in /tmp (safety check)
            if [[ "$temp_file" == /tmp/* ]] || [[ "$temp_file" == /var/folders/* ]]; then
                rm "$temp_file"
            fi

            # Parse the info (using worktree_path instead of path to avoid PATH conflict)
            IFS='|' read -r worktree_path branch auto_claude target_window script_command claude_session_name is_claude_initialized <<< "$switch_info"

            # Check if we got valid data (has at least two pipes)
            if [[ "$switch_info" == *"|"*"|"* ]]; then
                # Check if inside wezterm and wezterm CLI is available
                if [ -n "$WEZTERM_PANE" ] && command -v wezterm >/dev/null 2>&1; then
                    if [ "$debug_enabled" = "true" ]; then
                        echo "DEBUG wrapper: Inside wezterm, target_window=$target_window" >> "$debug_log"
                    fi

                    if [ "$target_window" = "claude" ]; then
                        # Claude tab
                        if command -v claude >/dev/null 2>&1; then
                            if [ "$is_claude_initialized" = "true" ]; then
                                # Try with --continue first, fallback to fresh start if it fails
                                wezterm cli spawn --cwd "$worktree_path" -- bash -c "claude --add-dir \"$worktree_path\" --continue --permission-mode plan || claude --add-dir \"$worktree_path\" --permission-mode plan"
                            else
                                wezterm cli spawn --cwd "$worktree_path" -- claude --add-dir "$worktree_path" --permission-mode plan
                            fi
                        else
                            # Fallback to shell if claude not available
                            wezterm cli spawn --cwd "$worktree_path"
                        fi
                    else
                        # Terminal tab
                        wezterm cli spawn --cwd "$worktree_path"
                    fi
                    continue
                else
                    # Not in wezterm - cd and optionally run claude
                    cd "$worktree_path" || return
                    if [ "$debug_enabled" = "true" ]; then
                        echo "DEBUG wrapper: No wezterm, cd to $worktree_path, target=$target_window" >> "$debug_log"
                    fi

                    # Colors
                    local GREEN='\033[0;32m'
                    local CYAN='\033[0;36m'
                    local MAGENTA='\033[0;35m'
                    local YELLOW='\033[1;33m'
                    local BOLD='\033[1m'
                    local NC='\033[0m' # No Color

                    if [ "$target_window" = "claude" ]; then
                        if command -v claude >/dev/null 2>&1; then
                            echo ""
                            echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
                            echo -e "${BOLD}${CYAN}  🤖 Claude Session${NC}"
                            echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
                            echo -e "  ${GREEN}Branch:${NC}    ${BOLD}$branch${NC}"
                            echo -e "  ${GREEN}Path:${NC}      $worktree_path"
                            if [ "$is_claude_initialized" = "true" ]; then
                                echo -e "  ${GREEN}Status:${NC}    ${YELLOW}Resuming session...${NC}"
                            else
                                echo -e "  ${GREEN}Status:${NC}    ${CYAN}Starting new session...${NC}"
                            fi
                            echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
                            echo ""
                            if [ "$is_claude_initialized" = "true" ]; then
                                claude --add-dir "$worktree_path" --continue --permission-mode plan || claude --add-dir "$worktree_path" --permission-mode plan
                            else
                                claude --add-dir "$worktree_path" --permission-mode plan
                            fi
                        else
                            echo -e "${YELLOW}⚠ Claude not found, switched to: ${BOLD}$branch${NC}"
                        fi
                    else
                        echo ""
                        echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
                        echo -e "${BOLD}${CYAN}  📁 Terminal Session${NC}"
                        echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
                        echo -e "  ${GREEN}Branch:${NC}    ${BOLD}$branch${NC}"
                        echo -e "  ${GREEN}Path:${NC}      $worktree_path"
                        echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
                        echo ""
                    fi
                    return
                fi
            else
                return 1
            fi
        else
            # No switch file, user quit jean without selecting a worktree
            # Only remove if it's in /tmp (safety check)
            if [[ "$temp_file" == /tmp/* ]] || [[ "$temp_file" == /var/folders/* ]]; then
                rm -f "$temp_file"
            fi
            # Exit the loop
            return $exit_code
        fi
    done
}
# END JEAN INTEGRATION
`

// FishWrapper is the wrapper function for fish shell
const FishWrapper = `# BEGIN JEAN INTEGRATION
# jean - Git Worktree TUI Manager shell wrapper (Fish shell)
# Source this in your config.fish to enable jean with directory switching

function jean
    # Check if debug logging is enabled in config
    set debug_enabled false
    if test -f "$HOME/.config/jean/config.json"
        if grep -q '"debug_logging_enabled"\s*:\s*true' "$HOME/.config/jean/config.json"
            set debug_enabled true
        end
    end

    # Loop until user explicitly quits jean
    while true
        # Create a temp file for communication
        set temp_file (mktemp)

        # Set environment variable so jean knows to write to file
        set -x JEAN_SWITCH_FILE $temp_file
        command jean $argv
        set exit_code $status

        # Check if switch info was written
        if test -f "$temp_file" -a -s "$temp_file"
            # Read the switch info: path|branch|auto-claude|target-window|script-command|claude-session-name|is-claude-initialized
            set switch_info (cat $temp_file)
            rm $temp_file

            # Parse the info (using worktree_path instead of path to avoid PATH conflict)
            set parts (string split '|' $switch_info)

            # Check if we got valid data (has at least 3 parts)
            if test (count $parts) -ge 3
                set worktree_path $parts[1]
                set branch $parts[2]
                set auto_claude $parts[3]
                set target_window "terminal"
                if test (count $parts) -ge 4
                    set target_window $parts[4]
                end
                set is_claude_initialized "false"
                if test (count $parts) -ge 7
                    set is_claude_initialized $parts[7]
                end

                # Check if inside wezterm and wezterm CLI is available
                if test -n "$WEZTERM_PANE"; and command -v wezterm &> /dev/null
                    if test "$target_window" = "claude"
                        # Claude tab
                        if command -v claude &> /dev/null
                            if test "$is_claude_initialized" = "true"
                                # Try with --continue first, fallback to fresh start if it fails
                                wezterm cli spawn --cwd "$worktree_path" -- bash -c "claude --add-dir \"$worktree_path\" --continue --permission-mode plan || claude --add-dir \"$worktree_path\" --permission-mode plan"
                            else
                                wezterm cli spawn --cwd "$worktree_path" -- claude --add-dir "$worktree_path" --permission-mode plan
                            end
                        else
                            # Fallback to shell if claude not available
                            wezterm cli spawn --cwd "$worktree_path"
                        end
                    else
                        # Terminal tab
                        wezterm cli spawn --cwd "$worktree_path"
                    end
                    continue
                else
                    # Not in wezterm - cd and optionally run claude
                    cd $worktree_path

                    if test "$target_window" = "claude"
                        if command -v claude &> /dev/null
                            echo ""
                            set_color magenta; echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; set_color normal
                            set_color --bold cyan; echo "  🤖 Claude Session"; set_color normal
                            set_color magenta; echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; set_color normal
                            set_color green; echo -n "  Branch:    "; set_color --bold normal; echo "$branch"
                            set_color green; echo -n "  Path:      "; set_color normal; echo "$worktree_path"
                            if test "$is_claude_initialized" = "true"
                                set_color green; echo -n "  Status:    "; set_color yellow; echo "Resuming session..."; set_color normal
                            else
                                set_color green; echo -n "  Status:    "; set_color cyan; echo "Starting new session..."; set_color normal
                            end
                            set_color magenta; echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; set_color normal
                            echo ""
                            if test "$is_claude_initialized" = "true"
                                claude --add-dir "$worktree_path" --continue --permission-mode plan; or claude --add-dir "$worktree_path" --permission-mode plan
                            else
                                claude --add-dir "$worktree_path" --permission-mode plan
                            end
                        else
                            set_color yellow; echo "⚠ Claude not found, switched to: $branch"; set_color normal
                        end
                    else
                        echo ""
                        set_color green; echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; set_color normal
                        set_color --bold cyan; echo "  📁 Terminal Session"; set_color normal
                        set_color green; echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; set_color normal
                        set_color green; echo -n "  Branch:    "; set_color --bold normal; echo "$branch"
                        set_color green; echo -n "  Path:      "; set_color normal; echo "$worktree_path"
                        set_color green; echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; set_color normal
                        echo ""
                    end
                    return
                end
            end
        else
            # No switch file, just clean up
            rm -f $temp_file
            # Exit the loop
            return $exit_code
        end
    end
end
# END JEAN INTEGRATION
`
