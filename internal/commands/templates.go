package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// pendingTemplate holds a template to be prepended to the next prompt.
var pendingTemplate string

// GetPendingTemplate returns and clears any pending template injection.
func GetPendingTemplate() string {
	t := pendingTemplate
	pendingTemplate = ""
	return t
}

// templatesFilePath returns the path to the templates store.
func templatesFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "templates.json")
}

// loadTemplatesFromDisk reads the templates JSON file.
func loadTemplatesFromDisk() map[string]string {
	path := templatesFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]string)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return make(map[string]string)
	}
	return m
}

// saveTemplatesToDisk writes the templates map to disk.
func saveTemplatesToDisk(m map[string]string) error {
	path := templatesFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// RegisterTemplateCommands adds /template and /t commands.
func RegisterTemplateCommands(r *Registry) {
	r.Register(Command{
		Name:        "/template",
		Aliases:     []string{},
		Description: "manage prompt templates: save|list|use|delete <name>",
		Category:    "session",
		Handler:     cmdTemplateManager,
	})

	r.Register(Command{
		Name:        "/t",
		Aliases:     []string{},
		Description: "use a saved template: /t <name>",
		Category:    "session",
		Handler:     cmdTemplateUseShortcut,
	})
}

func cmdTemplateManager(ctx Context) Result {
	sub := ctx.Arg(1)
	switch strings.ToLower(sub) {
	case "save":
		return cmdTemplateSave(ctx)
	case "list", "ls", "":
		return cmdTemplateList(ctx)
	case "use":
		return cmdTemplateUse(ctx)
	case "delete", "del", "rm":
		return cmdTemplateDelete(ctx)
	default:
		// If no recognised sub-command, treat the argument as a name to use.
		return cmdTemplateUseByName(ctx, sub)
	}
}

func cmdTemplateSave(ctx Context) Result {
	// /template save <name>
	if len(ctx.Parts) < 3 {
		PrintError("usage: /template save <name>")
		return Result{Handled: true}
	}
	name := strings.Join(ctx.Parts[2:], " ")
	if ctx.LastPrompt == nil || *ctx.LastPrompt == "" {
		PrintError("no previous prompt to save as template")
		return Result{Handled: true}
	}
	m := loadTemplatesFromDisk()
	m[name] = *ctx.LastPrompt
	if err := saveTemplatesToDisk(m); err != nil {
		PrintError("failed to save template: %v", err)
		return Result{Handled: true}
	}
	PrintSuccess("template %q saved", name)
	return Result{Handled: true}
}

func cmdTemplateList(ctx Context) Result {
	m := loadTemplatesFromDisk()
	if len(m) == 0 {
		fmt.Println("No templates saved. Use: /template save <name>")
		return Result{Handled: true}
	}
	fmt.Printf("%s── Saved Templates ────────────────%s\n", ColorDim, ColorReset)
	for name, tmpl := range m {
		preview := tmpl
		if len(preview) > 60 {
			preview = preview[:60] + "…"
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		fmt.Printf("  %s%-20s%s  %s%s%s\n",
			ColorBold, name, ColorReset,
			ColorDim, preview, ColorReset)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdTemplateUse(ctx Context) Result {
	if len(ctx.Parts) < 3 {
		PrintError("usage: /template use <name>")
		return Result{Handled: true}
	}
	name := strings.Join(ctx.Parts[2:], " ")
	return cmdTemplateUseByName(ctx, name)
}

func cmdTemplateUseByName(ctx Context, name string) Result {
	m := loadTemplatesFromDisk()
	tmpl, ok := m[name]
	if !ok {
		PrintError("template %q not found — use /template list to see all", name)
		return Result{Handled: true}
	}
	pendingTemplate = tmpl
	PrintSuccess("template %q queued — it will be prepended to your next message", name)
	return Result{Handled: true}
}

func cmdTemplateDelete(ctx Context) Result {
	if len(ctx.Parts) < 3 {
		PrintError("usage: /template delete <name>")
		return Result{Handled: true}
	}
	name := strings.Join(ctx.Parts[2:], " ")
	m := loadTemplatesFromDisk()
	if _, ok := m[name]; !ok {
		PrintError("template %q not found", name)
		return Result{Handled: true}
	}
	delete(m, name)
	if err := saveTemplatesToDisk(m); err != nil {
		PrintError("failed to delete template: %v", err)
		return Result{Handled: true}
	}
	PrintSuccess("template %q deleted", name)
	return Result{Handled: true}
}

func cmdTemplateUseShortcut(ctx Context) Result {
	// /t <name> — shortcut for /template use <name>
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /t <name>  (use /template list to see all)")
		return Result{Handled: true}
	}
	name := ctx.Args()
	return cmdTemplateUseByName(ctx, name)
}

// templateCreatedTime is a sentinel zero time for sorting.
var templateCreatedTime = time.Time{}

// ensure templateCreatedTime is used to suppress "declared but not used" errors.
var _ = templateCreatedTime
