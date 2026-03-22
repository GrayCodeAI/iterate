package commands

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
// /tree — git ls-files based tree display
// ---------------------------------------------------------------------------

type TreeNode struct {
	children map[string]*TreeNode
	isFile   bool
}

func BuildProjectTree(repoPath string, maxDepth int) string {
	out, err := exec.Command("git", "-C", repoPath, "ls-files").Output()
	var paths []string
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		for _, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if p = strings.TrimSpace(p); p != "" {
				paths = append(paths, p)
			}
		}
	} else {
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
	return FormatTreeFromPaths(paths, maxDepth)
}

func FormatTreeFromPaths(paths []string, maxDepth int) string {
	root := &TreeNode{children: make(map[string]*TreeNode)}
	for _, p := range paths {
		parts := strings.Split(p, "/")
		if maxDepth > 0 && len(parts) > maxDepth {
			continue
		}
		cur := root
		for _, part := range parts {
			if _, ok := cur.children[part]; !ok {
				cur.children[part] = &TreeNode{children: make(map[string]*TreeNode)}
			}
			cur = cur.children[part]
		}
		cur.isFile = true
	}

	var lines []string
	var walk func(n *TreeNode, prefix string)
	walk = func(n *TreeNode, prefix string) {
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

type IndexEntry struct {
	Path    string
	Lines   int
	Summary string
}

var BinaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
	".svg": true, ".wasm": true, ".bin": true, ".exe": true, ".pdf": true,
	".zip": true, ".tar": true, ".gz": true, ".so": true, ".a": true,
	".dylib": true, ".dll": true, ".webp": true, ".mp4": true, ".mp3": true,
}

func BuildProjectIndex(repoPath string) []IndexEntry {
	out, err := exec.Command("git", "-C", repoPath, "ls-files").Output()
	var paths []string
	if err == nil {
		for _, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if p = strings.TrimSpace(p); p != "" {
				paths = append(paths, p)
			}
		}
	}

	var entries []IndexEntry
	for _, rel := range paths {
		ext := strings.ToLower(filepath.Ext(rel))
		if BinaryExtensions[ext] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoPath, rel))
		if err != nil {
			continue
		}
		content := string(data)
		lines := strings.Count(content, "\n") + 1
		summary := ExtractFirstMeaningfulLine(content)
		entries = append(entries, IndexEntry{Path: rel, Lines: lines, Summary: summary})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

func FormatProjectIndex(entries []IndexEntry) string {
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

func ExtractFirstMeaningfulLine(content string) string {
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

func FetchGoPkgDoc(pkg string) (string, error) {
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
	text := StripHTMLTags(string(body))
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

func StripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
			b.WriteRune(' ')
		} else if !inTag {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// generateIterateMD — scaffold an ITERATE.md context file for any project
// ---------------------------------------------------------------------------

func GenerateIterateMD(repoPath string) string {
	name := filepath.Base(repoPath)
	pt := detectProjectType(repoPath)
	buildCmd, testCmd, lintCmd := resolveBuildCommands(pt)
	importantFiles := findImportantFiles(repoPath)

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

// resolveBuildCommands returns build/test/lint commands for the given project type.
func resolveBuildCommands(pt projType) (build, test, lint string) {
	switch pt {
	case projGo:
		return "go build ./...", "go test ./...", "go vet ./..."
	case projRust:
		return "cargo build", "cargo test", "cargo clippy"
	case projNode:
		return "npm run build", "npm test", "npm run lint"
	case projPython:
		return "python -m build", "python -m pytest", "flake8 ."
	case projMake:
		return "make", "make test", "make lint"
	default:
		return "# your build command", "# your test command", "# your lint command"
	}
}

// findImportantFiles returns candidate files that exist in the repo.
func findImportantFiles(repoPath string) []string {
	candidates := []string{
		"README.md", "go.mod", "Cargo.toml", "package.json",
		"Makefile", "main.go", "src/main.rs", "index.js", "index.ts",
	}
	var found []string
	for _, f := range candidates {
		if _, err := os.Stat(filepath.Join(repoPath, f)); err == nil {
			found = append(found, f)
		}
	}
	return found
}

type projType int

const (
	projGo projType = iota
	projRust
	projNode
	projPython
	projMake
	projUnknown
)

func (pt projType) String() string {
	switch pt {
	case projGo:
		return "Go"
	case projRust:
		return "Rust"
	case projNode:
		return "Node"
	case projPython:
		return "Python"
	case projMake:
		return "Make"
	default:
		return "Unknown"
	}
}

func detectProjectType(dir string) projType {
	check := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}
	switch {
	case check("go.mod"):
		return projGo
	case check("Cargo.toml"):
		return projRust
	case check("package.json"):
		return projNode
	case check("requirements.txt"), check("pyproject.toml"), check("setup.py"):
		return projPython
	case check("Makefile"), check("makefile"):
		return projMake
	default:
		return projUnknown
	}
}
