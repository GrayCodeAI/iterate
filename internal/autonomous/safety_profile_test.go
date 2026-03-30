// Package autonomous - Task 32: Tests for Safety Profile system
package autonomous

import (
	"testing"
)

func TestSafetyProfileName_Constants(t *testing.T) {
	if SafetyProfileStrict != "strict" {
		t.Error("SafetyProfileStrict should be 'strict'")
	}
	if SafetyProfileBalanced != "balanced" {
		t.Error("SafetyProfileBalanced should be 'balanced'")
	}
	if SafetyProfilePermissive != "permissive" {
		t.Error("SafetyProfilePermissive should be 'permissive'")
	}
	if SafetyProfileCustom != "custom" {
		t.Error("SafetyProfileCustom should be 'custom'")
	}
}

func TestNewSafetyProfileManager(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	if mgr == nil {
		t.Fatal("Expected non-nil manager")
	}

	if mgr.activeProfile != SafetyProfileBalanced {
		t.Errorf("Default profile should be balanced, got: %s", mgr.activeProfile)
	}

	if len(mgr.profiles) != 3 {
		t.Errorf("Expected 3 built-in profiles, got: %d", len(mgr.profiles))
	}
}

func TestSafetyProfileManager_GetProfile(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Get strict profile
	profile, exists := mgr.GetProfile(SafetyProfileStrict)
	if !exists {
		t.Fatal("Strict profile should exist")
	}

	if profile.Name != SafetyProfileStrict {
		t.Errorf("Expected strict profile, got: %s", profile.Name)
	}

	if profile.DisplayName != "Strict" {
		t.Errorf("Expected 'Strict' display name, got: %s", profile.DisplayName)
	}
}

func TestSafetyProfileManager_GetActiveProfile(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	profile := mgr.GetActiveProfile()
	if profile == nil {
		t.Fatal("Expected non-nil active profile")
	}

	if profile.Name != SafetyProfileBalanced {
		t.Errorf("Default active profile should be balanced, got: %s", profile.Name)
	}
}

func TestSafetyProfileManager_SetActiveProfile(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	err := mgr.SetActiveProfile(SafetyProfileStrict)
	if err != nil {
		t.Fatalf("SetActiveProfile failed: %v", err)
	}

	if mgr.GetActiveProfileName() != SafetyProfileStrict {
		t.Error("Active profile should be strict")
	}
}

func TestSafetyProfileManager_SetActiveProfile_Invalid(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	err := mgr.SetActiveProfile(SafetyProfileName("invalid"))
	if err == nil {
		t.Error("Should fail for invalid profile")
	}
}

func TestSafetyProfileManager_ListProfiles(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	profiles := mgr.ListProfiles()
	if len(profiles) != 3 {
		t.Errorf("Expected 3 profiles, got: %d", len(profiles))
	}
}

func TestSafetyProfile_Strict_Settings(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	profile, _ := mgr.GetProfile(SafetyProfileStrict)

	// Check approval settings
	if profile.Approval.Mode != ApprovalModeStrict {
		t.Error("Strict profile should have strict approval mode")
	}
	if profile.Approval.AutoApproveSafe {
		t.Error("Strict profile should not auto-approve safe operations")
	}

	// Check network settings
	if profile.Network.Enabled {
		t.Error("Strict profile should have network disabled")
	}

	// Check protection settings
	if !profile.Protection.ProtectGit {
		t.Error("Strict profile should protect git directories")
	}
	if !profile.Protection.ProtectHome {
		t.Error("Strict profile should protect home directory")
	}
}

func TestSafetyProfile_Balanced_Settings(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	profile, _ := mgr.GetProfile(SafetyProfileBalanced)

	// Check approval settings
	if profile.Approval.Mode != ApprovalModeBalanced {
		t.Error("Balanced profile should have balanced approval mode")
	}
	if !profile.Approval.AutoApproveSafe {
		t.Error("Balanced profile should auto-approve safe operations")
	}

	// Check network settings
	if !profile.Network.Enabled {
		t.Error("Balanced profile should have network enabled")
	}

	// Check danger thresholds
	if profile.DangerThresholds.ApprovalRequired != DangerLevelHigh {
		t.Error("Balanced profile should require approval for high danger")
	}
}

func TestSafetyProfile_Permisive_Settings(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	profile, _ := mgr.GetProfile(SafetyProfilePermissive)

	// Check approval settings
	if profile.Approval.Mode != ApprovalModePermissive {
		t.Error("Permissive profile should have permissive approval mode")
	}

	// Check network settings
	if !profile.Network.Enabled {
		t.Error("Permissive profile should have network enabled")
	}

	// Check danger thresholds
	if profile.DangerThresholds.ApprovalRequired != DangerLevelCritical {
		t.Error("Permissive profile should only require approval for critical danger")
	}
}

