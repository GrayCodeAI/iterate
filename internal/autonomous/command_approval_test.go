// Package autonomous - Task 28: Tests for Command Approval Workflow
package autonomous

import (
	"context"
	"testing"
	"time"
)

func TestApprovalStatus_Constants(t *testing.T) {
	if ApprovalStatusPending != "pending" {
		t.Error("ApprovalStatusPending should be 'pending'")
	}
	if ApprovalStatusApproved != "approved" {
		t.Error("ApprovalStatusApproved should be 'approved'")
	}
	if ApprovalStatusDenied != "denied" {
		t.Error("ApprovalStatusDenied should be 'denied'")
	}
	if ApprovalStatusExpired != "expired" {
		t.Error("ApprovalStatusExpired should be 'expired'")
	}
	if ApprovalStatusAutoApproved != "auto_approved" {
		t.Error("ApprovalStatusAutoApproved should be 'auto_approved'")
	}
}

func TestApprovalMode_Constants(t *testing.T) {
	if ApprovalModeStrict != "strict" {
		t.Error("ApprovalModeStrict should be 'strict'")
	}
	if ApprovalModeBalanced != "balanced" {
		t.Error("ApprovalModeBalanced should be 'balanced'")
	}
	if ApprovalModePermissive != "permissive" {
		t.Error("ApprovalModePermissive should be 'permissive'")
	}
	if ApprovalModeAuto != "auto" {
		t.Error("ApprovalModeAuto should be 'auto'")
	}
}

func TestDefaultApprovalPolicy(t *testing.T) {
	policy := DefaultApprovalPolicy()
	
	if policy.Mode != ApprovalModeBalanced {
		t.Error("Default mode should be balanced")
	}
	if !policy.AutoApproveSafe {
		t.Error("Default should auto-approve safe operations")
	}
	if !policy.AutoApproveLow {
		t.Error("Default should auto-approve low-risk operations")
	}
	if policy.RequireApprovalMedium {
		t.Error("Default should not require approval for medium risk")
	}
	if !policy.RequireApprovalHigh {
		t.Error("Default should require approval for high risk")
	}
	if !policy.RequireApprovalCritical {
		t.Error("Default should require approval for critical risk")
	}
}

func TestNewCommandApprovalManager(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	if cam == nil {
		t.Fatal("Expected non-nil manager")
	}
	
	if cam.assessor == nil {
		t.Error("Assessor should be set")
	}
	
	if cam.requests == nil {
		t.Error("Requests map should be initialized")
	}
}

func TestCommandApprovalManager_RequestApproval_AutoApproved(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Safe command should be auto-approved
	ctx := context.Background()
	request, err := cam.RequestApproval(ctx, "ls", []string{"-la"}, "/home/user")
	
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if request.Status != ApprovalStatusAutoApproved {
		t.Errorf("Safe command should be auto-approved, got: %s", request.Status)
	}
	
	if request.Assessment == nil {
		t.Error("Assessment should be populated")
	}
}

func TestCommandApprovalManager_RequestApproval_NeedsApproval(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Dangerous command should need approval
	ctx := context.Background()
	request, err := cam.RequestApproval(ctx, "rm", []string{"-rf", "/"}, "/")
	
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if request.Status != ApprovalStatusPending {
		t.Errorf("Dangerous command should be pending, got: %s", request.Status)
	}
}

func TestCommandApprovalManager_Approve(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	ctx := context.Background()
	request, _ := cam.RequestApproval(ctx, "rm", []string{"-rf", "/tmp/test"}, "/tmp")
	
	if request.Status != ApprovalStatusPending {
		t.Fatal("Request should be pending")
	}
	
	err := cam.Approve(request.ID, "test-user")
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	
	updated, exists := cam.GetRequest(request.ID)
	if !exists {
		t.Fatal("Request should exist")
	}
	
	if updated.Status != ApprovalStatusApproved {
		t.Errorf("Status should be approved, got: %s", updated.Status)
	}
	
	if updated.ApprovedBy != "test-user" {
		t.Error("ApprovedBy should be set")
	}
}

