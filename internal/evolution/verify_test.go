package evolution

import "testing"

// ---------------------------------------------------------------------------
// isProtected
// ---------------------------------------------------------------------------

func TestIsProtected(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "engine.go is protected (exact match)",
			path: "internal/evolution/engine.go",
			want: true,
		},
		{
			name: "journal.go is protected (glob *.go in evolution dir)",
			path: "internal/evolution/journal.go",
			want: true,
		},
		{
			name: "parsing.go is protected (glob *.go in evolution dir)",
			path: "internal/evolution/parsing.go",
			want: true,
		},
		{
			name: "evolve.yml is protected",
			path: ".github/workflows/evolve.yml",
			want: true,
		},
		{
			name: "ci.yml is protected by .github/workflows/*.yml glob",
			path: ".github/workflows/ci.yml",
			want: true,
		},
		{
			name: "repl.go is protected",
			path: "cmd/iterate/repl.go",
			want: true,
		},
		{
			name: "main.go in cmd/iterate is protected",
			path: "cmd/iterate/main.go",
			want: true,
		},
		{
			name: "config.json in .iterate is protected",
			path: ".iterate/config.json",
			want: true,
		},
		{
			name: "evolve.sh is protected",
			path: "scripts/evolution/evolve.sh",
			want: true,
		},
		{
			name: "social.sh is protected",
			path: "scripts/social/social.sh",
			want: true,
		},
		{
			name: "regular Go source file is not protected",
			path: "internal/commands/dev.go",
			want: false,
		},
		{
			name: "test file is not protected",
			path: "internal/evolution/engine_test.go",
			want: true, // matches internal/evolution/*.go
		},
		{
			name: "main app file outside cmd/iterate is not protected",
			path: "cmd/other/main.go",
			want: false,
		},
		{
			name: "non-protected script is not protected",
			path: "scripts/build.sh",
			want: false,
		},
		{
			name: "README is not protected",
			path: "README.md",
			want: false,
		},
		{
			name: "Go test file in commands package is not protected",
			path: "internal/commands/registry_test.go",
			want: false,
		},
		{
			name: "cleaned path with .. is handled",
			path: "internal/evolution/../evolution/engine.go",
			want: true,
		},
		{
			name: "IDENTITY.md is not in protected list",
			path: "docs/IDENTITY.md",
			want: false,
		},
		{
			name: "JOURNAL.md is not in protected list",
			path: "docs/JOURNAL.md",
			want: false,
		},
		{
			name: "features.go in cmd/iterate is not protected",
			path: "cmd/iterate/features.go",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isProtected(tt.path)
			if got != tt.want {
				t.Errorf("isProtected(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestProtectedFiles_List(t *testing.T) {
	if len(ProtectedFiles) == 0 {
		t.Error("ProtectedFiles should not be empty")
	}
	expectedPatterns := []string{
		"internal/evolution/engine.go",
		"internal/evolution/*.go",
		".github/workflows/evolve.yml",
		".github/workflows/*.yml",
		"cmd/iterate/repl.go",
		"cmd/iterate/main.go",
		".iterate/config.json",
		"scripts/evolution/evolve.sh",
	}
	for _, pattern := range expectedPatterns {
		found := false
		for _, p := range ProtectedFiles {
			if p == pattern {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected pattern %q in ProtectedFiles", pattern)
		}
	}
}

// ---------------------------------------------------------------------------
// VerificationResult struct tests
// ---------------------------------------------------------------------------

func TestVerificationResult_DefaultValues(t *testing.T) {
	result := &VerificationResult{}
	if result.BuildPassed {
		t.Error("BuildPassed should default to false")
	}
	if result.TestPassed {
		t.Error("TestPassed should default to false")
	}
	if result.Output != "" {
		t.Error("Output should default to empty")
	}
	if result.Error != nil {
		t.Error("Error should default to nil")
	}
}

// ---------------------------------------------------------------------------
// RunResult struct tests
// ---------------------------------------------------------------------------

func TestRunResult_StatusValues(t *testing.T) {
	validStatuses := []string{
		"running", "no_changes", "reverted", "committed",
		"commit_failed", "merged", "merge_pending", "error",
	}
	for _, status := range validStatuses {
		result := &RunResult{Status: status}
		if result.Status != status {
			t.Errorf("RunResult.Status = %q, want %q", result.Status, status)
		}
	}
}
