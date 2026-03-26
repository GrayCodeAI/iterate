package main

import (
	"os"
	"path/filepath"
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
