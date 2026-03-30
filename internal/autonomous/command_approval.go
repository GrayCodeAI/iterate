// Package autonomous - Task 28: Command Approval Workflow for risky operations
package autonomous

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ApprovalStatus represents the status of an approval request.
type ApprovalStatus string

const (
	// ApprovalStatusPending - Request is awaiting approval
	ApprovalStatusPending ApprovalStatus = "pending"

	// ApprovalStatusApproved - Request has been approved
	ApprovalStatusApproved ApprovalStatus = "approved"

	// ApprovalStatusDenied - Request has been denied
	ApprovalStatusDenied ApprovalStatus = "denied"

	// ApprovalStatusExpired - Request has expired without decision
	ApprovalStatusExpired ApprovalStatus = "expired"

	// ApprovalStatusCancelled - Request was cancelled
	ApprovalStatusCancelled ApprovalStatus = "cancelled"

	// ApprovalStatusAutoApproved - Request was auto-approved (safe operation)
	ApprovalStatusAutoApproved ApprovalStatus = "auto_approved"
)

// ApprovalMode determines how approvals are handled.
type ApprovalMode string

const (
	// ApprovalModeStrict - All operations require approval
	ApprovalModeStrict ApprovalMode = "strict"

	// ApprovalModeBalanced - Only risky operations require approval (default)
	ApprovalModeBalanced ApprovalMode = "balanced"

	// ApprovalModePermissive - Only critical operations require approval
	ApprovalModePermissive ApprovalMode = "permissive"

	// ApprovalModeAuto - Auto-approve everything (dangerous!)
	ApprovalModeAuto ApprovalMode = "auto"
)

// ApprovalRequest represents a request for operation approval.
type ApprovalRequest struct {
	// ID is the unique request identifier
	ID string `json:"id"`

	// Command is the command to be executed
	Command string `json:"command"`

	// Args are the command arguments
	Args []string `json:"args,omitempty"`

	// WorkingDir is the working directory
	WorkingDir string `json:"working_dir,omitempty"`

	// Assessment is the danger assessment
	Assessment *DangerAssessment `json:"assessment"`

	// Status is the current approval status
	Status ApprovalStatus `json:"status"`

	// CreatedAt is when the request was created
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the request expires
	ExpiresAt time.Time `json:"expires_at"`

	// DecidedAt is when a decision was made
	DecidedAt *time.Time `json:"decided_at,omitempty"`

	// ApprovedBy is who approved (if applicable)
	ApprovedBy string `json:"approved_by,omitempty"`

	// DenialReason is why it was denied (if applicable)
	DenialReason string `json:"denial_reason,omitempty"`

	// Metadata contains additional context
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Checksum for request integrity
	Checksum string `json:"checksum"`
}

// ApprovalDecision represents a decision on an approval request.
type ApprovalDecision struct {
	// RequestID is the request being decided
	RequestID string `json:"request_id"`

	// Approved indicates if the request is approved
	Approved bool `json:"approved"`

	// Reason for the decision
	Reason string `json:"reason,omitempty"`

	// ApprovedBy is who made the decision
	ApprovedBy string `json:"approved_by,omitempty"`

	// Timestamp of the decision
	Timestamp time.Time `json:"timestamp"`
}

// ApprovalPolicy defines rules for automatic approval decisions.
type ApprovalPolicy struct {
	// Mode is the approval mode
	Mode ApprovalMode `json:"mode"`

	// AutoApproveSafe auto-approves safe operations
	AutoApproveSafe bool `json:"auto_approve_safe"`

	// AutoApproveLow auto-approves low-risk operations
	AutoApproveLow bool `json:"auto_approve_low"`

	// RequireApprovalMedium requires approval for medium risk
	RequireApprovalMedium bool `json:"require_approval_medium"`

	// RequireApprovalHigh requires approval for high risk
	RequireApprovalHigh bool `json:"require_approval_high"`

	// RequireApprovalCritical requires approval for critical risk
	RequireApprovalCritical bool `json:"require_approval_critical"`

	// Timeout is the default approval timeout
	Timeout time.Duration `json:"timeout"`

	// MaxPendingRequests is the maximum pending requests allowed
	MaxPendingRequests int `json:"max_pending_requests"`

	// AllowedApprovers is a list of who can approve (empty = anyone)
	AllowedApprovers []string `json:"allowed_approvers,omitempty"`

	// Blocklist are commands that always require approval
	Blocklist []string `json:"blocklist,omitempty"`

	// Allowlist are commands that can be auto-approved
	Allowlist []string `json:"allowlist,omitempty"`
}

