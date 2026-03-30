// Package autonomous - Task 33: Restricted Commands list configuration
package autonomous

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// RestrictionLevel represents the level of restriction for a command.
type RestrictionLevel string

const (
	// RestrictionLevelNone - No restriction
	RestrictionLevelNone RestrictionLevel = "none"

	// RestrictionLevelWarn - Warning only, command can proceed
	RestrictionLevelWarn RestrictionLevel = "warn"

	// RestrictionLevelConfirm - Requires user confirmation
	RestrictionLevelConfirm RestrictionLevel = "confirm"

	// RestrictionLevelApproval - Requires explicit approval
	RestrictionLevelApproval RestrictionLevel = "approval"

	// RestrictionLevelBlock - Command is blocked completely
	RestrictionLevelBlock RestrictionLevel = "block"
)

// CommandCategory represents a category of commands.
type CommandCategory string

const (
	CommandCategoryFileSystem CommandCategory = "filesystem"
	CommandCategoryNetwork    CommandCategory = "network"
	CommandCategoryProcess    CommandCategory = "process"
	CommandCategorySystem     CommandCategory = "system"
	CommandCategoryPackage    CommandCategory = "package"
	CommandCategoryGit        CommandCategory = "git"
	CommandCategoryDatabase   CommandCategory = "database"
	CommandCategoryCloud      CommandCategory = "cloud"
	CommandCategoryContainer  CommandCategory = "container"
	CommandCategoryCustom     CommandCategory = "custom"
)

// RestrictedCommand represents a restricted command configuration.
type RestrictedCommand struct {
	// ID is the unique identifier
	ID string `json:"id"`

	// Name is the command name
	Name string `json:"name"`

	// Pattern is the regex or glob pattern to match
	Pattern string `json:"pattern"`

	// Description explains why it's restricted
	Description string `json:"description"`

	// Category is the command category
	Category CommandCategory `json:"category"`

	// Restriction is the restriction level
	Restriction RestrictionLevel `json:"restriction"`

	// Enabled turns the restriction on/off
	Enabled bool `json:"enabled"`

	// Message shown when the command is restricted
	Message string `json:"message,omitempty"`

	// AllowWithFlag allows the command with specific flags
	AllowWithFlag []string `json:"allow_with_flag,omitempty"`

	// BlockWithFlag blocks the command with specific flags
	BlockWithFlag []string `json:"block_with_flag,omitempty"`

	// RequireFlag requires these flags to be present
	RequireFlag []string `json:"require_flag,omitempty"`

	// DangerLevelOverride overrides the danger level
	DangerLevelOverride *DangerLevel `json:"danger_level_override,omitempty"`

	// TimeRestriction restricts when the command can run
	TimeRestriction *TimeRestriction `json:"time_restriction,omitempty"`

	// CreatedAt is when this was created
	CreatedAt time.Time `json:"created_at"`

	// ModifiedAt is when this was last modified
	ModifiedAt time.Time `json:"modified_at"`
}

// TimeRestriction restricts when commands can run.
type TimeRestriction struct {
	// AllowedHours are the hours (0-23) when the command is allowed
	AllowedHours []int `json:"allowed_hours,omitempty"`

	// BlockedHours are the hours when the command is blocked
	BlockedHours []int `json:"blocked_hours,omitempty"`

	// AllowedDays are the days (0=Sunday, 6=Saturday) when allowed
	AllowedDays []int `json:"allowed_days,omitempty"`

	// BlockedDays are the days when blocked
	BlockedDays []int `json:"blocked_days,omitempty"`

	// StartTime is the start time (HH:MM format)
	StartTime string `json:"start_time,omitempty"`

	// EndTime is the end time (HH:MM format)
	EndTime string `json:"end_time,omitempty"`
}

// RestrictionResult contains the result of a restriction check.
type RestrictionResult struct {
	// Restricted indicates if the command is restricted
	Restricted bool `json:"restricted"`

	// Restriction is the restriction level
	Restriction RestrictionLevel `json:"restriction"`

	// Command is the matched command config
	Command *RestrictedCommand `json:"command,omitempty"`

	// Message explains why it's restricted
	Message string `json:"message"`

	// MatchedPattern is the pattern that matched
	MatchedPattern string `json:"matched_pattern,omitempty"`

	// Suggestion for alternative commands
	Suggestion string `json:"suggestion,omitempty"`
}

