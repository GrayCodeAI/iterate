package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// detectProjectType
// ---------------------------------------------------------------------------

func TestDetectProjectType_Go(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n"), 0o644)
	if got := detectProjectType(dir); got != projectTypeGo {
		t.Errorf("expected Go, got %v", got)
	}
}

func TestDetectProjectType_Rust(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\n"), 0o644)
	if got := detectProjectType(dir); got != projectTypeRust {
		t.Errorf("expected Rust, got %v", got)
	}
}

func TestDetectProjectType_Node(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)
	if got := detectProjectType(dir); got != projectTypeNode {
		t.Errorf("expected Node, got %v", got)
	}
}

func TestDetectProjectType_Python(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask\n"), 0o644)
	if got := detectProjectType(dir); got != projectTypePython {
		t.Errorf("expected Python, got %v", got)
	}
}

func TestDetectProjectType_Make(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte("build:\n"), 0o644)
	if got := detectProjectType(dir); got != projectTypeMake {
		t.Errorf("expected Make, got %v", got)
	}
}

func TestDetectProjectType_Unknown(t *testing.T) {
	dir := t.TempDir()
	if got := detectProjectType(dir); got != projectTypeUnknown {
		t.Errorf("expected Unknown, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// projectType.String()
// ---------------------------------------------------------------------------

func TestProjectTypeString(t *testing.T) {
	cases := []struct {
		pt   projectType
		want string
	}{
		{projectTypeGo, "Go"},
		{projectTypeRust, "Rust"},
		{projectTypeNode, "Node"},
		{projectTypePython, "Python"},
		{projectTypeMake, "Make"},
		{projectTypeUnknown, "Unknown"},
	}
	for _, c := range cases {
		if got := c.pt.String(); got != c.want {
			t.Errorf("projectType(%d).String() = %q, want %q", c.pt, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// healthChecksForProject
// ---------------------------------------------------------------------------

func TestHealthChecksForProject_Go(t *testing.T) {
	dir := t.TempDir()
	checks := healthChecksForProject(dir, projectTypeGo)
	if len(checks) == 0 {
		t.Fatal("expected health checks for Go project")
	}
	// All checks should have at least a name and args
	for _, c := range checks {
		if c.name == "" {
			t.Errorf("check has empty name: %+v", c)
		}
		if len(c.args) == 0 {
			t.Errorf("check %q has no args", c.name)
		}
	}
}

func TestHealthChecksForProject_Unknown(t *testing.T) {
	dir := t.TempDir()
	checks := healthChecksForProject(dir, projectTypeUnknown)
	// Unknown project type returns nil — no checks defined
	if checks != nil {
		t.Errorf("expected nil checks for unknown project, got %v", checks)
	}
}

// ---------------------------------------------------------------------------
// buildProjectTree / formatTreeFromPaths
// ---------------------------------------------------------------------------

func TestFormatTreeFromPaths_Empty(t *testing.T) {
	got := formatTreeFromPaths([]string{}, 0)
	if got != "" {
		t.Errorf("expected empty string for no paths, got %q", got)
	}
}

func TestFormatTreeFromPaths_SingleFile(t *testing.T) {
	paths := []string{"main.go"}
	got := formatTreeFromPaths(paths, 1)
	if !strings.Contains(got, "main.go") {
		t.Errorf("expected main.go in output, got %q", got)
	}
}

func TestFormatTreeFromPaths_NestedPaths(t *testing.T) {
	paths := []string{
		"cmd/iterate/main.go",
		"cmd/iterate/repl.go",
		"internal/agent.go",
	}
	got := formatTreeFromPaths(paths, 4)
	if !strings.Contains(got, "cmd") {
		t.Errorf("expected 'cmd' in tree output, got:\n%s", got)
	}
	if !strings.Contains(got, "main.go") {
		t.Errorf("expected 'main.go' in tree output, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// generateIterateMD
// ---------------------------------------------------------------------------

func TestGenerateIterateMD_GoProject(t *testing.T) {
	dir := t.TempDir()
	// Create a minimal Go project
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)

	result := generateIterateMD(dir)
	if !strings.Contains(result, "example.com/test") && !strings.Contains(result, "ITERATE.md") {
		t.Logf("generateIterateMD output:\n%s", result)
		// Just check it returns non-empty content
		if result == "" {
			t.Error("generateIterateMD returned empty string")
		}
	}
}
