// Package autonomous - Task 27: Protected Paths configuration
package autonomous

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// ProtectedPathAction defines what action to take when a protected path is accessed.
type ProtectedPathAction string

const (
	// ActionBlock blocks access completely
	ActionBlock ProtectedPathAction = "block"

	// ActionWarn warns but allows access
	ActionWarn ProtectedPathAction = "warn"

	// ActionLog logs access without warning
	ActionLog ProtectedPathAction = "log"

	// ActionAllow allows access with tracking
	ActionAllow ProtectedPathAction = "allow"
)

// ProtectedPathType defines the type of protection.
type ProtectedPathType string

const (
	// TypeExact matches exact path
	TypeExact ProtectedPathType = "exact"

	// TypePrefix matches path prefix
	TypePrefix ProtectedPathType = "prefix"

	// TypeGlob matches glob pattern
	TypeGlob ProtectedPathType = "glob"

	// TypeRegex matches regex pattern
	TypeRegex ProtectedPathType = "regex"
)

// ProtectedPath defines a single protected path rule.
type ProtectedPath struct {
	// Path is the path or pattern to protect
	Path string `json:"path"`

	// Type is the matching type
	Type ProtectedPathType `json:"type"`

	// Action is the action to take on access
	Action ProtectedPathAction `json:"action"`

	// Description explains why this path is protected
	Description string `json:"description"`

	// Operations restricts protection to specific operations (empty = all)
	Operations []string `json:"operations,omitempty"`

	// Enabled toggles the rule
	Enabled bool `json:"enabled"`

	// Priority for rule ordering (higher = checked first)
	Priority int `json:"priority"`

	// compiledRegex caches compiled regex patterns
	compiledRegex *regexp.Regexp `json:"-"`
}

// ProtectedPathConfig holds the configuration for protected paths.
type ProtectedPathConfig struct {
	// Paths is the list of protected path rules
	Paths []ProtectedPath `json:"paths"`

	// DefaultAction is the action for unmatched sensitive paths
	DefaultAction ProtectedPathAction `json:"default_action"`

	// EnableAudit enables audit logging
	EnableAudit bool `json:"enable_audit"`

	// AuditFile is the path to the audit log file
	AuditFile string `json:"audit_file"`
}

// ProtectedPathsManager manages protected path rules.
type ProtectedPathsManager struct {
	mu sync.RWMutex

	// config is the current configuration
	config ProtectedPathConfig

	// pathsByPriority is sorted by priority for efficient checking
	pathsByPriority []ProtectedPath

	// auditLog stores access attempts
	auditLog []ProtectedPathAccess
}

// ProtectedPathAccess records an access attempt.
type ProtectedPathAccess struct {
	Path        string              `json:"path"`
	Operation   string              `json:"operation"`
	Action      ProtectedPathAction `json:"action"`
	MatchedRule string              `json:"matched_rule,omitempty"`
	Timestamp   int64               `json:"timestamp"`
	Allowed     bool                `json:"allowed"`
	UserID      string              `json:"user_id,omitempty"`
	SessionID   string              `json:"session_id,omitempty"`
}

