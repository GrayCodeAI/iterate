package ui_test

// Feature: code-reorganization, Property 4: internal/ui has no upward imports

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestUIPackageHasNoUpwardImports verifies that no file within internal/ui or its
// sub-packages imports internal/repl, internal/evolution, or internal/commands.
//
// Validates: Requirements 5.2
func TestUIPackageHasNoUpwardImports(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedImports | packages.NeedName,
	}

	pkgs, err := packages.Load(cfg, "github.com/GrayCodeAI/iterate/internal/ui/...")
	if err != nil {
		t.Fatalf("failed to load packages: %v", err)
	}

	forbidden := []string{
		"internal/repl",
		"internal/evolution",
		"internal/commands",
	}

	for _, pkg := range pkgs {
		for importPath := range pkg.Imports {
			for _, bad := range forbidden {
				if strings.Contains(importPath, bad) {
					t.Errorf("package %s imports forbidden path %q (contains %q)",
						pkg.PkgPath, importPath, bad)
				}
			}
		}
	}
}
