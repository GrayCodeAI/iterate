// Package autonomous - Task 6: Review Checkpoint tests
package autonomous

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDangerLevelString(t *testing.T) {
	levels := []DangerLevel{
		DangerLevelSafe,
		DangerLevelLow,
		DangerLevelMedium,
		DangerLevelHigh,
		DangerLevelCritical,
	}

	expected := []string{"safe", "low", "medium", "high", "critical"}

	for i, level := range levels {
		if level.String() != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], level.String())
		}
	}
}

func TestDefaultReviewPolicy(t *testing.T) {
	policy := DefaultReviewPolicy()

	if policy.requireReviewAboveLevel != DangerLevelMedium {
		t.Errorf("Expected Medium threshold, got %d", policy.requireReviewAboveLevel)
	}

	if len(policy.alwaysReviewOperations) == 0 {
		t.Error("Expected some always-review operations")
	}

	if len(policy.protectedPaths) == 0 {
		t.Error("Expected some protected paths")
	}
}

func TestPolicyNeedsReview(t *testing.T) {
	policy := DefaultReviewPolicy()

	// Always reviewed operations
	req := &ReviewRequest{
		Operation:   OperationFileDelete,
		DangerLevel: DangerLevelLow,
		Target:      "test.go",
	}
	if !policy.NeedsReview(req) {
		t.Error("File delete should always require review")
	}

	// Low danger, not always reviewed
	req2 := &ReviewRequest{
		Operation:   OperationNetworkRequest,
		DangerLevel: DangerLevelLow,
		Target:      "api.example.com",
	}
	if policy.NeedsReview(req2) {
		t.Error("Low danger network request should not require review")
	}

	// High danger
	req3 := &ReviewRequest{
		Operation:   OperationNetworkRequest,
		DangerLevel: DangerLevelHigh,
		Target:      "api.example.com",
	}
	if !policy.NeedsReview(req3) {
		t.Error("High danger operation should require review")
	}

	// Protected path
	req4 := &ReviewRequest{
		Operation:   OperationFileOverwrite,
		DangerLevel: DangerLevelLow,
		Target:      ".env",
	}
	if !policy.NeedsReview(req4) {
		t.Error("Protected path should require review")
	}
}

func TestPolicySetRequireReviewAboveLevel(t *testing.T) {
	policy := DefaultReviewPolicy()
	policy.SetRequireReviewAboveLevel(DangerLevelHigh)

	if policy.requireReviewAboveLevel != DangerLevelHigh {
		t.Errorf("Expected High threshold, got %d", policy.requireReviewAboveLevel)
	}
}

func TestPolicyAddProtectedPath(t *testing.T) {
	policy := DefaultReviewPolicy()
	initial := len(policy.protectedPaths)
	
	policy.AddProtectedPath("secrets/*")
	
	if len(policy.protectedPaths) != initial+1 {
		t.Error("Failed to add protected path")
	}
}

func TestNewReviewCheckpoint(t *testing.T) {
	rc := NewReviewCheckpoint(nil, nil)
	
	if rc == nil {
		t.Fatal("Expected non-nil review checkpoint")
	}
	if rc.policy == nil {
		t.Error("Expected default policy to be created")
	}
}

func TestRequestReviewAutoApprove(t *testing.T) {
	rc := NewReviewCheckpoint(nil, nil)
	
	req, err := rc.RequestReview(context.Background(), OperationNetworkRequest, "api.example.com", "fetch data", nil)
	if err != nil {
		t.Fatalf("Failed to request review: %v", err)
	}

	// Low danger, not in always-review list -> auto-approve
	if !req.AutoApprove {
		t.Error("Expected auto-approve for safe operation")
	}
}

