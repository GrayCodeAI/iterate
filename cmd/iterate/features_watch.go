package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// /watch — watch repo for file changes and auto-run tests
// ---------------------------------------------------------------------------

// watchState tracks the active watch goroutine.
var watchCancel context.CancelFunc
var watchMu sync.Mutex

// watchConfig holds debounce and filter settings for the watcher.
var watchConfig = struct {
	debounce time.Duration
	// include is a list of glob suffixes to watch (e.g. ".go", ".ts").
	// If empty, all files are watched.
	include []string
	// exclude is a list of path substrings to ignore.
	exclude []string
}{
	// 2-second debounce: collect all file change events in the window and send
	// ONE batched prompt listing every changed file instead of per-file prompts.
	debounce: 2 * time.Second,
	exclude:  []string{".git", "node_modules", ".iterate"},
}

func stopWatch() {
	watchMu.Lock()
	defer watchMu.Unlock()
	if watchCancel != nil {
		watchCancel()
		watchCancel = nil
	}
}

// startWatch starts a polling-based file watcher that runs `go test ./...`
// (or a custom command) whenever a watched file changes.
// It debounces rapid changes and respects include/exclude filter patterns.
func startWatch(repoPath string) {
	watchMu.Lock()
	if watchCancel != nil {
		watchCancel() // stop any previous watcher
	}
	ctx, cancel := context.WithCancel(context.Background())
	watchCancel = cancel
	watchMu.Unlock()

	go runWatcher(ctx, repoPath)
}

// runWatcher polls for file modifications using mtimes (no fsnotify dependency).
// File change events are batched: all files that change within the debounce window
// are collected and trigger a single test run, rather than one run per file.
func runWatcher(ctx context.Context, repoPath string) {
	snapshots := snapshotMTimes(repoPath)

	var debounceTimer *time.Timer
	var debounceMu sync.Mutex
	// pendingChanged accumulates file paths seen during the current debounce window.
	var pendingChanged []string
	pendingSet := make(map[string]struct{})

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := snapshotMTimes(repoPath)
			changed := diffSnapshots(snapshots, current)
			snapshots = current

			if len(changed) == 0 {
				continue
			}

			// Accumulate changed files across the debounce window (deduplicated).
			debounceMu.Lock()
			for _, p := range changed {
				if _, seen := pendingSet[p]; !seen {
					pendingSet[p] = struct{}{}
					pendingChanged = append(pendingChanged, p)
				}
			}

			// Reset the timer: fire after debounce elapses with no new changes.
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			// Capture the batch for the closure.
			batchSnapshot := append([]string(nil), pendingChanged...)
			pendingChanged = pendingChanged[:0]
			for k := range pendingSet {
				delete(pendingSet, k)
			}
			debounceTimer = time.AfterFunc(watchConfig.debounce, func() {
				fmt.Printf("\n%s[watch] %d file(s) changed — running tests…%s\n",
					colorYellow, len(batchSnapshot), colorReset)
				for _, p := range batchSnapshot {
					rel, _ := filepath.Rel(repoPath, p)
					fmt.Printf("  %s%s%s\n", colorDim, rel, colorReset)
				}
				runWatchTests(repoPath)
			})
			debounceMu.Unlock()
		}
	}
}

// snapshotMTimes returns a map of path → mtime for all watched files.
func snapshotMTimes(repoPath string) map[string]time.Time {
	result := make(map[string]time.Time)
	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			// Skip excluded directories.
			for _, ex := range watchConfig.exclude {
				if strings.Contains(path, ex) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !shouldWatch(path) {
			return nil
		}
		result[path] = info.ModTime()
		return nil
	})
	return result
}

// shouldWatch returns true if path passes include/exclude filters.
func shouldWatch(path string) bool {
	for _, ex := range watchConfig.exclude {
		if strings.Contains(path, ex) {
			return false
		}
	}
	if len(watchConfig.include) == 0 {
		return true
	}
	for _, inc := range watchConfig.include {
		if strings.HasSuffix(path, inc) {
			return true
		}
	}
	return false
}

// diffSnapshots returns paths that are new or have a newer mtime.
func diffSnapshots(old, current map[string]time.Time) []string {
	var changed []string
	for path, newTime := range current {
		if oldTime, ok := old[path]; !ok || newTime.After(oldTime) {
			changed = append(changed, path)
		}
	}
	return changed
}

// runWatchTests runs the test command and prints output.
func runWatchTests(repoPath string) {
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("%s[watch] tests FAILED%s\n%s\n", colorRed, colorReset, string(out))
	} else {
		fmt.Printf("%s[watch] ✓ tests passed%s\n", colorLime, colorReset)
		if len(out) > 0 {
			fmt.Printf("%s%s%s\n", colorDim, strings.TrimSpace(string(out)), colorReset)
		}
	}
}
