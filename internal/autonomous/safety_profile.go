// Package autonomous - Task 32: Safety Profile system (strict/balanced/permissive)
package autonomous

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SafetyProfileName represents predefined safety profile names.
type SafetyProfileName string

const (
	// SafetyProfileStrict - Maximum safety, minimal automation
	// - All operations require approval
	// - Auto-snapshot on all changes
	// - All paths protected by default
	// - Network disabled
	SafetyProfileStrict SafetyProfileName = "strict"

	// SafetyProfileBalanced - Default profile, good balance
	// - Risky operations require approval
	// - Auto-snapshot on destructive ops
	// - Critical paths protected
	// - Network with restrictions
	SafetyProfileBalanced SafetyProfileName = "balanced"

	// SafetyProfilePermissive - High trust, high automation
	// - Only critical operations need approval
	// - Snapshots on user request
	// - Minimal path protection
	// - Full network access
	SafetyProfilePermissive SafetyProfileName = "permissive"

	// SafetyProfileCustom - User-defined profile
	SafetyProfileCustom SafetyProfileName = "custom"
)

// SafetyProfile contains all safety-related configuration.
type SafetyProfile struct {
	// Name is the profile name
	Name SafetyProfileName `json:"name"`

	// DisplayName is a human-readable name
	DisplayName string `json:"display_name"`

	// Description explains the profile
	Description string `json:"description"`

	// Approval settings
	Approval ApprovalProfileSettings `json:"approval"`

	// DangerThresholds defines when actions are triggered
	DangerThresholds DangerThresholdSettings `json:"danger_thresholds"`

	// Protection settings
	Protection ProtectionSettings `json:"protection"`

	// Network settings
	Network NetworkSafetySettings `json:"network"`

	// Snapshot settings
	Snapshot SnapshotSafetySettings `json:"snapshot"`

	// Resource limits
	Resources ResourceSafetySettings `json:"resources"`

	// Audit settings
	Audit AuditSafetySettings `json:"audit"`

	// CustomRules are user-defined rules
	CustomRules []CustomSafetyRule `json:"custom_rules,omitempty"`

	// CreatedAt is when the profile was created
	CreatedAt time.Time `json:"created_at"`

	// ModifiedAt is when the profile was last modified
	ModifiedAt time.Time `json:"modified_at"`
}

// ApprovalProfileSettings controls approval behavior.
type ApprovalProfileSettings struct {
	// Mode is the approval mode
	Mode ApprovalMode `json:"mode"`

	// AutoApproveSafe automatically approves safe operations
	AutoApproveSafe bool `json:"auto_approve_safe"`

	// AutoApproveLow automatically approves low-risk operations
	AutoApproveLow bool `json:"auto_approve_low"`

	// RequireReason requires a reason for approval
	RequireReason bool `json:"require_reason"`

	// Timeout is how long to wait for approval
	Timeout time.Duration `json:"timeout"`

	// MaxPendingRequests is the maximum pending approvals
	MaxPendingRequests int `json:"max_pending_requests"`

	// NotifyOnRequest sends notification on approval request
	NotifyOnRequest bool `json:"notify_on_request"`
}

// DangerThresholdSettings defines danger level thresholds.
type DangerThresholdSettings struct {
	// ApprovalRequired is the minimum danger level requiring approval
	ApprovalRequired DangerLevel `json:"approval_required"`

	// ConfirmationRequired is the minimum level requiring confirmation
	ConfirmationRequired DangerLevel `json:"confirmation_required"`

	// AutoReject is the level at which operations are auto-rejected
	AutoReject DangerLevel `json:"auto_reject"`

	// WarningLevel is the level at which warnings are shown
	WarningLevel DangerLevel `json:"warning_level"`

	// BlockLevel is the level at which operations are blocked
	BlockLevel DangerLevel `json:"block_level"`
}