func TestRequestReviewRequiresReview(t *testing.T) {
	rc := NewReviewCheckpoint(nil, nil)
	
	req, err := rc.RequestReview(context.Background(), OperationFileDelete, "important.go", "delete file", map[string]any{
		"git_tracked": true,
	})
	if err != nil {
		t.Fatalf("Failed to request review: %v", err)
	}

	// File delete is in always-review list
	if req.AutoApprove {
		t.Error("Expected review required for file delete")
	}

	// Check it's in pending
	pending := rc.GetPendingReviews()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending review, got %d", len(pending))
	}
}

func TestApproveReview(t *testing.T) {
	rc := NewReviewCheckpoint(nil, nil)
	
	req, _ := rc.RequestReview(context.Background(), OperationFileDelete, "test.go", "delete file", nil)
	
	err := rc.Approve(req.ID, "looks safe")
	if err != nil {
		t.Fatalf("Failed to approve: %v", err)
	}

	// Should no longer be pending
	pending := rc.GetPendingReviews()
	if len(pending) != 0 {
		t.Error("Expected no pending reviews after approval")
	}
}

func TestRejectReview(t *testing.T) {
	rc := NewReviewCheckpoint(nil, nil)
	
	req, _ := rc.RequestReview(context.Background(), OperationFileDelete, "test.go", "delete file", nil)
	
	err := rc.Reject(req.ID, "too risky")
	if err != nil {
		t.Fatalf("Failed to reject: %v", err)
	}
}

func TestWaitForReview(t *testing.T) {
	rc := NewReviewCheckpoint(nil, nil)
	rc.policy.reviewTimeout = 100 * time.Millisecond
	
	req, _ := rc.RequestReview(context.Background(), OperationFileDelete, "test.go", "delete file", nil)
	
	// Approve in goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		rc.Approve(req.ID, "approved")
	}()

	resp, err := rc.WaitForReview(context.Background(), req.ID)
	if err != nil {
		t.Fatalf("Failed to wait for review: %v", err)
	}

	if resp.Decision != DecisionApproved {
		t.Errorf("Expected approved, got %s", resp.Decision)
	}
}

func TestWaitForReviewTimeout(t *testing.T) {
	rc := NewReviewCheckpoint(nil, nil)
	rc.policy.reviewTimeout = 50 * time.Millisecond
	
	req, _ := rc.RequestReview(context.Background(), OperationFileDelete, "test.go", "delete file", nil)
	
	resp, err := rc.WaitForReview(context.Background(), req.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should auto-reject on timeout
	if resp.Decision != DecisionRejected {
		t.Errorf("Expected rejected on timeout, got %s", resp.Decision)
	}
}

func TestAssessDangerLevel(t *testing.T) {
	tests := []struct {
		op       OperationType
		target   string
		details  map[string]any
		expected DangerLevel
	}{
		{OperationGitForcePush, "main", nil, DangerLevelCritical},
		{OperationGitResetHard, "HEAD~5", nil, DangerLevelHigh},
		{OperationDatabaseDrop, "production", nil, DangerLevelCritical},
		{OperationDatabaseTruncate, "users", nil, DangerLevelCritical},
		{OperationFileDelete, ".env", nil, DangerLevelCritical},
		{OperationFileDelete, "test.go", map[string]any{"git_tracked": true}, DangerLevelHigh},
		{OperationFileDelete, "test.go", nil, DangerLevelMedium},
		{OperationFileOverwrite, "test.go", map[string]any{"file_exists": false}, DangerLevelLow},
		{OperationFileOverwrite, "test.go", map[string]any{"file_exists": true, "file_size": int64(5000)}, DangerLevelMedium},
		{OperationFileOverwrite, "test.go", map[string]any{"file_exists": true, "file_size": int64(50000)}, DangerLevelHigh},
		{OperationSystemCommand, "ls -la", nil, DangerLevelLow},
		{OperationSystemCommand, "rm -rf /", nil, DangerLevelCritical},
		{OperationSystemCommand, "sudo make me a sandwich", nil, DangerLevelCritical},
		{OperationNetworkRequest, "api.example.com", nil, DangerLevelLow},
		{OperationConfigChange, "config.yaml", nil, DangerLevelMedium},
		{OperationDependencyChange, "package.json", nil, DangerLevelMedium},
	}

	for _, tt := range tests {
		result := AssessDangerLevel(tt.op, tt.target, tt.details)
		if result != tt.expected {
			t.Errorf("AssessDangerLevel(%s, %s) = %s, expected %s", tt.op, tt.target, result, tt.expected)
		}
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		s        string
		pattern  string
		expected bool
	}{
		{"test.go", "*.go", true},
		{"credentials.json", "credentials.*", true},
		{".env", ".env", true},
		{"main.go", "*.js", false},
		{"config.yaml", "config.*", true},
		{"secrets/key.pem", "*.pem", true},
	}

	for _, tt := range tests {
		result := matchesPattern(tt.s, tt.pattern)
		if result != tt.expected {
			t.Errorf("matchesPattern(%s, %s) = %v, expected %v", tt.s, tt.pattern, result, tt.expected)
		}
	}
}

func TestContainsStr(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"rm -rf /", "rm", true},
		{"sudo make install", "sudo", true},
		{"ls -la", "rm", false},
		{"curl example.com", "curl", true},
	}

	for _, tt := range tests {
		result := containsStr(tt.s, tt.substr)
		if result != tt.expected {
			t.Errorf("containsStr(%s, %s) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
		}
	}
}