// RestrictedCommandsConfig configures the restricted commands manager.
type RestrictedCommandsConfig struct {
	// Enabled turns on restriction checking
	Enabled bool `json:"enabled"`

	// ConfigPath is where to load/save restrictions
	ConfigPath string `json:"config_path"`

	// DefaultRestriction is the default level for unknown commands
	DefaultRestriction RestrictionLevel `json:"default_restriction"`

	// EnforceStrict enforces strict mode for all restrictions
	EnforceStrict bool `json:"enforce_strict"`

	// LogRestrictions logs all restriction checks
	LogRestrictions bool `json:"log_restrictions"`
}

// DefaultRestrictedCommandsConfig returns the default configuration.
func DefaultRestrictedCommandsConfig() RestrictedCommandsConfig {
	return RestrictedCommandsConfig{
		Enabled:            true,
		DefaultRestriction: RestrictionLevelNone,
		EnforceStrict:      false,
		LogRestrictions:    true,
	}
}

// RestrictedCommandsManager manages the restricted commands list.
type RestrictedCommandsManager struct {
	mu sync.RWMutex

	// config is the configuration
	config RestrictedCommandsConfig

	// commands is the map of restricted commands
	commands map[string]*RestrictedCommand

	// patterns is the list of compiled regex patterns
	patterns map[string]*regexp.Regexp

	// categories groups commands by category
	categories map[CommandCategory][]string

	// stats tracks restriction statistics
	stats RestrictionStats

	// auditLogger is the optional audit logger
	auditLogger *AuditLogger

	// timeNow is a function to get current time (for testing)
	timeNow func() time.Time
}

// RestrictionStats tracks restriction statistics.
type RestrictionStats struct {
	TotalCommands    int            `json:"total_commands"`
	TotalChecks      int            `json:"total_checks"`
	TotalBlocked     int            `json:"total_blocked"`
	TotalWarned      int            `json:"total_warned"`
	TotalApproved    int            `json:"total_approved"`
	ByCategory       map[string]int `json:"by_category"`
	ByRestriction    map[string]int `json:"by_restriction"`
	MostBlockedCount int            `json:"most_blocked_count"`
	MostBlockedCmd   string         `json:"most_blocked_cmd"`
}

// NewRestrictedCommandsManager creates a new restricted commands manager.
func NewRestrictedCommandsManager(config RestrictedCommandsConfig) *RestrictedCommandsManager {
	mgr := &RestrictedCommandsManager{
		config:     config,
		commands:   make(map[string]*RestrictedCommand),
		patterns:   make(map[string]*regexp.Regexp),
		categories: make(map[CommandCategory][]string),
		stats: RestrictionStats{
			ByCategory:    make(map[string]int),
			ByRestriction: make(map[string]int),
		},
		timeNow: time.Now,
	}

	// Initialize with default restricted commands
	mgr.initDefaultCommands()

	// Load from config if exists
	mgr.loadFromConfig()

	return mgr
}