// ProtectionSettings controls file and path protection.
type ProtectionSettings struct {
	// Enabled turns on path protection
	Enabled bool `json:"enabled"`

	// ProtectedPaths are paths that cannot be modified
	ProtectedPaths []string `json:"protected_paths"`

	// ProtectedPatterns are glob patterns for protected paths
	ProtectedPatterns []string `json:"protected_patterns"`

	// AllowOverride allows overriding protection with explicit approval
	AllowOverride bool `json:"allow_override"`

	// ProtectGit protects .git directories
	ProtectGit bool `json:"protect_git"`

	// ProtectEnv protects .env files
	ProtectEnv bool `json:"protect_env"`

	// ProtectConfig protects config files
	ProtectConfig bool `json:"protect_config"`

	// ProtectHome protects home directory
	ProtectHome bool `json:"protect_home"`

	// ProtectSystem protects system directories
	ProtectSystem bool `json:"protect_system"`
}

// NetworkSafetySettings controls network access.
type NetworkSafetySettings struct {
	// Enabled allows network operations
	Enabled bool `json:"enabled"`

	// AllowedHosts are hosts that can be accessed
	AllowedHosts []string `json:"allowed_hosts"`

	// BlockedHosts are hosts that cannot be accessed
	BlockedHosts []string `json:"blocked_hosts"`

	// AllowedPorts are ports that can be used
	AllowedPorts []int `json:"allowed_ports"`

	// RequireApprovalForExternal requires approval for external network access
	RequireApprovalForExternal bool `json:"require_approval_for_external"`

	// MaxConnections is the maximum concurrent connections
	MaxConnections int `json:"max_connections"`

	// Timeout is the network operation timeout
	Timeout time.Duration `json:"timeout"`
}

// SnapshotSafetySettings controls snapshot behavior.
type SnapshotSafetySettings struct {
	// Enabled turns on auto-snapshot
	Enabled bool `json:"enabled"`

	// AutoOnDestructive creates snapshot before destructive ops
	AutoOnDestructive bool `json:"auto_on_destructive"`

	// AutoOnModify creates snapshot before modifications
	AutoOnModify bool `json:"auto_on_modify"`

	// AutoOnAll creates snapshot before any operation
	AutoOnAll bool `json:"auto_on_all"`

	// MinDangerLevel is the minimum danger level to trigger snapshot
	MinDangerLevel DangerLevel `json:"min_danger_level"`

	// RetentionCount is how many snapshots to keep
	RetentionCount int `json:"retention_count"`

	// RetentionDuration is how long to keep snapshots
	RetentionDuration time.Duration `json:"retention_duration"`

	// VerifyAfterRestore verifies integrity after restore
	VerifyAfterRestore bool `json:"verify_after_restore"`
}

// ResourceSafetySettings controls resource limits.
type ResourceSafetySettings struct {
	// MaxCPU is the maximum CPU percentage (0-100)
	MaxCPU int `json:"max_cpu"`

	// MaxMemoryMB is the maximum memory in MB
	MaxMemoryMB int `json:"max_memory_mb"`

	// MaxFileSize is the maximum file size in bytes
	MaxFileSize int64 `json:"max_file_size"`

	// MaxExecutionTime is the maximum operation duration
	MaxExecutionTime time.Duration `json:"max_execution_time"`

	// MaxFileOperations is the max file operations per session
	MaxFileOperations int `json:"max_file_operations"`

	// MaxNetworkBytes is the max network transfer in bytes
	MaxNetworkBytes int64 `json:"max_network_bytes"`
}

// AuditSafetySettings controls audit logging.
type AuditSafetySettings struct {
	// Enabled turns on audit logging
	Enabled bool `json:"enabled"`

	// LogAllOperations logs all operations
	LogAllOperations bool `json:"log_all_operations"`

	// LogApprovals logs approval decisions
	LogApprovals bool `json:"log_approvals"`

	// LogRejections logs rejections
	LogRejections bool `json:"log_rejections"`

	// LogSnapshots logs snapshot operations
	LogSnapshots bool `json:"log_snapshots"`

	// LogNetwork logs network operations
	LogNetwork bool `json:"log_network"`

	// RetentionDays is how long to keep logs
	RetentionDays int `json:"retention_days"`

	// IncludeContent includes file content in logs
	IncludeContent bool `json:"include_content"`
}

