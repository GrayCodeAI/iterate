package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

// ---------------------------------------------------------------------------
// /tree — git ls-files based tree display
// ---------------------------------------------------------------------------

func buildProjectTree(repoPath string, maxDepth int) string {
	out, err := exec.Command("git", "-C", repoPath, "ls-files").Output()
	var paths []string
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		for _, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if p = strings.TrimSpace(p); p != "" {
				paths = append(paths, p)
			}
		}
	} else {
		// Fallback: filesystem walk
		_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "target" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(repoPath, path)
			paths = append(paths, rel)
			return nil
		})
	}
	sort.Strings(paths)
	return formatTreeFromPaths(paths, maxDepth)
}

type treeNode struct {
	children map[string]*treeNode
	isFile   bool
}

func formatTreeFromPaths(paths []string, maxDepth int) string {
	root := &treeNode{children: make(map[string]*treeNode)}
	for _, p := range paths {
		parts := strings.Split(p, "/")
		if maxDepth > 0 && len(parts) > maxDepth {
			continue
		}
		cur := root
		for _, part := range parts {
			if _, ok := cur.children[part]; !ok {
				cur.children[part] = &treeNode{children: make(map[string]*treeNode)}
			}
			cur = cur.children[part]
		}
		cur.isFile = true
	}

	var lines []string
	var walk func(n *treeNode, prefix string)
	walk = func(n *treeNode, prefix string) {
		keys := make([]string, 0, len(n.children))
		for k := range n.children {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, key := range keys {
			child := n.children[key]
			isLast := i == len(keys)-1
			connector := "├── "
			nextPrefix := prefix + "│   "
			if isLast {
				connector = "└── "
				nextPrefix = prefix + "    "
			}
			if child.isFile {
				lines = append(lines, prefix+connector+key)
			} else {
				lines = append(lines, prefix+connector+key+"/")
				walk(child, nextPrefix)
			}
		}
	}
	walk(root, "")
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// /index — structured project file index
// ---------------------------------------------------------------------------

type indexEntry struct {
	Path    string
	Lines   int
	Summary string
}

var binaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
	".svg": true, ".wasm": true, ".bin": true, ".exe": true, ".pdf": true,
	".zip": true, ".tar": true, ".gz": true, ".so": true, ".a": true,
	".dylib": true, ".dll": true, ".webp": true, ".mp4": true, ".mp3": true,
}

func buildProjectIndex(repoPath string) []indexEntry {
	out, err := exec.Command("git", "-C", repoPath, "ls-files").Output()
	var paths []string
	if err == nil {
		for _, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if p = strings.TrimSpace(p); p != "" {
				paths = append(paths, p)
			}
		}
	}

	var entries []indexEntry
	for _, rel := range paths {
		ext := strings.ToLower(filepath.Ext(rel))
		if binaryExtensions[ext] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoPath, rel))
		if err != nil {
			continue
		}
		content := string(data)
		lines := strings.Count(content, "\n") + 1
		summary := extractFirstMeaningfulLine(content)
		entries = append(entries, indexEntry{Path: rel, Lines: lines, Summary: summary})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

func formatProjectIndex(entries []indexEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		if e.Summary != "" {
			sb.WriteString(fmt.Sprintf("  %-50s %5d  %s\n", e.Path, e.Lines, e.Summary))
		} else {
			sb.WriteString(fmt.Sprintf("  %-50s %5d\n", e.Path, e.Lines))
		}
	}
	return sb.String()
}

func extractFirstMeaningfulLine(content string) string {
	for _, l := range strings.Split(content, "\n") {
		l = strings.TrimSpace(l)
		if l == "" ||
			strings.HasPrefix(l, "//") || strings.HasPrefix(l, "#") ||
			strings.HasPrefix(l, "/*") || strings.HasPrefix(l, "*") ||
			strings.HasPrefix(l, "package ") || strings.HasPrefix(l, "import ") ||
			strings.HasPrefix(l, "module ") || strings.HasPrefix(l, "use ") ||
			strings.HasPrefix(l, "\"use strict\"") {
			continue
		}
		if len(l) > 80 {
			l = l[:80] + "…"
		}
		return l
	}
	return ""
}

// ---------------------------------------------------------------------------
// /pkgdoc — look up Go package documentation on pkg.go.dev
// ---------------------------------------------------------------------------

func fetchGoPkgDoc(pkg string) (string, error) {
	url := "https://pkg.go.dev/" + pkg
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return "", fmt.Errorf("package %q not found on pkg.go.dev", pkg)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	if err != nil {
		return "", err
	}
	text := stripHTMLTags(string(body))
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimSpace(l)
		if t != "" {
			lines = append(lines, t)
		}
	}
	result := strings.Join(lines, "\n")
	if len(result) > 3000 {
		result = result[:3000] + "\n…[truncated — see " + url + " for full docs]"
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// generateIterateMD — scaffold an ITERATE.md context file for any project
// ---------------------------------------------------------------------------

func generateIterateMD(repoPath string) string {
	name := filepath.Base(repoPath)
	pt := detectProjectType(repoPath)

	var buildCmd, testCmd, lintCmd string
	switch pt {
	case projectTypeGo:
		buildCmd = "go build ./..."
		testCmd = "go test ./..."
		lintCmd = "go vet ./..."
	case projectTypeRust:
		buildCmd = "cargo build"
		testCmd = "cargo test"
		lintCmd = "cargo clippy"
	case projectTypeNode:
		buildCmd = "npm run build"
		testCmd = "npm test"
		lintCmd = "npm run lint"
	case projectTypePython:
		buildCmd = "python -m build"
		testCmd = "python -m pytest"
		lintCmd = "flake8 ."
	case projectTypeMake:
		buildCmd = "make"
		testCmd = "make test"
		lintCmd = "make lint"
	default:
		buildCmd = "# your build command"
		testCmd = "# your test command"
		lintCmd = "# your lint command"
	}

	candidates := []string{
		"README.md", "go.mod", "Cargo.toml", "package.json",
		"Makefile", "main.go", "src/main.rs", "index.js", "index.ts",
	}
	var importantFiles []string
	for _, f := range candidates {
		if _, err := os.Stat(filepath.Join(repoPath, f)); err == nil {
			importantFiles = append(importantFiles, f)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s — iterate context\n\n", name))
	sb.WriteString(fmt.Sprintf("Generated: %s | Type: %s\n\n", time.Now().Format("2006-01-02"), pt.String()))
	sb.WriteString("## About\n\nDescribe what this project does.\n\n")
	sb.WriteString("## Build & Test\n\n```bash\n")
	sb.WriteString(fmt.Sprintf("%s    # build\n", buildCmd))
	sb.WriteString(fmt.Sprintf("%s    # test\n", testCmd))
	sb.WriteString(fmt.Sprintf("%s    # lint\n", lintCmd))
	sb.WriteString("```\n\n")
	sb.WriteString("## Coding Conventions\n\n")
	sb.WriteString("- Keep functions small and focused\n")
	sb.WriteString("- Write tests for new functionality\n")
	sb.WriteString("- Use meaningful variable names\n\n")
	if len(importantFiles) > 0 {
		sb.WriteString("## Important Files\n\n")
		for _, f := range importantFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("## Notes\n\nAdd project-specific notes for iterate here.\n")
	return sb.String()
}