// initDefaultCommands initializes the default restricted commands.
func (mgr *RestrictedCommandsManager) initDefaultCommands() {
	now := mgr.timeNow()

	// Define default restricted commands
	defaults := []RestrictedCommand{
		// File system - destructive
		{
			ID:          "rm-rf",
			Name:        "rm -rf",
			Pattern:     `^rm\s+(-[rf]+\s+|--recursive\s+--force\s+).+`,
			Description: "Recursive forced deletion",
			Category:    CommandCategoryFileSystem,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "Recursive forced deletion requires approval due to risk of data loss",
		},
		{
			ID:          "format",
			Name:        "Disk formatting",
			Pattern:     `^(mkfs|format)\s+.*`,
			Description: "Disk formatting commands",
			Category:    CommandCategoryFileSystem,
			Restriction: RestrictionLevelBlock,
			Enabled:     true,
			Message:     "Disk formatting is blocked for safety",
		},
		{
			ID:          "dd",
			Name:        "dd command",
			Pattern:     `^dd\s+.*`,
			Description: "Low-level disk operations",
			Category:    CommandCategoryFileSystem,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "dd command requires approval due to potential data loss risk",
		},
		{
			ID:          "chmod-777",
			Name:        "chmod 777",
			Pattern:     `^chmod\s+(-R\s+)?777\s+.*`,
			Description: "Insecure permissions",
			Category:    CommandCategoryFileSystem,
			Restriction: RestrictionLevelWarn,
			Enabled:     true,
			Message:     "chmod 777 creates security risk by allowing world write access",
		},
		{
			ID:          "chown-root",
			Name:        "chown to root",
			Pattern:     `^chown\s+.*root.*`,
			Description: "Changing ownership to root",
			Category:    CommandCategoryFileSystem,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "Changing file ownership to root requires approval",
		},

		// System commands
		{
			ID:          "shutdown",
			Name:        "System shutdown",
			Pattern:     `^(shutdown|poweroff|reboot|halt)\s*.*`,
			Description: "System power control",
			Category:    CommandCategorySystem,
			Restriction: RestrictionLevelBlock,
			Enabled:     true,
			Message:     "System power commands are blocked",
		},
		{
			ID:          "init",
			Name:        "Init system",
			Pattern:     `^init\s+[0-6]$`,
			Description: "Runlevel changes",
			Category:    CommandCategorySystem,
			Restriction: RestrictionLevelBlock,
			Enabled:     true,
			Message:     "Changing system runlevel is blocked",
		},
		{
			ID:          "sysctl",
			Name:        "Kernel parameters",
			Pattern:     `^sysctl\s+-w\s+.*`,
			Description: "Modifying kernel parameters",
			Category:    CommandCategorySystem,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "Modifying kernel parameters requires approval",
		},
		{
			ID:          "iptables",
			Name:        "Firewall rules",
			Pattern:     `^iptables\s+.*`,
			Description: "Modifying firewall rules",
			Category:    CommandCategorySystem,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "Modifying firewall rules requires approval",
		},

		// Network commands
		{
			ID:          "wget-exec",
			Name:        "wget with exec",
			Pattern:     `^wget\s+.*\|.*`,
			Description: "Downloading and executing",
			Category:    CommandCategoryNetwork,
			Restriction: RestrictionLevelBlock,
			Enabled:     true,
			Message:     "Downloading and piping to shell is blocked for security",
		},
		{
			ID:          "curl-exec",
			Name:        "curl with exec",
			Pattern:     `^curl\s+.*\|.*`,
			Description: "Downloading and executing",
			Category:    CommandCategoryNetwork,
			Restriction: RestrictionLevelBlock,
			Enabled:     true,
			Message:     "Downloading and piping to shell is blocked for security",
		},
		{
			ID:          "nc-connect",
			Name:        "Netcat connections",
			Pattern:     `^(nc|netcat)\s+.*`,
			Description: "Netcat connections",
			Category:    CommandCategoryNetwork,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "Netcat connections require approval",
		},

		// Process commands
		{
			ID:          "killall",
			Name:        "killall",
			Pattern:     `^killall\s+.*`,
			Description: "Kill all processes by name",
			Category:    CommandCategoryProcess,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "killall requires approval due to potential system impact",
		},
		{
			ID:          "pkill",
			Name:        "pkill",
			Pattern:     `^pkill\s+.*`,
			Description: "Kill processes by pattern",
			Category:    CommandCategoryProcess,
			Restriction: RestrictionLevelConfirm,
			Enabled:     true,
			Message:     "pkill may affect multiple processes",
		},
		{
			ID:          "kill-9",
			Name:        "Force kill",
			Pattern:     `^kill\s+(-9\s+|-SIGKILL\s+).*`,
			Description: "Force kill signal",
			Category:    CommandCategoryProcess,
			Restriction: RestrictionLevelConfirm,
			Enabled:     true,
			Message:     "SIGKILL cannot be caught by processes, use with caution",
		},

		// Package management
		{
			ID:          "apt-remove",
			Name:        "apt remove",
			Pattern:     `^(apt|apt-get)\s+(remove|purge)\s+.*`,
			Description: "Removing packages",
			Category:    CommandCategoryPackage,
			Restriction: RestrictionLevelConfirm,
			Enabled:     true,
			Message:     "Removing packages requires confirmation",
		},
		{
			ID:          "npm-global",
			Name:        "npm global",
			Pattern:     `^npm\s+(install|i)\s+-g\s+.*`,
			Description: "Global npm installation",
			Category:    CommandCategoryPackage,
			Restriction: RestrictionLevelWarn,
			Enabled:     true,
			Message:     "Global npm installs can cause conflicts",
		},
		{
			ID:          "pip-force",
			Name:        "pip force install",
			Pattern:     `^pip\s+install\s+--force.*`,
			Description: "Force pip installation",
			Category:    CommandCategoryPackage,
			Restriction: RestrictionLevelWarn,
			Enabled:     true,
			Message:     "Force installing packages may break dependencies",
		},

		// Git commands
		{
			ID:          "git-force-push",
			Name:        "git force push",
			Pattern:     `^git\s+push\s+.*--force.*`,
			Description: "Force pushing to remote",
			Category:    CommandCategoryGit,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "Force pushing can overwrite remote history",
		},
		{
			ID:          "git-reset-hard",
			Name:        "git reset --hard",
			Pattern:     `^git\s+reset\s+--hard.*`,
			Description: "Hard reset losing changes",
			Category:    CommandCategoryGit,
			Restriction: RestrictionLevelConfirm,
			Enabled:     true,
			Message:     "Hard reset will lose uncommitted changes",
		},
		{
			ID:          "git-clean-fd",
			Name:        "git clean -fd",
			Pattern:     `^git\s+clean\s+-[fd]+.*`,
			Description: "Removing untracked files",
			Category:    CommandCategoryGit,
			Restriction: RestrictionLevelConfirm,
			Enabled:     true,
			Message:     "This will remove untracked files and directories",
		},

		// Database commands
		{
			ID:          "drop-database",
			Name:        "DROP DATABASE",
			Pattern:     `(DROP|drop)\s+(DATABASE|database)\s+.*`,
			Description: "Dropping databases",
			Category:    CommandCategoryDatabase,
			Restriction: RestrictionLevelBlock,
			Enabled:     true,
			Message:     "Dropping databases is blocked for safety",
		},
		{
			ID:          "truncate-table",
			Name:        "TRUNCATE TABLE",
			Pattern:     `(TRUNCATE|truncate)\s+(TABLE|table)?.*`,
			Description: "Truncating tables",
			Category:    CommandCategoryDatabase,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "Truncating tables will delete all data",
		},

		// Container commands
		{
			ID:          "docker-rm-force",
			Name:        "docker rm -f",
			Pattern:     `^docker\s+rm\s+(-f|--force).*`,
			Description: "Force remove containers",
			Category:    CommandCategoryContainer,
			Restriction: RestrictionLevelConfirm,
			Enabled:     true,
			Message:     "Force removing containers will stop running containers",
		},
		{
			ID:          "docker-rmi",
			Name:        "docker rmi",
			Pattern:     `^docker\s+rmi\s+.*`,
			Description: "Removing images",
			Category:    CommandCategoryContainer,
			Restriction: RestrictionLevelConfirm,
			Enabled:     true,
			Message:     "Removing images may break dependent containers",
		},
		{
			ID:          "docker-system-prune",
			Name:        "docker system prune",
			Pattern:     `^docker\s+system\s+prune.*`,
			Description: "Pruning docker system",
			Category:    CommandCategoryContainer,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "System prune will remove unused containers, networks, and images",
		},

		// Cloud commands
		{
			ID:          "aws-terminate",
			Name:        "AWS terminate instances",
			Pattern:     `^aws\s+.*terminate-instances.*`,
			Description: "Terminating cloud instances",
			Category:    CommandCategoryCloud,
			Restriction: RestrictionLevelBlock,
			Enabled:     true,
			Message:     "Terminating cloud instances is blocked for safety",
		},
		{
			ID:          "kubectl-delete",
			Name:        "kubectl delete",
			Pattern:     `^kubectl\s+delete\s+.*`,
			Description: "Deleting Kubernetes resources",
			Category:    CommandCategoryCloud,
			Restriction: RestrictionLevelApproval,
			Enabled:     true,
			Message:     "Deleting Kubernetes resources requires approval",
		},
	}

	for _, cmd := range defaults {
		cmd.CreatedAt = now
		cmd.ModifiedAt = now
		mgr.commands[cmd.ID] = &cmd

		// Compile regex pattern
		if re, err := regexp.Compile(cmd.Pattern); err == nil {
			mgr.patterns[cmd.ID] = re
		}

		// Add to category
		mgr.categories[cmd.Category] = append(mgr.categories[cmd.Category], cmd.ID)
		mgr.stats.ByCategory[string(cmd.Category)]++
		mgr.stats.ByRestriction[string(cmd.Restriction)]++
	}

	mgr.stats.TotalCommands = len(defaults)
}

