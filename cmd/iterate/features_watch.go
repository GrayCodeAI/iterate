package main

import (
	"context"
	"sync"
)

// ---------------------------------------------------------------------------
// /watch — watch repo for file changes and auto-run tests
// ---------------------------------------------------------------------------

// watchState tracks the active watch goroutine.
var watchCancel context.CancelFunc
var watchMu sync.Mutex

func stopWatch() {
	watchMu.Lock()
	defer watchMu.Unlock()
	if watchCancel != nil {
		watchCancel()
		watchCancel = nil
	}
}
