// Package autonomous - Task 26: Danger Level assessment for commands
package autonomous

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// DangerLevel represents the risk level of a command or operation.
type DangerLevel int

const (
	// DangerLevelSafe - No risk, safe to execute automatically
	DangerLevelSafe DangerLevel = iota

	// DangerLevelLow - Minimal risk, standard operations
	DangerLevelLow

	// DangerLevelMedium - Moderate risk, may need review
	DangerLevelMedium

	// DangerLevelHigh - Significant risk, requires approval
	DangerLevelHigh

	// DangerLevelCritical - Maximum risk, requires explicit approval
	DangerLevelCritical
)

// String returns the string representation of the danger level.
func (d DangerLevel) String() string {
	switch d {
	case DangerLevelSafe:
		return "safe"
	case DangerLevelLow:
		return "low"
	case DangerLevelMedium:
		return "medium"
	case DangerLevelHigh:
		return "high"
	case DangerLevelCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// MarshalText implements encoding.TextMarshaler.
func (d DangerLevel) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *DangerLevel) UnmarshalText(text []byte) error {
	switch string(text) {
	case "safe":
		*d = DangerLevelSafe
	case "low":
		*d = DangerLevelLow
	case "medium":
		*d = DangerLevelMedium
	case "high":
		*d = DangerLevelHigh
	case "critical":
		*d = DangerLevelCritical
	default:
		return fmt.Errorf("unknown danger level: %s", string(text))
	}
	return nil
}

// NeedsApproval returns true if this danger level requires approval.
func (d DangerLevel) NeedsApproval() bool {
	return d >= DangerLevelHigh
}

// RequiresConfirmation returns true if this level needs user confirmation.
func (d DangerLevel) RequiresConfirmation() bool {
	return d >= DangerLevelMedium
}

// DangerCategory represents a category of dangerous operations.
type DangerCategory string

const (
	DangerCategoryFileSystem DangerCategory = "filesystem"
	DangerCategoryNetwork    DangerCategory = "network"
	DangerCategoryProcess    DangerCategory = "process"
	DangerCategorySystem     DangerCategory = "system"
	DangerCategorySecurity   DangerCategory = "security"
	DangerCategoryData       DangerCategory = "data"
	DangerCategoryGit        DangerCategory = "git"
	DangerCategoryPackage    DangerCategory = "package"
	DangerCategoryConfig     DangerCategory = "config"
	DangerCategoryUnknown    DangerCategory = "unknown"
)

// DangerAssessment contains the result of a danger assessment.
type DangerAssessment struct {
	// Level is the overall danger level
	Level DangerLevel `json:"level"`

	// Category is the primary danger category
	Category DangerCategory `json:"category"`

	// Score is a numeric risk score (0-100)
	Score int `json:"score"`

	// Confidence is the confidence in the assessment (0-1)
	Confidence float64 `json:"confidence"`

	// Reasons are the factors that contributed to the assessment
	Reasons []string `json:"reasons"`

	// Mitigations are suggested mitigations for the risks
	Mitigations []string `json:"mitigations"`

	// AffectedPaths are file paths that would be affected
	AffectedPaths []string `json:"affected_paths"`

	// IsDestructive indicates if the operation is destructive
	IsDestructive bool `json:"is_destructive"`

	// IsReversible indicates if the operation can be undone
	IsReversible bool `json:"is_reversible"`

	// RequiresSandbox indicates if sandboxing is recommended
	RequiresSandbox bool `json:"requires_sandbox"`

	// ApprovalMessage is a message to show when requesting approval
	ApprovalMessage string `json:"approval_message"`
}

// DangerAssessor assesses the danger level of commands and operations.
type DangerAssessor struct {
	mu sync.RWMutex

	// patterns maps danger levels to command patterns
	patterns map[DangerLevel][]DangerPattern

	// destructiveCommands are commands that are always destructive
	destructiveCommands map[string]bool

	// safeCommands are commands that are always safe
	safeCommands map[string]bool

	// protectedPaths are paths that require extra caution
	protectedPaths []string

	// customRules are user-defined rules
	customRules []DangerRule
}

// DangerPattern matches commands and assigns danger levels.
type DangerPattern struct {
	// Pattern is the regex pattern to match
	Pattern *regexp.Regexp `json:"-"`

	// PatternStr is the string representation of the pattern
	PatternStr string `json:"pattern"`

	// Category is the danger category
	Category DangerCategory `json:"category"`

	// Level is the base danger level
	Level DangerLevel `json:"level"`

	// IsDestructive indicates if the matched command is destructive
	IsDestructive bool `json:"is_destructive"`

	// Reason describes why this pattern is dangerous
	Reason string `json:"reason"`

	// Mitigation suggests how to reduce risk
	Mitigation string `json:"mitigation"`
}

// DangerRule is a custom rule for danger assessment.
type DangerRule struct {
	Name     string         `json:"name"`
	Pattern  string         `json:"pattern"`
	Level    DangerLevel    `json:"level"`
	Category DangerCategory `json:"category"`
	Enabled  bool           `json:"enabled"`
	Priority int            `json:"priority"`
}

// NewDangerAssessor creates a new danger assessor.
func NewDangerAssessor() *DangerAssessor {
	da := &DangerAssessor{
		patterns:            make(map[DangerLevel][]DangerPattern),
		destructiveCommands: make(map[string]bool),
		safeCommands:        make(map[string]bool),
		protectedPaths:      make([]string, 0),
		customRules:         make([]DangerRule, 0),
	}

	da.initializePatterns()
	da.initializeCommands()

	return da
}

// initializePatterns sets up default danger patterns.
func (da *DangerAssessor) initializePatterns() {
	// Critical patterns - maximum danger
	da.patterns[DangerLevelCritical] = []DangerPattern{
		{Pattern: regexp.MustCompile(`(?i)rm\s+-rf\s+/`), Category: DangerCategoryFileSystem, Level: DangerLevelCritical, IsDestructive: true, Reason: "Recursive force delete from root", Mitigation: "Specify exact paths, avoid -rf on root"},
		{Pattern: regexp.MustCompile(`(?i)mkfs\.`), Category: DangerCategorySystem, Level: DangerLevelCritical, IsDestructive: true, Reason: "Format filesystem", Mitigation: "Verify target device carefully"},
		{Pattern: regexp.MustCompile(`(?i)dd\s+.*of=/dev/`), Category: DangerCategorySystem, Level: DangerLevelCritical, IsDestructive: true, Reason: "Direct disk write", Mitigation: "Double-check target device"},
		{Pattern: regexp.MustCompile(`(?i):(){ :|:& };:`), Category: DangerCategoryProcess, Level: DangerLevelCritical, IsDestructive: true, Reason: "Fork bomb", Mitigation: "This is malicious, do not run"},
		{Pattern: regexp.MustCompile(`(?i)chmod\s+(-R\s+)?777\s+/`), Category: DangerCategorySecurity, Level: DangerLevelCritical, IsDestructive: true, Reason: "World-writable root", Mitigation: "Use restrictive permissions"},
		{Pattern: regexp.MustCompile(`(?i)chown\s+(-R\s+)?[^\s]+\s+/`), Category: DangerCategorySecurity, Level: DangerLevelCritical, Reason: "Change root ownership", Mitigation: "Specify exact paths"},
		{Pattern: regexp.MustCompile(`(?i)>\s*/dev/sd[a-z]`), Category: DangerCategorySystem, Level: DangerLevelCritical, IsDestructive: true, Reason: "Direct disk overwrite", Mitigation: "Never redirect to disk devices"},
	}

	// High patterns - significant danger
	da.patterns[DangerLevelHigh] = []DangerPattern{
		{Pattern: regexp.MustCompile(`(?i)rm\s+(-rf|--recursive)`), Category: DangerCategoryFileSystem, Level: DangerLevelHigh, IsDestructive: true, Reason: "Recursive force delete", Mitigation: "Verify paths before deletion"},
		{Pattern: regexp.MustCompile(`(?i)git\s+push\s+.*--force`), Category: DangerCategoryGit, Level: DangerLevelHigh, IsDestructive: true, Reason: "Force push can overwrite history", Mitigation: "Use --force-with-lease instead"},
		{Pattern: regexp.MustCompile(`(?i)git\s+reset\s+--hard`), Category: DangerCategoryGit, Level: DangerLevelHigh, IsDestructive: true, Reason: "Hard reset loses uncommitted changes", Mitigation: "Stash changes first"},
		{Pattern: regexp.MustCompile(`(?i)DROP\s+(TABLE|DATABASE|SCHEMA)`), Category: DangerCategoryData, Level: DangerLevelHigh, IsDestructive: true, Reason: "Database drop operation", Mitigation: "Backup before dropping"},
		{Pattern: regexp.MustCompile(`(?i)TRUNCATE\s+TABLE?`), Category: DangerCategoryData, Level: DangerLevelHigh, IsDestructive: true, Reason: "Truncate removes all data", Mitigation: "Backup before truncating"},
		{Pattern: regexp.MustCompile(`(?i)kubectl\s+delete\s+`), Category: DangerCategorySystem, Level: DangerLevelHigh, IsDestructive: true, Reason: "Kubernetes resource deletion", Mitigation: "Verify resource name and namespace"},
		{Pattern: regexp.MustCompile(`(?i)docker\s+(rmi|image\s+rm)\s+`), Category: DangerCategorySystem, Level: DangerLevelHigh, Reason: "Docker image removal", Mitigation: "Check image dependencies"},
		{Pattern: regexp.MustCompile(`(?i)sudo\s+`), Category: DangerCategorySecurity, Level: DangerLevelHigh, Reason: "Elevated privileges", Mitigation: "Verify command before running"},
		{Pattern: regexp.MustCompile(`(?i)curl\s+.*\|\s*(sudo\s+)?bash`), Category: DangerCategorySecurity, Level: DangerLevelHigh, Reason: "Remote script execution", Mitigation: "Download and review script first"},
		{Pattern: regexp.MustCompile(`(?i)wget\s+.*\|\s*(sudo\s+)?bash`), Category: DangerCategorySecurity, Level: DangerLevelHigh, Reason: "Remote script execution", Mitigation: "Download and review script first"},
	}

	// Medium patterns - moderate danger
	da.patterns[DangerLevelMedium] = []DangerPattern{
		{Pattern: regexp.MustCompile(`(?i)rm\s+`), Category: DangerCategoryFileSystem, Level: DangerLevelMedium, IsDestructive: true, Reason: "File deletion", Mitigation: "Verify file path"},
		{Pattern: regexp.MustCompile(`(?i)git\s+push\s+`), Category: DangerCategoryGit, Level: DangerLevelMedium, Reason: "Git push to remote", Mitigation: "Review changes before pushing"},
		{Pattern: regexp.MustCompile(`(?i)npm\s+(publish|unpublish)`), Category: DangerCategoryPackage, Level: DangerLevelMedium, Reason: "NPM registry operation", Mitigation: "Verify package.json and version"},
		{Pattern: regexp.MustCompile(`(?i)pip\s+uninstall`), Category: DangerCategoryPackage, Level: DangerLevelMedium, Reason: "Package uninstallation", Mitigation: "Check dependencies"},
		{Pattern: regexp.MustCompile(`(?i)mv\s+.*\s+/dev/null`), Category: DangerCategoryFileSystem, Level: DangerLevelMedium, IsDestructive: true, Reason: "Moving to null device", Mitigation: "Use rm explicitly"},
		{Pattern: regexp.MustCompile(`(?i)DELETE\s+FROM`), Category: DangerCategoryData, Level: DangerLevelMedium, IsDestructive: true, Reason: "SQL DELETE operation", Mitigation: "Use WHERE clause, backup first"},
		{Pattern: regexp.MustCompile(`(?i)UPDATE\s+.*\s+SET`), Category: DangerCategoryData, Level: DangerLevelMedium, Reason: "SQL UPDATE operation", Mitigation: "Use WHERE clause, verify changes"},
		{Pattern: regexp.MustCompile(`(?i)ALTER\s+TABLE`), Category: DangerCategoryData, Level: DangerLevelMedium, Reason: "Schema modification", Mitigation: "Backup database first"},
		{Pattern: regexp.MustCompile(`(?i)docker\s+system\s+prune`), Category: DangerCategorySystem, Level: DangerLevelMedium, IsDestructive: true, Reason: "Docker cleanup", Mitigation: "Review what will be removed"},
		{Pattern: regexp.MustCompile(`(?i)kill\s+(-9|-KILL)`), Category: DangerCategoryProcess, Level: DangerLevelMedium, Reason: "Force kill process", Mitigation: "Try graceful termination first"},
		{Pattern: regexp.MustCompile(`(?i)chmod\s+-R`), Category: DangerCategorySecurity, Level: DangerLevelMedium, Reason: "Recursive permission change", Mitigation: "Specify exact permissions"},
	}

	// Low patterns - minimal danger
	da.patterns[DangerLevelLow] = []DangerPattern{
		{Pattern: regexp.MustCompile(`(?i)git\s+(status|log|diff|branch)`), Category: DangerCategoryGit, Level: DangerLevelLow, Reason: "Read-only git operation", Mitigation: "None needed"},
		{Pattern: regexp.MustCompile(`(?i)docker\s+(ps|images|logs)`), Category: DangerCategorySystem, Level: DangerLevelLow, Reason: "Read-only docker operation", Mitigation: "None needed"},
		{Pattern: regexp.MustCompile(`(?i)ls\s+`), Category: DangerCategoryFileSystem, Level: DangerLevelLow, Reason: "Directory listing", Mitigation: "None needed"},
		{Pattern: regexp.MustCompile(`(?i)cat\s+`), Category: DangerCategoryFileSystem, Level: DangerLevelLow, Reason: "File reading", Mitigation: "None needed"},
		{Pattern: regexp.MustCompile(`(?i)grep\s+`), Category: DangerCategoryFileSystem, Level: DangerLevelLow, Reason: "Text search", Mitigation: "None needed"},
		{Pattern: regexp.MustCompile(`(?i)npm\s+(install|i|ci)`), Category: DangerCategoryPackage, Level: DangerLevelLow, Reason: "Package installation", Mitigation: "Review package.json"},
		{Pattern: regexp.MustCompile(`(?i)pip\s+install`), Category: DangerCategoryPackage, Level: DangerLevelLow, Reason: "Package installation", Mitigation: "Check requirements.txt"},
		{Pattern: regexp.MustCompile(`(?i)go\s+(build|test|mod)`), Category: DangerCategoryPackage, Level: DangerLevelLow, Reason: "Go build/test operation", Mitigation: "None needed"},
		{Pattern: regexp.MustCompile(`(?i)make\s+`), Category: DangerCategoryProcess, Level: DangerLevelLow, Reason: "Build operation", Mitigation: "Review Makefile"},
		{Pattern: regexp.MustCompile(`(?i)ps\s+`), Category: DangerCategoryProcess, Level: DangerLevelLow, Reason: "Process listing", Mitigation: "None needed"},
	}
}

// initializeCommands sets up known safe and destructive commands.
func (da *DangerAssessor) initializeCommands() {
	// Safe commands
	safeCmds := []string{
		"ls", "dir", "pwd", "whoami", "echo", "cat", "head", "tail",
		"grep", "find", "which", "type", "whereis", "stat",
		"git status", "git log", "git diff", "git branch", "git remote",
		"docker ps", "docker images", "docker logs",
		"npm list", "npm outdated",
		"go version", "go env", "go list",
		"python --version", "python3 --version", "node --version",
	}
	for _, cmd := range safeCmds {
		da.safeCommands[cmd] = true
	}

	// Destructive commands
	destructiveCmds := []string{
		"rm", "rmdir", "del", "format", "erase",
		"shred", "wipe", "unlink",
	}
	for _, cmd := range destructiveCmds {
		da.destructiveCommands[cmd] = true
	}

	// Default protected paths
	da.protectedPaths = []string{
		"/etc/passwd",
		"/etc/shadow",
		"/etc/ssh/",
		"~/.ssh/",
		".env",
		"credentials.json",
		"secrets.json",
		"*.pem",
		"*.key",
		"id_rsa*",
	}
}

// AssessCommand evaluates a command and returns a danger assessment.
func (da *DangerAssessor) AssessCommand(command string) *DangerAssessment {
	da.mu.RLock()
	defer da.mu.RUnlock()

	assessment := &DangerAssessment{
		Level:           DangerLevelSafe,
		Category:        DangerCategoryUnknown,
		Score:           0,
		Confidence:      0.5,
		Reasons:         make([]string, 0),
		Mitigations:     make([]string, 0),
		AffectedPaths:   make([]string, 0),
		IsDestructive:   false,
		IsReversible:    true,
		RequiresSandbox: false,
	}

	// Check custom rules first (highest priority)
	da.applyCustomRules(command, assessment)

	// Check against patterns from highest to lowest danger
	for level := DangerLevelCritical; level >= DangerLevelSafe; level-- {
		for _, pattern := range da.patterns[level] {
			if pattern.Pattern.MatchString(command) {
				da.applyPattern(pattern, assessment)
			}
		}
	}

	// Check for destructive commands
	da.checkDestructiveCommand(command, assessment)

	// Check for protected paths
	da.checkProtectedPaths(command, assessment)

	// Check for network operations
	da.checkNetworkOperations(command, assessment)

	// Check for elevated privileges
	da.checkElevatedPrivileges(command, assessment)

	// Calculate final score
	da.calculateScore(assessment)

	// Generate approval message
	da.generateApprovalMessage(assessment)

	return assessment
}

// applyPattern applies a matched pattern to the assessment.
func (da *DangerAssessor) applyPattern(pattern DangerPattern, assessment *DangerAssessment) {
	if pattern.Level > assessment.Level {
		assessment.Level = pattern.Level
	}

	assessment.Category = pattern.Category
	assessment.IsDestructive = assessment.IsDestructive || pattern.IsDestructive

	if pattern.Reason != "" {
		assessment.Reasons = append(assessment.Reasons, pattern.Reason)
	}
	if pattern.Mitigation != "" {
		assessment.Mitigations = append(assessment.Mitigations, pattern.Mitigation)
	}

	assessment.Confidence = 0.9
}

// checkDestructiveCommand checks if the command is inherently destructive.
func (da *DangerAssessor) checkDestructiveCommand(command string, assessment *DangerAssessment) {
	cmdParts := strings.Fields(command)
	if len(cmdParts) == 0 {
		return
	}

	baseCmd := cmdParts[0]

	// Check if it's a known destructive command
	if da.destructiveCommands[baseCmd] {
		assessment.IsDestructive = true
		assessment.IsReversible = false

		if assessment.Level < DangerLevelMedium {
			assessment.Level = DangerLevelMedium
		}

		assessment.Reasons = append(assessment.Reasons, fmt.Sprintf("Command '%s' is inherently destructive", baseCmd))
	}
}

// checkProtectedPaths checks if the command affects protected paths.
func (da *DangerAssessor) checkProtectedPaths(command string, assessment *DangerAssessment) {
	for _, path := range da.protectedPaths {
		if strings.Contains(command, path) {
			if assessment.Level < DangerLevelHigh {
				assessment.Level = DangerLevelHigh
			}

			assessment.Reasons = append(assessment.Reasons, fmt.Sprintf("Affects protected path: %s", path))
			assessment.AffectedPaths = append(assessment.AffectedPaths, path)
		}
	}
}

// checkNetworkOperations checks for network-related operations.
func (da *DangerAssessor) checkNetworkOperations(command string, assessment *DangerAssessment) {
	networkPatterns := []struct {
		pattern string
		level   DangerLevel
		reason  string
	}{
		{`curl\s+`, DangerLevelLow, "Network request"},
		{`wget\s+`, DangerLevelLow, "Network download"},
		{`ssh\s+`, DangerLevelMedium, "SSH connection"},
		{`scp\s+`, DangerLevelMedium, "Secure copy over network"},
		{`rsync\s+.*-e\s+ssh`, DangerLevelMedium, "Remote sync"},
		{`nc\s+`, DangerLevelHigh, "Netcat - potential security risk"},
	}

	for _, np := range networkPatterns {
		matched, _ := regexp.MatchString(np.pattern, command)
		if matched {
			if assessment.Level < np.level {
				assessment.Level = np.level
			}
			assessment.Category = DangerCategoryNetwork
			assessment.Reasons = append(assessment.Reasons, np.reason)
			assessment.RequiresSandbox = true
		}
	}
}

// checkElevatedPrivileges checks for sudo or elevated permissions.
func (da *DangerAssessor) checkElevatedPrivileges(command string, assessment *DangerAssessment) {
	if strings.Contains(command, "sudo") || strings.Contains(command, "su ") {
		if assessment.Level < DangerLevelHigh {
			assessment.Level = DangerLevelHigh
		}
		assessment.Reasons = append(assessment.Reasons, "Command requires elevated privileges")
		assessment.RequiresSandbox = true
	}
}

// applyCustomRules applies user-defined rules.
func (da *DangerAssessor) applyCustomRules(command string, assessment *DangerAssessment) {
	for _, rule := range da.customRules {
		if !rule.Enabled {
			continue
		}

		matched, _ := regexp.MatchString(rule.Pattern, command)
		if matched {
			if rule.Level > assessment.Level {
				assessment.Level = rule.Level
			}
			assessment.Category = rule.Category
			assessment.Reasons = append(assessment.Reasons, fmt.Sprintf("Custom rule: %s", rule.Name))
		}
	}
}

// calculateScore calculates a numeric risk score.
func (da *DangerAssessor) calculateScore(assessment *DangerAssessment) {
	baseScore := int(assessment.Level) * 20

	// Add points for additional risk factors
	if assessment.IsDestructive {
		baseScore += 15
	}
	if !assessment.IsReversible {
		baseScore += 10
	}
	if len(assessment.AffectedPaths) > 0 {
		baseScore += len(assessment.AffectedPaths) * 5
	}
	if assessment.RequiresSandbox {
		baseScore += 5
	}

	// Cap at 100
	if baseScore > 100 {
		baseScore = 100
	}

	assessment.Score = baseScore
}

// generateApprovalMessage creates a message for approval requests.
func (da *DangerAssessor) generateApprovalMessage(assessment *DangerAssessment) {
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("⚠️ **Danger Level: %s** (Score: %d/100)\n\n",
		strings.ToUpper(assessment.Level.String()), assessment.Score))

	if len(assessment.Reasons) > 0 {
		msg.WriteString("**Risk Factors:**\n")
		for _, reason := range assessment.Reasons {
			msg.WriteString(fmt.Sprintf("- %s\n", reason))
		}
		msg.WriteString("\n")
	}

	if assessment.IsDestructive {
		msg.WriteString("⛔ **This operation is DESTRUCTIVE and cannot be easily undone.**\n\n")
	}

	if len(assessment.Mitigations) > 0 {
		msg.WriteString("**Suggested Mitigations:**\n")
		for _, mitigation := range assessment.Mitigations {
			msg.WriteString(fmt.Sprintf("- %s\n", mitigation))
		}
	}

	assessment.ApprovalMessage = msg.String()
}

