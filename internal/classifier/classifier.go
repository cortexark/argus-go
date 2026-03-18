// Package classifier provides 6-signal AI process classification.
package classifier

import (
	"strings"

	"github.com/cortexark/argus/internal/aiapps"
)

// Confidence levels
const (
	ConfirmedAI = "CONFIRMED_AI" // score >= 50
	LikelyAI    = "LIKELY_AI"   // score 30-49
	NotAI       = "NOT_AI"      // score < 30
)

// Result holds the classification result for a process.
type Result struct {
	Confidence string
	Score      int
	App        *aiapps.App // non-nil if in known registry
	Signals    []string
}

// Classify scores a process using 6 signals.
func Classify(name, cmdline, ppidName string, localPorts []int, envVars []string) Result {
	score := 0
	var signals []string

	// Signal 1: known AI app registry (strongest signal)
	app, found := aiapps.Lookup(name)
	if found {
		score += 60
		signals = append(signals, "known-registry")
	}

	// Signal 2: keyword match in process name or cmdline
	if matchesKeywords(name, cmdline) {
		score += 20
		signals = append(signals, "keyword-match")
	}

	// Signal 3: AI API endpoint usage (network)
	// Checked externally by network monitor — passed in via localPorts heuristic
	// We check well-known AI ports: 11434 (Ollama), 1234 (LM Studio), 8080
	for _, p := range localPorts {
		if p == 11434 || p == 1234 || p == 8080 || p == 5001 {
			score += 15
			signals = append(signals, "ai-port")
			break
		}
	}

	// Signal 4: ancestry — spawned by known AI parent
	if ppidName != "" {
		if _, ok := aiapps.Lookup(ppidName); ok {
			score += 20
			signals = append(signals, "ai-parent")
		}
	}

	// Signal 5: environment variables common in AI tools
	for _, env := range envVars {
		upper := strings.ToUpper(env)
		if strings.HasPrefix(upper, "ANTHROPIC_") ||
			strings.HasPrefix(upper, "OPENAI_") ||
			strings.HasPrefix(upper, "GEMINI_") ||
			strings.HasPrefix(upper, "CLAUDE_") ||
			strings.HasPrefix(upper, "CURSOR_") ||
			strings.HasPrefix(upper, "COPILOT_") {
			score += 15
			signals = append(signals, "ai-env-var")
			break
		}
	}

	// Signal 6: pipe / stdin connected to LLM tools (heuristic via cmdline)
	if strings.Contains(cmdline, "llm") || strings.Contains(cmdline, "openai") ||
		strings.Contains(cmdline, "anthropic") || strings.Contains(cmdline, "langchain") {
		score += 10
		signals = append(signals, "cmdline-llm")
	}

	confidence := NotAI
	if score >= 50 {
		confidence = ConfirmedAI
	} else if score >= 30 {
		confidence = LikelyAI
	}

	var appPtr *aiapps.App
	if found {
		appCopy := app
		appPtr = &appCopy
	}

	return Result{
		Confidence: confidence,
		Score:      score,
		App:        appPtr,
		Signals:    signals,
	}
}

var aiKeywords = []string{
	"claude", "cursor", "copilot", "chatgpt", "openai", "anthropic",
	"gemini", "ollama", "llm", "gpt", "ai", "codeium", "tabnine",
	"continue", "aider", "devin", "windsurf",
}

func matchesKeywords(name, cmdline string) bool {
	nameLower := strings.ToLower(name)
	cmdLower := strings.ToLower(cmdline)
	for _, kw := range aiKeywords {
		if strings.Contains(nameLower, kw) || strings.Contains(cmdLower, kw) {
			return true
		}
	}
	return false
}