func TestCommandApprovalManager_Deny(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	ctx := context.Background()
	request, _ := cam.RequestApproval(ctx, "rm", []string{"-rf", "/important"}, "/")
	
	err := cam.Deny(request.ID, "Too risky")
	if err != nil {
		t.Fatalf("Deny failed: %v", err)
	}
	
	updated, _ := cam.GetRequest(request.ID)
	if updated.Status != ApprovalStatusDenied {
		t.Errorf("Status should be denied, got: %s", updated.Status)
	}
	
	if updated.DenialReason != "Too risky" {
		t.Error("Denial reason should be set")
	}
}

func TestCommandApprovalManager_Cancel(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	ctx := context.Background()
	request, _ := cam.RequestApproval(ctx, "rm", []string{"-rf", "/tmp"}, "/")
	
	err := cam.Cancel(request.ID)
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	
	updated, _ := cam.GetRequest(request.ID)
	if updated.Status != ApprovalStatusCancelled {
		t.Errorf("Status should be cancelled, got: %s", updated.Status)
	}
}

func TestCommandApprovalManager_ExpirePending(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Set very short timeout
	cam.SetPolicy(ApprovalPolicy{
		Mode:                ApprovalModeStrict,
		Timeout:             1 * time.Millisecond,
		MaxPendingRequests:  100,
	})
	
	ctx := context.Background()
	request, _ := cam.RequestApproval(ctx, "rm", []string{"test"}, "/")
	
	// Wait for expiration
	time.Sleep(10 * time.Millisecond)
	
	expired := cam.ExpirePending()
	if expired != 1 {
		t.Errorf("Expected 1 expired, got: %d", expired)
	}
	
	updated, _ := cam.GetRequest(request.ID)
	if updated.Status != ApprovalStatusExpired {
		t.Errorf("Status should be expired, got: %s", updated.Status)
	}
}

func TestCommandApprovalManager_GetPending(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Use strict mode to ensure requests stay pending
	cam.SetPolicy(ApprovalPolicy{
		Mode:               ApprovalModeStrict,
		Timeout:            5 * time.Minute,
		MaxPendingRequests: 100,
	})
	
	ctx := context.Background()
	
	// Create multiple requests
	cam.RequestApproval(ctx, "rm", []string{"test1"}, "/")
	cam.RequestApproval(ctx, "rm", []string{"test2"}, "/")
	
	pending := cam.GetPending()
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending, got: %d", len(pending))
	}
}

func TestCommandApprovalManager_IsApproved(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	ctx := context.Background()
	
	// Auto-approved request
	safeRequest, _ := cam.RequestApproval(ctx, "ls", []string{}, "/")
	if !cam.IsApproved(safeRequest.ID) {
		t.Error("Auto-approved request should be approved")
	}
	
	// Use strict mode to test pending
	cam.SetPolicy(ApprovalPolicy{
		Mode:               ApprovalModeStrict,
		Timeout:            5 * time.Minute,
		MaxPendingRequests: 100,
	})
	
	// Pending request
	pendingRequest, _ := cam.RequestApproval(ctx, "rm", []string{"test"}, "/")
	if cam.IsApproved(pendingRequest.ID) {
		t.Error("Pending request should not be approved")
	}
	
	// Manually approved request
	cam.Approve(pendingRequest.ID, "user")
	if !cam.IsApproved(pendingRequest.ID) {
		t.Error("Approved request should be approved")
	}
}

func TestCommandApprovalManager_GetStats(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Use strict mode to have pending requests
	cam.SetPolicy(ApprovalPolicy{
		Mode:               ApprovalModeStrict,
		Timeout:            5 * time.Minute,
		MaxPendingRequests: 100,
	})
	
	ctx := context.Background()
	
	// Create various requests
	cam.RequestApproval(ctx, "ls", []string{}, "/")           // pending (strict mode)
	cam.RequestApproval(ctx, "rm", []string{"test"}, "/")     // pending
	cam.RequestApproval(ctx, "rm", []string{"test2"}, "/")    // pending
	
	pending := cam.GetPending()
	cam.Deny(pending[0].ID, "test deny")
	
	stats := cam.GetStats()
	
	if stats.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got: %d", stats.TotalRequests)
	}
	
	if stats.DeniedRequests != 1 {
		t.Errorf("Expected 1 denied, got: %d", stats.DeniedRequests)
	}
}

