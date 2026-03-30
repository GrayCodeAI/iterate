// Package autonomous - Task 6: Review Checkpoint system for destructive operations
package autonomous

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// OperationType represents the type of operation being reviewed.
type OperationType string

const (
	OperationFileDelete       OperationType = "file_delete"
	OperationFileOverwrite    OperationType = "file_overwrite"
	OperationGitForcePush     OperationType = "git_force_push"
	OperationGitResetHard     OperationType = "git_reset_hard"
	OperationDatabaseDrop     OperationType = "database_drop"
	OperationDatabaseTruncate OperationType = "database_truncate"
	OperationSystemCommand    OperationType = "system_command"
	OperationNetworkRequest   OperationType = "network_request"
	OperationConfigChange     OperationType = "config_change"
	OperationDependencyChange OperationType = "dependency_change"
)

// ReviewRequest represents a pending review for a destructive operation.
type ReviewRequest struct {
	ID          string         `json:"id"`
	Timestamp   int64          `json:"timestamp"`
	Operation   OperationType  `json:"operation"`
	DangerLevel DangerLevel    `json:"danger_level"`
	Target      string         `json:"target"`
	Description string         `json:"description"`
	Details     map[string]any `json:"details,omitempty"`
	AutoApprove bool           `json:"auto_approve"`
	Timeout     time.Duration  `json:"timeout"`
}

// ReviewDecision represents the user's decision on a review request.
type ReviewDecision string

const (
	DecisionApproved  ReviewDecision = "approved"
	DecisionRejected  ReviewDecision = "rejected"
	DecisionModified  ReviewDecision = "modified"  // Approved with modifications
	DecisionEscalated ReviewDecision = "escalated" // Needs higher approval
	DecisionDeferred  ReviewDecision = "deferred"  // Ask again later
)

// ReviewResponse represents the response to a review request.
type ReviewResponse struct {
	RequestID   string         `json:"request_id"`
	Decision    ReviewDecision `json:"decision"`
	Reason      string         `json:"reason,omitempty"`
	ModifiedOp  *ReviewRequest `json:"modified_operation,omitempty"`
	RespondedBy string         `json:"responded_by,omitempty"`
	Timestamp   int64          `json:"timestamp"`
}

// ReviewPolicy defines when reviews are required.
type ReviewPolicy struct {
	mu                      sync.RWMutex
	requireReviewAboveLevel DangerLevel
	autoApproveBelowLevel   DangerLevel
	alwaysReviewOperations  map[OperationType]bool
	neverReviewOperations   map[OperationType]bool
	protectedPaths          []string
	protectedBranches       []string
	maxPendingReviews       int
	reviewTimeout           time.Duration
}

// DefaultReviewPolicy returns a sensible default policy.
func DefaultReviewPolicy() *ReviewPolicy {
	return &ReviewPolicy{
		requireReviewAboveLevel: DangerLevelMedium,
		autoApproveBelowLevel:   DangerLevelLow,
		alwaysReviewOperations: map[OperationType]bool{
			OperationFileDelete:       true,
			OperationGitForcePush:     true,
			OperationGitResetHard:     true,
			OperationDatabaseDrop:     true,
			OperationDatabaseTruncate: true,
		},
		neverReviewOperations: map[OperationType]bool{},
		protectedPaths: []string{
			".env",
			".git/config",
			"credentials.*",
			"*.key",
			"*.pem",
		},
		protectedBranches: []string{
			"main",
			"master",
			"production",
		},
		maxPendingReviews: 10,
		reviewTimeout:     5 * time.Minute,
	}
}

// NeedsReview determines if an operation requires review.
func (p *ReviewPolicy) NeedsReview(req *ReviewRequest) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check if always reviewed
	if p.alwaysReviewOperations[req.Operation] {
		return true
	}

	// Check if never reviewed
	if p.neverReviewOperations[req.Operation] {
		return false
	}

	// Check danger level threshold
	if req.DangerLevel >= p.requireReviewAboveLevel {
		return true
	}

	// Check for protected paths
	for _, protected := range p.protectedPaths {
		if matchesPattern(req.Target, protected) {
			return true
		}
	}

	return false
}

