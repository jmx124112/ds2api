package config

import "strings"

type ModelInfo struct {
	ID         string `json:"id"`
	Object     string `json:"object"`
	Created    int64  `json:"created"`
	OwnedBy    string `json:"owned_by"`
	Permission []any  `json:"permission,omitempty"`
}

type ModelAliasReader interface {
	ModelAliases() map[string]string
}

var DeepSeekModels = []ModelInfo{
	{ID: "deepseek-v4-flash", Object: "model", Created: 1677610602, OwnedBy: "deepseek", Permission: []any{}},
	{ID: "deepseek-v4-flash-thinking", Object: "model", Created: 1677610602, OwnedBy: "deepseek", Permission: []any{}},
	{ID: "deepseek-v4-flash-search", Object: "model", Created: 1677610602, OwnedBy: "deepseek", Permission: []any{}},
	{ID: "deepseek-v4-flash-thinking-search", Object: "model", Created: 1677610602, OwnedBy: "deepseek", Permission: []any{}},
	{ID: "deepseek-v4-pro", Object: "model", Created: 1677610602, OwnedBy: "deepseek", Permission: []any{}},
	{ID: "deepseek-v4-pro-thinking", Object: "model", Created: 1677610602, OwnedBy: "deepseek", Permission: []any{}},
	{ID: "deepseek-v4-pro-search", Object: "model", Created: 1677610602, OwnedBy: "deepseek", Permission: []any{}},
	{ID: "deepseek-v4-pro-thinking-search", Object: "model", Created: 1677610602, OwnedBy: "deepseek", Permission: []any{}},
}