func TestCommandApprovalManager_ApprovalModes(t *testing.T) {
	assessor := NewDangerAssessor()
	
	tests := []struct {
		name          string
		mode          ApprovalMode
		command       string
		args          []string
		expectPending bool
	}{
		{"Strict mode - safe command", ApprovalModeStrict, "ls", []string{}, true},
		{"Auto mode - dangerous command", ApprovalModeAuto, "rm", []string{"-rf", "/"}, false},
		{"Permissive mode - medium risk", ApprovalModePermissive, "npm", []string{"install"}, false},
		{"Permissive mode - critical", ApprovalModePermissive, "rm", []string{"-rf", "/"}, true},
		{"Balanced mode - low risk", ApprovalModeBalanced, "cat", []string{"file.txt"}, false},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cam := NewCommandApprovalManager(assessor)
			cam.SetPolicy(ApprovalPolicy{
				Mode:                    tc.mode,
				Timeout:                 5 * time.Minute,
				MaxPendingRequests:      100,
				AutoApproveSafe:         true,
				AutoApproveLow:          true,
				RequireApprovalHigh:     true,
				RequireApprovalCritical: true,
			})
			
			ctx := context.Background()
			request, _ := cam.RequestApproval(ctx, tc.command, tc.args, "/")
			
			if tc.expectPending && request.Status == ApprovalStatusAutoApproved {
				t.Error("Expected pending, got auto-approved")
			}
			if !tc.expectPending && request.Status == ApprovalStatusPending {
				t.Error("Expected auto-approved, got pending")
			}
		})
	}
}

func TestCommandApprovalManager_Blocklist(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Set policy with blocklist
	cam.SetPolicy(ApprovalPolicy{
		Mode:               ApprovalModeBalanced,
		AutoApproveSafe:    true,
		AutoApproveLow:     true,
		Blocklist:          []string{"npm"},
		Timeout:            5 * time.Minute,
		MaxPendingRequests: 100,
	})
	
	ctx := context.Background()
	request, _ := cam.RequestApproval(ctx, "npm", []string{"install"}, "/")
	
	// npm should be blocked even if normally safe
	if request.Status == ApprovalStatusAutoApproved {
		t.Error("Blocklisted command should not be auto-approved")
	}
}

func TestCommandApprovalManager_Allowlist(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Set policy with allowlist (strict mode with exceptions)
	cam.SetPolicy(ApprovalPolicy{
		Mode:               ApprovalModeStrict,
		Allowlist:          []string{"ls"},
		Timeout:            5 * time.Minute,
		MaxPendingRequests: 100,
	})
	
	ctx := context.Background()
	request, _ := cam.RequestApproval(ctx, "ls", []string{}, "/")
	
	// ls should be auto-approved due to allowlist
	if request.Status != ApprovalStatusAutoApproved {
		t.Errorf("Allowlisted command should be auto-approved, got: %s", request.Status)
	}
}

func TestCommandApprovalManager_MaxPendingRequests(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Set very low max pending
	cam.SetPolicy(ApprovalPolicy{
		Mode:               ApprovalModeStrict,
		MaxPendingRequests: 2,
		Timeout:            5 * time.Minute,
	})
	
	ctx := context.Background()
	
	// First two should succeed
	_, err1 := cam.RequestApproval(ctx, "rm", []string{"test1"}, "/")
	_, err2 := cam.RequestApproval(ctx, "rm", []string{"test2"}, "/")
	
	if err1 != nil || err2 != nil {
		t.Fatal("First two requests should succeed")
	}
	
	// Third should fail
	_, err3 := cam.RequestApproval(ctx, "rm", []string{"test3"}, "/")
	if err3 == nil {
		t.Error("Third request should fail due to max pending limit")
	}
}

func TestCommandApprovalManager_WaitForApproval(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Use strict mode to ensure pending
	cam.SetPolicy(ApprovalPolicy{
		Mode:               ApprovalModeStrict,
		Timeout:            5 * time.Minute,
		MaxPendingRequests: 100,
	})
	
	ctx := context.Background()
	request, _ := cam.RequestApproval(ctx, "rm", []string{"test"}, "/")
	
	// Approve in goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		cam.Approve(request.ID, "async-user")
	}()
	
	waitCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	
	result, err := cam.WaitForApproval(waitCtx, request.ID)
	if err != nil {
		t.Fatalf("WaitForApproval failed: %v", err)
	}
	
	if result.Status != ApprovalStatusApproved {
		t.Errorf("Expected approved status, got: %s", result.Status)
	}
}

