package commands

import (
	"fmt"
	"os"
	"strings"
)

// RegisterConfigCommands adds configuration and settings commands.
func RegisterConfigCommands(r *Registry) {
	registerConfigCoreCommands(r)
	registerConfigMCPCommands(r)
	registerConfigEnvCommands(r)
}

func registerConfigCoreCommands(r *Registry) {
	r.Register(Command{
		Name:        "/set",
		Aliases:     []string{},
		Description: "set runtime config (temperature, max_tokens, reset)",
		Category:    "config",
		Handler:     cmdSet,
	})

	r.Register(Command{
		Name:        "/alias",
		Aliases:     []string{},
		Description: "manage command aliases",
		Category:    "config",
		Handler:     cmdAlias,
	})

	r.Register(Command{
		Name:        "/aliases",
		Aliases:     []string{},
		Description: "list all aliases",
		Category:    "config",
		Handler:     cmdAliases,
	})

	r.Register(Command{
		Name:        "/notify",
		Aliases:     []string{},
		Description: "toggle notifications",
		Category:    "config",
		Handler:     cmdNotify,
	})
}

func registerConfigMCPCommands(r *Registry) {
	r.Register(Command{
		Name:        "/mcp-add",
		Aliases:     []string{},
		Description: "add MCP server",
		Category:    "config",
		Handler:     cmdMCPAdd,
	})

	r.Register(Command{
		Name:        "/mcp-list",
		Aliases:     []string{},
		Description: "list MCP servers",
		Category:    "config",
		Handler:     cmdMCPList,
	})

	r.Register(Command{
		Name:        "/mcp-remove",
		Aliases:     []string{},
		Description: "remove MCP server",
		Category:    "config",
		Handler:     cmdMCPRemove,
	})
}

func registerConfigEnvCommands(r *Registry) {
	r.Register(Command{
		Name:        "/env",
		Aliases:     []string{},
		Description: "show/set environment variables",
		Category:    "config",
		Handler:     cmdEnv,
	})

	r.Register(Command{
		Name:        "/debug",
		Aliases:     []string{},
		Description: "toggle debug mode",
		Category:    "config",
		Handler:     cmdDebug,
	})
}

func printCurrentConfig(cfg *RuntimeConfig) {
	temp := "default"
	if cfg.Temperature != nil {
		temp = fmt.Sprintf("%.2f", *cfg.Temperature)
	}
	maxt := "default"
	if cfg.MaxTokens != nil {
		maxt = fmt.Sprintf("%d", *cfg.MaxTokens)
	}
	fmt.Printf("%s── Runtime config ──────────────────%s\n", ColorDim, ColorReset)
	fmt.Printf("  temperature:  %s\n", temp)
	fmt.Printf("  max_tokens:   %s\n", maxt)
	fmt.Printf("%sUsage: /set temperature 0.7 | /set max_tokens 4096 | /set reset%s\n\n", ColorDim, ColorReset)
}

func cmdSet(ctx Context) Result {
	if ctx.RuntimeConfig == nil {
		PrintError("runtime config not available")
		return Result{Handled: true}
	}

	if !ctx.HasArg(2) {
		printCurrentConfig(ctx.RuntimeConfig)
		return Result{Handled: true}
	}

	switch ctx.Arg(1) {
	case "temperature", "temp":
		var v float64
		fmt.Sscanf(ctx.Arg(2), "%f", &v)
		f := float32(v)
		ctx.RuntimeConfig.Temperature = &f
		if ctx.REPL.MakeAgent != nil {
			ctx.REPL.MakeAgent()
		}
		PrintSuccess("temperature = %.2f", f)
	case "max_tokens", "max-tokens", "tokens":
		var v int
		fmt.Sscanf(ctx.Arg(2), "%d", &v)
		ctx.RuntimeConfig.MaxTokens = &v
		if ctx.REPL.MakeAgent != nil {
			ctx.REPL.MakeAgent()
		}
		PrintSuccess("max_tokens = %d", v)
	case "reset":
		ctx.RuntimeConfig.Temperature = nil
		ctx.RuntimeConfig.MaxTokens = nil
		ctx.RuntimeConfig.CacheEnabled = nil
		if ctx.REPL.MakeAgent != nil {
			ctx.REPL.MakeAgent()
		}
		PrintSuccess("runtime config reset to defaults")
	default:
		fmt.Printf("Unknown setting: %s (try: temperature, max_tokens, reset)\n", ctx.Arg(1))
	}
	return Result{Handled: true}
}

