package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coollabsio/gcool/install"
	"github.com/coollabsio/gcool/tui"
)

const version = "0.1.0"

func main() {
	// Check if the first argument is a subcommand
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			handleInit()
			return
		case "version":
			fmt.Printf("gcool version %s\n", version)
			os.Exit(0)
		case "help":
			printHelp()
			os.Exit(0)
		}
	}

	// Parse flags
	pathFlag := flag.String("path", ".", "Path to git repository (default: current directory)")
	noClaudeFlag := flag.Bool("no-claude", false, "Don't auto-start Claude CLI in tmux session")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	helpFlag := flag.Bool("help", false, "Show help")

	flag.Parse()

	// Handle flags
	if *versionFlag {
		fmt.Printf("gcool version %s\n", version)
		os.Exit(0)
	}

	if *helpFlag {
		printHelp()
		os.Exit(0)
	}

	// Get repo path and auto-claude setting
	repoPath := *pathFlag
	autoClaude := !*noClaudeFlag

	// Create and run TUI
	model := tui.NewModel(repoPath, autoClaude)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	// Check if we need to switch directories
	if m, ok := finalModel.(tui.Model); ok {
		switchInfo := m.GetSwitchInfo()
		if switchInfo.Path != "" {
			// Format: path|branch|auto-claude|terminal-only
			autoCl := "false"
			if switchInfo.AutoClaude {
				autoCl = "true"
			}
			termOnly := "false"
			if switchInfo.TerminalOnly {
				termOnly = "true"
			}
			switchData := fmt.Sprintf("%s|%s|%s|%s", switchInfo.Path, switchInfo.Branch, autoCl, termOnly)

			// Check if we should write to a file (for shell wrapper integration)
			if switchFile := os.Getenv("GCOOL_SWITCH_FILE"); switchFile != "" {
				// Write to file for shell wrapper
				if err := os.WriteFile(switchFile, []byte(switchData), 0600); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not write switch file: %v\n", err)
				}
			} else {
				// Print to stdout (legacy behavior)
				fmt.Println(switchData)
			}
		}
	}
}

func handleInit() {
	// Parse init subcommand flags
	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	updateFlag := initCmd.Bool("update", false, "Update existing gcool integration")
	removeFlag := initCmd.Bool("remove", false, "Remove gcool integration")
	dryRunFlag := initCmd.Bool("dry-run", false, "Show what would be done without making changes")
	shellFlag := initCmd.String("shell", "", "Specify shell (bash, zsh, fish). Auto-detected if not specified")

	initCmd.Parse(os.Args[2:])

	detector, err := install.NewDetector()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Override shell if specified
	if *shellFlag != "" {
		switch *shellFlag {
		case "bash":
			detector.Shell = install.Bash
			detector.RCFile = install.GetRCFileForShell(install.Bash, detector.HomeDir)
		case "zsh":
			detector.Shell = install.Zsh
			detector.RCFile = install.GetRCFileForShell(install.Zsh, detector.HomeDir)
		case "fish":
			detector.Shell = install.Fish
			detector.RCFile = install.GetRCFileForShell(install.Fish, detector.HomeDir)
		default:
			fmt.Fprintf(os.Stderr, "Error: unknown shell '%s'\n", *shellFlag)
			os.Exit(1)
		}
	}

	fmt.Printf("Detected shell: %s\n", detector.Shell)
	fmt.Printf("RC file: %s\n", detector.RCFile)

	var err2 error
	if *removeFlag {
		err2 = detector.Remove(*dryRunFlag)
	} else if *updateFlag {
		err2 = detector.Update(*dryRunFlag)
	} else {
		err2 = detector.Install(*dryRunFlag)
	}

	if err2 != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err2)
		os.Exit(1)
	}
}

// GetRCFileForShell is exported from install package wrapper
func getRCFileForShell(shell install.Shell, homeDir string) string {
	switch shell {
	case install.Zsh:
		return homeDir + "/.zshrc"
	case install.Fish:
		return homeDir + "/.config/fish/config.fish"
	case install.Bash:
		fallthrough
	default:
		return homeDir + "/.bashrc"
	}
}

func printHelp() {
	fmt.Printf(`gcool - A Cool TUI for Git Worktrees & Running CLI-Based AI Assistants Simultaneously v%s

A beautiful terminal user interface for managing Git worktrees with integrated tmux
session management, letting you run multiple Claude CLI sessions across different
branches effortlessly.

USAGE:
    gcool [OPTIONS]
    gcool init [FLAGS]

COMMANDS:
    init            Install or manage gcool shell integration
    help            Show this help message
    version         Print version and exit

MAIN OPTIONS:
    -path <path>    Path to git repository (default: current directory)
    -no-claude      Don't auto-start Claude CLI in tmux session
    -help           Show this help message
    -version        Print version and exit

INIT COMMAND FLAGS:
    -update         Update existing gcool integration
    -remove         Remove gcool integration
    -dry-run        Show what would be done without making changes
    -shell <shell>  Specify shell (bash, zsh, fish). Auto-detected if not specified

KEYBINDINGS:
    Navigation:
        ↑/k         Move up
        ↓/j         Move down

    Actions:
        Enter       Switch to selected worktree
        n           Create new worktree with new branch
        a           Create worktree from existing branch
        d           Delete selected worktree
        r           Refresh worktree list
        q/Ctrl+C    Quit

    Modal Navigation:
        Tab         Cycle through inputs/buttons
        Enter       Confirm action
        Esc         Cancel/close modal

SHELL INTEGRATION SETUP:
    One-time setup to enable directory switching:

        gcool init

    This will auto-detect your shell and install the necessary wrapper.
    After installation, restart your terminal or run: source ~/.bashrc (or ~/.zshrc, etc.)

    To update an existing installation:
        gcool init --update

    To remove the integration:
        gcool init --remove

EXAMPLES:
    # Run in current directory
    gcool

    # Run for a specific repository
    gcool -path /path/to/repo

    # Set up shell integration (one-time)
    gcool init

    # Update shell integration
    gcool init --update

    # Remove shell integration
    gcool init --remove

For more information, visit: https://github.com/coollabsio/gcool
`, version)
}

// This is a wrapper to make GetRCFile accessible from main package
// The actual implementation is in install/install.go