// loadFromConfig loads custom restrictions from config file.
func (mgr *RestrictedCommandsManager) loadFromConfig() {
	if mgr.config.ConfigPath == "" {
		return
	}

	data, err := os.ReadFile(filepath.Join(mgr.config.ConfigPath, "restricted_commands.json"))
	if err != nil {
		return
	}

	var customCommands []RestrictedCommand
	if err := json.Unmarshal(data, &customCommands); err != nil {
		return
	}

	now := mgr.timeNow()
	for _, cmd := range customCommands {
		if cmd.ID == "" {
			cmd.ID = fmt.Sprintf("custom-%d", now.UnixNano())
		}
		cmd.CreatedAt = now
		cmd.ModifiedAt = now
		mgr.commands[cmd.ID] = &cmd

		if re, err := regexp.Compile(cmd.Pattern); err == nil {
			mgr.patterns[cmd.ID] = re
		}

		mgr.categories[cmd.Category] = append(mgr.categories[cmd.Category], cmd.ID)
	}
}

// SetAuditLogger sets the audit logger.
func (mgr *RestrictedCommandsManager) SetAuditLogger(logger *AuditLogger) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.auditLogger = logger
}

// AddCommand adds a new restricted command.
func (mgr *RestrictedCommandsManager) AddCommand(cmd RestrictedCommand) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if cmd.ID == "" {
		cmd.ID = fmt.Sprintf("cmd-%d", mgr.timeNow().UnixNano())
	}

	// Validate pattern
	re, err := regexp.Compile(cmd.Pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	now := mgr.timeNow()
	cmd.CreatedAt = now
	cmd.ModifiedAt = now

	mgr.commands[cmd.ID] = &cmd
	mgr.patterns[cmd.ID] = re
	mgr.categories[cmd.Category] = append(mgr.categories[cmd.Category], cmd.ID)
	mgr.stats.TotalCommands++
	mgr.stats.ByCategory[string(cmd.Category)]++
	mgr.stats.ByRestriction[string(cmd.Restriction)]++

	// Save to config
	mgr.saveToConfig()

	return nil
}