// ApprovalStats tracks approval workflow statistics.
type ApprovalStats struct {
	TotalRequests    int            `json:"total_requests"`
	PendingRequests  int            `json:"pending_requests"`
	ApprovedRequests int            `json:"approved_requests"`
	DeniedRequests   int            `json:"denied_requests"`
	ExpiredRequests  int            `json:"expired_requests"`
	AutoApproved     int            `json:"auto_approved"`
	AverageWaitTime  time.Duration  `json:"average_wait_time"`
	ApprovalRate     float64        `json:"approval_rate"`
	ByDangerLevel    map[string]int `json:"by_danger_level"`
	LastRequestTime  *time.Time     `json:"last_request_time,omitempty"`
}

// ApprovalCallback is called when a decision is needed.
type ApprovalCallback func(request *ApprovalRequest) (*ApprovalDecision, error)

// CommandApprovalManager manages the approval workflow for commands.
type CommandApprovalManager struct {
	mu sync.RWMutex

	// assessor is the danger assessor
	assessor *DangerAssessor

	// policy is the current approval policy
	policy ApprovalPolicy

	// requests stores all approval requests
	requests map[string]*ApprovalRequest

	// pending stores pending request IDs in order
	pending []string

	// history stores completed request IDs
	history []string

	// maxHistory is the maximum history to keep
	maxHistory int

	// callback is called for approval decisions
	callback ApprovalCallback

	// timeNow is a function to get current time (for testing)
	timeNow func() time.Time

	// stats tracks approval statistics
	stats ApprovalStats
}

// NewCommandApprovalManager creates a new approval manager.
func NewCommandApprovalManager(assessor *DangerAssessor) *CommandApprovalManager {
	return &CommandApprovalManager{
		assessor:   assessor,
		policy:     DefaultApprovalPolicy(),
		requests:   make(map[string]*ApprovalRequest),
		pending:    make([]string, 0),
		history:    make([]string, 0),
		maxHistory: 1000,
		timeNow:    time.Now,
		stats: ApprovalStats{
			ByDangerLevel: make(map[string]int),
		},
	}
}

// DefaultApprovalPolicy returns the default approval policy.
func DefaultApprovalPolicy() ApprovalPolicy {
	return ApprovalPolicy{
		Mode:                    ApprovalModeBalanced,
		AutoApproveSafe:         true,
		AutoApproveLow:          true,
		RequireApprovalMedium:   false,
		RequireApprovalHigh:     true,
		RequireApprovalCritical: true,
		Timeout:                 5 * time.Minute,
		MaxPendingRequests:      100,
	}
}

// SetPolicy sets the approval policy.
func (cam *CommandApprovalManager) SetPolicy(policy ApprovalPolicy) {
	cam.mu.Lock()
	defer cam.mu.Unlock()
	cam.policy = policy
}

// GetPolicy returns the current policy.
func (cam *CommandApprovalManager) GetPolicy() ApprovalPolicy {
	cam.mu.RLock()
	defer cam.mu.RUnlock()
	return cam.policy
}

// SetCallback sets the approval callback.
func (cam *CommandApprovalManager) SetCallback(callback ApprovalCallback) {
	cam.mu.Lock()
	defer cam.mu.Unlock()
	cam.callback = callback
}

// RequestApproval creates a new approval request.
func (cam *CommandApprovalManager) RequestApproval(ctx context.Context, command string, args []string, workingDir string) (*ApprovalRequest, error) {
	// Assess the danger level - combine command and args for assessment
	fullCommand := command
	if len(args) > 0 {
		fullCommand = command + " " + joinArgs(args)
	}
	assessment := cam.assessor.AssessCommand(fullCommand)

	// Create the request
	now := cam.timeNow()
	request := &ApprovalRequest{
		ID:         generateApprovalRequestID(command, args, now),
		Command:    command,
		Args:       args,
		WorkingDir: workingDir,
		Assessment: assessment,
		Status:     ApprovalStatusPending,
		CreatedAt:  now,
		ExpiresAt:  now.Add(cam.policy.Timeout),
		Metadata:   make(map[string]interface{}),
		Checksum:   calculateChecksum(command, args),
	}

	cam.mu.Lock()
	defer cam.mu.Unlock()

	// Check max pending
	if len(cam.pending) >= cam.policy.MaxPendingRequests {
		return nil, fmt.Errorf("maximum pending requests (%d) reached", cam.policy.MaxPendingRequests)
	}

	// Store the request
	cam.requests[request.ID] = request

	// Check if auto-approval applies
	if cam.shouldAutoApprove(request) {
		request.Status = ApprovalStatusAutoApproved
		request.DecidedAt = &now
		cam.stats.AutoApproved++
	} else {
		// Add to pending queue
		cam.pending = append(cam.pending, request.ID)
	}

	// Update stats
	cam.stats.TotalRequests++
	cam.stats.ByDangerLevel[assessment.Level.String()]++
	cam.stats.LastRequestTime = &now

	return request, nil
}

