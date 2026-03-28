package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RegisterDocsCommands adds dependency summary command.
func RegisterDocsCommands(r *Registry) {
	r.Register(Command{
		Name:        "/deps-summary",
		Aliases:     []string{},
		Description: "summarize project dependencies",
		Category:    "context",
		Handler:     cmdDepsSummary,
	})
}

func cmdDepsSummary(ctx Context) Result {
	repoPath := ctx.RepoPath

	goMod := filepath.Join(repoPath, "go.mod")
	if data, err := os.ReadFile(goMod); err == nil {
		fmt.Printf("%s── Go Dependencies ────────────────%s\n", ColorDim, ColorReset)
		lines := strings.Split(string(data), "\n")
		inRequire := false
		depCount := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "require") {
				inRequire = true
				continue
			}
			if line == ")" {
				inRequire = false
				continue
			}
			if inRequire && line != "" && !strings.HasPrefix(line, "//") {
				fmt.Printf("  %s\n", line)
				depCount++
			}
		}
		fmt.Printf("\n  %s%d dependencies%s\n", ColorDim, depCount, ColorReset)
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	packageJSON := filepath.Join(repoPath, "package.json")
	if data, err := os.ReadFile(packageJSON); err == nil {
		fmt.Printf("%s── Node Dependencies ──────────────%s\n", ColorDim, ColorReset)
		content := string(data)
		if strings.Contains(content, "\"dependencies\"") {
			fmt.Println("  (dependencies found in package.json — use npm ls for full tree)")
		}
		if strings.Contains(content, "\"devDependencies\"") {
			fmt.Println("  (devDependencies found in package.json)")
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	reqTxt := filepath.Join(repoPath, "requirements.txt")
	if data, err := os.ReadFile(reqTxt); err == nil {
		fmt.Printf("%s── Python Dependencies ────────────%s\n", ColorDim, ColorReset)
		lines := strings.Split(string(data), "\n")
		depCount := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				fmt.Printf("  %s\n", line)
				depCount++
			}
		}
		fmt.Printf("\n  %s%d dependencies%s\n", ColorDim, depCount, ColorReset)
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	fmt.Println("No recognized dependency file found (go.mod, package.json, requirements.txt)")
	return Result{Handled: true}
}
