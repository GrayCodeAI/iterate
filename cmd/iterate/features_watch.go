package main

import (
	"context"
	"fmt"
	"io/fs"
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

func startWatch(repoPath string) {
	watchMu.Lock()
	defer watchMu.Unlock()
	if watchCancel != nil {
		watchCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	watchCancel = cancel

	go func() {
		fmt.Printf("%s[watch] monitoring %s for changes…%s\n", colorDim, repoPath, colorReset)
		lastMod := latestModTime(repoPath)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("%s[watch] stopped%s\n", colorDim, colorReset)
				return
			case <-ticker.C:
				cur := latestModTime(repoPath)
				if cur.After(lastMod) {
					lastMod = cur
					fmt.Printf("\n%s[watch] change detected — running tests…%s\n", colorYellow, colorReset)
					cmd := exec.Command("go", "test", "./...")
					cmd.Dir = repoPath
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					if err := cmd.Run(); err != nil {
						fmt.Printf("%s[watch] tests failed%s\n", colorRed, colorReset)
					} else {
						fmt.Printf("%s[watch] ✓ all tests pass%s\n", colorLime, colorReset)
					}
				}
			}
		}
	}()
}

func stopWatch() {
	watchMu.Lock()
	defer watchMu.Unlock()
	if watchCancel != nil {
		watchCancel()
		watchCancel = nil
	}
}

func latestModTime(repoPath string) time.Time {
	var latest time.Time
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest
}