// CustomSafetyRule defines a user-defined safety rule.
type CustomSafetyRule struct {
	// ID is the unique rule identifier
	ID string `json:"id"`

	// Name is the rule name
	Name string `json:"name"`

	// Description explains the rule
	Description string `json:"description"`

	// Enabled turns the rule on/off
	Enabled bool `json:"enabled"`

	// Pattern is the command/operation pattern to match
	Pattern string `json:"pattern"`

	// Action is what to do when matched (allow, deny, require_approval, warn)
	Action string `json:"action"`

	// Priority is the rule priority (higher = checked first)
	Priority int `json:"priority"`

	// DangerLevelOverride overrides the danger level for matched operations
	DangerLevelOverride *DangerLevel `json:"danger_level_override,omitempty"`

	// Message is shown when the rule is triggered
	Message string `json:"message,omitempty"`
}

// SafetyProfileManager manages safety profiles.
type SafetyProfileManager struct {
	mu sync.RWMutex

	// profiles stores available profiles
	profiles map[SafetyProfileName]*SafetyProfile

	// activeProfile is the currently active profile
	activeProfile SafetyProfileName

	// configPath is where profiles are saved
	configPath string

	// timeNow is a function to get current time (for testing)
	timeNow func() time.Time
}

// NewSafetyProfileManager creates a new safety profile manager.
func NewSafetyProfileManager(configPath string) *SafetyProfileManager {
	mgr := &SafetyProfileManager{
		profiles:      make(map[SafetyProfileName]*SafetyProfile),
		activeProfile: SafetyProfileBalanced,
		configPath:    configPath,
		timeNow:       time.Now,
	}

	// Initialize built-in profiles
	mgr.initBuiltinProfiles()

	// Load custom profiles if they exist
	mgr.loadProfiles()

	return mgr
}