func TestReviewWithStateManager(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "review_test")
	defer os.RemoveAll(tmpDir)

	sm := NewStateManager(tmpDir, "test_session")
	rc := NewReviewCheckpoint(nil, sm)

	req, err := rc.RequestReview(context.Background(), OperationFileDelete, "test.go", "delete test file", nil)
	if err != nil {
		t.Fatalf("Failed to request review: %v", err)
	}

	// Check checkpoint was created
	cp := sm.GetLatestCheckpoint()
	if cp == nil {
		t.Fatal("Expected checkpoint to be created")
	}
	if cp.Phase != "review_pending" {
		t.Errorf("Expected phase 'review_pending', got %s", cp.Phase)
	}

	// Approve the review
	rc.Approve(req.ID, "approved for testing")

	// Verify response
	rc.mu.RLock()
	resp, exists := rc.responses[req.ID]
	rc.mu.RUnlock()

	if !exists {
		t.Fatal("Expected response to exist")
	}
	if resp.Decision != DecisionApproved {
		t.Errorf("Expected approved, got %s", resp.Decision)
	}
}

func TestReviewHook(t *testing.T) {
	rc := NewReviewCheckpoint(nil, nil)

	beforeCalled := false
	afterCalled := false

	hook := &testHook{
		beforeFn: func(req *ReviewRequest) error {
			beforeCalled = true
			return nil
		},
		afterFn: func(req *ReviewRequest, resp *ReviewResponse) error {
			afterCalled = true
			return nil
		},
	}

	rc.AddHook(hook)

	req, _ := rc.RequestReview(context.Background(), OperationFileDelete, "test.go", "delete", nil)

	if !beforeCalled {
		t.Error("Expected BeforeReview hook to be called")
	}

	rc.Approve(req.ID, "ok")

	if !afterCalled {
		t.Error("Expected AfterReview hook to be called")
	}
}

type testHook struct {
	beforeFn func(*ReviewRequest) error
	afterFn  func(*ReviewRequest, *ReviewResponse) error
}

func (h *testHook) BeforeReview(req *ReviewRequest) error {
	if h.beforeFn != nil {
		return h.beforeFn(req)
	}
	return nil
}

func (h *testHook) AfterReview(req *ReviewRequest, resp *ReviewResponse) error {
	if h.afterFn != nil {
		return h.afterFn(req, resp)
	}
	return nil
}