// RemoveCommand removes a restricted command.
func (mgr *RestrictedCommandsManager) RemoveCommand(id string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	cmd, exists := mgr.commands[id]
	if !exists {
		return fmt.Errorf("command %s not found", id)
	}

	delete(mgr.commands, id)
	delete(mgr.patterns, id)

	// Remove from category
	for i, cmdID := range mgr.categories[cmd.Category] {
		if cmdID == id {
			mgr.categories[cmd.Category] = append(
				mgr.categories[cmd.Category][:i],
				mgr.categories[cmd.Category][i+1:]...,
			)
			break
		}
	}

	mgr.stats.TotalCommands--
	mgr.stats.ByCategory[string(cmd.Category)]--
	mgr.stats.ByRestriction[string(cmd.Restriction)]--

	// Save to config
	mgr.saveToConfig()

	return nil
}

// UpdateCommand updates an existing restricted command.
func (mgr *RestrictedCommandsManager) UpdateCommand(cmd RestrictedCommand) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	existing, exists := mgr.commands[cmd.ID]
	if !exists {
		return fmt.Errorf("command %s not found", cmd.ID)
	}

	// Validate new pattern
	re, err := regexp.Compile(cmd.Pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	// Update category counts if changed
	if existing.Category != cmd.Category {
		mgr.stats.ByCategory[string(existing.Category)]--
		mgr.stats.ByCategory[string(cmd.Category)]++
	}

	// Update restriction counts if changed
	if existing.Restriction != cmd.Restriction {
		mgr.stats.ByRestriction[string(existing.Restriction)]--
		mgr.stats.ByRestriction[string(cmd.Restriction)]++
	}

	cmd.ModifiedAt = mgr.timeNow()
	cmd.CreatedAt = existing.CreatedAt

	mgr.commands[cmd.ID] = &cmd
	mgr.patterns[cmd.ID] = re

	// Save to config
	mgr.saveToConfig()

	return nil
}