// initBuiltinProfiles initializes the built-in safety profiles.
func (mgr *SafetyProfileManager) initBuiltinProfiles() {
	now := mgr.timeNow()

	// Strict profile - maximum safety
	mgr.profiles[SafetyProfileStrict] = &SafetyProfile{
		Name:        SafetyProfileStrict,
		DisplayName: "Strict",
		Description: "Maximum safety with minimal automation. All operations require approval.",
		Approval: ApprovalProfileSettings{
			Mode:               ApprovalModeStrict,
			AutoApproveSafe:    false,
			AutoApproveLow:     false,
			RequireReason:      true,
			Timeout:            5 * time.Minute,
			MaxPendingRequests: 10,
			NotifyOnRequest:    true,
		},
		DangerThresholds: DangerThresholdSettings{
			ApprovalRequired:     DangerLevelLow,
			ConfirmationRequired: DangerLevelSafe,
			AutoReject:           DangerLevelCritical,
			WarningLevel:         DangerLevelLow,
			BlockLevel:           DangerLevelCritical,
		},
		Protection: ProtectionSettings{
			Enabled:           true,
			ProtectedPaths:    []string{},
			ProtectedPatterns: []string{"*", "**/*"},
			AllowOverride:     true,
			ProtectGit:        true,
			ProtectEnv:        true,
			ProtectConfig:     true,
			ProtectHome:       true,
			ProtectSystem:     true,
		},
		Network: NetworkSafetySettings{
			Enabled:                    false,
			AllowedHosts:               []string{},
			BlockedHosts:               []string{"*"},
			AllowedPorts:               []int{},
			RequireApprovalForExternal: true,
			MaxConnections:             0,
			Timeout:                    30 * time.Second,
		},
		Snapshot: SnapshotSafetySettings{
			Enabled:            true,
			AutoOnDestructive:  true,
			AutoOnModify:       true,
			AutoOnAll:          false,
			MinDangerLevel:     DangerLevelSafe,
			RetentionCount:     100,
			RetentionDuration:  7 * 24 * time.Hour,
			VerifyAfterRestore: true,
		},
		Resources: ResourceSafetySettings{
			MaxCPU:            50,
			MaxMemoryMB:       512,
			MaxFileSize:       10 * 1024 * 1024, // 10MB
			MaxExecutionTime:  30 * time.Second,
			MaxFileOperations: 100,
			MaxNetworkBytes:   0,
		},
		Audit: AuditSafetySettings{
			Enabled:          true,
			LogAllOperations: true,
			LogApprovals:     true,
			LogRejections:    true,
			LogSnapshots:     true,
			LogNetwork:       true,
			RetentionDays:    30,
			IncludeContent:   false,
		},
		CreatedAt:  now,
		ModifiedAt: now,
	}

	// Balanced profile - default
	mgr.profiles[SafetyProfileBalanced] = &SafetyProfile{
		Name:        SafetyProfileBalanced,
		DisplayName: "Balanced",
		Description: "Default profile with good balance between safety and automation.",
		Approval: ApprovalProfileSettings{
			Mode:               ApprovalModeBalanced,
			AutoApproveSafe:    true,
			AutoApproveLow:     true,
			RequireReason:      false,
			Timeout:            2 * time.Minute,
			MaxPendingRequests: 20,
			NotifyOnRequest:    true,
		},
		DangerThresholds: DangerThresholdSettings{
			ApprovalRequired:     DangerLevelHigh,
			ConfirmationRequired: DangerLevelMedium,
			AutoReject:           DangerLevelCritical,
			WarningLevel:         DangerLevelMedium,
			BlockLevel:           DangerLevelCritical,
		},
		Protection: ProtectionSettings{
			Enabled:           true,
			ProtectedPaths:    []string{},
			ProtectedPatterns: []string{".env", "*.key", "*.pem", "*.p12"},
			AllowOverride:     true,
			ProtectGit:        true,
			ProtectEnv:        true,
			ProtectConfig:     true,
			ProtectHome:       false,
			ProtectSystem:     true,
		},
		Network: NetworkSafetySettings{
			Enabled:                    true,
			AllowedHosts:               []string{},
			BlockedHosts:               []string{},
			AllowedPorts:               []int{80, 443},
			RequireApprovalForExternal: false,
			MaxConnections:             10,
			Timeout:                    60 * time.Second,
		},
		Snapshot: SnapshotSafetySettings{
			Enabled:            true,
			AutoOnDestructive:  true,
			AutoOnModify:       false,
			AutoOnAll:          false,
			MinDangerLevel:     DangerLevelMedium,
			RetentionCount:     50,
			RetentionDuration:  24 * time.Hour,
			VerifyAfterRestore: true,
		},
		Resources: ResourceSafetySettings{
			MaxCPU:            80,
			MaxMemoryMB:       1024,
			MaxFileSize:       100 * 1024 * 1024, // 100MB
			MaxExecutionTime:  5 * time.Minute,
			MaxFileOperations: 500,
			MaxNetworkBytes:   100 * 1024 * 1024, // 100MB
		},
		Audit: AuditSafetySettings{
			Enabled:          true,
			LogAllOperations: false,
			LogApprovals:     true,
			LogRejections:    true,
			LogSnapshots:     true,
			LogNetwork:       false,
			RetentionDays:    14,
			IncludeContent:   false,
		},
		CreatedAt:  now,
		ModifiedAt: now,
	}

	// Permissive profile - high trust
	mgr.profiles[SafetyProfilePermissive] = &SafetyProfile{
		Name:        SafetyProfilePermissive,
		DisplayName: "Permissive",
		Description: "High trust environment with maximum automation. Only critical operations need approval.",
		Approval: ApprovalProfileSettings{
			Mode:               ApprovalModePermissive,
			AutoApproveSafe:    true,
			AutoApproveLow:     true,
			RequireReason:      false,
			Timeout:            1 * time.Minute,
			MaxPendingRequests: 50,
			NotifyOnRequest:    false,
		},
		DangerThresholds: DangerThresholdSettings{
			ApprovalRequired:     DangerLevelCritical,
			ConfirmationRequired: DangerLevelHigh,
			AutoReject:           DangerLevelCritical,
			WarningLevel:         DangerLevelHigh,
			BlockLevel:           DangerLevelCritical,
		},
		Protection: ProtectionSettings{
			Enabled:           true,
			ProtectedPaths:    []string{},
			ProtectedPatterns: []string{"*.key"},
			AllowOverride:     true,
			ProtectGit:        true,
			ProtectEnv:        false,
			ProtectConfig:     false,
			ProtectHome:       false,
			ProtectSystem:     false,
		},
		Network: NetworkSafetySettings{
			Enabled:                    true,
			AllowedHosts:               []string{},
			BlockedHosts:               []string{},
			AllowedPorts:               []int{},
			RequireApprovalForExternal: false,
			MaxConnections:             50,
			Timeout:                    120 * time.Second,
		},
		Snapshot: SnapshotSafetySettings{
			Enabled:            true,
			AutoOnDestructive:  false,
			AutoOnModify:       false,
			AutoOnAll:          false,
			MinDangerLevel:     DangerLevelCritical,
			RetentionCount:     20,
			RetentionDuration:  6 * time.Hour,
			VerifyAfterRestore: true,
		},
		Resources: ResourceSafetySettings{
			MaxCPU:            100,
			MaxMemoryMB:       2048,
			MaxFileSize:       1024 * 1024 * 1024, // 1GB
			MaxExecutionTime:  30 * time.Minute,
			MaxFileOperations: 0,                  // unlimited
			MaxNetworkBytes:   1024 * 1024 * 1024, // 1GB
		},
		Audit: AuditSafetySettings{
			Enabled:          true,
			LogAllOperations: false,
			LogApprovals:     true,
			LogRejections:    false,
			LogSnapshots:     true,
			LogNetwork:       false,
			RetentionDays:    7,
			IncludeContent:   false,
		},
		CreatedAt:  now,
		ModifiedAt: now,
	}
}

