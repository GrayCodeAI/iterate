package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Snapshot struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	CommitSHA string    `json:"commit_sha"`
	Message   string    `json:"message"`
	Files     []string  `json:"files"`
}

type SnapshotManager struct {
	repoPath  string
	snapshots map[string]*Snapshot
	logger    *slog.Logger
	engine    *Engine
}

func NewSnapshotManager(repoPath string, logger *slog.Logger) *SnapshotManager {
	return &SnapshotManager{
		repoPath:  repoPath,
		snapshots: make(map[string]*Snapshot),
		logger:    logger,
	}
}

func (sm *SnapshotManager) Create(ctx context.Context, message string) (*Snapshot, error) {
	snapshotID := generateTraceID()

	cmd := fmt.Sprintf("git add -A && git commit -m %q", fmt.Sprintf("Snapshot %s: %s", snapshotID[:8], message))
	out, err := sm.runGitCommand(ctx, cmd)
	if err != nil {
		if strings.Contains(out, "nothing to commit") {
			snapshotID = generateTraceID()
			cmd = fmt.Sprintf("git commit --allow-empty -m %q", fmt.Sprintf("Snapshot %s: %s", snapshotID[:8], message))
			out, err = sm.runGitCommand(ctx, cmd)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create snapshot: %w", err)
		}
	}

	sha, err := sm.runGitCommand(ctx, "git rev-parse HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get commit SHA: %w", err)
	}

	files, err := sm.listTrackedFiles(ctx)
	if err != nil {
		sm.logger.Warn("Failed to list tracked files", "err", err)
		files = []string{}
	}

	snapshot := &Snapshot{
		ID:        snapshotID,
		CreatedAt: time.Now(),
		CommitSHA: strings.TrimSpace(sha),
		Message:   message,
		Files:     files,
	}

	sm.snapshots[snapshotID] = snapshot
	sm.logger.Info("Created snapshot", "id", snapshotID[:8], "sha", snapshot.CommitSHA[:8])

	return snapshot, nil
}

func (sm *SnapshotManager) Restore(ctx context.Context, snapshotID string) error {
	snapshot, ok := sm.snapshots[snapshotID]
	if !ok {
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	_, err := sm.runGitCommand(ctx, fmt.Sprintf("git checkout %s -- .", snapshot.CommitSHA))
	if err != nil {
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	sm.logger.Info("Restored snapshot", "id", snapshotID[:8])
	return nil
}

func (sm *SnapshotManager) Delete(ctx context.Context, snapshotID string) error {
	if _, ok := sm.snapshots[snapshotID]; !ok {
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	delete(sm.snapshots, snapshotID)
	sm.logger.Info("Deleted snapshot", "id", snapshotID[:8])
	return nil
}

func (sm *SnapshotManager) List() []*Snapshot {
	snapshots := make([]*Snapshot, 0, len(sm.snapshots))
	for _, s := range sm.snapshots {
		snapshots = append(snapshots, s)
	}
	return snapshots
}

func (sm *SnapshotManager) Get(snapshotID string) (*Snapshot, bool) {
	s, ok := sm.snapshots[snapshotID]
	return s, ok
}

func (sm *SnapshotManager) Diff(ctx context.Context, snapshotID string) (string, error) {
	snapshot, ok := sm.snapshots[snapshotID]
	if !ok {
		return "", fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	commitRange := fmt.Sprintf("%s..HEAD", snapshot.CommitSHA)
	diff, err := sm.runGitCommand(ctx, fmt.Sprintf("git diff %s", commitRange))
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return diff, nil
}

func (sm *SnapshotManager) CreateBranch(ctx context.Context, branchName, baseSnapshotID string) error {
	if baseSnapshotID != "" {
		snapshot, ok := sm.snapshots[baseSnapshotID]
		if !ok {
			return fmt.Errorf("snapshot not found: %s", baseSnapshotID)
		}

		_, err := sm.runGitCommand(ctx, fmt.Sprintf("git checkout -b %s %s", branchName, snapshot.CommitSHA))
		if err != nil {
			return fmt.Errorf("failed to create branch from snapshot: %w", err)
		}
	} else {
		_, err := sm.runGitCommand(ctx, fmt.Sprintf("git checkout -b %s", branchName))
		if err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}
	}

	sm.logger.Info("Created branch from snapshot", "branch", branchName)
	return nil
}

func (sm *SnapshotManager) SaveSnapshotMetadata(snapshot *Snapshot) error {
	metadataPath := filepath.Join(sm.repoPath, ".iterate", "snapshots", snapshot.ID+".json")
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot metadata: %w", err)
	}

	return nil
}

func (sm *SnapshotManager) LoadSnapshots() error {
	snapshotsDir := filepath.Join(sm.repoPath, ".iterate", "snapshots")
	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read snapshots directory: %w", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(snapshotsDir, entry.Name()))
		if err != nil {
			sm.logger.Warn("Failed to read snapshot file", "file", entry.Name(), "err", err)
			continue
		}

		var snapshot Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			sm.logger.Warn("Failed to parse snapshot file", "file", entry.Name(), "err", err)
			continue
		}

		sm.snapshots[snapshot.ID] = &snapshot
	}

	sm.logger.Info("Loaded snapshots", "count", len(sm.snapshots))
	return nil
}

func (sm *SnapshotManager) runGitCommand(ctx context.Context, cmd string) (string, error) {
	if sm.engine != nil {
		return sm.engine.runTool(ctx, "bash", map[string]interface{}{"cmd": cmd})
	}

	out, err := runBashCommand(sm.repoPath, cmd)
	return out, err
}

func runBashCommand(repoPath, cmd string) (string, error) {
	return "", nil
}

func (sm *SnapshotManager) listTrackedFiles(ctx context.Context) ([]string, error) {
	out, err := sm.runGitCommand(ctx, "git ls-files")
	if err != nil {
		return nil, err
	}

	files := strings.Split(strings.TrimSpace(out), "\n")
	var tracked []string
	for _, f := range files {
		if f != "" {
			tracked = append(tracked, f)
		}
	}
	return tracked, nil
}

func (sm *SnapshotManager) HasUncommittedChanges(ctx context.Context) (bool, error) {
	out, err := sm.runGitCommand(ctx, "git status --porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (sm *SnapshotManager) DiscardUncommittedChanges(ctx context.Context) error {
	_, err := sm.runGitCommand(ctx, "git checkout -- .")
	if err != nil {
		return fmt.Errorf("failed to discard changes: %w", err)
	}

	sm.runGitCommand(ctx, "git clean -fd")

	sm.logger.Info("Discarded uncommitted changes")
	return nil
}

func (e *Engine) CreateSnapshot(ctx context.Context, message string) (*Snapshot, error) {
	sm := NewSnapshotManager(e.repoPath, e.logger)
	sm.engine = e
	return sm.Create(ctx, message)
}

func (e *Engine) RestoreSnapshot(ctx context.Context, snapshotID string) error {
	sm := NewSnapshotManager(e.repoPath, e.logger)
	sm.engine = e
	return sm.Restore(ctx, snapshotID)
}

func (e *Engine) GetSnapshotList() []*Snapshot {
	sm := NewSnapshotManager(e.repoPath, e.logger)
	sm.logger = e.logger
	if err := sm.LoadSnapshots(); err != nil {
		e.logger.Warn("Failed to load snapshots", "err", err)
	}
	return sm.List()
}