func TestSafetyProfileManager_ShouldApprove(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Test with balanced profile (default)
	if !mgr.ShouldApprove(DangerLevelCritical) {
		t.Error("Balanced should require approval for critical")
	}
	if !mgr.ShouldApprove(DangerLevelHigh) {
		t.Error("Balanced should require approval for high")
	}
	if mgr.ShouldApprove(DangerLevelMedium) {
		t.Error("Balanced should NOT require approval for medium")
	}
	if mgr.ShouldApprove(DangerLevelLow) {
		t.Error("Balanced should NOT require approval for low")
	}
}

func TestSafetyProfileManager_ShouldApprove_Strict(t *testing.T) {
	mgr := NewSafetyProfileManager("")
	mgr.SetActiveProfile(SafetyProfileStrict)

	// Strict requires approval for everything
	if !mgr.ShouldApprove(DangerLevelSafe) {
		t.Error("Strict should require approval for safe")
	}
	if !mgr.ShouldApprove(DangerLevelLow) {
		t.Error("Strict should require approval for low")
	}
}

func TestSafetyProfileManager_ShouldApprove_Permisive(t *testing.T) {
	mgr := NewSafetyProfileManager("")
	mgr.SetActiveProfile(SafetyProfilePermissive)

	// Permissive only requires approval for critical
	if !mgr.ShouldApprove(DangerLevelCritical) {
		t.Error("Permissive should require approval for critical")
	}
	if mgr.ShouldApprove(DangerLevelHigh) {
		t.Error("Permissive should NOT require approval for high")
	}
}

func TestSafetyProfileManager_ShouldConfirm(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Test with balanced profile
	if !mgr.ShouldConfirm(DangerLevelHigh) {
		t.Error("Balanced should require confirmation for high")
	}
	if !mgr.ShouldConfirm(DangerLevelMedium) {
		t.Error("Balanced should require confirmation for medium")
	}
	if mgr.ShouldConfirm(DangerLevelLow) {
		t.Error("Balanced should NOT require confirmation for low")
	}
}

func TestSafetyProfileManager_ShouldSnapshot(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Test with balanced profile (auto on destructive, min level medium)
	if !mgr.ShouldSnapshot(DangerLevelHigh, "delete") {
		t.Error("Balanced should snapshot for high danger")
	}
	if !mgr.ShouldSnapshot(DangerLevelCritical, "delete") {
		t.Error("Balanced should snapshot for critical danger")
	}
	if mgr.ShouldSnapshot(DangerLevelLow, "read") {
		t.Error("Balanced should NOT snapshot for low danger")
	}
}

func TestSafetyProfileManager_IsPathProtected(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Test .git protection (balanced)
	protected, reason := mgr.IsPathProtected("/project/.git/config")
	if !protected {
		t.Error("Balanced should protect .git directory")
	}
	if reason == "" {
		t.Error("Should provide reason for protection")
	}

	// Test .env protection
	protected, _ = mgr.IsPathProtected("/project/.env")
	if !protected {
		t.Error("Balanced should protect .env files")
	}
}

func TestSafetyProfileManager_IsPathProtected_Strict(t *testing.T) {
	mgr := NewSafetyProfileManager("")
	mgr.SetActiveProfile(SafetyProfileStrict)

	// Strict protects everything
	protected, _ := mgr.IsPathProtected("/any/path.txt")
	if !protected {
		t.Error("Strict profile should protect all paths")
	}
}

func TestSafetyProfileManager_IsNetworkAllowed(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Test with balanced profile (ports 80, 443 allowed)
	allowed, _ := mgr.IsNetworkAllowed("example.com", 443)
	if !allowed {
		t.Error("Balanced should allow port 443")
	}

	allowed, _ = mgr.IsNetworkAllowed("example.com", 22)
	if allowed {
		t.Error("Balanced should NOT allow port 22")
	}
}

func TestSafetyProfileManager_IsNetworkAllowed_Strict(t *testing.T) {
	mgr := NewSafetyProfileManager("")
	mgr.SetActiveProfile(SafetyProfileStrict)

	// Strict blocks all network
	allowed, reason := mgr.IsNetworkAllowed("example.com", 443)
	if allowed {
		t.Error("Strict should block all network")
	}
	if reason == "" {
		t.Error("Should provide reason for blocking")
	}
}

func TestSafetyProfileManager_CreateCustomProfile(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	profile, err := mgr.CreateCustomProfile(SafetyProfileBalanced,
		WithApprovalMode(ApprovalModeStrict),
		WithNetworkEnabled(false),
	)

	if err != nil {
		t.Fatalf("CreateCustomProfile failed: %v", err)
	}

	if profile.Name != SafetyProfileCustom {
		t.Error("Custom profile should have custom name")
	}
	if profile.Approval.Mode != ApprovalModeStrict {
		t.Error("Custom profile should have strict approval mode")
	}
	if profile.Network.Enabled {
		t.Error("Custom profile should have network disabled")
	}
}

