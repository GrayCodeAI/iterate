// Package autonomous - Task 23: Sandbox container templates for common languages
package autonomous

import (
	"fmt"
	"os"
	"time"
)

// SandboxTemplate provides a pre-configured sandbox setup for a specific language/runtime.
type SandboxTemplate struct {
	Name            string                `json:"name"`
	Image           string                `json:"image"`
	Description     string                `json:"description"`
	DefaultWorkDir  string                `json:"default_work_dir"`
	ResourceLimits  SandboxResourceLimits `json:"resource_limits"`
	EnvVars         map[string]string     `json:"env_vars"`
	DefaultPackages []string              `json:"default_packages"`
}

// Predefined sandbox templates for common development environments.
var sandboxTemplates = map[string]SandboxTemplate{
	"node": {
		Name:           "Node.js",
		Image:          "node:18-slim",
		Description:    "Node.js 18 runtime with npm",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 512,
			MemoryMB:  512,
			PidsLimit: 100,
			Timeout:   5 * time.Minute,
		},
		EnvVars: map[string]string{
			"NODE_ENV": "development",
		},
		DefaultPackages: []string{},
	},
	"node-20": {
		Name:           "Node.js 20",
		Image:          "node:20-slim",
		Description:    "Node.js 20 runtime with npm",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 512,
			MemoryMB:  512,
			PidsLimit: 100,
			Timeout:   5 * time.Minute,
		},
		EnvVars: map[string]string{
			"NODE_ENV": "development",
		},
	},
	"python": {
		Name:           "Python 3.11",
		Image:          "python:3.11-slim",
		Description:    "Python 3.11 runtime with pip",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 512,
			MemoryMB:  512,
			PidsLimit: 100,
			Timeout:   5 * time.Minute,
		},
		EnvVars: map[string]string{
			"PYTHONDONTWRITEBYTECODE": "1",
			"PYTHONUNBUFFERED":        "1",
		},
	},
	"python-3.12": {
		Name:           "Python 3.12",
		Image:          "python:3.12-slim",
		Description:    "Python 3.12 runtime with pip",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 512,
			MemoryMB:  512,
			PidsLimit: 100,
			Timeout:   5 * time.Minute,
		},
		EnvVars: map[string]string{
			"PYTHONDONTWRITEBYTECODE": "1",
			"PYTHONUNBUFFERED":        "1",
		},
	},
	"go": {
		Name:           "Go 1.22",
		Image:          "golang:1.22-bookworm",
		Description:    "Go 1.22 toolchain with Go modules",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 1024,
			MemoryMB:  1024,
			PidsLimit: 200,
			Timeout:   10 * time.Minute,
		},
		EnvVars: map[string]string{
			"GOCACHE":    "/tmp/go-cache",
			"GOMODCACHE": "/tmp/go-mod-cache",
		},
	},
	"go-1.23": {
		Name:           "Go 1.23",
		Image:          "golang:1.23-bookworm",
		Description:    "Go 1.23 toolchain with Go modules",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 1024,
			MemoryMB:  1024,
			PidsLimit: 200,
			Timeout:   10 * time.Minute,
		},
		EnvVars: map[string]string{
			"GOCACHE":    "/tmp/go-cache",
			"GOMODCACHE": "/tmp/go-mod-cache",
		},
	},
	"rust": {
		Name:           "Rust Latest",
		Image:          "rust:1.76-slim-bookworm",
		Description:    "Rust toolchain with Cargo",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 1024,
			MemoryMB:  2048,
			PidsLimit: 200,
			Timeout:   15 * time.Minute,
		},
		EnvVars: map[string]string{
			"CARGO_HOME": "/tmp/cargo",
			"RUSTFLAGS":  "-C opt-level=0",
		},
	},
	"rust-1.77": {
		Name:           "Rust 1.77",
		Image:          "rust:1.77-slim-bookworm",
		Description:    "Rust 1.77 toolchain with Cargo",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 1024,
			MemoryMB:  2048,
			PidsLimit: 200,
			Timeout:   15 * time.Minute,
		},
		EnvVars: map[string]string{
			"CARGO_HOME": "/tmp/cargo",
		},
	},
	"alpine": {
		Name:           "Alpine Linux",
		Image:          "alpine:3.19",
		Description:    "Minimal Alpine Linux for shell scripts",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 256,
			MemoryMB:  128,
			PidsLimit: 50,
			Timeout:   2 * time.Minute,
		},
	},
	"ubuntu": {
		Name:           "Ubuntu 22.04",
		Image:          "ubuntu:22.04",
		Description:    "Ubuntu 22.04 LTS base image",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 512,
			MemoryMB:  512,
			PidsLimit: 100,
			Timeout:   5 * time.Minute,
		},
		EnvVars: map[string]string{
			"DEBIAN_FRONTEND": "noninteractive",
		},
	},
	"java": {
		Name:           "Java 21",
		Image:          "eclipse-temurin:21-jdk-jammy",
		Description:    "Eclipse Temurin JDK 21",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 1024,
			MemoryMB:  2048,
			PidsLimit: 200,
			Timeout:   10 * time.Minute,
		},
		EnvVars: map[string]string{
			"JAVA_OPTS": "-Xmx1024m",
		},
	},
	"deno": {
		Name:           "Deno",
		Image:          "denoland/deno:2.0.0",
		Description:    "Deno 2.0 runtime",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 512,
			MemoryMB:  512,
			PidsLimit: 100,
			Timeout:   5 * time.Minute,
		},
	},
	"bun": {
		Name:           "Bun",
		Image:          "oven/bun:1-debian",
		Description:    "Bun JavaScript runtime",
		DefaultWorkDir: "/workspace",
		ResourceLimits: SandboxResourceLimits{
			CPUShares: 512,
			MemoryMB:  512,
			PidsLimit: 100,
			Timeout:   5 * time.Minute,
		},
	},
}

