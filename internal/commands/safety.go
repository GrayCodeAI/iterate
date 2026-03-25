package commands

import (
	"fmt"
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
	fmt.Printf("%s── Configuration ───────────────────%s\n", ColorDim, ColorReset)
	if ctx.SafeMode != nil {
		fmt.Printf("  Safe mode:    %v\n", *ctx.SafeMode)
	}
	if ctx.Thinking != nil {
		fmt.Printf("  Thinking:     %s\n", *ctx.Thinking)
	}
	if ctx.Provider != nil {
		fmt.Printf("  Model:        %s\n", ctx.Provider.Name())
	}
	if ctx.State.GetDeniedList != nil {
		if denied := ctx.State.GetDeniedList(); len(denied) > 0 {
			fmt.Printf("  Denied tools: %v\n", denied)
		}
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}