// AddCustomRule adds a custom danger rule.
func (da *DangerAssessor) AddCustomRule(rule DangerRule) error {
	da.mu.Lock()
	defer da.mu.Unlock()

	// Validate pattern
	_, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	da.customRules = append(da.customRules, rule)
	return nil
}

// RemoveCustomRule removes a custom rule by name.
func (da *DangerAssessor) RemoveCustomRule(name string) bool {
	da.mu.Lock()
	defer da.mu.Unlock()

	for i, rule := range da.customRules {
		if rule.Name == name {
			da.customRules = append(da.customRules[:i], da.customRules[i+1:]...)
			return true
		}
	}
	return false
}

// AddProtectedPath adds a path to the protected list.
func (da *DangerAssessor) AddProtectedPath(path string) {
	da.mu.Lock()
	defer da.mu.Unlock()
	da.protectedPaths = append(da.protectedPaths, path)
}

// RemoveProtectedPath removes a path from the protected list.
func (da *DangerAssessor) RemoveProtectedPath(path string) bool {
	da.mu.Lock()
	defer da.mu.Unlock()

	for i, p := range da.protectedPaths {
		if p == path {
			da.protectedPaths = append(da.protectedPaths[:i], da.protectedPaths[i+1:]...)
			return true
		}
	}
	return false
}