// SetRequireReviewAboveLevel sets the danger level threshold for reviews.
func (p *ReviewPolicy) SetRequireReviewAboveLevel(level DangerLevel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requireReviewAboveLevel = level
}

// AddProtectedPath adds a path to the protected list.
func (p *ReviewPolicy) AddProtectedPath(pattern string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.protectedPaths = append(p.protectedPaths, pattern)
}

// AddProtectedBranch adds a branch to the protected list.
func (p *ReviewPolicy) AddProtectedBranch(branch string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.protectedBranches = append(p.protectedBranches, branch)
}

// ReviewCheckpoint manages the review process for destructive operations.
type ReviewCheckpoint struct {
	mu           sync.RWMutex
	policy       *ReviewPolicy
	pending      map[string]*ReviewRequest
	responses    map[string]*ReviewResponse
	stateManager *StateManager
	hooks        []ReviewHook
}

// ReviewHook is called before/after review decisions.
type ReviewHook interface {
	BeforeReview(req *ReviewRequest) error
	AfterReview(req *ReviewRequest, resp *ReviewResponse) error
}

// NewReviewCheckpoint creates a new review checkpoint system.
func NewReviewCheckpoint(policy *ReviewPolicy, stateManager *StateManager) *ReviewCheckpoint {
	if policy == nil {
		policy = DefaultReviewPolicy()
	}

	return &ReviewCheckpoint{
		policy:       policy,
		pending:      make(map[string]*ReviewRequest),
		responses:    make(map[string]*ReviewResponse),
		stateManager: stateManager,
		hooks:        make([]ReviewHook, 0),
	}
}

// RequestReview creates a review request for an operation.
func (rc *ReviewCheckpoint) RequestReview(ctx context.Context, op OperationType, target string, description string, details map[string]any) (*ReviewRequest, error) {
	dangerLevel := AssessDangerLevel(op, target, details)

	req := &ReviewRequest{
		ID:          fmt.Sprintf("review_%d_%d", time.Now().UnixNano(), len(rc.pending)),
		Timestamp:   time.Now().Unix(),
		Operation:   op,
		DangerLevel: dangerLevel,
		Target:      target,
		Description: description,
		Details:     details,
		Timeout:     rc.policy.reviewTimeout,
	}

	// Check if review is needed
	if !rc.policy.NeedsReview(req) {
		req.AutoApprove = true
		return req, nil
	}

	// Run pre-review hooks
	for _, hook := range rc.hooks {
		if err := hook.BeforeReview(req); err != nil {
			return nil, fmt.Errorf("pre-review hook failed: %w", err)
		}
	}

	// Add to pending
	rc.mu.Lock()
	rc.pending[req.ID] = req
	rc.mu.Unlock()

	// Save checkpoint if state manager exists
	if rc.stateManager != nil {
		rc.stateManager.CreateCheckpoint(
			"review_pending",
			0,
			description,
			nil,
			nil,
			nil,
			&Result{
				Status:       "awaiting_review",
				FinalMessage: fmt.Sprintf("review_id=%s,operation=%s,danger_level=%s", req.ID, op, dangerLevel.String()),
			},
		)
	}

	return req, nil
}

// WaitForReview blocks until a review decision is made.
func (rc *ReviewCheckpoint) WaitForReview(ctx context.Context, requestID string) (*ReviewResponse, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(rc.policy.reviewTimeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			// Auto-reject on timeout
			resp, _ := rc.SubmitResponse(requestID, DecisionRejected, "review timeout", "")
			return resp, nil
		case <-ticker.C:
			rc.mu.RLock()
			resp, exists := rc.responses[requestID]
			rc.mu.RUnlock()

			if exists {
				return resp, nil
			}
		}
	}
}

// SubmitResponse submits a review decision.
func (rc *ReviewCheckpoint) SubmitResponse(requestID string, decision ReviewDecision, reason string, respondedBy string) (*ReviewResponse, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	req, exists := rc.pending[requestID]
	if !exists {
		return nil, fmt.Errorf("review request %s not found", requestID)
	}

	resp := &ReviewResponse{
		RequestID:   requestID,
		Decision:    decision,
		Reason:      reason,
		RespondedBy: respondedBy,
		Timestamp:   time.Now().Unix(),
	}

	// Run post-review hooks
	for _, hook := range rc.hooks {
		if err := hook.AfterReview(req, resp); err != nil {
			// Log but don't fail
		}
	}

	// Move from pending to responses
	delete(rc.pending, requestID)
	rc.responses[requestID] = resp

	return resp, nil
}