// loadProfiles loads custom profiles from disk.
func (mgr *SafetyProfileManager) loadProfiles() {
	if mgr.configPath == "" {
		return
	}

	data, err := os.ReadFile(filepath.Join(mgr.configPath, "safety_profiles.json"))
	if err != nil {
		return
	}

	var customProfiles []*SafetyProfile
	if err := json.Unmarshal(data, &customProfiles); err != nil {
		return
	}

	for _, profile := range customProfiles {
		profile.Name = SafetyProfileCustom
		mgr.profiles[SafetyProfileCustom] = profile
	}
}

// GetProfile returns a safety profile by name.
func (mgr *SafetyProfileManager) GetProfile(name SafetyProfileName) (*SafetyProfile, bool) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	profile, exists := mgr.profiles[name]
	if !exists {
		return nil, false
	}

	// Return a copy
	copy := *profile
	return &copy, true
}

// GetActiveProfile returns the currently active profile.
func (mgr *SafetyProfileManager) GetActiveProfile() *SafetyProfile {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if profile, exists := mgr.profiles[mgr.activeProfile]; exists {
		copy := *profile
		return &copy
	}

	// Fallback to balanced
	if profile, exists := mgr.profiles[SafetyProfileBalanced]; exists {
		copy := *profile
		return &copy
	}

	return nil
}

// SetActiveProfile sets the active profile.
func (mgr *SafetyProfileManager) SetActiveProfile(name SafetyProfileName) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if _, exists := mgr.profiles[name]; !exists {
		return fmt.Errorf("profile %s not found", name)
	}

	mgr.activeProfile = name
	return nil
}

// GetActiveProfileName returns the name of the active profile.
func (mgr *SafetyProfileManager) GetActiveProfileName() SafetyProfileName {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.activeProfile
}

// ListProfiles returns all available profile names.
func (mgr *SafetyProfileManager) ListProfiles() []SafetyProfileName {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	names := make([]SafetyProfileName, 0, len(mgr.profiles))
	for name := range mgr.profiles {
		names = append(names, name)
	}
	return names
}

// CreateCustomProfile creates a new custom profile based on an existing one.
func (mgr *SafetyProfileManager) CreateCustomProfile(base SafetyProfileName, customizations ...ProfileCustomization) (*SafetyProfile, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	baseProfile, exists := mgr.profiles[base]
	if !exists {
		return nil, fmt.Errorf("base profile %s not found", base)
	}

	// Copy the base profile
	newProfile := *baseProfile
	newProfile.Name = SafetyProfileCustom
	newProfile.DisplayName = "Custom"
	newProfile.Description = "Custom safety profile"
	newProfile.CreatedAt = mgr.timeNow()
	newProfile.ModifiedAt = mgr.timeNow()

	// Apply customizations
	for _, custom := range customizations {
		custom(&newProfile)
	}

	mgr.profiles[SafetyProfileCustom] = &newProfile

	// Save to disk
	mgr.saveProfiles()

	return &newProfile, nil
}