// shouldAutoApprove checks if a request should be auto-approved.
func (cam *CommandApprovalManager) shouldAutoApprove(request *ApprovalRequest) bool {
	policy := cam.policy
	assessment := request.Assessment

	// Check allowlist first (overrides mode)
	for _, allowed := range policy.Allowlist {
		if request.Command == allowed {
			return true
		}
	}

	// Check blocklist
	for _, blocked := range policy.Blocklist {
		if request.Command == blocked {
			return false
		}
	}

	// Check mode
	switch policy.Mode {
	case ApprovalModeAuto:
		return true
	case ApprovalModeStrict:
		return false
	case ApprovalModePermissive:
		// Only critical needs approval
		return assessment.Level < DangerLevelCritical
	case ApprovalModeBalanced:
		// Fall through to detailed checks
	}

	// Check by danger level
	switch assessment.Level {
	case DangerLevelSafe:
		return policy.AutoApproveSafe
	case DangerLevelLow:
		return policy.AutoApproveLow
	case DangerLevelMedium:
		return !policy.RequireApprovalMedium
	case DangerLevelHigh:
		return !policy.RequireApprovalHigh
	case DangerLevelCritical:
		return !policy.RequireApprovalCritical
	default:
		return false
	}
}

// Approve approves a pending request.
func (cam *CommandApprovalManager) Approve(requestID string, approvedBy string) error {
	cam.mu.Lock()
	defer cam.mu.Unlock()

	request, exists := cam.requests[requestID]
	if !exists {
		return fmt.Errorf("request %s not found", requestID)
	}

	if request.Status != ApprovalStatusPending {
		return fmt.Errorf("request %s is not pending (status: %s)", requestID, request.Status)
	}

	now := cam.timeNow()
	request.Status = ApprovalStatusApproved
	request.DecidedAt = &now
	request.ApprovedBy = approvedBy

	cam.moveToHistory(requestID)
	cam.stats.ApprovedRequests++

	return nil
}

// Deny denies a pending request.
func (cam *CommandApprovalManager) Deny(requestID string, reason string) error {
	cam.mu.Lock()
	defer cam.mu.Unlock()

	request, exists := cam.requests[requestID]
	if !exists {
		return fmt.Errorf("request %s not found", requestID)
	}

	if request.Status != ApprovalStatusPending {
		return fmt.Errorf("request %s is not pending (status: %s)", requestID, request.Status)
	}

	now := cam.timeNow()
	request.Status = ApprovalStatusDenied
	request.DecidedAt = &now
	request.DenialReason = reason

	cam.moveToHistory(requestID)
	cam.stats.DeniedRequests++

	return nil
}

// Cancel cancels a pending request.
func (cam *CommandApprovalManager) Cancel(requestID string) error {
	cam.mu.Lock()
	defer cam.mu.Unlock()

	request, exists := cam.requests[requestID]
	if !exists {
		return fmt.Errorf("request %s not found", requestID)
	}

	if request.Status != ApprovalStatusPending {
		return fmt.Errorf("request %s is not pending (status: %s)", requestID, request.Status)
	}

	now := cam.timeNow()
	request.Status = ApprovalStatusCancelled
	request.DecidedAt = &now

	cam.moveToHistory(requestID)

	return nil
}

// ExpirePending expires all pending requests that have timed out.
func (cam *CommandApprovalManager) ExpirePending() int {
	cam.mu.Lock()
	defer cam.mu.Unlock()

	now := cam.timeNow()
	expired := 0

	for _, requestID := range cam.pending {
		request := cam.requests[requestID]
		if request != nil && request.Status == ApprovalStatusPending && now.After(request.ExpiresAt) {
			request.Status = ApprovalStatusExpired
			request.DecidedAt = &now
			cam.moveToHistory(requestID)
			cam.stats.ExpiredRequests++
			expired++
		}
	}

	return expired
}

// GetPending returns all pending requests.
func (cam *CommandApprovalManager) GetPending() []*ApprovalRequest {
	cam.mu.RLock()
	defer cam.mu.RUnlock()

	pending := make([]*ApprovalRequest, 0, len(cam.pending))
	for _, id := range cam.pending {
		if request, exists := cam.requests[id]; exists && request.Status == ApprovalStatusPending {
			pending = append(pending, request)
		}
	}

	return pending
}

