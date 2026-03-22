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

func cmdSet(ctx Context) Result {
	if ctx.RuntimeConfig == nil {
		PrintError("runtime config not available")
		return Result{Handled: true}
	}

	if !ctx.HasArg(2) {
		temp := "default"
		if ctx.RuntimeConfig.Temperature != nil {
			temp = fmt.Sprintf("%.2f", *ctx.RuntimeConfig.Temperature)
		}
		maxt := "default"
		if ctx.RuntimeConfig.MaxTokens != nil {
			maxt = fmt.Sprintf("%d", *ctx.RuntimeConfig.MaxTokens)
		}
		fmt.Printf("%s── Runtime config ──────────────────%s\n", ColorDim, ColorReset)
		fmt.Printf("  temperature:  %s\n", temp)
		fmt.Printf("  max_tokens:   %s\n", maxt)
		fmt.Printf("%sUsage: /set temperature 0.7 | /set max_tokens 4096 | /set reset%s\n\n", ColorDim, ColorReset)
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
		filter := []string{"ITERATE", "OLLAMA", "ANTHROPIC", "OPENAI", "GEMINI", "GROQ", "GITHUB", "GO"}
		fmt.Printf("%s── Environment ─────────────────────%s\n", ColorDim, ColorReset)
		for _, f := range filter {
			val := os.Getenv(f)
			if val != "" {
				if len(val) > 60 {
					val = val[:60] + "…"
				}
				fmt.Printf("  %s=%s\n", f, val)
			}
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}
	if ctx.HasArg(2) {
		os.Setenv(ctx.Arg(1), ctx.Arg(2))
		PrintSuccess("%s=%s", ctx.Arg(1), ctx.Arg(2))
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