// DefaultProtectedPathConfig returns the default configuration.
func DefaultProtectedPathConfig() ProtectedPathConfig {
	return ProtectedPathConfig{
		Paths: []ProtectedPath{
			// System critical files
			{Path: "/etc/passwd", Type: TypeExact, Action: ActionBlock, Description: "User account database", Enabled: true, Priority: 100},
			{Path: "/etc/shadow", Type: TypeExact, Action: ActionBlock, Description: "Password database", Enabled: true, Priority: 100},
			{Path: "/etc/sudoers", Type: TypeExact, Action: ActionBlock, Description: "Sudo configuration", Enabled: true, Priority: 100},
			{Path: "/etc/ssh/", Type: TypePrefix, Action: ActionBlock, Description: "SSH configuration directory", Enabled: true, Priority: 90},

			// User secrets
			{Path: "~/.ssh/", Type: TypePrefix, Action: ActionBlock, Description: "SSH keys directory", Enabled: true, Priority: 90},
			{Path: "~/.gnupg/", Type: TypePrefix, Action: ActionBlock, Description: "GPG keys directory", Enabled: true, Priority: 90},
			{Path: "~/.netrc", Type: TypeExact, Action: ActionBlock, Description: "Network credentials", Enabled: true, Priority: 90},

			// Project secrets
			{Path: ".env", Type: TypeGlob, Action: ActionBlock, Description: "Environment files", Enabled: true, Priority: 80},
			{Path: "*.pem", Type: TypeGlob, Action: ActionBlock, Description: "Certificate files", Enabled: true, Priority: 80},
			{Path: "*.key", Type: TypeGlob, Action: ActionBlock, Description: "Key files", Enabled: true, Priority: 80},
			{Path: "id_rsa*", Type: TypeGlob, Action: ActionBlock, Description: "SSH private keys", Enabled: true, Priority: 80},
			{Path: "id_ed25519*", Type: TypeGlob, Action: ActionBlock, Description: "ED25519 keys", Enabled: true, Priority: 80},

			// Credentials files
			{Path: "credentials.json", Type: TypeExact, Action: ActionBlock, Description: "Credentials file", Enabled: true, Priority: 80},
			{Path: "secrets.json", Type: TypeExact, Action: ActionBlock, Description: "Secrets file", Enabled: true, Priority: 80},
			{Path: "service-account.json", Type: TypeExact, Action: ActionBlock, Description: "Service account key", Enabled: true, Priority: 80},
			{Path: "*.p12", Type: TypeGlob, Action: ActionBlock, Description: "PKCS12 certificates", Enabled: true, Priority: 80},
			{Path: "*.pfx", Type: TypeGlob, Action: ActionBlock, Description: "PFX certificates", Enabled: true, Priority: 80},

			// Cloud credentials
			{Path: "~/.aws/", Type: TypePrefix, Action: ActionWarn, Description: "AWS credentials", Enabled: true, Priority: 70},
			{Path: "~/.gcp/", Type: TypePrefix, Action: ActionWarn, Description: "GCP credentials", Enabled: true, Priority: 70},
			{Path: "~/.azure/", Type: TypePrefix, Action: ActionWarn, Description: "Azure credentials", Enabled: true, Priority: 70},
			{Path: "~/.kube/", Type: TypePrefix, Action: ActionWarn, Description: "Kubernetes config", Enabled: true, Priority: 70},

			// Database files
			{Path: "*.db", Type: TypeGlob, Action: ActionWarn, Description: "Database files", Enabled: true, Priority: 50},
			{Path: "*.sqlite", Type: TypeGlob, Action: ActionWarn, Description: "SQLite databases", Enabled: true, Priority: 50},
		},
		DefaultAction: ActionLog,
		EnableAudit:   true,
	}
}

// NewProtectedPathsManager creates a new manager.
func NewProtectedPathsManager(config ProtectedPathConfig) *ProtectedPathsManager {
	ppm := &ProtectedPathsManager{
		config:   config,
		auditLog: make([]ProtectedPathAccess, 0),
	}

	ppm.sortPathsByPriority()
	ppm.compileRegexPatterns()

	return ppm
}

// NewDefaultProtectedPathsManager creates a manager with default configuration.
func NewDefaultProtectedPathsManager() *ProtectedPathsManager {
	return NewProtectedPathsManager(DefaultProtectedPathConfig())
}

// sortPathsByPriority sorts paths by priority (highest first).
func (ppm *ProtectedPathsManager) sortPathsByPriority() {
	ppm.pathsByPriority = make([]ProtectedPath, len(ppm.config.Paths))
	copy(ppm.pathsByPriority, ppm.config.Paths)

	// Simple bubble sort by priority (descending)
	for i := 0; i < len(ppm.pathsByPriority)-1; i++ {
		for j := i + 1; j < len(ppm.pathsByPriority); j++ {
			if ppm.pathsByPriority[j].Priority > ppm.pathsByPriority[i].Priority {
				ppm.pathsByPriority[i], ppm.pathsByPriority[j] = ppm.pathsByPriority[j], ppm.pathsByPriority[i]
			}
		}
	}
}