// ProfileCustomization is a function that customizes a profile.
type ProfileCustomization func(*SafetyProfile)

// WithApprovalMode sets the approval mode.
func WithApprovalMode(mode ApprovalMode) ProfileCustomization {
	return func(p *SafetyProfile) {
		p.Approval.Mode = mode
	}
}

// WithNetworkEnabled sets network access.
func WithNetworkEnabled(enabled bool) ProfileCustomization {
	return func(p *SafetyProfile) {
		p.Network.Enabled = enabled
	}
}

// WithAutoSnapshot sets auto-snapshot behavior.
func WithAutoSnapshot(enabled bool, minDangerLevel DangerLevel) ProfileCustomization {
	return func(p *SafetyProfile) {
		p.Snapshot.Enabled = enabled
		p.Snapshot.MinDangerLevel = minDangerLevel
		p.Snapshot.AutoOnDestructive = enabled && minDangerLevel <= DangerLevelMedium
	}
}

// WithProtectionEnabled sets protection behavior.
func WithProtectionEnabled(enabled bool) ProfileCustomization {
	return func(p *SafetyProfile) {
		p.Protection.Enabled = enabled
	}
}

// WithDangerThreshold sets danger thresholds.
func WithDangerThreshold(approvalRequired, confirmationRequired DangerLevel) ProfileCustomization {
	return func(p *SafetyProfile) {
		p.DangerThresholds.ApprovalRequired = approvalRequired
		p.DangerThresholds.ConfirmationRequired = confirmationRequired
	}
}

// UpdateProfile updates an existing profile.
func (mgr *SafetyProfileManager) UpdateProfile(name SafetyProfileName, customizations ...ProfileCustomization) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	profile, exists := mgr.profiles[name]
	if !exists {
		return fmt.Errorf("profile %s not found", name)
	}

	// Built-in profiles can be customized but not renamed
	if name != SafetyProfileCustom {
		customProfile := *profile
		customProfile.Name = SafetyProfileCustom
		profile = &customProfile
		mgr.profiles[SafetyProfileCustom] = profile
	}

	for _, custom := range customizations {
		custom(profile)
	}

	profile.ModifiedAt = mgr.timeNow()

	// Save to disk
	mgr.saveProfiles()

	return nil
}

// ResetProfile resets a built-in profile to defaults.
func (mgr *SafetyProfileManager) ResetProfile(name SafetyProfileName) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if name == SafetyProfileCustom {
		return fmt.Errorf("cannot reset custom profile")
	}

	// Re-initialize built-in profiles
	mgr.initBuiltinProfiles()

	return nil
}

// ShouldApprove determines if an operation needs approval based on the active profile.
func (mgr *SafetyProfileManager) ShouldApprove(dangerLevel DangerLevel) bool {
	profile := mgr.GetActiveProfile()
	if profile == nil {
		return dangerLevel >= DangerLevelHigh // Default safe behavior
	}

	// Check approval mode
	switch profile.Approval.Mode {
	case ApprovalModeStrict:
		return true
	case ApprovalModeAuto:
		return false
	case ApprovalModePermissive:
		return dangerLevel >= DangerLevelCritical
	case ApprovalModeBalanced:
		fallthrough
	default:
		return dangerLevel >= profile.DangerThresholds.ApprovalRequired
	}
}

// ShouldConfirm determines if an operation needs confirmation.
func (mgr *SafetyProfileManager) ShouldConfirm(dangerLevel DangerLevel) bool {
	profile := mgr.GetActiveProfile()
	if profile == nil {
		return dangerLevel >= DangerLevelMedium
	}

	return dangerLevel >= profile.DangerThresholds.ConfirmationRequired
}

// ShouldSnapshot determines if an operation should trigger a snapshot.
func (mgr *SafetyProfileManager) ShouldSnapshot(dangerLevel DangerLevel, operation string) bool {
	profile := mgr.GetActiveProfile()
	if profile == nil || !profile.Snapshot.Enabled {
		return false
	}

	if dangerLevel < profile.Snapshot.MinDangerLevel {
		return false
	}

	if profile.Snapshot.AutoOnAll {
		return true
	}

	if profile.Snapshot.AutoOnDestructive {
		return dangerLevel >= DangerLevelHigh
	}

	if profile.Snapshot.AutoOnModify {
		return dangerLevel >= DangerLevelMedium
	}

	return false
}

