// Package autonomous - Task 23: Tests for sandbox container templates
package autonomous

import (
	"testing"
	"time"
)

func TestGetSandboxTemplate(t *testing.T) {
	// Test existing templates
	template, ok := GetSandboxTemplate("node")
	if !ok {
		t.Error("expected node template to exist")
	}
	if template.Name != "Node.js" {
		t.Errorf("expected Name 'Node.js', got %q", template.Name)
	}
	if template.Image != "node:18-slim" {
		t.Errorf("expected Image 'node:18-slim', got %q", template.Image)
	}

	// Test non-existent template
	_, ok = GetSandboxTemplate("nonexistent")
	if ok {
		t.Error("expected nonexistent template to not exist")
	}
}

func TestListSandboxTemplates(t *testing.T) {
	templates := ListSandboxTemplates()
	if len(templates) == 0 {
		t.Error("expected at least one template")
	}

	// Verify all templates have required fields
	for _, tmpl := range templates {
		if tmpl.Name == "" {
			t.Error("template has empty Name")
		}
		if tmpl.Image == "" {
			t.Errorf("template %q has empty Image", tmpl.Name)
		}
		if tmpl.DefaultWorkDir == "" {
			t.Errorf("template %q has empty DefaultWorkDir", tmpl.Name)
		}
	}
}

func TestSandboxTemplateNames(t *testing.T) {
	names := SandboxTemplateNames()
	if len(names) == 0 {
		t.Error("expected at least one template name")
	}

	// Verify expected templates exist
	expected := []string{"node", "python", "go", "rust", "alpine"}
	for _, e := range expected {
		found := false
		for _, n := range names {
			if n == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected template %q to exist", e)
		}
	}
}

func TestNewSandboxFromTemplate(t *testing.T) {
	sandbox, err := NewSandboxFromTemplate("go", "/tmp/test-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sandbox == nil {
		t.Fatal("expected sandbox to be created")
	}
	if sandbox.image != "golang:1.22-bookworm" {
		t.Errorf("expected image 'golang:1.22-bookworm', got %q", sandbox.image)
	}
	if sandbox.workDir != "/workspace" {
		t.Errorf("expected workDir '/workspace', got %q", sandbox.workDir)
	}
	if len(sandbox.volumeMounts) != 1 {
		t.Errorf("expected 1 volume mount, got %d", len(sandbox.volumeMounts))
	}
	if sandbox.volumeMounts[0].HostPath != "/tmp/test-repo" {
		t.Errorf("expected HostPath '/tmp/test-repo', got %q", sandbox.volumeMounts[0].HostPath)
	}

	// Test invalid template
	_, err = NewSandboxFromTemplate("invalid", "/tmp/test-repo")
	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestSandboxTemplateResourceLimits(t *testing.T) {
	template, _ := GetSandboxTemplate("go")
	if template.ResourceLimits.MemoryMB < 512 {
		t.Errorf("Go template should have at least 512MB memory, got %d", template.ResourceLimits.MemoryMB)
	}
	if template.ResourceLimits.Timeout < time.Minute {
		t.Errorf("Go template should have at least 1 minute timeout, got %v", template.ResourceLimits.Timeout)
	}
}

func TestTask23SandboxTemplatesStruct(t *testing.T) {
	// Verify all predefined templates have proper struct initialization
	templates := ListSandboxTemplates()
	for _, tmpl := range templates {
		if tmpl.ResourceLimits.Timeout == 0 {
			t.Errorf("template %q has zero Timeout", tmpl.Name)
		}
		if tmpl.ResourceLimits.MemoryMB == 0 {
			t.Errorf("template %q has zero MemoryMB", tmpl.Name)
		}
	}
}
