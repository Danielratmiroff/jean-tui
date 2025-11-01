package install

// BashZshWrapper is the wrapper function for bash and zsh shells
const BashZshWrapper = `# BEGIN GCOOL INTEGRATION
# gcool - Git Worktree TUI Manager shell wrapper
# Source this in your shell rc file to enable gcool with directory switching

gcool() {
    # Loop until user explicitly quits gcool (not just detaches from tmux)
    while true; do
        # Save current PATH to restore it later
        local saved_path="$PATH"

        # Create a temp file for communication
        local temp_file=$(mktemp)

        # Set environment variable so gcool knows to write to file
        GCOOL_SWITCH_FILE="$temp_file" command gcool "$@"
        local exit_code=$?

        # Restore PATH if it got corrupted
        if [ -z "$PATH" ] || [ "$PATH" != "$saved_path" ]; then
            export PATH="$saved_path"
        fi

        # Check if switch info was written
        if [ -f "$temp_file" ] && [ -s "$temp_file" ]; then
        # Read the switch info: path|branch|auto-claude|terminal-only|script-command|claude-session-name
        local switch_info=$(cat "$temp_file")
        # Only remove if it's in /tmp (safety check)
        if [[ "$temp_file" == /tmp/* ]] || [[ "$temp_file" == /var/folders/* ]]; then
            rm "$temp_file"
        fi

        # Parse the info (using worktree_path instead of path to avoid PATH conflict)
        IFS='|' read -r worktree_path branch auto_claude terminal_only script_command claude_session_name <<< "$switch_info"

        # Check if we got valid data (has at least two pipes)
        if [[ "$switch_info" == *"|"*"|"* ]]; then
            # Check if tmux is available
            if ! command -v tmux >/dev/null 2>&1; then
                # No tmux, just cd
                cd "$worktree_path" || return
                echo "Switched to worktree: $branch (no tmux)"
                return
            fi

            # Sanitize branch name for tmux session
            local session_name="gcool-${branch//[^a-zA-Z0-9\-_]/-}"
            session_name="${session_name//--/-}"
            session_name="${session_name#-}"
            session_name="${session_name%-}"

            # If terminal-only, append -terminal suffix
            if [ "$terminal_only" = "true" ]; then
                session_name="${session_name}-terminal"
            fi

            # Check if already in a tmux session and if it's the same session we want
            if [ -n "$TMUX" ]; then
                # Get current tmux session name
                local current_session=$(tmux display-message -p '#S')
                if [ "$current_session" = "$session_name" ]; then
                    # Already in the correct session, just cd
                    cd "$worktree_path" || return
                    echo "Switched to worktree: $branch"
                    return
                fi
                # Different session - fall through to switch to it
            fi

            # Check if session exists (use exact matching with =)
            if tmux has-session -t "=$session_name" 2>/dev/null; then
                # Attach to existing session
                tmux attach-session -t "$session_name"
                # After detaching, loop back to gcool
                continue
            else
                # Create new session in detached mode first
                # Terminal-only sessions always use shell, never Claude
                if [ "$terminal_only" = "true" ]; then
                    # Always start with shell for terminal sessions
                    echo "DEBUG wrapper: Creating terminal-only session: $session_name" >&2
                    tmux new-session -d -s "$session_name" -c "$worktree_path"
                elif [ "$auto_claude" = "true" ]; then
                    # Check if claude is available
                    if command -v claude >/dev/null 2>&1; then
                        # Create detached session with claude in plan mode
                        # Use --session flag for persistent sessions (use claude_session_name if provided, otherwise branch name)
                        local claude_session="${claude_session_name:-$branch}"

                        echo "DEBUG wrapper: Creating Claude session with --session flag" >&2
                        echo "DEBUG wrapper: tmux_session_name=$session_name" >&2
                        echo "DEBUG wrapper: branch=$branch" >&2
                        echo "DEBUG wrapper: claude_session_name_provided=$claude_session_name" >&2
                        echo "DEBUG wrapper: claude_session (final)=$claude_session" >&2
                        echo "DEBUG wrapper: Command: tmux new-session -d -s '$session_name' -c '$worktree_path' claude --session '$claude_session' --permission-mode plan" >&2
                        tmux new-session -d -s "$session_name" -c "$worktree_path" claude --session "$claude_session" --permission-mode plan
                        sleep 1
                        echo "DEBUG wrapper: Checking process:" >&2
                        ps aux | grep -E "claude.*--session" | grep -v grep >&2
                    else
                        # Fallback: create detached session with shell and show message
                        echo "DEBUG wrapper: Claude not found, creating shell session" >&2
                        tmux new-session -d -s "$session_name" -c "$worktree_path" \; \
                            send-keys "echo 'Note: Claude CLI not found. Install it or use --no-claude flag.'" C-m \; \
                            send-keys "echo 'You are in: $worktree_path'" C-m
                    fi
                else
                    # Create detached session with shell
                    echo "DEBUG wrapper: Creating shell session: $session_name" >&2
                    tmux new-session -d -s "$session_name" -c "$worktree_path"
                fi

                # Now attach to the session
                tmux attach-session -t "$session_name"
                # After detaching from newly created session, loop back to gcool
                continue
            fi
        else
            return 1
        fi
        else
            # No switch file, user quit gcool without selecting a worktree
            # Only remove if it's in /tmp (safety check)
            if [[ "$temp_file" == /tmp/* ]] || [[ "$temp_file" == /var/folders/* ]]; then
                rm -f "$temp_file"
            fi
            # Exit the loop
            return $exit_code
        fi
    done
}
# END GCOOL INTEGRATION
`