// IsPathProtected determines if a path is protected.
func (mgr *SafetyProfileManager) IsPathProtected(path string) (bool, string) {
	profile := mgr.GetActiveProfile()
	if profile == nil || !profile.Protection.Enabled {
		return false, ""
	}

	// Check explicit protected paths
	for _, protected := range profile.Protection.ProtectedPaths {
		if path == protected {
			return true, "explicitly protected"
		}
	}

	// Check patterns
	for _, pattern := range profile.Protection.ProtectedPatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true, "matches protected pattern: " + pattern
		}
	}

	// Check built-in protections
	if profile.Protection.ProtectGit {
		if filepath.Base(path) == ".git" || filepath.Base(filepath.Dir(path)) == ".git" {
			return true, "git directory protected"
		}
	}

	if profile.Protection.ProtectEnv {
		if filepath.Base(path) == ".env" || filepath.Ext(path) == ".env" {
			return true, "environment file protected"
		}
	}

	if profile.Protection.ProtectConfig {
		configPatterns := []string{"*.json", "*.yaml", "*.yml", "*.toml", "*.ini", "*.conf"}
		for _, pattern := range configPatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return true, "config file protected"
			}
		}
	}

	if profile.Protection.ProtectHome {
		home, _ := os.UserHomeDir()
		if home != "" && (path == home || filepath.HasPrefix(path, home+string(filepath.Separator))) {
			return true, "home directory protected"
		}
	}

	if profile.Protection.ProtectSystem {
		systemPaths := []string{"/etc", "/usr", "/bin", "/sbin", "/var", "/root"}
		for _, sysPath := range systemPaths {
			if path == sysPath || filepath.HasPrefix(path, sysPath+string(filepath.Separator)) {
				return true, "system directory protected"
			}
		}
	}

	return false, ""
}

// IsNetworkAllowed determines if network access is allowed.
func (mgr *SafetyProfileManager) IsNetworkAllowed(host string, port int) (bool, string) {
	profile := mgr.GetActiveProfile()
	if profile == nil {
		return false, "no active profile"
	}

	if !profile.Network.Enabled {
		return false, "network access disabled"
	}

	// Check blocked hosts
	for _, blocked := range profile.Network.BlockedHosts {
		if blocked == "*" || blocked == host {
			return false, "host blocked"
		}
	}

	// Check allowed hosts (empty means all allowed)
	if len(profile.Network.AllowedHosts) > 0 {
		allowed := false
		for _, allowedHost := range profile.Network.AllowedHosts {
			if allowedHost == host {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, "host not in allowed list"
		}
	}

	// Check ports (empty means all allowed)
	if len(profile.Network.AllowedPorts) > 0 {
		allowed := false
		for _, allowedPort := range profile.Network.AllowedPorts {
			if allowedPort == port {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, "port not in allowed list"
		}
	}

	return true, ""
}

// AddCustomRule adds a custom safety rule.
func (mgr *SafetyProfileManager) AddCustomRule(rule CustomSafetyRule) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	profile, exists := mgr.profiles[SafetyProfileCustom]
	if !exists {
		return fmt.Errorf("custom profile not found")
	}

	// Generate ID if not set
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule-%d", mgr.timeNow().UnixNano())
	}

	profile.CustomRules = append(profile.CustomRules, rule)
	profile.ModifiedAt = mgr.timeNow()

	return nil
}

// RemoveCustomRule removes a custom safety rule.
func (mgr *SafetyProfileManager) RemoveCustomRule(ruleID string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	profile, exists := mgr.profiles[SafetyProfileCustom]
	if !exists {
		return fmt.Errorf("custom profile not found")
	}

	for i, rule := range profile.CustomRules {
		if rule.ID == ruleID {
			profile.CustomRules = append(profile.CustomRules[:i], profile.CustomRules[i+1:]...)
			profile.ModifiedAt = mgr.timeNow()
			return nil
		}
	}

	return fmt.Errorf("rule %s not found", ruleID)
}

