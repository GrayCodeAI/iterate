package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// git helpers — /log, /stash, /branch, /checkout, /merge, /tag, /revert-file
// ---------------------------------------------------------------------------

func gitLog(repoPath string, n int) string {
	out, err := exec.Command("git", "-C", repoPath, "log",
		fmt.Sprintf("-%d", n),
		"--oneline", "--decorate", "--color").Output()
	if err != nil {
		return fmt.Sprintf("git log error: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func gitStash(repoPath string, pop bool) error {
	args := []string{"-C", repoPath, "stash"}
	if pop {
		args = append(args, "pop")
	}
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitBranches(repoPath string) []string {
	out, err := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)").Output()
	if err != nil {
		return nil
	}
	var branches []string
	for _, b := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if b = strings.TrimSpace(b); b != "" {
			branches = append(branches, b)
		}
	}
	return branches
}

func gitCurrentBranch(repoPath string) string {
	out, _ := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output()
	return strings.TrimSpace(string(out))
}

func gitTags(repoPath string) []string {
	out, err := exec.Command("git", "-C", repoPath, "tag", "--sort=-creatordate").Output()
	if err != nil {
		return nil
	}
	var tags []string
	for _, t := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// ---------------------------------------------------------------------------
// /hotspots — files changed most in git history
// ---------------------------------------------------------------------------

func gitHotspots(repoPath string, n int) string {
	out, err := exec.Command("git", "-C", repoPath, "log",
		"--pretty=format:", "--name-only", "--diff-filter=M").Output()
	if err != nil {
		return ""
	}
	freq := map[string]int{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			freq[line]++
		}
	}
	type entry struct {
		file  string
		count int
	}
	var entries []entry
	for f, c := range freq {
		entries = append(entries, entry{f, c})
	}
	// sort descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})
	if len(entries) > n {
		entries = entries[:n]
	}
	var lines []string
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("  %3d  %s", e.count, e.file))
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// /contributors
// ---------------------------------------------------------------------------

func gitContributors(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "shortlog", "-sn", "--no-merges").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ---------------------------------------------------------------------------
// /amend — amend last commit
// ---------------------------------------------------------------------------

func gitAmend(repoPath, msg string) error {
	args := []string{"-C", repoPath, "commit", "--amend"}
	if msg != "" {
		args = append(args, "-m", msg)
	} else {
		args = append(args, "--no-edit")
	}
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// /stash-list
// ---------------------------------------------------------------------------

func gitStashList(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "stash", "list").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
