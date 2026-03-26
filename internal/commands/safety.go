package commands

import (
	"fmt"
	"strconv"
	"strings"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RegisterSafetyCommands adds safety/config commands.
func RegisterSafetyCommands(r *Registry) {
	r.Register(Command{
		Name:        "/safe",
		Aliases:     []string{},
		Description: "enable safe mode (confirm before writes)",
		Category:    "safety",
		Handler:     cmdSafe,
	})

	r.Register(Command{
		Name:        "/trust",
		Aliases:     []string{},
		Description: "disable safe mode (no confirmations)",
		Category:    "safety",
		Handler:     cmdTrust,
	})

	r.Register(Command{
		Name:        "/allow",
		Aliases:     []string{},
		Description: "remove tool from deny list",
		Category:    "safety",
		Handler:     cmdAllow,
	})

	r.Register(Command{
		Name:        "/deny",
		Aliases:     []string{},
		Description: "add tool to deny list",
		Category:    "safety",
		Handler:     cmdDeny,
	})

	r.Register(Command{
		Name:        "/config",
		Aliases:     []string{},
		Description: "show all configuration",
		Category:    "safety",
		Handler:     cmdConfig,
	})
}

func cmdSafe(ctx Context) Result {
	if ctx.SafeMode != nil {
		*ctx.SafeMode = true
	}
	if ctx.PersistConfig != nil {
		ctx.PersistConfig()
	}
	fmt.Printf("%s✓ Safe mode on — will ask before bash/write_file/edit_file%s\n\n", ColorLime, ColorReset)
	return Result{Handled: true}
}

func cmdTrust(ctx Context) Result {
	if ctx.SafeMode != nil {
		*ctx.SafeMode = false
	}
	if ctx.PersistConfig != nil {
		ctx.PersistConfig()
	}
	fmt.Printf("%s✓ Trust mode — tools run without confirmation%s\n\n", ColorLime, ColorReset)
	return Result{Handled: true}
}

func cmdAllow(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /allow <tool>")
		return Result{Handled: true}
	}
	tool := ctx.Arg(1)
	if ctx.State.AllowTool != nil {
		ctx.State.AllowTool(tool)
	}
	if ctx.PersistConfig != nil {
		ctx.PersistConfig()
	}
	PrintSuccess("%s removed from deny list", tool)
	return Result{Handled: true}
}

func cmdDeny(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /deny <tool>")
		return Result{Handled: true}
	}
	tool := ctx.Arg(1)
	if ctx.State.DenyTool != nil {
		ctx.State.DenyTool(tool)
	}
	if ctx.PersistConfig != nil {
		ctx.PersistConfig()
	}
	PrintSuccess("%s added to deny list", tool)
	return Result{Handled: true}
}

func cmdConfig(ctx Context) Result {
	if ctx.HasArg(1) {
		return cmdConfigSet(ctx)
	}

	printConfigDisplay(ctx)
	fmt.Println()
	return cmdConfigWizard(ctx)
}

func printConfigDisplay(ctx Context) {
	var c interface{}
	if ctx.Config.LoadConfig != nil {
		c = ctx.Config.LoadConfig()
	}

	fmt.Printf("%s── Configuration ──────────────────────────────────%s\n", ColorDim, ColorReset)

	if ctx.Provider != nil {
		fmt.Printf("  %-18s %s\n", "model:", ctx.Provider.Name())
	}
	if c != nil {
		if gc, ok := c.(interface{ GetAPIKey() string }); ok {
			if key := gc.GetAPIKey(); key != "" {
				fmt.Printf("  %-18s %s\n", "api-key:", maskKey(key))
			} else {
				fmt.Printf("  %-18s %s(not set)%s\n", "api-key:", ColorDim, ColorReset)
			}
		}
	}
	if ctx.Thinking != nil {
		fmt.Printf("  %-18s %s\n", "thinking:", *ctx.Thinking)
	}
	if ctx.SafeMode != nil {
		fmt.Printf("  %-18s %v\n", "safe-mode:", *ctx.SafeMode)
	}
	if ctx.RuntimeConfig != nil {
		temp := "default"
		if ctx.RuntimeConfig.Temperature != nil {
			temp = fmt.Sprintf("%.2f", *ctx.RuntimeConfig.Temperature)
		}
		fmt.Printf("  %-18s %s\n", "temperature:", temp)
		maxt := "default"
		if ctx.RuntimeConfig.MaxTokens != nil {
			maxt = fmt.Sprintf("%d", *ctx.RuntimeConfig.MaxTokens)
		}
		fmt.Printf("  %-18s %s\n", "max-tokens:", maxt)
		cache := "false"
		if ctx.RuntimeConfig.CacheEnabled != nil {
			cache = fmt.Sprintf("%v", *ctx.RuntimeConfig.CacheEnabled)
		}
		fmt.Printf("  %-18s %s\n", "cache:", cache)
	}
	if ctx.NotifyEnabled != nil {
		fmt.Printf("  %-18s %v\n", "notify:", *ctx.NotifyEnabled)
	}
	if ctx.State.GetDeniedList != nil {
		if denied := ctx.State.GetDeniedList(); len(denied) > 0 {
			fmt.Printf("  %-18s %s\n", "denied-tools:", strings.Join(denied, ", "))
		}
	}
	fmt.Printf("%s──────────────────────────────────────────────────%s\n", ColorDim, ColorReset)
}

