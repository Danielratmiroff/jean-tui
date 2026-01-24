package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Client wraps the Claude CLI for AI operations
type Client struct{}

// PRContent represents the JSON structure for PR title and description
type PRContent struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// ClaudeMessage represents a single message in the Claude CLI JSON array output
type ClaudeMessage struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Result  string `json:"result"`
	Message struct {
		Model   string `json:"model"`
		ID      string `json:"id"`
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	} `json:"message"`
}

// NewClient creates a new Claude CLI client
// Claude CLI uses OAuth authentication from CLAUDE_CODE_OAUTH_TOKEN environment variable
// The model is determined by the Claude CLI configuration
func NewClient() *Client {
	return &Client{}
}

// GenerateCommitMessage generates a one-line conventional commit message based on git context
// If customPrompt is empty, uses the default prompt
func (c *Client) GenerateCommitMessage(status, diff, branch, log, customPrompt string) (subject string, err error) {
	// Limit diff to reasonable size to avoid token limits
	if len(diff) > 5000 {
		diff = diff[:5000]
	}

	// Use custom prompt if provided, otherwise use default
	prompt := customPrompt
	if prompt == "" {
		prompt = DefaultCommitPrompt
	}

	// Replace all placeholders with actual context
	prompt = strings.ReplaceAll(prompt, "{status}", status)
	prompt = strings.ReplaceAll(prompt, "{diff}", diff)
	prompt = strings.ReplaceAll(prompt, "{branch}", branch)
	prompt = strings.ReplaceAll(prompt, "{log}", log)

	response, err := c.callAPI(prompt)
	if err != nil {
		return "", err
	}

	// Parse plain text response (no JSON)
	subject = strings.TrimSpace(response)
	if subject == "" {
		return "", fmt.Errorf("AI generated empty commit subject")
	}

	return subject, nil
}

// GenerateBranchName generates a semantic branch name based on git diff
// If customPrompt is empty, uses the default prompt
func (c *Client) GenerateBranchName(diff, customPrompt string) (string, error) {
	// Limit diff to reasonable size
	if len(diff) > 3000 {
		diff = diff[:3000]
	}

	// Use custom prompt if provided, otherwise use default
	prompt := customPrompt
	if prompt == "" {
		prompt = DefaultBranchNamePrompt
	}
	// Replace {diff} placeholder with actual diff
	prompt = strings.ReplaceAll(prompt, "{diff}", diff)

	name, err := c.callAPI(prompt)
	if err != nil {
		return "", err
	}

	// Clean up response
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove non-alphanumeric except hyphens
	var result []rune
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result = append(result, r)
		}
	}
	name = string(result)

	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Limit to 40 chars
	if len(name) > 40 {
		name = name[:40]
	}

	if name == "" {
		return "", fmt.Errorf("AI generated invalid branch name")
	}

	return name, nil
}

// GeneratePRContent generates a PR title and description from a git diff
// If customPrompt is empty, uses the default prompt
func (c *Client) GeneratePRContent(diff, customPrompt string) (title, description string, err error) {
	// Limit diff to reasonable size
	if len(diff) > 5000 {
		diff = diff[:5000]
	}

	// Use custom prompt if provided, otherwise use default
	prompt := customPrompt
	if prompt == "" {
		prompt = DefaultPRPrompt
	}
	// Replace {diff} placeholder with actual diff
	prompt = strings.ReplaceAll(prompt, "{diff}", diff)

	response, err := c.callAPI(prompt)
	if err != nil {
		return "", "", err
	}

	// Parse JSON response
	var content PRContent
	if err := json.Unmarshal([]byte(response), &content); err != nil {
		return "", "", fmt.Errorf("failed to parse AI response: %w", err)
	}

	// Validate and clean title
	content.Title = strings.TrimSpace(content.Title)
	if content.Title == "" {
		return "", "", fmt.Errorf("AI generated empty PR title")
	}

	// Clean description (optional)
	content.Description = strings.TrimSpace(content.Description)

	return content.Title, content.Description, nil
}

// TestConnection tests the API key by making a simple request
func (c *Client) TestConnection() error {
	_, err := c.callAPI("Say 'test' and nothing else.")
	return err
}

// DebugMode enables verbose logging to /tmp/jean-claude-debug.log
var DebugMode = true

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func debugLog(format string, args ...interface{}) {
	if !DebugMode {
		return
	}
	f, err := os.OpenFile("/tmp/jean-claude-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, format+"\n", args...)
}

// callAPI makes a request to Claude using the Claude CLI headless mode
func (c *Client) callAPI(prompt string) (string, error) {
	// Build command: claude -p "prompt" --output-format json
	cmd := exec.Command("claude", "-p", prompt, "--output-format", "json")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	debugLog("=== CLAUDE CLI REQUEST ===")
	debugLog("Prompt: %s", prompt[:minInt(500, len(prompt))])

	if err := cmd.Run(); err != nil {
		debugLog("ERROR: %v", err)
		debugLog("STDERR: %s", stderr.String())
		return "", fmt.Errorf("claude CLI failed: %w: %s", err, stderr.String())
	}

	debugLog("=== CLAUDE CLI RAW RESPONSE ===")
	debugLog("STDOUT: %s", stdout.String())
	debugLog("STDERR: %s", stderr.String())

	// Parse the output - could be JSON array or JSONL
	content, err := c.parseClaudeOutput(stdout.String())
	if err != nil {
		return "", err
	}

	debugLog("=== EXTRACTED CONTENT ===")
	debugLog("Content: %s", content)

	if content == "" {
		return "", fmt.Errorf("no content in Claude CLI response")
	}

	// Clean up markdown code blocks if present
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		// Remove opening ``` with optional language specifier
		if idx := strings.Index(content, "\n"); idx != -1 {
			content = content[idx+1:]
		} else {
			content = strings.TrimPrefix(content, "```json")
			content = strings.TrimPrefix(content, "```")
		}
		// Remove closing ```
		if strings.HasSuffix(content, "```") {
			content = strings.TrimSuffix(content, "```")
		}
		content = strings.TrimSpace(content)
	}

	return content, nil
}

// parseClaudeOutput extracts content from Claude CLI JSON array output
func (c *Client) parseClaudeOutput(output string) (string, error) {
	output = strings.TrimSpace(output)

	// Parse as array of ClaudeMessage structs
	var messages []ClaudeMessage
	if err := json.Unmarshal([]byte(output), &messages); err != nil {
		debugLog("Failed to parse JSON array: %v", err)
		return "", fmt.Errorf("failed to parse Claude CLI output: %w", err)
	}

	debugLog("Parsed %d messages from JSON array", len(messages))

	// Process each message to find content
	for i, msg := range messages {
		debugLog("Message %d: type=%s, subtype=%s", i, msg.Type, msg.Subtype)

		// Prefer "result" type which contains the final output
		if msg.Type == "result" && msg.Result != "" {
			debugLog("Message %d: found result: %s", i, msg.Result[:minInt(100, len(msg.Result))])
			return msg.Result, nil
		}

		// Fallback to "assistant" type with message content
		if msg.Type == "assistant" && len(msg.Message.Content) > 0 {
			for _, block := range msg.Message.Content {
				if block.Type == "text" && block.Text != "" {
					debugLog("Message %d: found assistant text: %s", i, block.Text[:minInt(100, len(block.Text))])
					return block.Text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no content found in %d messages", len(messages))
}