func TestSafetyProfileManager_CreateCustomProfile_InvalidBase(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	_, err := mgr.CreateCustomProfile(SafetyProfileName("invalid"))
	if err == nil {
		t.Error("Should fail with invalid base profile")
	}
}

func TestSafetyProfileManager_AddCustomRule(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Create custom profile first
	_, _ = mgr.CreateCustomProfile(SafetyProfileBalanced)

	rule := CustomSafetyRule{
		Name:        "Block npm install",
		Description: "Block npm install commands",
		Enabled:     true,
		Pattern:     "npm install*",
		Action:      "deny",
		Priority:    100,
		Message:     "npm install is blocked for security",
	}

	err := mgr.AddCustomRule(rule)
	if err != nil {
		t.Fatalf("AddCustomRule failed: %v", err)
	}

	// Verify rule was added
	customProfile, _ := mgr.GetProfile(SafetyProfileCustom)
	if len(customProfile.CustomRules) != 1 {
		t.Error("Custom rule should be added")
	}
}

func TestSafetyProfileManager_RemoveCustomRule(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Create custom profile with rule
	_, _ = mgr.CreateCustomProfile(SafetyProfileBalanced)

	rule := CustomSafetyRule{
		ID:       "test-rule",
		Name:     "Test Rule",
		Enabled:  true,
		Pattern:  "test*",
		Action:   "deny",
		Priority: 100,
	}
	mgr.AddCustomRule(rule)

	// Remove the rule
	err := mgr.RemoveCustomRule("test-rule")
	if err != nil {
		t.Fatalf("RemoveCustomRule failed: %v", err)
	}

	// Verify rule was removed
	customProfile, _ := mgr.GetProfile(SafetyProfileCustom)
	if len(customProfile.CustomRules) != 0 {
		t.Error("Custom rule should be removed")
	}
}

func TestSafetyProfileManager_RemoveCustomRule_NotFound(t *testing.T) {
	mgr := NewSafetyProfileManager("")
	_, _ = mgr.CreateCustomProfile(SafetyProfileBalanced)

	err := mgr.RemoveCustomRule("nonexistent")
	if err == nil {
		t.Error("Should fail for non-existent rule")
	}
}

func TestSafetyProfileManager_EvaluateCustomRules(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Create custom profile with rule
	_, _ = mgr.CreateCustomProfile(SafetyProfileBalanced)

	rule := CustomSafetyRule{
		Name:     "Block dangerous",
		Enabled:  true,
		Pattern:  "rm -rf*",
		Action:   "deny",
		Priority: 100,
		Message:  "Recursive delete is blocked",
	}
	mgr.AddCustomRule(rule)
	mgr.SetActiveProfile(SafetyProfileCustom)

	// Evaluate matching operation
	action, _, msg := mgr.EvaluateCustomRules("rm -rf /")
	if action != "deny" {
		t.Error("Should deny matching operation")
	}
	if msg != "Recursive delete is blocked" {
		t.Error("Should return custom message")
	}

	// Evaluate non-matching operation
	action, _, _ = mgr.EvaluateCustomRules("ls -la")
	if action != "allow" {
		t.Error("Should allow non-matching operation")
	}
}

func TestSafetyProfileManager_UpdateProfile(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	err := mgr.UpdateProfile(SafetyProfileBalanced,
		WithApprovalMode(ApprovalModeStrict),
	)

	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}

	// Custom profile should be created
	custom, exists := mgr.GetProfile(SafetyProfileCustom)
	if !exists {
		t.Fatal("Custom profile should exist after update")
	}

	if custom.Approval.Mode != ApprovalModeStrict {
		t.Error("Custom profile should have strict mode")
	}
}

func TestSafetyProfileManager_ResetProfile(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Reset should work for built-in profiles
	err := mgr.ResetProfile(SafetyProfileStrict)
	if err != nil {
		t.Fatalf("ResetProfile failed: %v", err)
	}

	// Reset should fail for custom
	err = mgr.ResetProfile(SafetyProfileCustom)
	if err == nil {
		t.Error("Should not be able to reset custom profile")
	}
}

func TestSafetyProfileManager_ExportImport(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Export balanced profile
	data, err := mgr.ExportProfile(SafetyProfileBalanced)
	if err != nil {
		t.Fatalf("ExportProfile failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Export should produce data")
	}

	// Import as custom
	err = mgr.ImportProfile(data)
	if err != nil {
		t.Fatalf("ImportProfile failed: %v", err)
	}

	// Verify custom profile exists
	_, exists := mgr.GetProfile(SafetyProfileCustom)
	if !exists {
		t.Error("Custom profile should exist after import")
	}
}