func TestCommandApprovalManager_WaitForApproval_Timeout(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	ctx := context.Background()
	request, _ := cam.RequestApproval(ctx, "rm", []string{"test"}, "/")
	
	// Very short timeout
	waitCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	
	_, err := cam.WaitForApproval(waitCtx, request.ID)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestCommandApprovalManager_RequestAndWait(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Use strict mode to ensure pending
	cam.SetPolicy(ApprovalPolicy{
		Mode:               ApprovalModeStrict,
		Timeout:            5 * time.Minute,
		MaxPendingRequests: 100,
	})
	
	// Set callback that auto-approves
	cam.SetCallback(func(req *ApprovalRequest) (*ApprovalDecision, error) {
		return &ApprovalDecision{
			RequestID:  req.ID,
			Approved:   true,
			ApprovedBy: "callback",
		}, nil
	})
	
	ctx := context.Background()
	request, err := cam.RequestAndWait(ctx, "rm", []string{"test"}, "/")
	
	if err != nil {
		t.Fatalf("RequestAndWait failed: %v", err)
	}
	
	if request.Status != ApprovalStatusApproved {
		t.Errorf("Expected approved, got: %s", request.Status)
	}
}

func TestCommandApprovalManager_ExportRequests(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	ctx := context.Background()
	cam.RequestApproval(ctx, "ls", []string{}, "/")
	cam.RequestApproval(ctx, "rm", []string{"test"}, "/")
	
	data, err := cam.ExportRequests()
	if err != nil {
		t.Fatalf("ExportRequests failed: %v", err)
	}
	
	if len(data) == 0 {
		t.Error("Export should produce data")
	}
}

func TestCommandApprovalManager_ApproveNonExistent(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	err := cam.Approve("nonexistent", "user")
	if err == nil {
		t.Error("Should fail for non-existent request")
	}
}

func TestCommandApprovalManager_ApproveAlreadyDecided(t *testing.T) {
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	ctx := context.Background()
	request, _ := cam.RequestApproval(ctx, "rm", []string{"test"}, "/")
	
	// Approve once
	cam.Approve(request.ID, "user1")
	
	// Try to approve again
	err := cam.Approve(request.ID, "user2")
	if err == nil {
		t.Error("Should fail for already decided request")
	}
}

func TestTask28CommandApproval(t *testing.T) {
	// Comprehensive test for Task 28
	
	// Test 1: Create manager with default policy
	assessor := NewDangerAssessor()
	cam := NewCommandApprovalManager(assessor)
	
	// Test 2: Auto-approve safe command
	ctx := context.Background()
	safeReq, err := cam.RequestApproval(ctx, "echo", []string{"hello"}, "/")
	if err != nil {
		t.Fatalf("Safe request failed: %v", err)
	}
	if safeReq.Status != ApprovalStatusAutoApproved {
		t.Error("Safe command should be auto-approved")
	}
	
	// Test 3: High-risk command needs approval
	riskReq, _ := cam.RequestApproval(ctx, "rm", []string{"-rf", "/important"}, "/")
	if riskReq.Status != ApprovalStatusPending {
		t.Error("Risky command should be pending")
	}
	
	// Test 4: Approve the risky command
	cam.Approve(riskReq.ID, "admin")
	if !cam.IsApproved(riskReq.ID) {
		t.Error("Approved request should be marked as approved")
	}
	
	// Test 5: Check stats
	stats := cam.GetStats()
	if stats.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got: %d", stats.TotalRequests)
	}
	if stats.AutoApproved != 1 {
		t.Errorf("Expected 1 auto-approved, got: %d", stats.AutoApproved)
	}
	if stats.ApprovedRequests != 1 {
		t.Errorf("Expected 1 approved, got: %d", stats.ApprovedRequests)
	}
	
	// Test 6: Change policy to strict
	cam.SetPolicy(ApprovalPolicy{
		Mode:               ApprovalModeStrict,
		Timeout:            5 * time.Minute,
		MaxPendingRequests: 100,
	})
	
	strictReq, _ := cam.RequestApproval(ctx, "ls", []string{}, "/")
	if strictReq.Status != ApprovalStatusPending {
		t.Error("Strict mode should require approval for all commands")
	}
	
	// Test 7: Deny request
	cam.Deny(strictReq.ID, "Not allowed in strict mode")
	updated, _ := cam.GetRequest(strictReq.ID)
	if updated.Status != ApprovalStatusDenied {
		t.Error("Request should be denied")
	}
}