func TestMultiplePendingReviews(t *testing.T) {
	rc := NewReviewCheckpoint(nil, nil)

	// Create multiple review requests
	req1, _ := rc.RequestReview(context.Background(), OperationFileDelete, "file1.go", "delete", nil)
	req2, _ := rc.RequestReview(context.Background(), OperationFileDelete, "file2.go", "delete", nil)
	req3, _ := rc.RequestReview(context.Background(), OperationGitForcePush, "main", "force push", nil)

	pending := rc.GetPendingReviews()
	if len(pending) != 3 {
		t.Errorf("Expected 3 pending reviews, got %d", len(pending))
	}

	// Approve one
	rc.Approve(req1.ID, "ok")

	pending = rc.GetPendingReviews()
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending reviews, got %d", len(pending))
	}

	// Reject another
	rc.Reject(req2.ID, "nope")

	pending = rc.GetPendingReviews()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending review, got %d", len(pending))
	}

	// Last one
	rc.Approve(req3.ID, "go ahead")
	pending = rc.GetPendingReviews()
	if len(pending) != 0 {
		t.Error("Expected no pending reviews")
	}
}

func TestTask6FullIntegration(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "review_integration")
	defer os.RemoveAll(tmpDir)

	sm := NewStateManager(tmpDir, "integration_test")
	policy := DefaultReviewPolicy()
	policy.SetRequireReviewAboveLevel(DangerLevelMedium)
	policy.AddProtectedPath("secrets/*")

	rc := NewReviewCheckpoint(policy, sm)

	// Test 1: Safe operation auto-approves
	req1, err := rc.RequestReview(context.Background(), OperationNetworkRequest, "api.github.com", "fetch repos", nil)
	if err != nil {
		t.Fatalf("Failed to request review: %v", err)
	}
	if !req1.AutoApprove {
		t.Error("Expected auto-approve for safe operation")
	}
	t.Logf("✓ Safe operation auto-approved: %s", req1.ID)

	// Test 2: Dangerous operation requires review
	req2, err := rc.RequestReview(context.Background(), OperationFileDelete, "important.go", "delete important file", map[string]any{
		"git_tracked": true,
	})
	if err != nil {
		t.Fatalf("Failed to request review: %v", err)
	}
	if req2.AutoApprove {
		t.Error("Expected review required for dangerous operation")
	}
	t.Logf("✓ Dangerous operation requires review: %s (danger: %s)", req2.ID, req2.DangerLevel)

	// Test 3: Critical operation
	req3, err := rc.RequestReview(context.Background(), OperationGitForcePush, "main", "force push to main", nil)
	if err != nil {
		t.Fatalf("Failed to request review: %v", err)
	}
	if req3.DangerLevel != DangerLevelCritical {
		t.Errorf("Expected critical danger level, got %s", req3.DangerLevel)
	}
	t.Logf("✓ Critical operation detected: %s (danger: %s)", req3.ID, req3.DangerLevel)

	// Test 4: Approve and continue
	go func() {
		time.Sleep(10 * time.Millisecond)
		rc.Approve(req2.ID, "file is safe to delete")
	}()

	resp, err := rc.WaitForReview(context.Background(), req2.ID)
	if err != nil {
		t.Fatalf("Failed to wait for review: %v", err)
	}
	if resp.Decision != DecisionApproved {
		t.Errorf("Expected approved, got %s", resp.Decision)
	}
	t.Logf("✓ Review approved: %s -> %s", req2.ID, resp.Decision)

	// Test 5: Verify state persistence
	cp := sm.GetLatestCheckpoint()
	if cp == nil {
		t.Fatal("Expected checkpoint in state manager")
	}
	t.Logf("✓ State persisted with checkpoint: %s", cp.ID)

	// Test 6: Protected path triggers review
	req4, err := rc.RequestReview(context.Background(), OperationFileOverwrite, "secrets/api_key.txt", "update API key", nil)
	if err != nil {
		t.Fatalf("Failed to request review: %v", err)
	}
	if req4.AutoApprove {
		t.Error("Expected review required for protected path")
	}
	t.Logf("✓ Protected path triggers review: %s", req4.Target)

	t.Log("✅ Task 6: Review Checkpoint System - Full integration PASSED")
}
