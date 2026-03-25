package evolution

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// CodeAnalysis holds analysis results for the codebase.
type CodeAnalysis struct {
	TODOs      []string // file:line: content
	Hotspots   []string // most changed files
	NoTestPkgs []string // packages without test files
	BuildOK    bool
	TestOK     bool
}

// AnalyzeCodebase scans the repo for improvement opportunities.
func AnalyzeCodebase(repoPath string) CodeAnalysis {
	analysis := CodeAnalysis{BuildOK: true, TestOK: true}

	// Check build
	if err := exec.Command("go", "build", "./...").Run(); err != nil {
		analysis.BuildOK = false
	}

	// Check tests
	if err := exec.Command("go", "test", "./...").Run(); err != nil {
		analysis.TestOK = false
	}

	// Find TODOs/FIXMEs
	analysis.TODOs = findTODOs(repoPath)

	// Find hotspots (most changed files)
	analysis.Hotspots = findHotspots(repoPath)

	// Find packages without tests
	analysis.NoTestPkgs = findNoTestPackages(repoPath)

	return analysis
}

// FormatAnalysis returns a human-readable summary for the agent.
func (a CodeAnalysis) FormatAnalysis() string {
	var sb strings.Builder

	if !a.BuildOK {
		sb.WriteString("🔴 BUILD BROKEN — fix this first!\n\n")
	}
	if !a.TestOK {
		sb.WriteString("🔴 TESTS FAILING — fix this first!\n\n")
	}

	if len(a.TODOs) > 0 {
		sb.WriteString("## TODOs found in code\n\n")
		limit := 10
		if len(a.TODOs) < limit {
			limit = len(a.TODOs)
		}
		for _, todo := range a.TODOs[:limit] {
			sb.WriteString("- " + todo + "\n")
		}
		sb.WriteString("\n")
	}

	if len(a.Hotspots) > 0 {
		sb.WriteString("## Most changed files (hotspots)\n\n")
		limit := 5
		if len(a.Hotspots) < limit {
			limit = len(a.Hotspots)
		}
		for _, h := range a.Hotspots[:limit] {
			sb.WriteString("- " + h + "\n")
		}
		sb.WriteString("\n")
	}

	if len(a.NoTestPkgs) > 0 {
		sb.WriteString("## Packages without tests\n\n")
		for _, pkg := range a.NoTestPkgs {
			sb.WriteString("- " + pkg + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func findTODOs(repoPath string) []string {
	out, err := exec.Command("grep", "-rn", "--include=*.go",
		"TODO\\|FIXME\\|HACK\\|XXX",
		filepath.Join(repoPath, "cmd"),
		filepath.Join(repoPath, "internal"),
	).CombinedOutput()
	if err != nil {
		return nil
	}

	var results []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// Strip repo path prefix
		line = strings.TrimPrefix(line, repoPath+"/")
		results = append(results, line)
	}
	return results
}

func findHotspots(repoPath string) []string {
	out, err := exec.Command("git", "-C", repoPath, "log", "--pretty=format:",
		"--name-only", "-50").CombinedOutput()
	if err != nil {
		return nil
	}

	counts := map[string]int{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "_test.go") {
			continue
		}
		counts[line]++
	}

	type entry struct {
		name  string
		count int
	}
	var entries []entry
	for name, count := range counts {
		entries = append(entries, entry{name, count})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})

	var results []string
	limit := 10
	if len(entries) < limit {
		limit = len(entries)
	}
	for _, e := range entries[:limit] {
		results = append(results, e.name+" (changed "+string(rune('0'+e.count))+"x)")
	}
	return results
}

func findNoTestPackages(repoPath string) []string {
	out, err := exec.Command("go", "list", "./...").Output()
	if err != nil {
		return nil
	}

	var noTest []string
	for _, pkg := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if pkg == "" || strings.Contains(pkg, "vendor") {
			continue
		}
		// Check if package has test files
		testOut, _ := exec.Command("go", "list", "-f", "{{.TestGoFiles}}", pkg).Output()
		if strings.TrimSpace(string(testOut)) == "[]" {
			noTest = append(noTest, pkg)
		}
	}
	return noTest
}
