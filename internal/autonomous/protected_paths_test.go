// Package autonomous - Task 27: Tests for protected paths
package autonomous

import (
	"testing"
)

func TestProtectedPathAction_Constants(t *testing.T) {
	if ActionBlock != "block" {
		t.Error("ActionBlock should be 'block'")
	}
	if ActionWarn != "warn" {
		t.Error("ActionWarn should be 'warn'")
	}
	if ActionLog != "log" {
		t.Error("ActionLog should be 'log'")
	}
	if ActionAllow != "allow" {
		t.Error("ActionAllow should be 'allow'")
	}
}

func TestProtectedPathType_Constants(t *testing.T) {
	if TypeExact != "exact" {
		t.Error("TypeExact should be 'exact'")
	}
	if TypePrefix != "prefix" {
		t.Error("TypePrefix should be 'prefix'")
	}
	if TypeGlob != "glob" {
		t.Error("TypeGlob should be 'glob'")
	}
	if TypeRegex != "regex" {
		t.Error("TypeRegex should be 'regex'")
	}
}

func TestDefaultProtectedPathConfig(t *testing.T) {
	config := DefaultProtectedPathConfig()

	if len(config.Paths) == 0 {
		t.Error("Default config should have protected paths")
	}

	// Check for critical paths
	foundPasswd := false
	for _, p := range config.Paths {
		if p.Path == "/etc/passwd" {
			foundPasswd = true
			if p.Action != ActionBlock {
				t.Error("/etc/passwd should be blocked")
			}
		}
	}

	if !foundPasswd {
		t.Error("Default config should protect /etc/passwd")
	}
}

func TestNewProtectedPathsManager(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()
	if ppm == nil {
		t.Fatal("Expected non-nil manager")
	}

	if len(ppm.config.Paths) == 0 {
		t.Error("Manager should have default paths")
	}
}

func TestProtectedPathsManager_CheckPath_Exact(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	action, rule := ppm.CheckPath("/etc/passwd", "read")
	if action != ActionBlock {
		t.Error("/etc/passwd should be blocked")
	}
	if rule == nil {
		t.Error("Should match a rule")
	}

	// Non-protected path
	action, _ = ppm.CheckPath("/tmp/file.txt", "read")
	if action == ActionBlock {
		t.Error("/tmp/file.txt should not be blocked")
	}
}

func TestProtectedPathsManager_CheckPath_Prefix(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	action, _ := ppm.CheckPath("/etc/ssh/sshd_config", "read")
	if action != ActionBlock {
		t.Error("/etc/ssh/ prefix should be blocked")
	}
}

func TestProtectedPathsManager_CheckPath_Glob(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	action, _ := ppm.CheckPath("secrets.pem", "read")
	if action != ActionBlock {
		t.Error("*.pem should be blocked")
	}

	action, _ = ppm.CheckPath("mykey.key", "read")
	if action != ActionBlock {
		t.Error("*.key should be blocked")
	}
}

func TestProtectedPathsManager_IsAccessAllowed(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	if ppm.IsAccessAllowed("/etc/passwd", "read") {
		t.Error("/etc/passwd access should not be allowed")
	}

	if !ppm.IsAccessAllowed("/tmp/file.txt", "read") {
		t.Error("/tmp/file.txt access should be allowed")
	}
}

func TestProtectedPathsManager_ShouldWarn(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	// AWS credentials should warn (not block)
	// Note: this expands to home directory
	action, _ := ppm.CheckPath("~/.aws/credentials", "read")
	if action != ActionWarn {
		t.Error("~/.aws/ should warn, not block")
	}
}

func TestProtectedPathsManager_AddProtectedPath(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	initialCount := len(ppm.GetProtectedPaths())

	ppm.AddProtectedPath(ProtectedPath{
		Path:        "/custom/protected",
		Type:        TypeExact,
		Action:      ActionBlock,
		Description: "Custom protected path",
		Enabled:     true,
		Priority:    50,
	})

	if len(ppm.GetProtectedPaths()) != initialCount+1 {
		t.Error("Path should be added")
	}

	action, _ := ppm.CheckPath("/custom/protected", "read")
	if action != ActionBlock {
		t.Error("Added path should be protected")
	}
}

func TestProtectedPathsManager_RemoveProtectedPath(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	// Add then remove
	ppm.AddProtectedPath(ProtectedPath{
		Path:    "/test/remove",
		Type:    TypeExact,
		Action:  ActionBlock,
		Enabled: true,
	})

	removed := ppm.RemoveProtectedPath("/test/remove")
	if !removed {
		t.Error("Path should be removed")
	}

	removed = ppm.RemoveProtectedPath("/test/remove")
	if removed {
		t.Error("Path should already be removed")
	}
}

func TestProtectedPathsManager_GetAuditLog(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	// Access a protected path to generate audit entry
	ppm.CheckPath("/etc/passwd", "read")

	log := ppm.GetAuditLog()
	if len(log) == 0 {
		t.Error("Access should generate audit log entry")
	}
}

func TestProtectedPathsManager_ClearAuditLog(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	ppm.CheckPath("/etc/passwd", "read")
	ppm.ClearAuditLog()

	log := ppm.GetAuditLog()
	if len(log) != 0 {
		t.Error("Audit log should be cleared")
	}
}

func TestProtectedPathsManager_GetStats(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	stats := ppm.GetStats()

	if stats["total_rules"].(int) == 0 {
		t.Error("Should have some rules")
	}

	if stats["enabled_rules"].(int) == 0 {
		t.Error("Should have enabled rules")
	}
}