// GetCommand retrieves a restricted command by ID.
func (mgr *RestrictedCommandsManager) GetCommand(id string) (*RestrictedCommand, bool) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	cmd, exists := mgr.commands[id]
	return cmd, exists
}

// ListCommands lists all restricted commands.
func (mgr *RestrictedCommandsManager) ListCommands() []*RestrictedCommand {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	result := make([]*RestrictedCommand, 0, len(mgr.commands))
	for _, cmd := range mgr.commands {
		result = append(result, cmd)
	}
	return result
}

// ListByCategory lists commands by category.
func (mgr *RestrictedCommandsManager) ListByCategory(category CommandCategory) []*RestrictedCommand {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	result := make([]*RestrictedCommand, 0)
	for _, id := range mgr.categories[category] {
		if cmd, exists := mgr.commands[id]; exists {
			result = append(result, cmd)
		}
	}
	return result
}

// ListByRestriction lists commands by restriction level.
func (mgr *RestrictedCommandsManager) ListByRestriction(level RestrictionLevel) []*RestrictedCommand {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	result := make([]*RestrictedCommand, 0)
	for _, cmd := range mgr.commands {
		if cmd.Restriction == level {
			result = append(result, cmd)
		}
	}
	return result
}

// CheckCommand checks if a command is restricted.
func (mgr *RestrictedCommandsManager) CheckCommand(command string) *RestrictionResult {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.stats.TotalChecks++

	// Skip if not enabled
	if !mgr.config.Enabled {
		return &RestrictionResult{
			Restricted:  false,
			Restriction: RestrictionLevelNone,
			Message:     "Restriction checking is disabled",
		}
	}

	// Check each command pattern
	for id, re := range mgr.patterns {
		cmd, exists := mgr.commands[id]
		if !exists || !cmd.Enabled {
			continue
		}

		if re.MatchString(command) {
			result := &RestrictionResult{
				Restricted:     cmd.Restriction != RestrictionLevelNone,
				Restriction:    cmd.Restriction,
				Command:        cmd,
				Message:        cmd.Message,
				MatchedPattern: cmd.Pattern,
			}

			// Check flag restrictions
			if len(cmd.BlockWithFlag) > 0 {
				for _, flag := range cmd.BlockWithFlag {
					if strings.Contains(command, flag) {
						result.Restriction = RestrictionLevelBlock
						result.Message = fmt.Sprintf("Command blocked due to flag: %s", flag)
					}
				}
			}

			// Check time restrictions
			if cmd.TimeRestriction != nil {
				if !mgr.isTimeAllowed(cmd.TimeRestriction) {
					result.Restriction = RestrictionLevelBlock
					result.Message = "Command not allowed at this time"
				}
			}

			// Update stats
			switch result.Restriction {
			case RestrictionLevelBlock:
				mgr.stats.TotalBlocked++
				if mgr.stats.TotalBlocked > mgr.stats.MostBlockedCount {
					mgr.stats.MostBlockedCount = mgr.stats.TotalBlocked
					mgr.stats.MostBlockedCmd = cmd.Name
				}
			case RestrictionLevelWarn:
				mgr.stats.TotalWarned++
			case RestrictionLevelApproval:
				mgr.stats.TotalApproved++
			}

			// Log to audit
			if mgr.auditLogger != nil && mgr.config.LogRestrictions {
				details := fmt.Sprintf("command=%s restriction=%s matched=%s",
					command, string(result.Restriction), cmd.Name)
				mgr.auditLogger.LogSecurity("command_restriction", details, AuditSeverityWarning)
			}

			return result
		}
	}

	// No match - return default
	return &RestrictionResult{
		Restricted:  mgr.config.DefaultRestriction != RestrictionLevelNone,
		Restriction: mgr.config.DefaultRestriction,
		Message:     "Command not in restricted list",
	}
}