// EvaluateCustomRules evaluates custom rules for an operation.
func (mgr *SafetyProfileManager) EvaluateCustomRules(operation string) (string, *DangerLevel, string) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	profile, exists := mgr.profiles[mgr.activeProfile]
	if !exists {
		return "allow", nil, ""
	}

	// Sort rules by priority (higher first)
	rules := make([]CustomSafetyRule, len(profile.CustomRules))
	copy(rules, profile.CustomRules)

	// Simple bubble sort by priority
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[j].Priority > rules[i].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}

	// Evaluate rules
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Try filepath.Match first
		if matched, _ := filepath.Match(rule.Pattern, operation); matched {
			return rule.Action, rule.DangerLevelOverride, rule.Message
		}

		// Also try simple prefix/suffix matching for convenience
		// Pattern like "rm*" should match "rm -rf /"
		if strings.HasSuffix(rule.Pattern, "*") {
			prefix := strings.TrimSuffix(rule.Pattern, "*")
			if strings.HasPrefix(operation, prefix) {
				return rule.Action, rule.DangerLevelOverride, rule.Message
			}
		}

		// Pattern like "*.js" should match files ending in .js
		if strings.HasPrefix(rule.Pattern, "*") {
			suffix := strings.TrimPrefix(rule.Pattern, "*")
			if strings.HasSuffix(operation, suffix) {
				return rule.Action, rule.DangerLevelOverride, rule.Message
			}
		}

		// Exact match
		if rule.Pattern == operation {
			return rule.Action, rule.DangerLevelOverride, rule.Message
		}
	}

	return "allow", nil, ""
}

// saveProfiles saves custom profiles to disk.
func (mgr *SafetyProfileManager) saveProfiles() {
	if mgr.configPath == "" {
		return
	}

	customProfiles := make([]*SafetyProfile, 0)
	for _, profile := range mgr.profiles {
		if profile.Name == SafetyProfileCustom {
			customProfiles = append(customProfiles, profile)
		}
	}

	if len(customProfiles) == 0 {
		return
	}

	data, err := json.MarshalIndent(customProfiles, "", "  ")
	if err != nil {
		return
	}

	os.MkdirAll(mgr.configPath, 0755)
	os.WriteFile(filepath.Join(mgr.configPath, "safety_profiles.json"), data, 0644)
}

// ExportProfile exports a profile to JSON.
func (mgr *SafetyProfileManager) ExportProfile(name SafetyProfileName) ([]byte, error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	profile, exists := mgr.profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile %s not found", name)
	}

	return json.MarshalIndent(profile, "", "  ")
}

// ImportProfile imports a profile from JSON.
func (mgr *SafetyProfileManager) ImportProfile(data []byte) error {
	var profile SafetyProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return fmt.Errorf("failed to parse profile: %w", err)
	}

	// Mark as custom
	profile.Name = SafetyProfileCustom
	profile.ModifiedAt = mgr.timeNow()

	mgr.mu.Lock()
	mgr.profiles[SafetyProfileCustom] = &profile
	mgr.mu.Unlock()

	// Save to disk
	mgr.saveProfiles()

	return nil
}

// CompareProfiles compares two profiles and returns the differences.
func CompareProfiles(p1, p2 *SafetyProfile) map[string]interface{} {
	diffs := make(map[string]interface{})

	if p1.Approval.Mode != p2.Approval.Mode {
		diffs["approval_mode"] = map[string]string{
			"from": string(p1.Approval.Mode),
			"to":   string(p2.Approval.Mode),
		}
	}

	if p1.Network.Enabled != p2.Network.Enabled {
		diffs["network_enabled"] = map[string]bool{
			"from": p1.Network.Enabled,
			"to":   p2.Network.Enabled,
		}
	}

	if p1.Snapshot.Enabled != p2.Snapshot.Enabled {
		diffs["snapshot_enabled"] = map[string]bool{
			"from": p1.Snapshot.Enabled,
			"to":   p2.Snapshot.Enabled,
		}
	}

	if p1.DangerThresholds.ApprovalRequired != p2.DangerThresholds.ApprovalRequired {
		diffs["approval_threshold"] = map[string]string{
			"from": p1.DangerThresholds.ApprovalRequired.String(),
			"to":   p2.DangerThresholds.ApprovalRequired.String(),
		}
	}

	return diffs
}