func TestProtectedPathsManager_ValidatePath(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	err := ppm.ValidatePath("/etc/passwd", "read")
	if err == nil {
		t.Error("/etc/passwd should fail validation")
	}

	err = ppm.ValidatePath("/tmp/safe.txt", "read")
	if err != nil {
		t.Errorf("/tmp/safe.txt should pass validation: %v", err)
	}
}

func TestProtectedPathsManager_ExportImport(t *testing.T) {
	ppm := NewDefaultProtectedPathsManager()

	data, err := ppm.ExportConfig()
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	newPpm := NewProtectedPathsManager(ProtectedPathConfig{})
	err = newPpm.ImportConfig(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if len(newPpm.GetProtectedPaths()) != len(ppm.GetProtectedPaths()) {
		t.Error("Imported config should match original")
	}
}

func TestProtectedPathsBuilder(t *testing.T) {
	ppm := NewProtectedPathsBuilder().
		WithDefaultAction(ActionBlock).
		WithAuditEnabled(true).
		AddExactPath("/exact/path", ActionBlock, "Exact path test").
		AddPrefixPath("/prefix/", ActionWarn, "Prefix test").
		AddGlobPath("*.test", ActionLog, "Glob test").
		Build()

	if ppm == nil {
		t.Fatal("Builder should create manager")
	}

	// Test exact path
	action, _ := ppm.CheckPath("/exact/path", "read")
	if action != ActionBlock {
		t.Error("Exact path should block")
	}

	// Test prefix path
	action, _ = ppm.CheckPath("/prefix/subpath/file", "read")
	if action != ActionWarn {
		t.Error("Prefix path should warn")
	}

	// Test glob path
	action, _ = ppm.CheckPath("file.test", "read")
	if action != ActionLog {
		t.Error("Glob path should log")
	}
}

func TestProtectedPathsBuilder_AddRegexPath(t *testing.T) {
	ppm := NewProtectedPathsBuilder().
		WithDefaultAction(ActionAllow).
		AddRegexPath(`^/var/log/.*\.log$`, ActionLog, "Log files").
		Build()

	action, _ := ppm.CheckPath("/var/log/app.log", "read")
	if action != ActionLog {
		t.Error("Regex path should match")
	}

	action, _ = ppm.CheckPath("/var/log/other.txt", "read")
	if action == ActionLog {
		t.Error("Regex path should not match .txt files")
	}
}

func TestProtectedPathsManager_Priority(t *testing.T) {
	ppm := NewProtectedPathsBuilder().
		WithPriority(100).AddExactPath("/test", ActionBlock, "High priority").
		WithPriority(50).AddPrefixPath("/test/", ActionWarn, "Low priority").
		Build()

	// Higher priority rule should win
	action, _ := ppm.CheckPath("/test", "read")
	if action != ActionBlock {
		t.Error("Higher priority exact match should win")
	}
}

func TestProtectedPathsManager_DisabledRule(t *testing.T) {
	ppm := NewProtectedPathsManager(ProtectedPathConfig{
		Paths: []ProtectedPath{
			{Path: "/disabled", Type: TypeExact, Action: ActionBlock, Enabled: false},
		},
		DefaultAction: ActionAllow,
	})

	action, _ := ppm.CheckPath("/disabled", "read")
	if action != ActionAllow {
		t.Error("Disabled rule should not apply")
	}
}

func TestProtectedPathsManager_OperationSpecific(t *testing.T) {
	ppm := NewProtectedPathsManager(ProtectedPathConfig{
		Paths: []ProtectedPath{
			{
				Path:       "/data",
				Type:       TypeExact,
				Action:     ActionBlock,
				Enabled:    true,
				Operations: []string{"write", "delete"},
			},
		},
		DefaultAction: ActionAllow,
	})

	// Read should be allowed
	action, _ := ppm.CheckPath("/data", "read")
	if action != ActionAllow {
		t.Error("Read should be allowed on operation-specific rule")
	}

	// Write should be blocked
	action, _ = ppm.CheckPath("/data", "write")
	if action != ActionBlock {
		t.Error("Write should be blocked")
	}
}

func TestTask27ProtectedPaths(t *testing.T) {
	// Comprehensive test for Task 27

	// Test 1: Default configuration
	ppm := NewDefaultProtectedPathsManager()

	if !ppm.IsAccessAllowed("/tmp/file.txt", "read") {
		t.Error("Regular files should be accessible")
	}

	if ppm.IsAccessAllowed("/etc/passwd", "read") {
		t.Error("System files should be blocked")
	}

	// Test 2: Custom rules
	ppm.AddProtectedPath(ProtectedPath{
		Path:        "/project/secrets",
		Type:        TypePrefix,
		Action:      ActionBlock,
		Description: "Project secrets directory",
		Enabled:     true,
		Priority:    90,
	})

	if ppm.IsAccessAllowed("/project/secrets/api.key", "read") {
		t.Error("Custom protected path should block")
	}

	// Test 3: Audit logging
	ppm.ClearAuditLog()
	ppm.CheckPath("/etc/shadow", "read")

	log := ppm.GetAuditLog()
	if len(log) == 0 {
		t.Error("Protected access should be logged")
	}

	// Test 4: Stats
	stats := ppm.GetStats()
	if stats["total_rules"].(int) == 0 {
		t.Error("Should have rules configured")
	}

	// Test 5: Builder pattern
	custom := NewProtectedPathsBuilder().
		AddGlobPath("*.secret", ActionBlock, "Secret files").
		Build()

	if custom.IsAccessAllowed("api.secret", "read") {
		t.Error("Custom glob pattern should block")
	}
}