// GetProtectedPaths returns the list of protected paths.
func (da *DangerAssessor) GetProtectedPaths() []string {
	da.mu.RLock()
	defer da.mu.RUnlock()
	return append([]string{}, da.protectedPaths...)
}

// QuickAssess provides a quick danger level for a command.
func QuickAssess(command string) DangerLevel {
	da := NewDangerAssessor()
	assessment := da.AssessCommand(command)
	return assessment.Level
}

// IsCommandSafe returns true if a command is safe to execute automatically.
func IsCommandSafe(command string) bool {
	return QuickAssess(command) <= DangerLevelLow
}

// NeedsApproval returns true if a command needs approval before execution.
func NeedsApproval(command string) bool {
	return QuickAssess(command) >= DangerLevelHigh
}

// DangerAssessmentBuilder helps create danger assessments for custom scenarios.
type DangerAssessmentBuilder struct {
	assessment *DangerAssessment
}

// NewDangerAssessmentBuilder creates a new builder.
func NewDangerAssessmentBuilder() *DangerAssessmentBuilder {
	return &DangerAssessmentBuilder{
		assessment: &DangerAssessment{
			Level:         DangerLevelSafe,
			Reasons:       make([]string, 0),
			Mitigations:   make([]string, 0),
			AffectedPaths: make([]string, 0),
			Confidence:    1.0,
		},
	}
}

