package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Profile captures a named agent configuration snapshot.
type Profile struct {
	Name         string    `json:"name"`
	Model        string    `json:"model,omitempty"`
	Temperature  float32   `json:"temperature,omitempty"`
	MaxTokens    int       `json:"max_tokens,omitempty"`
	SystemSuffix string    `json:"system_suffix,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// profilesPath returns the path to ~/.iterate/profiles.json.
func profilesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "profiles.json")
}

// loadProfiles reads all profiles from disk. Returns an empty map on error.
func loadProfiles() map[string]Profile {
	data, err := os.ReadFile(profilesPath())
	if err != nil {
		return make(map[string]Profile)
	}
	var profiles map[string]Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return make(map[string]Profile)
	}
	return profiles
}

// saveProfiles writes profiles to disk, creating the directory if needed.
func saveProfiles(profiles map[string]Profile) error {
	path := profilesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// RegisterProfileCommands adds /profile commands.
func RegisterProfileCommands(r *Registry) {
	r.Register(Command{
		Name:        "/profile",
		Aliases:     []string{},
		Description: "save/load/list agent config profiles: /profile [save|load|list|delete] [name]",
		Category:    "config",
		Handler:     cmdProfile,
	})
}

func cmdProfile(ctx Context) Result {
	sub := ctx.Arg(1)
	switch sub {
	case "save":
		return cmdProfileSave(ctx)
	case "load":
		return cmdProfileLoad(ctx)
	case "list":
		return cmdProfileList(ctx)
	case "delete":
		return cmdProfileDelete(ctx)
	default:
		return cmdProfileStatus(ctx)
	}
}

func cmdProfileSave(ctx Context) Result {
	if !ctx.HasArg(2) {
		fmt.Println("Usage: /profile save <name>")
		return Result{Handled: true}
	}
	name := strings.Join(ctx.Parts[2:], "-")

	p := Profile{
		Name:      name,
		Model:     os.Getenv("ITERATE_MODEL"),
		CreatedAt: time.Now(),
	}

	if ctx.RuntimeConfig != nil {
		if ctx.RuntimeConfig.Temperature != nil {
			p.Temperature = *ctx.RuntimeConfig.Temperature
		}
		if ctx.RuntimeConfig.MaxTokens != nil {
			p.MaxTokens = *ctx.RuntimeConfig.MaxTokens
		}
	}

	profiles := loadProfiles()
	profiles[name] = p
	if err := saveProfiles(profiles); err != nil {
		PrintError("save profile: %v", err)
		return Result{Handled: true}
	}
	PrintSuccess("profile %q saved", name)
	return Result{Handled: true}
}

func cmdProfileLoad(ctx Context) Result {
	if !ctx.HasArg(2) {
		fmt.Println("Usage: /profile load <name>")
		return Result{Handled: true}
	}
	name := ctx.Arg(2)

	profiles := loadProfiles()
	p, ok := profiles[name]
	if !ok {
		PrintError("profile %q not found — use /profile list to see saved profiles", name)
		return Result{Handled: true}
	}

	// Apply model
	if p.Model != "" {
		os.Setenv("ITERATE_MODEL", p.Model)
	}

	// Apply temperature and max_tokens via RuntimeConfig, then rebuild the agent.
	// This matches the pattern used by /set temperature and /set max-tokens.
	if ctx.RuntimeConfig != nil {
		if p.Temperature != 0 {
			t := p.Temperature
			ctx.RuntimeConfig.Temperature = &t
		} else {
			ctx.RuntimeConfig.Temperature = nil
		}
		if p.MaxTokens != 0 {
			m := p.MaxTokens
			ctx.RuntimeConfig.MaxTokens = &m
		} else {
			ctx.RuntimeConfig.MaxTokens = nil
		}
	}

	// Rebuild the agent so all settings take effect atomically.
	if ctx.REPL.MakeAgent != nil {
		ctx.REPL.MakeAgent()
	}

	PrintSuccess("profile %q loaded", name)
	printProfileDetails(p)
	return Result{Handled: true}
}

func cmdProfileList(ctx Context) Result {
	profiles := loadProfiles()
	if len(profiles) == 0 {
		fmt.Println("No profiles saved. Use /profile save <name> to create one.")
		return Result{Handled: true}
	}

	fmt.Printf("%s── Profiles ────────────────────────%s\n", ColorDim, ColorReset)
	for _, p := range profiles {
		fmt.Printf("  %s%s%s\n", ColorBold, p.Name, ColorReset)
		printProfileDetails(p)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdProfileDelete(ctx Context) Result {
	if !ctx.HasArg(2) {
		fmt.Println("Usage: /profile delete <name>")
		return Result{Handled: true}
	}
	name := ctx.Arg(2)

	profiles := loadProfiles()
	if _, ok := profiles[name]; !ok {
		PrintError("profile %q not found", name)
		return Result{Handled: true}
	}
	delete(profiles, name)
	if err := saveProfiles(profiles); err != nil {
		PrintError("delete profile: %v", err)
		return Result{Handled: true}
	}
	PrintSuccess("profile %q deleted", name)
	return Result{Handled: true}
}

func cmdProfileStatus(ctx Context) Result {
	fmt.Printf("%s── Current settings ────────────────%s\n", ColorDim, ColorReset)
	model := os.Getenv("ITERATE_MODEL")
	if model == "" {
		model = "(default)"
	}
	fmt.Printf("  model:        %s\n", model)
	if ctx.RuntimeConfig != nil {
		temp := "default"
		if ctx.RuntimeConfig.Temperature != nil {
			temp = fmt.Sprintf("%.2f", *ctx.RuntimeConfig.Temperature)
		}
		maxt := "default"
		if ctx.RuntimeConfig.MaxTokens != nil {
			maxt = fmt.Sprintf("%d", *ctx.RuntimeConfig.MaxTokens)
		}
		fmt.Printf("  temperature:  %s\n", temp)
		fmt.Printf("  max_tokens:   %s\n", maxt)
	}
	fmt.Println()

	profiles := loadProfiles()
	if len(profiles) > 0 {
		fmt.Printf("%s── Saved profiles ──────────────────%s\n", ColorDim, ColorReset)
		for name, p := range profiles {
			parts := []string{}
			if p.Model != "" {
				parts = append(parts, "model="+p.Model)
			}
			if p.Temperature != 0 {
				parts = append(parts, fmt.Sprintf("temp=%.2f", p.Temperature))
			}
			if p.MaxTokens != 0 {
				parts = append(parts, fmt.Sprintf("max_tokens=%d", p.MaxTokens))
			}
			details := strings.Join(parts, "  ")
			fmt.Printf("  %-20s  %s%s%s\n", name, ColorDim, details, ColorReset)
		}
		fmt.Printf("%s──────────────────────────────────%s\n", ColorDim, ColorReset)
	}
	fmt.Println()
	return Result{Handled: true}
}

func printProfileDetails(p Profile) {
	if p.Model != "" {
		fmt.Printf("    model:        %s\n", p.Model)
	}
	if p.Temperature != 0 {
		fmt.Printf("    temperature:  %.2f\n", p.Temperature)
	}
	if p.MaxTokens != 0 {
		fmt.Printf("    max_tokens:   %d\n", p.MaxTokens)
	}
	if p.SystemSuffix != "" {
		suffix := p.SystemSuffix
		if len(suffix) > 60 {
			suffix = suffix[:60] + "…"
		}
		fmt.Printf("    sys suffix:   %s\n", suffix)
	}
	fmt.Printf("    created:      %s\n", p.CreatedAt.Format("2006-01-02 15:04"))
}
