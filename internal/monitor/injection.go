package monitor

import (
	"bufio"
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/cortexark/argus/internal/db"
)

// injectionPatterns are regex patterns for prompt injection detection.
var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(previous|prior|above|all)\s+(instructions?|commands?|prompts?)`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+(a\s+)?(?:an?\s+)?(evil|jailbroken|uncensored|unrestricted|DAN)`),
	regexp.MustCompile(`(?i)act\s+as\s+(if\s+you\s+are\s+)?(a\s+)?(?:an?\s+)?(evil|jailbroken|unrestricted)`),
	regexp.MustCompile(`(?i)(system|hidden|secret)\s*prompt\s*[:=]`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+)?(previous|prior|safety)\s+(rules?|instructions?|guidelines?)`),
	regexp.MustCompile(`(?i)you\s+have\s+no\s+(restrictions?|limitations?|guidelines?)`),
	regexp.MustCompile(`(?i)pretend\s+(you\s+are|to\s+be)\s+(a\s+)?(evil|malicious|unconstrained)`),
	regexp.MustCompile(`</?(system|instruction|prompt)>`),
	regexp.MustCompile(`(?i)SUDO\s+(MODE|OVERRIDE|UNLOCK)`),
	regexp.MustCompile(`(?i)developer\s+mode\s+(enabled|on|activated)`),
}

// InjectionDetector monitors clipboard and recent files for prompt injection.
type InjectionDetector struct {
	store    *db.Store
	interval time.Duration
	done     chan struct{}
}

// NewInjectionDetector creates an injection detector.
func NewInjectionDetector(store *db.Store, interval time.Duration) *InjectionDetector {
	return &InjectionDetector{
		store:    store,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start begins detection in a goroutine.
func (id *InjectionDetector) Start() {
	go id.loop()
}

// Stop signals the detector to stop.
func (id *InjectionDetector) Stop() {
	close(id.done)
}

func (id *InjectionDetector) loop() {
	id.scan()
	ticker := time.NewTicker(id.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			id.scan()
		case <-id.done:
			return
		}
	}
}

func (id *InjectionDetector) scan() {
	// Check clipboard
	clip := getClipboard()
	if clip != "" {
		id.analyze(clip, "clipboard")
	}

	// Check recent terminal output (macOS: pbpaste + last command approximation)
	// Linux: check /proc for pipe data (limited without ptrace)
}

func (id *InjectionDetector) analyze(text, source string) {
	for _, pattern := range injectionPatterns {
		if pattern.MatchString(text) {
			// Find matched text for context
			loc := pattern.FindStringIndex(text)
			snippet := text
			if len(text) > 200 {
				start := loc[0] - 50
				end := loc[1] + 50
				if start < 0 {
					start = 0
				}
				if end > len(text) {
					end = len(text)
				}
				snippet = "..." + text[start:end] + "..."
			}

			alert := db.InjectionAlert{
				Source:   source,
				Pattern:  pattern.String(),
				Snippet:  snippet,
				Severity: classifySeverity(pattern.String()),
			}
			_ = id.store.InsertInjectionAlert(alert)
			break
		}
	}
}

func classifySeverity(pattern string) string {
	critical := []string{"SUDO", "developer mode", "system prompt", "jailbroken"}
	for _, c := range critical {
		if strings.Contains(strings.ToLower(pattern), strings.ToLower(c)) {
			return "critical"
		}
	}
	return "high"
}

// getClipboard reads clipboard content using platform tools.
func getClipboard() string {
	var cmd *exec.Cmd
	cmd = exec.Command("pbpaste") // macOS; on Linux use xclip/xsel if available

	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ScanText checks arbitrary text for injection patterns and returns matches.
func ScanText(text string) []string {
	var matches []string
	scanner := bufio.NewScanner(bytes.NewReader([]byte(text)))
	for scanner.Scan() {
		line := scanner.Text()
		for _, p := range injectionPatterns {
			if p.MatchString(line) {
				matches = append(matches, p.String())
				break
			}
		}
	}
	return matches
}