func cmdAlias(ctx Context) Result {
	if ctx.Config.LoadAliases == nil || ctx.Config.SaveAliases == nil {
		PrintError("alias system not available")
		return Result{Handled: true}
	}

	aliases := ctx.Config.LoadAliases()

	if !ctx.HasArg(1) {
		if len(aliases) == 0 {
			fmt.Println("No aliases. Use: /alias <name> <command>")
			return Result{Handled: true}
		}
		fmt.Printf("%s── Aliases ────────────────────────%s\n", ColorDim, ColorReset)
		for k, v := range aliases {
			fmt.Printf("  %-16s → %s\n", k, v)
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	if !ctx.HasArg(2) {
		fmt.Println("Usage: /alias <name> <command>  or  /alias <name> delete")
		return Result{Handled: true}
	}

	name := ctx.Arg(1)
	if ctx.Arg(2) == "delete" {
		delete(aliases, name)
		ctx.Config.SaveAliases(aliases)
		PrintSuccess("alias %q removed", name)
		return Result{Handled: true}
	}

	expansion := strings.Join(ctx.Parts[2:], " ")
	aliases[name] = expansion
	ctx.Config.SaveAliases(aliases)
	PrintSuccess("alias %q → %s", name, expansion)
	return Result{Handled: true}
}

func cmdAliases(ctx Context) Result {
	if ctx.Config.LoadAliases == nil {
		PrintError("alias system not available")
		return Result{Handled: true}
	}

	aliases := ctx.Config.LoadAliases()
	if len(aliases) == 0 {
		fmt.Println("No aliases defined. Use: /alias <name> <command>")
		return Result{Handled: true}
	}
	fmt.Printf("%s── Aliases ────────────────────────%s\n", ColorDim, ColorReset)
	for k, v := range aliases {
		fmt.Printf("  %-16s → %s\n", k, v)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdNotify(ctx Context) Result {
	if ctx.NotifyEnabled == nil {
		PrintError("notification state not available")
		return Result{Handled: true}
	}

	*ctx.NotifyEnabled = !*ctx.NotifyEnabled

	if ctx.Config.LoadConfig != nil && ctx.Config.SaveConfig != nil {
		cfg := ctx.Config.LoadConfig()
		if c, ok := cfg.(interface{ SetNotify(bool) }); ok {
			c.SetNotify(*ctx.NotifyEnabled)
		}
		ctx.Config.SaveConfig(cfg)
	}

	state := "off"
	if *ctx.NotifyEnabled {
		state = "on"
	}
	PrintSuccess("notifications %s (terminal bell on completion)", state)
	return Result{Handled: true}
}

func cmdEnv(ctx Context) Result {
	if !ctx.HasArg(1) {
		// Prefixes: any env var whose name starts with one of these is shown.
		prefixes := []string{
			"ITERATE_", "OLLAMA_", "ANTHROPIC_", "OPENAI_", "GEMINI_",
			"GROQ_", "GITHUB_", "AZURE_", "VERTEX_", "OPENCODE_",
		}
		// Exact names also shown (without the _ prefix requirement).
		exact := []string{"GOPATH", "GOROOT", "HOME", "SHELL"}

		fmt.Printf("%s── Environment ─────────────────────%s\n", ColorDim, ColorReset)
		for _, e := range os.Environ() {
			kv := strings.SplitN(e, "=", 2)
			if len(kv) != 2 {
				continue
			}
			k, v := kv[0], kv[1]
			matched := false
			for _, p := range prefixes {
				if strings.HasPrefix(k, p) {
					matched = true
					break
				}
			}
			if !matched {
				for _, ex := range exact {
					if k == ex {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
			display := v
			// Mask keys — show first 8 chars + …
			lk := strings.ToLower(k)
			if strings.Contains(lk, "key") || strings.Contains(lk, "token") || strings.Contains(lk, "secret") {
				if len(v) > 8 {
					display = v[:8] + "…"
				} else if len(v) > 0 {
					display = "***"
				}
			} else if len(display) > 80 {
				display = display[:80] + "…"
			}
			fmt.Printf("  %s%s%s=%s\n", ColorBold, k, ColorReset, display)
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}
	if ctx.HasArg(2) {
		envName := ctx.Arg(1)
		if !isAllowedEnvVar(envName) {
			PrintError("setting %s is not allowed — only ITERATE_*, OLLAMA_*, GEMINI_*, ANTHROPIC_*, OPENAI_*, GROQ_*, AZURE_*, NVIDIA_*, GITHUB_TOKEN env vars can be set", envName)
			return Result{Handled: true}
		}
		os.Setenv(envName, ctx.Arg(2))
		PrintSuccess("%s=%s", envName, ctx.Arg(2))
	} else {
		val := os.Getenv(ctx.Arg(1))
		if val == "" {
			fmt.Printf("%s is not set\n", ctx.Arg(1))
		} else {
			fmt.Printf("%s=%s\n", ctx.Arg(1), val)
		}
	}
	return Result{Handled: true}
}

func cmdDebug(ctx Context) Result {
	if ctx.DebugMode == nil {
		PrintError("debug state not available")
		return Result{Handled: true}
	}
	*ctx.DebugMode = !*ctx.DebugMode
	state := "off"
	if *ctx.DebugMode {
		state = "on"
	}
	PrintSuccess("debug mode %s", state)
	return Result{Handled: true}
}

func cmdMCPAdd(ctx Context) Result {
	if ctx.Config.LoadMCPServers == nil || ctx.Config.SaveMCPServers == nil {
		PrintError("MCP system not available")
		return Result{Handled: true}
	}
	arg := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	toks := strings.Fields(arg)
	if len(toks) < 2 {
		fmt.Println("Usage: /mcp-add <name> <url-or-command> [args...]")
		return Result{Handled: true}
	}
	srv := MCPServerEntry{Name: toks[0]}
	if strings.HasPrefix(toks[1], "http") {
		srv.URL = toks[1]
	} else {
		srv.Command = toks[1]
		if len(toks) > 2 {
			srv.Args = toks[2:]
		}
	}
	servers := ctx.Config.LoadMCPServers()
	servers = append(servers, srv)
	ctx.Config.SaveMCPServers(servers)
	PrintSuccess("MCP server added: %s", srv.Name)
	return Result{Handled: true}
}

func cmdMCPList(ctx Context) Result {
	if ctx.Config.LoadMCPServers == nil {
		PrintError("MCP system not available")
		return Result{Handled: true}
	}
	servers := ctx.Config.LoadMCPServers()
	if len(servers) == 0 {
		fmt.Println("No MCP servers configured.")
		return Result{Handled: true}
	}
	for _, s := range servers {
		loc := s.URL
		if loc == "" {
			loc = s.Command + " " + strings.Join(s.Args, " ")
		}
		fmt.Printf("  %s%s%s → %s\n", ColorYellow, s.Name, ColorReset, strings.TrimSpace(loc))
	}
	return Result{Handled: true}
}

func cmdMCPRemove(ctx Context) Result {
	if ctx.Config.LoadMCPServers == nil || ctx.Config.SaveMCPServers == nil {
		PrintError("MCP system not available")
		return Result{Handled: true}
	}
	name := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	if name == "" {
		fmt.Println("Usage: /mcp-remove <name>")
		return Result{Handled: true}
	}
	servers := ctx.Config.LoadMCPServers()
	var kept []MCPServerEntry
	for _, s := range servers {
		if s.Name != name {
			kept = append(kept, s)
		}
	}
	ctx.Config.SaveMCPServers(kept)
	PrintSuccess("removed: %s", name)
	return Result{Handled: true}
}

// allowedEnvPrefixes lists the environment variable prefixes that /env is
// allowed to set. This prevents users from accidentally or maliciously
// overriding security-sensitive variables like PATH, HOME, or LD_PRELOAD.
var allowedEnvPrefixes = []string{
	"ITERATE_",
	"OLLAMA_",
	"GEMINI_",
	"ANTHROPIC_",
	"OPENAI_",
	"GROQ_",
	"AZURE_",
	"NVIDIA_",
	"XAI_",
	"MISTRAL_",
	"DEEPSEEK_",
}

// allowedEnvExact lists exact env var names (not prefixes) that are allowed.
var allowedEnvExact = []string{
	"GITHUB_TOKEN",
}

// isAllowedEnvVar reports whether name is in the whitelist.
func isAllowedEnvVar(name string) bool {
	upper := strings.ToUpper(name)
	for _, prefix := range allowedEnvPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	for _, exact := range allowedEnvExact {
		if upper == exact {
			return true
		}
	}
	return false
}