// FishWrapper is the wrapper function for fish shell
const FishWrapper = `# BEGIN GCOOL INTEGRATION
# gcool - Git Worktree TUI Manager shell wrapper (Fish shell)
# Source this in your config.fish to enable gcool with directory switching

function gcool
    # Loop until user explicitly quits gcool (not just detaches from tmux)
    while true
        # Create a temp file for communication
        set temp_file (mktemp)

        # Set environment variable so gcool knows to write to file
        set -x GCOOL_SWITCH_FILE $temp_file
        command gcool $argv
        set exit_code $status

        # Check if switch info was written
        if test -f "$temp_file" -a -s "$temp_file"
            # Read the switch info: path|branch|auto-claude|terminal-only|script-command|claude-session-name
            set switch_info (cat $temp_file)
            rm $temp_file

            # Parse the info (using worktree_path instead of path to avoid PATH conflict)
            set parts (string split '|' $switch_info)

            # Check if we got valid data (has at least 3 parts)
            if test (count $parts) -ge 3
                set worktree_path $parts[1]
                set branch $parts[2]
                set auto_claude $parts[3]
                set terminal_only "false"
                if test (count $parts) -ge 4
                    set terminal_only $parts[4]
                end
                set claude_session_name ""
                if test (count $parts) -ge 6
                    set claude_session_name $parts[6]
                end

                # Check if tmux is available
                if not command -v tmux &> /dev/null
                    # No tmux, just cd
                    cd $worktree_path
                    echo "Switched to worktree: $branch (no tmux)"
                    return
                end

                # Sanitize branch name for tmux session
                set session_name "gcool-"(string replace -ra '[^a-zA-Z0-9\-_]' '-' $branch)
                set session_name (string replace -ra '--+' '-' $session_name)
                set session_name (string trim -c '-' $session_name)

                # If terminal-only, append -terminal suffix
                if test "$terminal_only" = "true"
                    set session_name "$session_name-terminal"
                end

                # Check if already in a tmux session
                if test -n "$TMUX"
                    # Already in tmux, just cd
                    cd $worktree_path
                    echo "Switched to worktree: $branch"
                    echo "Note: Already in tmux. Session: $session_name would be available outside tmux."
                    return
                end

                # Check if session exists (use exact matching with =)
                if tmux has-session -t "=$session_name" 2>/dev/null
                    # Attach to existing session
                    tmux attach-session -t "$session_name"
                    # After detaching, continue the loop to allow switching sessions
                    continue
                else
                    # Create new session
                    # Terminal-only sessions always use shell, never Claude
                    if test "$terminal_only" = "true"
                        # Always start with shell for terminal sessions
                        tmux new-session -s "$session_name" -c "$worktree_path"
                    else if test "$auto_claude" = "true"
                        # Check if claude is available
                        if command -v claude &> /dev/null
                            # Start with claude in plan mode
                            # Use --session flag for persistent sessions (use claude_session_name if provided, otherwise branch name)
                            set claude_session "$claude_session_name"
                            if test -z "$claude_session"
                                set claude_session "$branch"
                            end

                            echo "DEBUG wrapper: Creating Claude session with --session flag" >&2
                            echo "DEBUG wrapper: tmux_session_name=$session_name" >&2
                            echo "DEBUG wrapper: branch=$branch" >&2
                            echo "DEBUG wrapper: claude_session_name_provided=$claude_session_name" >&2
                            echo "DEBUG wrapper: claude_session (final)=$claude_session" >&2
                            echo "DEBUG wrapper: Command: tmux new-session -s '$session_name' -c '$worktree_path' claude --session '$claude_session' --permission-mode plan" >&2
                            tmux new-session -s "$session_name" -c "$worktree_path" claude --session "$claude_session" --permission-mode plan
                        else
                            # Fallback: start with shell and show message
                            tmux new-session -s "$session_name" -c "$worktree_path"
                        end
                    else
                        # Start with shell
                        tmux new-session -s "$session_name" -c "$worktree_path"
                    end
                    # After creating and attaching, continue loop to allow switching
                    continue
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
# END GCOOL INTEGRATION
`