var ClaudeModels = []ModelInfo{
	// Current aliases
	{ID: "claude-opus-4-6", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-sonnet-4-5", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-haiku-4-5", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},

	// Current snapshots
	{ID: "claude-opus-4-5-20251101", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-opus-4-1", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-opus-4-1-20250805", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-opus-4-0", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-opus-4-20250514", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-sonnet-4-5-20250929", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-sonnet-4-0", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-sonnet-4-20250514", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-haiku-4-5-20251001", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},

	// Claude 3.x (legacy/deprecated snapshots and aliases)
	{ID: "claude-3-7-sonnet-latest", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-3-7-sonnet-20250219", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-3-5-sonnet-latest", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-3-5-sonnet-20240620", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-3-5-sonnet-20241022", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-3-opus-20240229", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-3-sonnet-20240229", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-3-5-haiku-latest", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-3-5-haiku-20241022", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-3-haiku-20240307", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},

	// Claude 2.x and 1.x (retired but accepted for compatibility)
	{ID: "claude-2.1", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-2.0", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-1.3", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-1.2", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-1.1", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-1.0", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-instant-1.2", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-instant-1.1", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
	{ID: "claude-instant-1.0", Object: "model", Created: 1715635200, OwnedBy: "anthropic"},
}

func GetModelConfig(model string) (thinking bool, search bool, ok bool) {
	switch canonicalDeepSeekModel(lower(strings.TrimSpace(model))) {
	case "deepseek-v4-flash", "deepseek-v4-pro":
		return false, false, true
	case "deepseek-v4-flash-thinking", "deepseek-v4-pro-thinking":
		return true, false, true
	case "deepseek-v4-flash-search", "deepseek-v4-pro-search":
		return false, true, true
	case "deepseek-v4-flash-thinking-search", "deepseek-v4-pro-thinking-search":
		return true, true, true
	default:
		return false, false, false
	}
}

func GetModelType(model string) (modelType string, ok bool) {
	switch canonicalDeepSeekModel(lower(strings.TrimSpace(model))) {
	case "deepseek-v4-flash", "deepseek-v4-flash-thinking", "deepseek-v4-flash-search", "deepseek-v4-flash-thinking-search":
		return "default", true
	case "deepseek-v4-pro", "deepseek-v4-pro-thinking", "deepseek-v4-pro-search", "deepseek-v4-pro-thinking-search":
		return "expert", true
	default:
		return "", false
	}
}

func IsSupportedDeepSeekModel(model string) bool {
	_, _, ok := GetModelConfig(model)
	return ok
}

func DefaultModelAliases() map[string]string {
	return map[string]string{
		// Legacy DeepSeek aliases for compatibility.
		"deepseek-chat":                   "deepseek-v4-flash",
		"deepseek-chat-search":            "deepseek-v4-flash-search",
		"deepseek-reasoner":               "deepseek-v4-pro-thinking",
		"deepseek-reasoner-search":        "deepseek-v4-pro-thinking-search",
		"deepseek-expert-chat":            "deepseek-v4-pro",
		"deepseek-expert-chat-search":     "deepseek-v4-pro-search",
		"deepseek-expert-reasoner":        "deepseek-v4-pro-thinking",
		"deepseek-expert-reasoner-search": "deepseek-v4-pro-thinking-search",
		"deepseek-vision-chat":            "deepseek-v4-flash",
		"deepseek-vision-chat-search":     "deepseek-v4-flash-search",
		"deepseek-vision-reasoner":        "deepseek-v4-pro-thinking",
		"deepseek-vision-reasoner-search": "deepseek-v4-pro-thinking-search",
		// Third-party naming aliases.
		"gpt-4o":                 "deepseek-v4-flash",
		"gpt-4.1":                "deepseek-v4-flash",
		"gpt-4.1-mini":           "deepseek-v4-flash",
		"gpt-4.1-nano":           "deepseek-v4-flash",
		"gpt-5":                  "deepseek-v4-flash",
		"gpt-5-mini":             "deepseek-v4-flash",
		"gpt-5-codex":            "deepseek-v4-pro-thinking",
		"o1":                     "deepseek-v4-pro-thinking",
		"o1-mini":                "deepseek-v4-pro-thinking",
		"o3":                     "deepseek-v4-pro-thinking",
		"o3-mini":                "deepseek-v4-pro-thinking",
		"claude-sonnet-4-5":      "deepseek-v4-flash",
		"claude-haiku-4-5":       "deepseek-v4-flash",
		"claude-opus-4-6":        "deepseek-v4-pro-thinking",
		"claude-3-5-sonnet":      "deepseek-v4-flash",
		"claude-3-5-haiku":       "deepseek-v4-flash",
		"claude-3-opus":          "deepseek-v4-pro-thinking",
		"gemini-2.5-pro":         "deepseek-v4-pro-thinking",
		"gemini-2.5-flash":       "deepseek-v4-flash",
		"llama-3.1-70b-instruct": "deepseek-v4-flash",
		"qwen-max":               "deepseek-v4-flash",
	}
}

func ResolveModel(store ModelAliasReader, requested string) (string, bool) {
	requestedModel := lower(strings.TrimSpace(requested))
	if requestedModel == "" {
		return "", false
	}
	canonicalRequested := canonicalDeepSeekModel(requestedModel)
	if IsSupportedDeepSeekModel(canonicalRequested) {
		return canonicalRequested, true
	}
	aliases := DefaultModelAliases()
	if store != nil {
		for k, v := range store.ModelAliases() {
			key := lower(strings.TrimSpace(k))
			val := canonicalDeepSeekModel(lower(strings.TrimSpace(v)))
			if key == "" || val == "" {
				continue
			}
			aliases[key] = val
		}
	}
	if mapped, ok := aliases[requestedModel]; ok {
		mapped = canonicalDeepSeekModel(mapped)
		if IsSupportedDeepSeekModel(mapped) {
			return mapped, true
		}
	}
	if strings.HasPrefix(requestedModel, "deepseek-") {
		return "", false
	}

	knownFamily := false
	for _, prefix := range []string{
		"gpt-", "o1", "o3", "claude-", "gemini-", "llama-", "qwen-", "mistral-", "command-",
	} {
		if strings.HasPrefix(requestedModel, prefix) {
			knownFamily = true
			break
		}
	}
	if !knownFamily {
		return "", false
	}

	usePro := strings.Contains(requestedModel, "pro") ||
		strings.Contains(requestedModel, "reason") ||
		strings.Contains(requestedModel, "reasoner") ||
		strings.HasPrefix(requestedModel, "o1") ||
		strings.HasPrefix(requestedModel, "o3") ||
		strings.Contains(requestedModel, "opus") ||
		strings.Contains(requestedModel, "r1")
	useThinking := strings.Contains(requestedModel, "thinking") ||
		strings.Contains(requestedModel, "reason") ||
		strings.Contains(requestedModel, "reasoner") ||
		strings.HasPrefix(requestedModel, "o1") ||
		strings.HasPrefix(requestedModel, "o3") ||
		strings.Contains(requestedModel, "opus") ||
		strings.Contains(requestedModel, "r1")
	useSearch := strings.Contains(requestedModel, "search")
	base := "deepseek-v4-flash"
	if usePro {
		base = "deepseek-v4-pro"
	}

	switch {
	case useThinking && useSearch:
		return base + "-thinking-search", true
	case useThinking:
		return base + "-thinking", true
	case useSearch:
		return base + "-search", true
	default:
		return base, true
	}
}

func canonicalDeepSeekModel(model string) string {
	switch lower(strings.TrimSpace(model)) {
	case "deepseek-chat":
		return "deepseek-v4-flash"
	case "deepseek-chat-search":
		return "deepseek-v4-flash-search"
	case "deepseek-reasoner":
		return "deepseek-v4-pro-thinking"
	case "deepseek-reasoner-search":
		return "deepseek-v4-pro-thinking-search"
	case "deepseek-expert-chat":
		return "deepseek-v4-pro"
	case "deepseek-expert-chat-search":
		return "deepseek-v4-pro-search"
	case "deepseek-expert-reasoner":
		return "deepseek-v4-pro-thinking"
	case "deepseek-expert-reasoner-search":
		return "deepseek-v4-pro-thinking-search"
	case "deepseek-vision-chat":
		return "deepseek-v4-flash"
	case "deepseek-vision-chat-search":
		return "deepseek-v4-flash-search"
	case "deepseek-vision-reasoner":
		return "deepseek-v4-pro-thinking"
	case "deepseek-vision-reasoner-search":
		return "deepseek-v4-pro-thinking-search"
	default:
		return lower(strings.TrimSpace(model))
	}
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func OpenAIModelsResponse() map[string]any {
	return map[string]any{"object": "list", "data": DeepSeekModels}
}

func OpenAIModelByID(store ModelAliasReader, id string) (ModelInfo, bool) {
	canonical, ok := ResolveModel(store, id)
	if !ok {
		return ModelInfo{}, false
	}
	for _, model := range DeepSeekModels {
		if model.ID == canonical {
			return model, true
		}
	}
	return ModelInfo{}, false
}

func ClaudeModelsResponse() map[string]any {
	resp := map[string]any{"object": "list", "data": ClaudeModels}
	if len(ClaudeModels) > 0 {
		resp["first_id"] = ClaudeModels[0].ID
		resp["last_id"] = ClaudeModels[len(ClaudeModels)-1].ID
	} else {
		resp["first_id"] = nil
		resp["last_id"] = nil
	}
	resp["has_more"] = false
	return resp
}