// compileRegexPatterns precompiles regex patterns.
func (ppm *ProtectedPathsManager) compileRegexPatterns() {
	for i := range ppm.pathsByPriority {
		if ppm.pathsByPriority[i].Type == TypeRegex {
			regex, err := regexp.Compile(ppm.pathsByPriority[i].Path)
			if err == nil {
				ppm.pathsByPriority[i].compiledRegex = regex
			}
		}
	}
}

// CheckPath checks if a path is protected and returns the appropriate action.
func (ppm *ProtectedPathsManager) CheckPath(path, operation string) (ProtectedPathAction, *ProtectedPath) {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()

	// Normalize the path
	normalizedPath := ppm.normalizePath(path)

	// Check each rule in priority order
	for i := range ppm.pathsByPriority {
		rule := &ppm.pathsByPriority[i]

		if !rule.Enabled {
			continue
		}

		// Check if operation is restricted
		if len(rule.Operations) > 0 {
			opAllowed := false
			for _, op := range rule.Operations {
				if op == operation {
					opAllowed = true
					break
				}
			}
			if !opAllowed {
				continue
			}
		}

		// Check if path matches
		if ppm.pathMatches(normalizedPath, rule) {
			// Log access
			if ppm.config.EnableAudit {
				ppm.logAccess(normalizedPath, operation, rule.Action, rule.Path, rule.Action != ActionBlock)
			}

			return rule.Action, rule
		}
	}

	// No rule matched, return default action
	return ppm.config.DefaultAction, nil
}

// normalizePath normalizes a path for comparison.
func (ppm *ProtectedPathsManager) normalizePath(path string) string {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Clean the path
	path = filepath.Clean(path)

	return path
}

// pathMatches checks if a path matches a protected path rule.
func (ppm *ProtectedPathsManager) pathMatches(path string, rule *ProtectedPath) bool {
	switch rule.Type {
	case TypeExact:
		return path == ppm.normalizePath(rule.Path)

	case TypePrefix:
		rulePath := ppm.normalizePath(rule.Path)
		return strings.HasPrefix(path, rulePath)

	case TypeGlob:
		matched, err := filepath.Match(rule.Path, filepath.Base(path))
		if err != nil {
			return false
		}
		return matched

	case TypeRegex:
		if rule.compiledRegex != nil {
			return rule.compiledRegex.MatchString(path)
		}
		return false

	default:
		return false
	}
}

// logAccess logs an access attempt.
func (ppm *ProtectedPathsManager) logAccess(path, operation string, action ProtectedPathAction, matchedRule string, allowed bool) {
	access := ProtectedPathAccess{
		Path:        path,
		Operation:   operation,
		Action:      action,
		MatchedRule: matchedRule,
		Timestamp:   currentTime(),
		Allowed:     allowed,
	}

	ppm.auditLog = append(ppm.auditLog, access)
}

// IsAccessAllowed checks if access to a path is allowed.
func (ppm *ProtectedPathsManager) IsAccessAllowed(path, operation string) bool {
	action, _ := ppm.CheckPath(path, operation)
	return action != ActionBlock
}

// ShouldWarn checks if access should trigger a warning.
func (ppm *ProtectedPathsManager) ShouldWarn(path, operation string) bool {
	action, _ := ppm.CheckPath(path, operation)
	return action == ActionWarn
}

// AddProtectedPath adds a new protected path rule.
func (ppm *ProtectedPathsManager) AddProtectedPath(rule ProtectedPath) {
	ppm.mu.Lock()
	defer ppm.mu.Unlock()

	ppm.config.Paths = append(ppm.config.Paths, rule)
	ppm.sortPathsByPriority()
	ppm.compileRegexPatterns()
}