// GetSandboxTemplate returns a sandbox template by name.
func GetSandboxTemplate(name string) (SandboxTemplate, bool) {
	t, ok := sandboxTemplates[name]
	return t, ok
}

// ListSandboxTemplates returns all available sandbox templates.
func ListSandboxTemplates() []SandboxTemplate {
	templates := make([]SandboxTemplate, 0, len(sandboxTemplates))
	for _, t := range sandboxTemplates {
		templates = append(templates, t)
	}
	return templates
}

// SandboxTemplateNames returns the names of all available templates.
func SandboxTemplateNames() []string {
	names := make([]string, 0, len(sandboxTemplates))
	for name := range sandboxTemplates {
		names = append(names, name)
	}
	return names
}

// NewSandboxFromTemplate creates a sandbox from a template.
func NewSandboxFromTemplate(templateName string, repoPath string) (*Sandbox, error) {
	template, ok := GetSandboxTemplate(templateName)
	if !ok {
		return nil, fmt.Errorf("unknown sandbox template: %s (available: %v)", templateName, SandboxTemplateNames())
	}

	config := SandboxConfig{
		Image:   template.Image,
		WorkDir: template.DefaultWorkDir,
		VolumeMounts: []VolumeMount{
			{
				HostPath:      repoPath,
				ContainerPath: template.DefaultWorkDir,
				ReadOnly:      false,
			},
		},
		EnvVars:        template.EnvVars,
		NetworkEnabled: false,
		ResourceLimits: template.ResourceLimits,
		ExecTimeout:    template.ResourceLimits.Timeout,
		CleanupOnExit:  true,
		PullImage:      true,
	}

	return NewSandbox(config), nil
}

// DetectSandboxTemplate attempts to detect the appropriate sandbox template
// based on files present in the repository.
func DetectSandboxTemplate(repoPath string) string {
	// Priority order for detection
	checks := []struct {
		file     string
		template string
	}{
		{"go.mod", "go"},
		{"Cargo.toml", "rust"},
		{"pyproject.toml", "python"},
		{"setup.py", "python"},
		{"requirements.txt", "python"},
		{"package.json", "node"},
		{"deno.json", "deno"},
		{"deno.jsonc", "deno"},
		{"bun.lockb", "bun"},
		{"pom.xml", "java"},
		{"build.gradle", "java"},
	}

	for _, check := range checks {
		if fileExists(repoPath + "/" + check.file) {
			return check.template
		}
	}

	// Default to alpine for unknown projects
	return "alpine"
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
