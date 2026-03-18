package aiapps

import "strings"

type App struct {
	Name     string
	Label    string
	Category string
}

type Endpoint struct {
	Pattern string
	Service string
}

// Registry maps process names to app info
var Registry = map[string]App{
	"Claude":         {Name: "Claude", Label: "Claude (Anthropic)", Category: "LLM Desktop"},
	"claude":         {Name: "claude", Label: "Claude (Anthropic)", Category: "LLM Desktop"},
	"Claude Desktop": {Name: "Claude Desktop", Label: "Claude (Anthropic)", Category: "LLM Desktop"},
	"Cursor":         {Name: "Cursor", Label: "Cursor (Anysphere)", Category: "AI Code Editor"},
	"cursor":         {Name: "cursor", Label: "Cursor (Anysphere)", Category: "AI Code Editor"},
	"ChatGPT":        {Name: "ChatGPT", Label: "ChatGPT (OpenAI)", Category: "LLM Desktop"},
	"copilot":        {Name: "copilot", Label: "GitHub Copilot", Category: "AI Code Assistant"},
	"Copilot":        {Name: "Copilot", Label: "GitHub Copilot", Category: "AI Code Assistant"},
	"ollama":         {Name: "ollama", Label: "Ollama (Local LLM)", Category: "Local LLM"},
	"Ollama":         {Name: "Ollama", Label: "Ollama (Local LLM)", Category: "Local LLM"},
	"LM Studio":      {Name: "LM Studio", Label: "LM Studio", Category: "Local LLM"},
	"lmstudio":       {Name: "lmstudio", Label: "LM Studio", Category: "Local LLM"},
	"Windsurf":       {Name: "Windsurf", Label: "Windsurf (Codeium)", Category: "AI Code Editor"},
	"windsurf":       {Name: "windsurf", Label: "Windsurf (Codeium)", Category: "AI Code Editor"},
	"codeium":        {Name: "codeium", Label: "Codeium", Category: "AI Code Assistant"},
	"Continue":       {Name: "Continue", Label: "Continue (Open Source)", Category: "AI Code Assistant"},
	"Tabnine":        {Name: "Tabnine", Label: "Tabnine", Category: "AI Code Assistant"},
	"aider":          {Name: "aider", Label: "Aider", Category: "AI Code Assistant"},
	"gpt4all":        {Name: "gpt4all", Label: "GPT4All", Category: "Local LLM"},
	"jan":            {Name: "jan", Label: "Jan (Local LLM)", Category: "Local LLM"},
	"perplexity":     {Name: "perplexity", Label: "Perplexity AI", Category: "LLM Desktop"},
}

// Endpoints are known AI API endpoints
var Endpoints = []Endpoint{
	{Pattern: "api.anthropic.com", Service: "Anthropic API"},
	{Pattern: "api.openai.com", Service: "OpenAI API"},
	{Pattern: "openai.azure.com", Service: "Azure OpenAI"},
	{Pattern: "api.cohere.ai", Service: "Cohere API"},
	{Pattern: "generativelanguage.googleapis.com", Service: "Google Gemini"},
	{Pattern: "api.mistral.ai", Service: "Mistral API"},
	{Pattern: "api.together.xyz", Service: "Together AI"},
	{Pattern: "api.groq.com", Service: "Groq API"},
	{Pattern: "ollama", Service: "Ollama (local)"},
	{Pattern: "localhost:11434", Service: "Ollama (local)"},
	{Pattern: "127.0.0.1:11434", Service: "Ollama (local)"},
	{Pattern: "api.perplexity.ai", Service: "Perplexity API"},
	{Pattern: "api.github.com", Service: "GitHub API"},
}

// SensitivePaths are path patterns that indicate sensitive file access
var SensitivePaths = map[string][]string{
	"credentials": {
		".ssh", ".aws", ".gnupg", ".netrc",
		"Library/Keychains", "Library/Application Support/1Password",
		"Library/Application Support/Bitwarden",
		".kube", ".azure", ".config/gcloud",
		".local/share/keyrings", ".password-store",
	},
	"browserData": {
		"Library/Application Support/Google/Chrome",
		"Library/Application Support/BraveSoftware",
		"Library/Application Support/Firefox",
		"Library/Safari",
		".config/google-chrome", ".mozilla/firefox",
		".config/BraveSoftware",
	},
	"documents": {
		"Documents", "Downloads", "Desktop",
	},
	"system": {
		"/etc/passwd", "/etc/shadow", "/etc/hosts",
		".env", ".env.local", ".env.production",
	},
}

// Lookup finds an app by process name (case-insensitive)
func Lookup(name string) (App, bool) {
	if app, ok := Registry[name]; ok {
		return app, true
	}
	// case-insensitive fallback
	lower := strings.ToLower(name)
	for k, v := range Registry {
		if strings.ToLower(k) == lower {
			return v, true
		}
	}
	return App{}, false
}

// MatchEndpoint returns the service name if host matches a known AI endpoint
func MatchEndpoint(host string) string {
	lower := strings.ToLower(host)
	for _, ep := range Endpoints {
		if strings.Contains(lower, ep.Pattern) {
			return ep.Service
		}
	}
	return ""
}

// ClassifyPath returns the sensitivity category for a file path
func ClassifyPath(path string) (category, severity string) {
	lower := strings.ToLower(path)
	severityMap := map[string]string{
		"credentials": "CRITICAL",
		"browserData": "HIGH",
		"documents":   "MEDIUM",
		"system":      "HIGH",
	}
	for cat, patterns := range SensitivePaths {
		for _, p := range patterns {
			if strings.Contains(lower, strings.ToLower(p)) {
				return cat, severityMap[cat]
			}
		}
	}
	return "", ""
}