// GetRequest retrieves a specific request.
func (cam *CommandApprovalManager) GetRequest(requestID string) (*ApprovalRequest, bool) {
	cam.mu.RLock()
	defer cam.mu.RUnlock()

	request, exists := cam.requests[requestID]
	return request, exists
}

// IsApproved checks if a request is approved (including auto-approved).
func (cam *CommandApprovalManager) IsApproved(requestID string) bool {
	cam.mu.RLock()
	defer cam.mu.RUnlock()

	request, exists := cam.requests[requestID]
	if !exists {
		return false
	}

	return request.Status == ApprovalStatusApproved || request.Status == ApprovalStatusAutoApproved
}

// GetStats returns approval statistics.
func (cam *CommandApprovalManager) GetStats() ApprovalStats {
	cam.mu.RLock()
	defer cam.mu.RUnlock()

	// Calculate derived stats
	stats := cam.stats
	stats.PendingRequests = len(cam.pending)

	if stats.TotalRequests > 0 {
		stats.ApprovalRate = float64(stats.ApprovedRequests+stats.AutoApproved) / float64(stats.TotalRequests)
	}

	return stats
}

// moveToHistory moves a request from pending to history.
func (cam *CommandApprovalManager) moveToHistory(requestID string) {
	// Remove from pending
	for i, id := range cam.pending {
		if id == requestID {
			cam.pending = append(cam.pending[:i], cam.pending[i+1:]...)
			break
		}
	}

	// Add to history
	cam.history = append(cam.history, requestID)

	// Trim history if needed
	if len(cam.history) > cam.maxHistory {
		// Remove oldest from history and requests map
		oldest := cam.history[0]
		delete(cam.requests, oldest)
		cam.history = cam.history[1:]
	}
}

// WaitForApproval waits for a decision on a request.
func (cam *CommandApprovalManager) WaitForApproval(ctx context.Context, requestID string) (*ApprovalRequest, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			request, exists := cam.GetRequest(requestID)
			if !exists {
				return nil, fmt.Errorf("request %s not found", requestID)
			}

			switch request.Status {
			case ApprovalStatusApproved, ApprovalStatusAutoApproved:
				return request, nil
			case ApprovalStatusDenied:
				return nil, fmt.Errorf("request denied: %s", request.DenialReason)
			case ApprovalStatusExpired:
				return nil, fmt.Errorf("request expired")
			case ApprovalStatusCancelled:
				return nil, fmt.Errorf("request cancelled")
			}
		}
	}
}

// RequestAndWait requests approval and waits for the decision.
func (cam *CommandApprovalManager) RequestAndWait(ctx context.Context, command string, args []string, workingDir string) (*ApprovalRequest, error) {
	request, err := cam.RequestApproval(ctx, command, args, workingDir)
	if err != nil {
		return nil, err
	}

	// If auto-approved, return immediately
	if request.Status == ApprovalStatusAutoApproved {
		return request, nil
	}

	// If callback is set, call it
	if cam.callback != nil {
		decision, err := cam.callback(request)
		if err != nil {
			return nil, err
		}

		if decision.Approved {
			if err := cam.Approve(request.ID, decision.ApprovedBy); err != nil {
				return nil, err
			}
		} else {
			if err := cam.Deny(request.ID, decision.Reason); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("request denied: %s", decision.Reason)
		}
	}

	return cam.WaitForApproval(ctx, request.ID)
}

// ExportRequests exports all requests for audit.
func (cam *CommandApprovalManager) ExportRequests() ([]byte, error) {
	cam.mu.RLock()
	defer cam.mu.RUnlock()

	export := struct {
		Requests []*ApprovalRequest `json:"requests"`
		Stats    ApprovalStats      `json:"stats"`
		Exported time.Time          `json:"exported"`
	}{
		Requests: make([]*ApprovalRequest, 0, len(cam.requests)),
		Stats:    cam.stats,
		Exported: cam.timeNow(),
	}

	for _, request := range cam.requests {
		export.Requests = append(export.Requests, request)
	}

	return json.MarshalIndent(export, "", "  ")
}

// Helper functions

func generateApprovalRequestID(command string, args []string, t time.Time) string {
	data := fmt.Sprintf("%s-%v-%d", command, args, t.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

func calculateChecksum(command string, args []string) string {
	data := fmt.Sprintf("%s|%v", command, args)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		// Quote args with spaces
		if containsSpace(arg) {
			result += "\"" + arg + "\""
		} else {
			result += arg
		}
	}
	return result
}

func containsSpace(s string) bool {
	for _, c := range s {
		if c == ' ' {
			return true
		}
	}
	return false
}