// isTimeAllowed checks if current time is allowed for the restriction.
func (mgr *RestrictedCommandsManager) isTimeAllowed(tr *TimeRestriction) bool {
	now := mgr.timeNow()
	hour := now.Hour()
	day := int(now.Weekday())

	// Check blocked hours
	for _, h := range tr.BlockedHours {
		if h == hour {
			return false
		}
	}

	// Check blocked days
	for _, d := range tr.BlockedDays {
		if d == day {
			return false
		}
	}

	// If allowed hours specified, must be in list
	if len(tr.AllowedHours) > 0 {
		allowed := false
		for _, h := range tr.AllowedHours {
			if h == hour {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	// If allowed days specified, must be in list
	if len(tr.AllowedDays) > 0 {
		allowed := false
		for _, d := range tr.AllowedDays {
			if d == day {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	return true
}

// GetStats returns restriction statistics.
func (mgr *RestrictedCommandsManager) GetStats() RestrictionStats {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.stats
}

// EnableCommand enables a restricted command.
func (mgr *RestrictedCommandsManager) EnableCommand(id string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	cmd, exists := mgr.commands[id]
	if !exists {
		return fmt.Errorf("command %s not found", id)
	}

	cmd.Enabled = true
	cmd.ModifiedAt = mgr.timeNow()

	return nil
}

// DisableCommand disables a restricted command.
func (mgr *RestrictedCommandsManager) DisableCommand(id string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	cmd, exists := mgr.commands[id]
	if !exists {
		return fmt.Errorf("command %s not found", id)
	}

	cmd.Enabled = false
	cmd.ModifiedAt = mgr.timeNow()

	return nil
}

// SetRestrictionLevel sets the restriction level for a command.
func (mgr *RestrictedCommandsManager) SetRestrictionLevel(id string, level RestrictionLevel) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	cmd, exists := mgr.commands[id]
	if !exists {
		return fmt.Errorf("command %s not found", id)
	}

	// Update stats
	mgr.stats.ByRestriction[string(cmd.Restriction)]--
	mgr.stats.ByRestriction[string(level)]++

	cmd.Restriction = level
	cmd.ModifiedAt = mgr.timeNow()

	return nil
}

// SetGlobalRestriction sets the default restriction level.
func (mgr *RestrictedCommandsManager) SetGlobalRestriction(level RestrictionLevel) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.config.DefaultRestriction = level
}

// Export exports all restricted commands to JSON.
func (mgr *RestrictedCommandsManager) Export() ([]byte, error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	commands := make([]RestrictedCommand, 0, len(mgr.commands))
	for _, cmd := range mgr.commands {
		commands = append(commands, *cmd)
	}

	return json.MarshalIndent(commands, "", "  ")
}

// Import imports restricted commands from JSON.
func (mgr *RestrictedCommandsManager) Import(data []byte) error {
	var commands []RestrictedCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		return fmt.Errorf("failed to parse commands: %w", err)
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	now := mgr.timeNow()
	for _, cmd := range commands {
		if cmd.ID == "" {
			cmd.ID = fmt.Sprintf("imported-%d", now.UnixNano())
		}
		cmd.CreatedAt = now
		cmd.ModifiedAt = now

		mgr.commands[cmd.ID] = &cmd

		if re, err := regexp.Compile(cmd.Pattern); err == nil {
			mgr.patterns[cmd.ID] = re
		}
	}

	return nil
}

// Reset resets all commands to defaults.
func (mgr *RestrictedCommandsManager) Reset() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.commands = make(map[string]*RestrictedCommand)
	mgr.patterns = make(map[string]*regexp.Regexp)
	mgr.categories = make(map[CommandCategory][]string)
	mgr.stats = RestrictionStats{
		ByCategory:    make(map[string]int),
		ByRestriction: make(map[string]int),
	}

	mgr.initDefaultCommands()
}

// saveToConfig saves custom commands to config file.
func (mgr *RestrictedCommandsManager) saveToConfig() {
	if mgr.config.ConfigPath == "" {
		return
	}

	// Only save custom commands (not defaults)
	customCommands := make([]RestrictedCommand, 0)
	for _, cmd := range mgr.commands {
		// Skip default commands (check if ID matches a default pattern)
		if !strings.HasPrefix(cmd.ID, "cmd-") && !strings.HasPrefix(cmd.ID, "custom-") {
			continue
		}
		customCommands = append(customCommands, *cmd)
	}

	if len(customCommands) == 0 {
		return
	}

	data, err := json.MarshalIndent(customCommands, "", "  ")
	if err != nil {
		return
	}

	os.MkdirAll(mgr.config.ConfigPath, 0755)
	os.WriteFile(filepath.Join(mgr.config.ConfigPath, "restricted_commands.json"), data, 0644)
}