func TestSafetyProfileManager_ExportProfile_NotFound(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	_, err := mgr.ExportProfile(SafetyProfileName("invalid"))
	if err == nil {
		t.Error("Should fail for invalid profile")
	}
}

func TestCompareProfiles(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	p1, _ := mgr.GetProfile(SafetyProfileStrict)
	p2, _ := mgr.GetProfile(SafetyProfilePermissive)

	diffs := CompareProfiles(p1, p2)

	if len(diffs) == 0 {
		t.Error("Should have differences between strict and permissive")
	}

	// Check specific differences
	if _, ok := diffs["approval_mode"]; !ok {
		t.Error("Should have approval_mode difference")
	}
	if _, ok := diffs["network_enabled"]; !ok {
		t.Error("Should have network_enabled difference")
	}
}

func TestProfileCustomizations(t *testing.T) {
	mgr := NewSafetyProfileManager("")

	// Test all customizations
	profile, err := mgr.CreateCustomProfile(SafetyProfileBalanced,
		WithApprovalMode(ApprovalModeStrict),
		WithNetworkEnabled(false),
		WithAutoSnapshot(true, DangerLevelLow),
		WithProtectionEnabled(false),
		WithDangerThreshold(DangerLevelMedium, DangerLevelLow),
	)

	if err != nil {
		t.Fatalf("CreateCustomProfile failed: %v", err)
	}

	if profile.Approval.Mode != ApprovalModeStrict {
		t.Error("Approval mode should be strict")
	}
	if profile.Network.Enabled {
		t.Error("Network should be disabled")
	}
	if !profile.Snapshot.Enabled {
		t.Error("Snapshot should be enabled")
	}
	if profile.Snapshot.MinDangerLevel != DangerLevelLow {
		t.Error("Snapshot min danger should be low")
	}
	if profile.Protection.Enabled {
		t.Error("Protection should be disabled")
	}
	if profile.DangerThresholds.ApprovalRequired != DangerLevelMedium {
		t.Error("Approval threshold should be medium")
	}
}

func TestTask32SafetyProfile(t *testing.T) {
	// Comprehensive test for Task 32: Safety Profile system

	// Setup
	mgr := NewSafetyProfileManager("")

	// Test 1: Verify built-in profiles
	profiles := mgr.ListProfiles()
	if len(profiles) != 3 {
		t.Errorf("Expected 3 built-in profiles, got: %d", len(profiles))
	}

	// Test 2: Default is balanced
	if mgr.GetActiveProfileName() != SafetyProfileBalanced {
		t.Error("Default should be balanced")
	}

	// Test 3: Switch profiles
	mgr.SetActiveProfile(SafetyProfileStrict)
	if mgr.GetActiveProfileName() != SafetyProfileStrict {
		t.Error("Should be able to switch to strict")
	}

	// Test 4: Strict blocks everything
	if !mgr.ShouldApprove(DangerLevelSafe) {
		t.Error("Strict should require approval for safe")
	}

	// Test 5: Strict blocks network
	allowed, _ := mgr.IsNetworkAllowed("any.com", 443)
	if allowed {
		t.Error("Strict should block network")
	}

	// Test 6: Switch to permissive
	mgr.SetActiveProfile(SafetyProfilePermissive)

	// Test 7: Permissive allows most things
	if mgr.ShouldApprove(DangerLevelHigh) {
		t.Error("Permissive should not require approval for high")
	}
	if !mgr.ShouldApprove(DangerLevelCritical) {
		t.Error("Permissive should require approval for critical")
	}

	// Test 8: Create custom profile
	custom, err := mgr.CreateCustomProfile(SafetyProfileBalanced,
		WithApprovalMode(ApprovalModeStrict),
	)
	if err != nil {
		t.Fatalf("Custom profile creation failed: %v", err)
	}

	// Test 9: Custom profile is stored
	if custom.Name != SafetyProfileCustom {
		t.Error("Custom profile should have custom name")
	}

	// Test 10: Add custom rule
	rule := CustomSafetyRule{
		Name:     "Test Rule",
		Enabled:  true,
		Pattern:  "dangerous*",
		Action:   "deny",
		Priority: 100,
	}
	if err := mgr.AddCustomRule(rule); err != nil {
		t.Errorf("AddCustomRule failed: %v", err)
	}

	// Test 11: Export/Import
	data, err := mgr.ExportProfile(SafetyProfileStrict)
	if err != nil {
		t.Errorf("Export failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Export should produce data")
	}

	// Test 12: Compare profiles
	p1, _ := mgr.GetProfile(SafetyProfileStrict)
	p2, _ := mgr.GetProfile(SafetyProfilePermissive)
	diffs := CompareProfiles(p1, p2)
	if len(diffs) == 0 {
		t.Error("Strict and Permissive should have differences")
	}
}