// RemoveProtectedPath removes a protected path rule.
func (ppm *ProtectedPathsManager) RemoveProtectedPath(path string) bool {
	ppm.mu.Lock()
	defer ppm.mu.Unlock()

	for i, p := range ppm.config.Paths {
		if p.Path == path {
			ppm.config.Paths = append(ppm.config.Paths[:i], ppm.config.Paths[i+1:]...)
			ppm.sortPathsByPriority()
			return true
		}
	}
	return false
}

// UpdateProtectedPath updates an existing rule.
func (ppm *ProtectedPathsManager) UpdateProtectedPath(path string, updates ProtectedPath) bool {
	ppm.mu.Lock()
	defer ppm.mu.Unlock()

	for i, p := range ppm.config.Paths {
		if p.Path == path {
			// Preserve immutable fields
			updates.Path = p.Path
			updates.compiledRegex = p.compiledRegex
			ppm.config.Paths[i] = updates
			ppm.sortPathsByPriority()
			ppm.compileRegexPatterns()
			return true
		}
	}
	return false
}

// GetProtectedPaths returns all protected path rules.
func (ppm *ProtectedPathsManager) GetProtectedPaths() []ProtectedPath {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()
	return append([]ProtectedPath{}, ppm.config.Paths...)
}

// GetAuditLog returns the audit log.
func (ppm *ProtectedPathsManager) GetAuditLog() []ProtectedPathAccess {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()
	return append([]ProtectedPathAccess{}, ppm.auditLog...)
}

// ClearAuditLog clears the audit log.
func (ppm *ProtectedPathsManager) ClearAuditLog() {
	ppm.mu.Lock()
	defer ppm.mu.Unlock()
	ppm.auditLog = make([]ProtectedPathAccess, 0)
}

// ExportConfig exports the configuration as JSON.
func (ppm *ProtectedPathsManager) ExportConfig() ([]byte, error) {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()
	return json.MarshalIndent(ppm.config, "", "  ")
}

// ImportConfig imports configuration from JSON.
func (ppm *ProtectedPathsManager) ImportConfig(data []byte) error {
	ppm.mu.Lock()
	defer ppm.mu.Unlock()

	var config ProtectedPathConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	ppm.config = config
	ppm.sortPathsByPriority()
	ppm.compileRegexPatterns()

	return nil
}

// LoadConfigFromFile loads configuration from a file.
func (ppm *ProtectedPathsManager) LoadConfigFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	return ppm.ImportConfig(data)
}