// WithLevel sets the danger level.
func (b *DangerAssessmentBuilder) WithLevel(level DangerLevel) *DangerAssessmentBuilder {
	b.assessment.Level = level
	return b
}

// WithCategory sets the category.
func (b *DangerAssessmentBuilder) WithCategory(category DangerCategory) *DangerAssessmentBuilder {
	b.assessment.Category = category
	return b
}

// WithReason adds a reason.
func (b *DangerAssessmentBuilder) WithReason(reason string) *DangerAssessmentBuilder {
	b.assessment.Reasons = append(b.assessment.Reasons, reason)
	return b
}

// WithMitigation adds a mitigation.
func (b *DangerAssessmentBuilder) WithMitigation(mitigation string) *DangerAssessmentBuilder {
	b.assessment.Mitigations = append(b.assessment.Mitigations, mitigation)
	return b
}

// WithAffectedPath adds an affected path.
func (b *DangerAssessmentBuilder) WithAffectedPath(path string) *DangerAssessmentBuilder {
	b.assessment.AffectedPaths = append(b.assessment.AffectedPaths, path)
	return b
}

// Destructive marks the assessment as destructive.
func (b *DangerAssessmentBuilder) Destructive() *DangerAssessmentBuilder {
	b.assessment.IsDestructive = true
	b.assessment.IsReversible = false
	return b
}

// Build returns the assessment.
func (b *DangerAssessmentBuilder) Build() *DangerAssessment {
	return b.assessment
}