// Approve approves a pending review.
func (rc *ReviewCheckpoint) Approve(requestID string, reason string) error {
	_, err := rc.SubmitResponse(requestID, DecisionApproved, reason, "user")
	return err
}

// Reject rejects a pending review.
func (rc *ReviewCheckpoint) Reject(requestID string, reason string) error {
	_, err := rc.SubmitResponse(requestID, DecisionRejected, reason, "user")
	return err
}

// GetPendingReviews returns all pending review requests.
func (rc *ReviewCheckpoint) GetPendingReviews() []*ReviewRequest {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	pending := make([]*ReviewRequest, 0, len(rc.pending))
	for _, req := range rc.pending {
		pending = append(pending, req)
	}
	return pending
}

// AddHook adds a review hook.
func (rc *ReviewCheckpoint) AddHook(hook ReviewHook) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.hooks = append(rc.hooks, hook)
}

// AssessDangerLevel determines the danger level of an operation.
func AssessDangerLevel(op OperationType, target string, details map[string]any) DangerLevel {
	switch op {
	case OperationFileDelete:
		return assessFileDeleteDanger(target, details)
	case OperationGitForcePush:
		return DangerLevelCritical
	case OperationGitResetHard:
		return DangerLevelHigh
	case OperationDatabaseDrop:
		return DangerLevelCritical
	case OperationDatabaseTruncate:
		return DangerLevelCritical
	case OperationFileOverwrite:
		return assessFileOverwriteDanger(target, details)
	case OperationSystemCommand:
		return assessSystemCommandDanger(target, details)
	case OperationNetworkRequest:
		return DangerLevelLow
	case OperationConfigChange:
		return DangerLevelMedium
	case OperationDependencyChange:
		return DangerLevelMedium
	default:
		return DangerLevelMedium
	}
}

func assessFileDeleteDanger(target string, details map[string]any) DangerLevel {
	// Critical files
	criticalPatterns := []string{".env", "credentials", "*.key", "*.pem", ".git"}
	for _, pattern := range criticalPatterns {
		if matchesPattern(target, pattern) {
			return DangerLevelCritical
		}
	}

	// Check if file is tracked in git
	if tracked, ok := details["git_tracked"].(bool); ok && tracked {
		return DangerLevelHigh
	}

	// Directory deletion
	if isDir, ok := details["is_directory"].(bool); ok && isDir {
		return DangerLevelHigh
	}

	return DangerLevelMedium
}

func assessFileOverwriteDanger(target string, details map[string]any) DangerLevel {
	// Check if file exists
	if exists, ok := details["file_exists"].(bool); ok && exists {
		// Overwriting existing file
		if size, ok := details["file_size"].(int64); ok && size > 10000 {
			return DangerLevelHigh
		}
		return DangerLevelMedium
	}

	return DangerLevelLow
}

func assessSystemCommandDanger(target string, details map[string]any) DangerLevel {
	// Check for dangerous commands
	dangerousCmds := []string{"rm", "sudo", "chmod", "chown", "mkfs", "dd", "shutdown", "reboot"}
	for _, cmd := range dangerousCmds {
		if containsStr(target, cmd) {
			return DangerLevelCritical
		}
	}

	// Network-related commands
	networkCmds := []string{"curl", "wget", "ssh", "scp", "rsync"}
	for _, cmd := range networkCmds {
		if containsStr(target, cmd) {
			return DangerLevelMedium
		}
	}

	return DangerLevelLow
}

// matchesPattern checks if a string matches a glob-like pattern.
func matchesPattern(s string, pattern string) bool {
	// Simple implementation - can be enhanced with proper glob matching
	if pattern[0] == '*' {
		suffix := pattern[1:]
		return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
	}
	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(s) >= len(prefix) && s[:len(prefix)] == prefix
	}
	return s == pattern
}

// containsStr checks if s contains substr.
func containsStr(s string, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStrHelper(s, substr))
}

func containsStrHelper(s string, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