// SaveConfigToFile saves configuration to a file.
func (ppm *ProtectedPathsManager) SaveConfigToFile(path string) error {
	data, err := ppm.ExportConfig()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetStats returns statistics about protected paths.
func (ppm *ProtectedPathsManager) GetStats() map[string]interface{} {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()

	enabled := 0
	blocked := 0
	warned := 0

	for _, p := range ppm.config.Paths {
		if p.Enabled {
			enabled++
			if p.Action == ActionBlock {
				blocked++
			} else if p.Action == ActionWarn {
				warned++
			}
		}
	}

	return map[string]interface{}{
		"total_rules":    len(ppm.config.Paths),
		"enabled_rules":  enabled,
		"blocked_rules":  blocked,
		"warning_rules":  warned,
		"audit_entries":  len(ppm.auditLog),
		"default_action": ppm.config.DefaultAction,
	}
}

// ValidatePath validates that a path is safe to access.
func (ppm *ProtectedPathsManager) ValidatePath(path, operation string) error {
	action, rule := ppm.CheckPath(path, operation)

	switch action {
	case ActionBlock:
		if rule != nil {
			return fmt.Errorf("access to '%s' is blocked: %s", path, rule.Description)
		}
		return fmt.Errorf("access to '%s' is blocked", path)
	case ActionWarn:
		if rule != nil {
			return fmt.Errorf("warning: accessing '%s' (%s)", path, rule.Description)
		}
		return nil
	default:
		return nil
	}
}

// ProtectedPathsBuilder helps create protected path configurations.
type ProtectedPathsBuilder struct {
	config       ProtectedPathConfig
	nextPriority int // Next priority for subsequent Add* calls (0 means use default 50)
}

// NewProtectedPathsBuilder creates a new builder.
func NewProtectedPathsBuilder() *ProtectedPathsBuilder {
	return &ProtectedPathsBuilder{
		config: ProtectedPathConfig{
			Paths:         make([]ProtectedPath, 0),
			DefaultAction: ActionLog,
			EnableAudit:   true,
		},
	}
}

// WithDefaultAction sets the default action.
func (b *ProtectedPathsBuilder) WithDefaultAction(action ProtectedPathAction) *ProtectedPathsBuilder {
	b.config.DefaultAction = action
	return b
}

// WithAuditEnabled enables/disables audit.
func (b *ProtectedPathsBuilder) WithAuditEnabled(enabled bool) *ProtectedPathsBuilder {
	b.config.EnableAudit = enabled
	return b
}

// WithPriority sets the priority for the next Add* call.
// After an Add* call, priority resets to 0 (default 50).
func (b *ProtectedPathsBuilder) WithPriority(priority int) *ProtectedPathsBuilder {
	b.nextPriority = priority
	return b
}

// AddExactPath adds an exact path rule.
func (b *ProtectedPathsBuilder) AddExactPath(path string, action ProtectedPathAction, description string) *ProtectedPathsBuilder {
	priority := b.nextPriority
	if priority == 0 {
		priority = 50
	}
	b.config.Paths = append(b.config.Paths, ProtectedPath{
		Path:        path,
		Type:        TypeExact,
		Action:      action,
		Description: description,
		Enabled:     true,
		Priority:    priority,
	})
	b.nextPriority = 0 // Reset after use
	return b
}

// AddPrefixPath adds a prefix path rule.
func (b *ProtectedPathsBuilder) AddPrefixPath(prefix string, action ProtectedPathAction, description string) *ProtectedPathsBuilder {
	priority := b.nextPriority
	if priority == 0 {
		priority = 50
	}
	b.config.Paths = append(b.config.Paths, ProtectedPath{
		Path:        prefix,
		Type:        TypePrefix,
		Action:      action,
		Description: description,
		Enabled:     true,
		Priority:    priority,
	})
	b.nextPriority = 0 // Reset after use
	return b
}

// AddGlobPath adds a glob pattern rule.
func (b *ProtectedPathsBuilder) AddGlobPath(pattern string, action ProtectedPathAction, description string) *ProtectedPathsBuilder {
	priority := b.nextPriority
	if priority == 0 {
		priority = 50
	}
	b.config.Paths = append(b.config.Paths, ProtectedPath{
		Path:        pattern,
		Type:        TypeGlob,
		Action:      action,
		Description: description,
		Enabled:     true,
		Priority:    priority,
	})
	b.nextPriority = 0 // Reset after use
	return b
}

// AddRegexPath adds a regex pattern rule.
func (b *ProtectedPathsBuilder) AddRegexPath(pattern string, action ProtectedPathAction, description string) *ProtectedPathsBuilder {
	priority := b.nextPriority
	if priority == 0 {
		priority = 50
	}
	b.config.Paths = append(b.config.Paths, ProtectedPath{
		Path:        pattern,
		Type:        TypeRegex,
		Action:      action,
		Description: description,
		Enabled:     true,
		Priority:    priority,
	})
	b.nextPriority = 0 // Reset after use
	return b
}

// Build creates the ProtectedPathsManager.
func (b *ProtectedPathsBuilder) Build() *ProtectedPathsManager {
	return NewProtectedPathsManager(b.config)
}

// BuildConfig returns the configuration.
func (b *ProtectedPathsBuilder) BuildConfig() ProtectedPathConfig {
	return b.config
}
