package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Project type detection
// ---------------------------------------------------------------------------

type projectType int

const (
	projectTypeGo projectType = iota
	projectTypeRust
	projectTypeNode
	projectTypePython
	projectTypeMake
	projectTypeUnknown
)

func detectProjectType(dir string) projectType {
	check := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}
	switch {
	case check("go.mod"):
		return projectTypeGo
	case check("Cargo.toml"):
		return projectTypeRust
	case check("package.json"):
		return projectTypeNode
	case check("requirements.txt"), check("pyproject.toml"), check("setup.py"):
		return projectTypePython
	case check("Makefile"), check("makefile"):
		return projectTypeMake
	default:
		return projectTypeUnknown
	}
}

func (pt projectType) String() string {
	switch pt {
	case projectTypeGo:
		return "Go"
	case projectTypeRust:
		return "Rust"
	case projectTypeNode:
		return "Node"
	case projectTypePython:
		return "Python"
	case projectTypeMake:
		return "Make"
	default:
		return "Unknown"
	}
}

// ---------------------------------------------------------------------------
// /health — project-type aware health checks
// ---------------------------------------------------------------------------

type projectCheck struct {
	name string
	args []string
}

func healthChecksForProject(dir string, pt projectType) []projectCheck {
	switch pt {
	case projectTypeGo:
		return []projectCheck{
			{"go build", []string{"go", "build", "./..."}},
			{"go vet", []string{"go", "vet", "./..."}},
			{"go test", []string{"go", "test", "-count=1", "-timeout=30s", "./..."}},
		}
	case projectTypeRust:
		return []projectCheck{
			{"cargo build", []string{"cargo", "build"}},
			{"cargo test", []string{"cargo", "test"}},
			{"cargo clippy", []string{"cargo", "clippy", "--", "-D", "warnings"}},
		}
	case projectTypeNode:
		npmOrYarn := "npm"
		if _, err := os.Stat(filepath.Join(dir, "yarn.lock")); err == nil {
			npmOrYarn = "yarn"
		} else if _, err := os.Stat(filepath.Join(dir, "pnpm-lock.yaml")); err == nil {
			npmOrYarn = "pnpm"
		}
		return []projectCheck{
			{"install", []string{npmOrYarn, "install"}},
			{"build", []string{npmOrYarn, "run", "build"}},
			{"test", []string{npmOrYarn, "test", "--", "--passWithNoTests"}},
		}
	case projectTypePython:
		return []projectCheck{
			{"syntax", []string{"python3", "-m", "compileall", "-q", "."}},
		}
	case projectTypeMake:
		return []projectCheck{
			{"make", []string{"make"}},
			{"make test", []string{"make", "test"}},
		}
	default:
		return nil
	}
}

func runProjectHealthChecks(repoPath string, pt projectType) []healthResult {
	checks := healthChecksForProject(repoPath, pt)
	var results []healthResult

	for _, c := range checks {
		cmd := exec.Command(c.args[0], c.args[1:]...)
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		detail := strings.TrimSpace(string(out))
		if len(detail) > 100 {
			detail = detail[:100] + "…"
		}
		results = append(results, healthResult{check: c.name, ok: err == nil, detail: detail})
	}

	// git clean check
	statusOut, _ := exec.Command("git", "-C", repoPath, "status", "--short").Output()
	dirty := strings.TrimSpace(string(statusOut)) != ""
	gitDetail := "working tree clean"
	if dirty {
		gitDetail = "uncommitted changes"
	}
	results = append(results, healthResult{check: "git clean", ok: !dirty, detail: gitDetail})

	return results
}

// buildFixPromptForProject builds a fix prompt using project-type-aware build output.
func buildFixPromptForProject(repoPath string, pt projectType) string {
	var buildCmd []string
	switch pt {
	case projectTypeGo:
		buildCmd = []string{"go", "build", "./..."}
	case projectTypeRust:
		buildCmd = []string{"cargo", "build"}
	case projectTypeNode:
		buildCmd = []string{"npm", "run", "build"}
	default:
		buildCmd = []string{"make"}
	}

	cmd := exec.Command(buildCmd[0], buildCmd[1:]...)
	cmd.Dir = repoPath
	out, _ := cmd.CombinedOutput()
	errText := strings.TrimSpace(string(out))
	if errText == "" {
		return ""
	}
	return fmt.Sprintf(
		"Fix the following build error. Read relevant files first, then apply the minimal fix. "+
			"Re-run the build to verify.\n\nBuild command: %s\n\nError:\n```\n%s\n```",
		strings.Join(buildCmd, " "), errText)
}