// cmdConfigWizard runs the interactive provider + API key setup flow.
func cmdConfigWizard(ctx Context) Result {
	if ctx.Session.SelectItem == nil || ctx.REPL.PromptLine == nil {
		fmt.Printf("%sUsage: /config <key> <value>%s\n", ColorDim, ColorReset)
		fmt.Printf("%sKeys: provider, model, api-key, thinking, safe-mode, temperature, max-tokens, cache, notify, theme, ollama-url%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	// Step 1: provider dropdown
	providerItems := []string{
		"anthropic      — Claude (sk-ant-…)",
		"openai         — GPT (sk-…)",
		"gemini         — Google (AIza…)",
		"groq           — fast, free tier (gsk_…)",
		"opencode       — OpenCode API (sk-oc-…)",
		"opencode-cli   — free, uses local CLI",
		"ollama         — local models",
	}
	choice, ok := ctx.Session.SelectItem("Select provider", providerItems)
	if !ok {
		return Result{Handled: true}
	}
	providerName := strings.Fields(choice)[0]

	// Step 2: API key (skip for keyless providers)
	var apiKey string
	if providerName != "opencode-cli" && providerName != "ollama" {
		hint := apiKeyHint(providerName)
		apiKey, ok = ctx.REPL.PromptLine(fmt.Sprintf("API key (%s):", hint))
		if !ok {
			return Result{Handled: true}
		}
		if apiKey == "" {
			fmt.Printf("%sno key entered — skipping key save%s\n\n", ColorDim, ColorReset)
		} else {
			// Validate format
			if valid, msg := validateAPIKeyFormat(providerName, apiKey); !valid {
				fmt.Printf("%s⚠ %s%s\n", ColorYellow, msg, ColorReset)
				confirm, ok2 := ctx.REPL.PromptLine("Save anyway? (y/N):")
				if !ok2 || strings.ToLower(strings.TrimSpace(confirm)) != "y" {
					fmt.Printf("%scancelled%s\n\n", ColorDim, ColorReset)
					return Result{Handled: true}
				}
			} else {
				fmt.Printf("%s✓ key format ok%s\n", ColorDim, ColorReset)
			}
		}
	}

	// Step 3: persist to config
	if ctx.Config.LoadConfig != nil && ctx.Config.SaveConfig != nil {
		c := ctx.Config.LoadConfig()
		if s, ok2 := c.(interface{ SetProvider(string) }); ok2 {
			s.SetProvider(providerName)
		}
		if apiKey != "" {
			if s, ok2 := c.(interface{ SetAPIKey(string) }); ok2 {
				s.SetAPIKey(apiKey)
			}
		}
		ctx.Config.SaveConfig(c)
	}

	// Step 4: confirm
	fmt.Println()
	PrintSuccess("provider = %s", providerName)
	if apiKey != "" {
		PrintSuccess("api-key saved (%s)", maskKey(apiKey))
	}
	fmt.Printf("%sUse /provider %s to switch now without restarting%s\n\n", ColorDim, providerName, ColorReset)
	return Result{Handled: true}
}

// apiKeyHint returns the expected key format for a provider.
func apiKeyHint(provider string) string {
	switch provider {
	case "anthropic":
		return "starts with sk-ant-"
	case "openai":
		return "starts with sk-"
	case "gemini":
		return "starts with AIza or alphanumeric"
	case "groq":
		return "starts with gsk_"
	case "opencode":
		return "starts with sk-oc-"
	case "nvidia":
		return "NVIDIA NIM API key"
	default:
		return "paste key or press Enter to skip"
	}
}

// validateAPIKeyFormat returns (valid, errorMsg). Soft validation — warns but doesn't block.
func validateAPIKeyFormat(provider, key string) (bool, string) {
	switch provider {
	case "anthropic":
		if !strings.HasPrefix(key, "sk-ant-") {
			return false, "Anthropic keys usually start with sk-ant-api03-…"
		}
	case "openai":
		if !strings.HasPrefix(key, "sk-") {
			return false, "OpenAI keys usually start with sk-…"
		}
	case "gemini":
		if len(key) < 20 {
			return false, "Gemini keys are typically 39 characters"
		}
	case "groq":
		if !strings.HasPrefix(key, "gsk_") {
			return false, "Groq keys usually start with gsk_…"
		}
	case "opencode":
		if !strings.HasPrefix(key, "sk-") {
			return false, "OpenCode keys usually start with sk-…"
		}
	}
	if len(key) < 8 {
		return false, "Key seems too short"
	}
	return true, ""
}

// maskKey returns a masked version of an API key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + "…" + key[len(key)-4:]
}

func cmdConfigSet(ctx Context) Result {
	if !ctx.HasArg(2) {
		fmt.Printf("Usage: /config %s <value>\n", ctx.Arg(1))
		return Result{Handled: true}
	}
	if ctx.Config.LoadConfig == nil || ctx.Config.SaveConfig == nil {
		PrintError("config persistence not available")
		return Result{Handled: true}
	}

	key := ctx.Arg(1)
	val := ctx.Arg(2)
	c := ctx.Config.LoadConfig()

	switch key {
	case "api-key", "api_key":
		type apiKeySetter interface{ SetAPIKey(string) }
		if s, ok := c.(apiKeySetter); ok {
			s.SetAPIKey(val)
			ctx.Config.SaveConfig(c)
			masked := val
			if len(masked) > 8 {
				masked = masked[:4] + "…" + masked[len(masked)-4:]
			}
			PrintSuccess("api-key saved (%s)", masked)
		}

	case "provider":
		type providerSetter interface{ SetProvider(string) }
		if s, ok := c.(providerSetter); ok {
			s.SetProvider(val)
			ctx.Config.SaveConfig(c)
			PrintSuccess("provider = %s", val)
		}

	case "model":
		type modelSetter interface{ SetModel(string) }
		if s, ok := c.(modelSetter); ok {
			s.SetModel(val)
			ctx.Config.SaveConfig(c)
			PrintSuccess("model = %s", val)
		}

	case "thinking", "thinking-level", "thinking_level":
		type thinkingSetter interface{ SetThinkingLevel(string) }
		if s, ok := c.(thinkingSetter); ok {
			s.SetThinkingLevel(val)
			ctx.Config.SaveConfig(c)
		}
		if ctx.Thinking != nil {
			*ctx.Thinking = iteragent.ThinkingLevel(val)
		}
		PrintSuccess("thinking = %s", val)

	case "safe-mode", "safe_mode":
		b := val == "true" || val == "1" || val == "on"
		type safeModeSetter interface{ SetSafeMode(bool) }
		if s, ok := c.(safeModeSetter); ok {
			s.SetSafeMode(b)
			ctx.Config.SaveConfig(c)
		}
		if ctx.SafeMode != nil {
			*ctx.SafeMode = b
		}
		PrintSuccess("safe-mode = %v", b)

	case "temperature", "temp":
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			PrintError("invalid temperature: %s", val)
			return Result{Handled: true}
		}
		type tempSetter interface{ SetTemperature(float64) }
		if s, ok := c.(tempSetter); ok {
			s.SetTemperature(f)
			ctx.Config.SaveConfig(c)
		}
		if ctx.RuntimeConfig != nil {
			fv := float32(f)
			ctx.RuntimeConfig.Temperature = &fv
		}
		PrintSuccess("temperature = %.2f", f)

	case "max-tokens", "max_tokens", "tokens":
		n, err := strconv.Atoi(val)
		if err != nil {
			PrintError("invalid max-tokens: %s", val)
			return Result{Handled: true}
		}
		type tokenSetter interface{ SetMaxTokens(int) }
		if s, ok := c.(tokenSetter); ok {
			s.SetMaxTokens(n)
			ctx.Config.SaveConfig(c)
		}
		if ctx.RuntimeConfig != nil {
			ctx.RuntimeConfig.MaxTokens = &n
		}
		PrintSuccess("max-tokens = %d", n)

	case "cache", "cache-enabled", "cache_enabled":
		b := val == "true" || val == "1" || val == "on"
		type cacheSetter interface{ SetCacheEnabled(bool) }
		if s, ok := c.(cacheSetter); ok {
			s.SetCacheEnabled(b)
			ctx.Config.SaveConfig(c)
		}
		if ctx.RuntimeConfig != nil {
			ctx.RuntimeConfig.CacheEnabled = &b
		}
		PrintSuccess("cache = %v", b)

	case "notify":
		b := val == "true" || val == "1" || val == "on"
		type notifySetter interface{ SetNotify(bool) }
		if s, ok := c.(notifySetter); ok {
			s.SetNotify(b)
			ctx.Config.SaveConfig(c)
		}
		if ctx.NotifyEnabled != nil {
			*ctx.NotifyEnabled = b
		}
		PrintSuccess("notify = %v", b)

	case "theme":
		type themeSetter interface{ SetTheme(string) }
		if s, ok := c.(themeSetter); ok {
			s.SetTheme(val)
			ctx.Config.SaveConfig(c)
		}
		if ctx.ApplyTheme != nil {
			ctx.ApplyTheme(val)
		}
		PrintSuccess("theme = %s", val)

	case "ollama-url", "ollama_base_url", "ollama_url":
		type ollamaSetter interface{ SetOllamaBaseURL(string) }
		if s, ok := c.(ollamaSetter); ok {
			s.SetOllamaBaseURL(val)
			ctx.Config.SaveConfig(c)
			PrintSuccess("ollama-url = %s", val)
		}

	default:
		fmt.Printf("Unknown key: %s\n", key)
		fmt.Printf("%sKeys: provider, model, api-key, thinking, safe-mode, temperature, max-tokens, cache, notify, theme, ollama-url%s\n\n", ColorDim, ColorReset)
	}
	return Result{Handled: true}
}

