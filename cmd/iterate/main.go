package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func main() {
	f := parseFlags()
	isREPL := f.chat || (!f.evolve && !f.socialOnly && f.phase == "")
	logger := setupLogging(isREPL)

	absRepo, err := filepath.Abs(f.repoPath)
	if err != nil {
		logger.Error("invalid repo path", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	runMode(ctx, f, absRepo, logger)
}

func incrementDayCount(repoPath string) {
	path := filepath.Join(repoPath, "DAY_COUNT")
	data, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		fmt.Fprintf(os.Stderr, "warn: failed to read DAY_COUNT: %v\n", readErr)
	}
	n, atoiErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if atoiErr != nil && len(data) > 0 {
		fmt.Fprintf(os.Stderr, "warn: failed to parse DAY_COUNT %q: %v\n", strings.TrimSpace(string(data)), atoiErr)
	}
	if writeErr := os.WriteFile(path, []byte(fmt.Sprintf("%d", n+1)), 0o644); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warn: failed to write DAY_COUNT: %v\n", writeErr)
	}
}

func saveSessionToFile(path string, messages []iteragent.Message) error {
	_ = os.MkdirAll(filepath.Dir(path), 0o755) // best-effort cleanup
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func loadSessionFromFile(path string) ([]iteragent.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	var messages []iteragent.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("unmarshal messages: %w", err)
	}
	return messages, nil
}
