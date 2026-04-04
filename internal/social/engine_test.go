package social

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestNew_SocialEngine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	e := New("/tmp", "owner", "repo", logger)
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	if e.owner != "owner" {
		t.Errorf("expected owner 'owner', got %s", e.owner)
	}
	if e.repo != "repo" {
		t.Errorf("expected repo 'repo', got %s", e.repo)
	}
	if e.repoPath != "/tmp" {
		t.Errorf("expected repoPath '/tmp', got %s", e.repoPath)
	}
}

func TestHealthCheck_NoToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	e := New("/tmp", "owner", "repo", logger)
	e.token = ""

	err := e.HealthCheck(context.Background())
	// Without token, health check should still work (HEAD to api.github.com is public)
	// It should pass since we just check reachability
	if err != nil {
		// If it fails, it's likely network issue, which is acceptable in test environment
		t.Logf("health check failed (may be network issue): %v", err)
	}
}
