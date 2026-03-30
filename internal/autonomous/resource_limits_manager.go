// Package autonomous - Task 34: Resource Limits (CPU, Memory, Execution Time) for sandboxes
package autonomous

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SandboxLimitsProfile represents a predefined sandbox resource limit profile.
type SandboxLimitsProfile string

const (
	// SandboxLimitsProfileLow - Low resource limits (suitable for simple tasks)
	SandboxLimitsProfileLow SandboxLimitsProfile = "low"

	// SandboxLimitsProfileMedium - Medium resource limits (balanced)
	SandboxLimitsProfileMedium SandboxLimitsProfile = "medium"

	// SandboxLimitsProfileHigh - High resource limits (for complex builds/tests)
	SandboxLimitsProfileHigh SandboxLimitsProfile = "high"

	// SandboxLimitsProfileUnlimited - No resource limits (use with caution)
	SandboxLimitsProfileUnlimited SandboxLimitsProfile = "unlimited"

	// SandboxLimitsProfileCustom - Custom resource limits
	SandboxLimitsProfileCustom SandboxLimitsProfile = "custom"
)

// SandboxLimits defines comprehensive resource constraints for sandbox execution.
type SandboxLimits struct {
	// ID is the unique identifier
	ID string `json:"id"`

	// Name is a human-readable name
	Name string `json:"name"`

	// Profile is the resource profile type
	Profile SandboxLimitsProfile `json:"profile"`

	// CPU limits
	CPUShares  int64   `json:"cpu_shares"`  // Relative CPU weight (1-1024)
	CPUPercent int     `json:"cpu_percent"` // CPU percentage (0-100)
	CPUs       float64 `json:"cpus"`        // Number of CPUs (e.g., 1.5)
	CPUQuota   int64   `json:"cpu_quota"`   // CPU quota in microseconds
	CPUPeriod  int64   `json:"cpu_period"`  // CPU period in microseconds

	// Memory limits
	MemoryMB          int64 `json:"memory_mb"`          // Memory limit in MB
	MemorySwapMB      int64 `json:"memory_swap_mb"`     // Memory + swap limit in MB
	MemorySwapiness   int   `json:"memory_swappiness"`  // Swappiness (0-100)
	MemoryReservation int64 `json:"memory_reservation"` // Memory soft limit

	// Process limits
	PidsLimit      int64 `json:"pids_limit"`       // Max number of processes
	OpenFilesLimit int64 `json:"open_files_limit"` // Max open files

	// I/O limits
	BlkioWeight    uint16 `json:"blkio_weight"`     // Block I/O weight (10-1000)
	BlkioReadBPS   int64  `json:"blkio_read_bps"`   // Read bytes per second
	BlkioWriteBPS  int64  `json:"blkio_write_bps"`  // Write bytes per second
	BlkioReadIOPS  int64  `json:"blkio_read_iops"`  // Read I/O operations per second
	BlkioWriteIOPS int64  `json:"blkio_write_iops"` // Write I/O operations per second

	// Time limits
	ExecTimeout time.Duration `json:"exec_timeout"`  // Command execution timeout
	MaxExecTime time.Duration `json:"max_exec_time"` // Maximum execution time
	IdleTimeout time.Duration `json:"idle_timeout"`  // Idle timeout before termination

	// Network limits
	NetworkIngressBPS int64 `json:"network_ingress_bps"` // Network ingress bytes/sec
	NetworkEgressBPS  int64 `json:"network_egress_bps"`  // Network egress bytes/sec

	// Enforcement settings
	StrictEnforcement bool `json:"strict_enforcement"` // Kill on violation
	WarnOnViolation   bool `json:"warn_on_violation"`  // Log warnings

	// CreatedAt timestamp
	CreatedAt time.Time `json:"created_at"`

	// ModifiedAt timestamp
	ModifiedAt time.Time `json:"modified_at"`
}

// ContainerUsage represents current resource usage for a container.
type ContainerUsage struct {
	// ContainerID is the container identifier
	ContainerID string `json:"container_id"`

	// CPU usage
	CPUPercent    float64 `json:"cpu_percent"`
	CPUUsageNanos int64   `json:"cpu_usage_nanos"`

	// Memory usage
	MemoryMB      int64   `json:"memory_mb"`
	MemoryLimitMB int64   `json:"memory_limit_mb"`
	MemoryPercent float64 `json:"memory_percent"`

	// Network I/O
	NetworkRxBytes int64 `json:"network_rx_bytes"`
	NetworkTxBytes int64 `json:"network_tx_bytes"`

	// Block I/O
	BlockReadBytes  int64 `json:"block_read_bytes"`
	BlockWriteBytes int64 `json:"block_write_bytes"`

	// Process count
	PidsCurrent int64 `json:"pids_current"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// ContainerViolation represents a resource limit violation for a container.
type ContainerViolation struct {
	// ContainerID is the container identifier
	ContainerID string `json:"container_id"`

	// Type of violation
	Type string `json:"type"`

	// Current value
	Current interface{} `json:"current"`

	// Limit value
	Limit interface{} `json:"limit"`

	// Severity of violation
	Severity string `json:"severity"`

	// Message describing the violation
	Message string `json:"message"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`

	// Action taken
	Action string `json:"action"`
}

// SandboxLimitsConfig configures the sandbox resource limits manager.
type SandboxLimitsConfig struct {
	// Enabled turns on resource limit enforcement
	Enabled bool `json:"enabled"`

	// ConfigPath is where to load/save custom profiles
	ConfigPath string `json:"config_path"`

	// DefaultProfile is the default profile to use
	DefaultProfile SandboxLimitsProfile `json:"default_profile"`

	// MonitorInterval is how often to check resource usage
	MonitorInterval time.Duration `json:"monitor_interval"`

	// LogUsage logs resource usage periodically
	LogUsage bool `json:"log_usage"`

	// KillOnViolation kills containers on violation
	KillOnViolation bool `json:"kill_on_violation"`
}

// DefaultSandboxLimitsConfig returns the default configuration.
func DefaultSandboxLimitsConfig() SandboxLimitsConfig {
	return SandboxLimitsConfig{
		Enabled:         true,
		DefaultProfile:  SandboxLimitsProfileMedium,
		MonitorInterval: 5 * time.Second,
		LogUsage:        true,
		KillOnViolation: true,
	}
}

// SandboxLimitsManager manages resource limits for sandboxes.
type SandboxLimitsManager struct {
	mu sync.RWMutex

	// config is the configuration
	config SandboxLimitsConfig

	// profiles stores predefined profiles
	profiles map[SandboxLimitsProfile]*SandboxLimits

	// customProfiles stores custom profiles
	customProfiles map[string]*SandboxLimits

	// containerLimits maps container ID to limits
	containerLimits map[string]*SandboxLimits

	// usageHistory stores usage history
	usageHistory map[string][]ContainerUsage

	// violations stores violations
	violations []ContainerViolation

	// stats tracks statistics
	stats SandboxResourceStats

	// timeNow is a function to get current time (for testing)
	timeNow func() time.Time
}

// SandboxResourceStats tracks resource statistics.
type SandboxResourceStats struct {
	TotalContainers  int            `json:"total_containers"`
	ActiveContainers int            `json:"active_containers"`
	TotalViolations  int            `json:"total_violations"`
	ViolationsByType map[string]int `json:"violations_by_type"`
	KilledContainers int            `json:"killed_containers"`
	AvgCPUUsage      float64        `json:"avg_cpu_usage"`
	AvgMemoryUsage   float64        `json:"avg_memory_usage"`
	PeakCPUUsage     float64        `json:"peak_cpu_usage"`
	PeakMemoryUsage  float64        `json:"peak_memory_usage"`
}

// NewSandboxLimitsManager creates a new resource limits manager.
func NewSandboxLimitsManager(config SandboxLimitsConfig) *SandboxLimitsManager {
	mgr := &SandboxLimitsManager{
		config:          config,
		profiles:        make(map[SandboxLimitsProfile]*SandboxLimits),
		customProfiles:  make(map[string]*SandboxLimits),
		containerLimits: make(map[string]*SandboxLimits),
		usageHistory:    make(map[string][]ContainerUsage),
		violations:      make([]ContainerViolation, 0),
		stats: SandboxResourceStats{
			ViolationsByType: make(map[string]int),
		},
		timeNow: time.Now,
	}

	// Initialize default profiles
	mgr.initDefaultProfiles()

	// Load custom profiles
	mgr.loadCustomProfiles()

	return mgr
}

// initDefaultProfiles initializes the default resource profiles.
func (mgr *SandboxLimitsManager) initDefaultProfiles() {
	now := mgr.timeNow()

	// Low profile - minimal resources
	mgr.profiles[SandboxLimitsProfileLow] = &SandboxLimits{
		ID:                "profile-low",
		Name:              "Low Resources",
		Profile:           SandboxLimitsProfileLow,
		CPUShares:         256,
		CPUs:              0.5,
		MemoryMB:          256,
		MemorySwapMB:      512,
		PidsLimit:         50,
		OpenFilesLimit:    1024,
		ExecTimeout:       1 * time.Minute,
		MaxExecTime:       5 * time.Minute,
		IdleTimeout:       30 * time.Second,
		StrictEnforcement: true,
		WarnOnViolation:   true,
		CreatedAt:         now,
		ModifiedAt:        now,
	}

	// Medium profile - balanced
	mgr.profiles[SandboxLimitsProfileMedium] = &SandboxLimits{
		ID:                "profile-medium",
		Name:              "Medium Resources",
		Profile:           SandboxLimitsProfileMedium,
		CPUShares:         512,
		CPUs:              1.0,
		MemoryMB:          512,
		MemorySwapMB:      1024,
		PidsLimit:         100,
		OpenFilesLimit:    2048,
		ExecTimeout:       5 * time.Minute,
		MaxExecTime:       30 * time.Minute,
		IdleTimeout:       1 * time.Minute,
		StrictEnforcement: true,
		WarnOnViolation:   true,
		CreatedAt:         now,
		ModifiedAt:        now,
	}

	// High profile - for heavy workloads
	mgr.profiles[SandboxLimitsProfileHigh] = &SandboxLimits{
		ID:                "profile-high",
		Name:              "High Resources",
		Profile:           SandboxLimitsProfileHigh,
		CPUShares:         1024,
		CPUs:              2.0,
		MemoryMB:          2048,
		MemorySwapMB:      4096,
		PidsLimit:         500,
		OpenFilesLimit:    8192,
		ExecTimeout:       15 * time.Minute,
		MaxExecTime:       2 * time.Hour,
		IdleTimeout:       5 * time.Minute,
		StrictEnforcement: false,
		WarnOnViolation:   true,
		CreatedAt:         now,
		ModifiedAt:        now,
	}

	// Unlimited profile - no limits
	mgr.profiles[SandboxLimitsProfileUnlimited] = &SandboxLimits{
		ID:                "profile-unlimited",
		Name:              "Unlimited Resources",
		Profile:           SandboxLimitsProfileUnlimited,
		ExecTimeout:       1 * time.Hour,
		MaxExecTime:       24 * time.Hour,
		StrictEnforcement: false,
		WarnOnViolation:   false,
		CreatedAt:         now,
		ModifiedAt:        now,
	}
}

// loadCustomProfiles loads custom profiles from config file.
func (mgr *SandboxLimitsManager) loadCustomProfiles() {
	if mgr.config.ConfigPath == "" {
		return
	}

	data, err := os.ReadFile(filepath.Join(mgr.config.ConfigPath, "sandbox_profiles.json"))
	if err != nil {
		return
	}

	var profiles []SandboxLimits
	if err := json.Unmarshal(data, &profiles); err != nil {
		return
	}

	for i := range profiles {
		profile := &profiles[i]
		profile.Profile = SandboxLimitsProfileCustom
		mgr.customProfiles[profile.ID] = profile
	}
}

// GetProfile returns a resource profile by name.
func (mgr *SandboxLimitsManager) GetProfile(profile SandboxLimitsProfile) *SandboxLimits {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if p, exists := mgr.profiles[profile]; exists {
		return p
	}
	return nil
}

// GetCustomProfile returns a custom profile by ID.
func (mgr *SandboxLimitsManager) GetCustomProfile(id string) *SandboxLimits {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.customProfiles[id]
}

// AddCustomProfile adds a new custom profile.
func (mgr *SandboxLimitsManager) AddCustomProfile(profile SandboxLimits) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if profile.ID == "" {
		profile.ID = fmt.Sprintf("custom-%d", mgr.timeNow().UnixNano())
	}

	profile.Profile = SandboxLimitsProfileCustom
	profile.CreatedAt = mgr.timeNow()
	profile.ModifiedAt = profile.CreatedAt

	mgr.customProfiles[profile.ID] = &profile

	// Save to config
	mgr.saveCustomProfiles()

	return nil
}

// RemoveCustomProfile removes a custom profile.
func (mgr *SandboxLimitsManager) RemoveCustomProfile(id string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if _, exists := mgr.customProfiles[id]; !exists {
		return fmt.Errorf("profile %s not found", id)
	}

	delete(mgr.customProfiles, id)
	mgr.saveCustomProfiles()

	return nil
}

// AssignLimits assigns resource limits to a container.
func (mgr *SandboxLimitsManager) AssignLimits(containerID string, limits *SandboxLimits) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.containerLimits[containerID] = limits
	mgr.usageHistory[containerID] = make([]ContainerUsage, 0)
	mgr.stats.TotalContainers++
	mgr.stats.ActiveContainers++
}

// AssignProfile assigns a predefined profile to a container.
func (mgr *SandboxLimitsManager) AssignProfile(containerID string, profile SandboxLimitsProfile) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	limits, exists := mgr.profiles[profile]
	if !exists {
		return fmt.Errorf("profile %s not found", profile)
	}

	mgr.containerLimits[containerID] = limits
	mgr.usageHistory[containerID] = make([]ContainerUsage, 0)
	mgr.stats.TotalContainers++
	mgr.stats.ActiveContainers++

	return nil
}

// RemoveContainer removes a container from tracking.
func (mgr *SandboxLimitsManager) RemoveContainer(containerID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	delete(mgr.containerLimits, containerID)
	delete(mgr.usageHistory, containerID)
	if mgr.stats.ActiveContainers > 0 {
		mgr.stats.ActiveContainers--
	}
}

// GetLimits returns the limits for a container.
func (mgr *SandboxLimitsManager) GetLimits(containerID string) *SandboxLimits {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.containerLimits[containerID]
}

// ToDockerArgs converts SandboxLimits to Docker run arguments.
func (limits *SandboxLimits) ToDockerArgs() []string {
	args := make([]string, 0)

	// CPU limits
	if limits.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", limits.CPUs))
	}
	if limits.CPUShares > 0 {
		args = append(args, "--cpu-shares", fmt.Sprintf("%d", limits.CPUShares))
	}
	if limits.CPUQuota > 0 && limits.CPUPeriod > 0 {
		args = append(args, "--cpu-quota", fmt.Sprintf("%d", limits.CPUQuota))
		args = append(args, "--cpu-period", fmt.Sprintf("%d", limits.CPUPeriod))
	}

	// Memory limits
	if limits.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", limits.MemoryMB))
	}
	if limits.MemorySwapMB > 0 {
		args = append(args, "--memory-swap", fmt.Sprintf("%dm", limits.MemorySwapMB))
	}
	if limits.MemorySwapiness >= 0 && limits.MemorySwapiness <= 100 {
		args = append(args, "--memory-swappiness", fmt.Sprintf("%d", limits.MemorySwapiness))
	}
	if limits.MemoryReservation > 0 {
		args = append(args, "--memory-reservation", fmt.Sprintf("%dm", limits.MemoryReservation))
	}

	// Process limits
	if limits.PidsLimit > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", limits.PidsLimit))
	}

	// I/O limits
	if limits.BlkioWeight >= 10 && limits.BlkioWeight <= 1000 {
		args = append(args, "--blkio-weight", fmt.Sprintf("%d", limits.BlkioWeight))
	}

	return args
}

// ToSandboxResourceLimits converts to SandboxResourceLimits.
func (limits *SandboxLimits) ToSandboxResourceLimits() SandboxResourceLimits {
	return SandboxResourceLimits{
		CPUShares:    limits.CPUShares,
		MemoryMB:     limits.MemoryMB,
		MemorySwapMB: limits.MemorySwapMB,
		CPUPercent:   limits.CPUPercent,
		PidsLimit:    limits.PidsLimit,
		Timeout:      limits.ExecTimeout,
	}
}

// ParseDockerStats parses docker stats output into ContainerUsage.
func ParseDockerStats(output string) (*ContainerUsage, error) {
	usage := &ContainerUsage{
		Timestamp: time.Now(),
	}

	// Parse CPU percentage
	cpuMatch := regexp.MustCompile(`"cpu":"(\d+(?:\.\d+)?%)?"`).FindStringSubmatch(output)
	if len(cpuMatch) > 1 && cpuMatch[1] != "" {
		cpuStr := strings.TrimSuffix(cpuMatch[1], "%")
		if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
			usage.CPUPercent = cpu
		}
	}

	// Parse memory usage (format: "used / limit MiB")
	memMatch := regexp.MustCompile(`"memory":"(\d+(?:\.\d+)?[A-Za-z]+)\s*/\s*(\d+(?:\.\d+)?[A-Za-z]+)"`).FindStringSubmatch(output)
	if len(memMatch) > 2 {
		usage.MemoryMB = parseMemoryValue(memMatch[1])
		usage.MemoryLimitMB = parseMemoryValue(memMatch[2])
		if usage.MemoryLimitMB > 0 {
			usage.MemoryPercent = float64(usage.MemoryMB) / float64(usage.MemoryLimitMB) * 100
		}
	}

	// Parse network I/O
	netMatch := regexp.MustCompile(`"net_io":"(\d+(?:\.\d+)?[A-Za-z]+)\s*/\s*(\d+(?:\.\d+)?[A-Za-z]+)"`).FindStringSubmatch(output)
	if len(netMatch) > 2 {
		usage.NetworkRxBytes = parseBytesValue(netMatch[1])
		usage.NetworkTxBytes = parseBytesValue(netMatch[2])
	}

	return usage, nil
}

// parseMemoryValue parses memory values like "512MiB", "1.5GiB"
func parseMemoryValue(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0B" {
		return 0
	}

	// Extract number and unit
	var value float64
	var unit string

	_, err := fmt.Sscanf(s, "%f%s", &value, &unit)
	if err != nil {
		return 0
	}

	switch strings.ToUpper(unit) {
	case "B":
		return int64(value)
	case "KIB", "KB":
		return int64(value * 1024 / (1024 * 1024)) // Convert to MB
	case "MIB", "MB":
		return int64(value)
	case "GIB", "GB":
		return int64(value * 1024)
	case "TIB", "TB":
		return int64(value * 1024 * 1024)
	}

	return int64(value)
}

// parseBytesValue parses byte values like "1.5kB", "2MB"
func parseBytesValue(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0B" {
		return 0
	}

	var value float64
	var unit string

	_, err := fmt.Sscanf(s, "%f%s", &value, &unit)
	if err != nil {
		// Try alternate parsing
		fmt.Sscanf(s, "%f%s", &value, &unit)
	}

	switch strings.ToUpper(unit) {
	case "B":
		return int64(value)
	case "KB", "KIB":
		return int64(value * 1024)
	case "MB", "MIB":
		return int64(value * 1024 * 1024)
	case "GB", "GIB":
		return int64(value * 1024 * 1024 * 1024)
	}

	return int64(value)
}

// CheckViolation checks if current usage violates limits.
func (mgr *SandboxLimitsManager) CheckViolation(containerID string, usage *ContainerUsage) *ContainerViolation {
	mgr.mu.RLock()
	limits, exists := mgr.containerLimits[containerID]
	mgr.mu.RUnlock()

	if !exists || limits == nil {
		return nil
	}

	var violation *ContainerViolation

	// Check memory limit
	if limits.MemoryMB > 0 && usage.MemoryMB > limits.MemoryMB {
		violation = &ContainerViolation{
			ContainerID: containerID,
			Type:        "memory",
			Current:     usage.MemoryMB,
			Limit:       limits.MemoryMB,
			Severity:    "high",
			Message:     fmt.Sprintf("Memory usage %dMB exceeds limit %dMB", usage.MemoryMB, limits.MemoryMB),
			Timestamp:   mgr.timeNow(),
			Action:      "warn",
		}
	}

	// Check CPU usage (only warn, don't enforce)
	if limits.CPUPercent > 0 && usage.CPUPercent > float64(limits.CPUPercent)*1.5 {
		if violation == nil {
			violation = &ContainerViolation{
				ContainerID: containerID,
				Type:        "cpu",
				Current:     usage.CPUPercent,
				Limit:       limits.CPUPercent,
				Severity:    "medium",
				Message:     fmt.Sprintf("CPU usage %.1f%% exceeds limit %d%%", usage.CPUPercent, limits.CPUPercent),
				Timestamp:   mgr.timeNow(),
				Action:      "warn",
			}
		}
	}

	// Check process count
	if limits.PidsLimit > 0 && usage.PidsCurrent > limits.PidsLimit {
		if violation == nil {
			violation = &ContainerViolation{
				ContainerID: containerID,
				Type:        "pids",
				Current:     usage.PidsCurrent,
				Limit:       limits.PidsLimit,
				Severity:    "high",
				Message:     fmt.Sprintf("Process count %d exceeds limit %d", usage.PidsCurrent, limits.PidsLimit),
				Timestamp:   mgr.timeNow(),
				Action:      "kill",
			}
		}
	}

	if violation != nil {
		mgr.recordViolation(violation)
	}

	return violation
}

// recordViolation records a resource violation.
func (mgr *SandboxLimitsManager) recordViolation(violation *ContainerViolation) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.violations = append(mgr.violations, *violation)
	mgr.stats.TotalViolations++
	mgr.stats.ViolationsByType[violation.Type]++

	if violation.Action == "kill" {
		mgr.stats.KilledContainers++
	}
}

// RecordUsage records resource usage for a container.
func (mgr *SandboxLimitsManager) RecordUsage(containerID string, usage *ContainerUsage) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if history, exists := mgr.usageHistory[containerID]; exists {
		mgr.usageHistory[containerID] = append(history, *usage)

		// Keep only last 100 entries
		if len(mgr.usageHistory[containerID]) > 100 {
			mgr.usageHistory[containerID] = mgr.usageHistory[containerID][len(mgr.usageHistory[containerID])-100:]
		}
	}

	// Update stats
	if usage.CPUPercent > mgr.stats.PeakCPUUsage {
		mgr.stats.PeakCPUUsage = usage.CPUPercent
	}
	if float64(usage.MemoryMB) > mgr.stats.PeakMemoryUsage {
		mgr.stats.PeakMemoryUsage = float64(usage.MemoryMB)
	}
}

// GetUsageHistory returns usage history for a container.
func (mgr *SandboxLimitsManager) GetUsageHistory(containerID string) []ContainerUsage {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	history, exists := mgr.usageHistory[containerID]
	if !exists {
		return nil
	}

	result := make([]ContainerUsage, len(history))
	copy(result, history)
	return result
}

// GetViolations returns all recorded violations.
func (mgr *SandboxLimitsManager) GetViolations() []ContainerViolation {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	result := make([]ContainerViolation, len(mgr.violations))
	copy(result, mgr.violations)
	return result
}

// GetStats returns resource statistics.
func (mgr *SandboxLimitsManager) GetStats() SandboxResourceStats {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.stats
}

// ListProfiles lists all available profiles.
func (mgr *SandboxLimitsManager) ListProfiles() []*SandboxLimits {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	result := make([]*SandboxLimits, 0, len(mgr.profiles)+len(mgr.customProfiles))

	for _, p := range mgr.profiles {
		result = append(result, p)
	}
	for _, p := range mgr.customProfiles {
		result = append(result, p)
	}

	return result
}

// CreateLimitsFromProfile creates SandboxLimits from a profile with overrides.
func (mgr *SandboxLimitsManager) CreateLimitsFromProfile(profile SandboxLimitsProfile, overrides map[string]interface{}) *SandboxLimits {
	base := mgr.GetProfile(profile)
	if base == nil {
		base = mgr.GetProfile(SandboxLimitsProfileMedium)
	}

	// Copy base limits
	limits := *base

	// Apply overrides
	for key, value := range overrides {
		switch key {
		case "cpu_shares":
			if v, ok := value.(int64); ok {
				limits.CPUShares = v
			}
		case "cpus":
			if v, ok := value.(float64); ok {
				limits.CPUs = v
			}
		case "memory_mb":
			if v, ok := value.(int64); ok {
				limits.MemoryMB = v
			}
		case "memory_swap_mb":
			if v, ok := value.(int64); ok {
				limits.MemorySwapMB = v
			}
		case "pids_limit":
			if v, ok := value.(int64); ok {
				limits.PidsLimit = v
			}
		case "exec_timeout":
			if v, ok := value.(time.Duration); ok {
				limits.ExecTimeout = v
			}
		case "max_exec_time":
			if v, ok := value.(time.Duration); ok {
				limits.MaxExecTime = v
			}
		}
	}

	limits.ID = fmt.Sprintf("custom-%d", mgr.timeNow().UnixNano())
	limits.Profile = SandboxLimitsProfileCustom
	limits.ModifiedAt = mgr.timeNow()

	return &limits
}

// Export exports all profiles to JSON.
func (mgr *SandboxLimitsManager) Export() ([]byte, error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	data := struct {
		Profiles       map[string]*SandboxLimits `json:"profiles"`
		CustomProfiles map[string]*SandboxLimits `json:"custom_profiles"`
	}{
		Profiles:       make(map[string]*SandboxLimits),
		CustomProfiles: make(map[string]*SandboxLimits),
	}

	for k, v := range mgr.profiles {
		data.Profiles[string(k)] = v
	}
	data.CustomProfiles = mgr.customProfiles

	return json.MarshalIndent(data, "", "  ")
}

// saveCustomProfiles saves custom profiles to config file.
func (mgr *SandboxLimitsManager) saveCustomProfiles() {
	if mgr.config.ConfigPath == "" {
		return
	}

	profiles := make([]SandboxLimits, 0, len(mgr.customProfiles))
	for _, p := range mgr.customProfiles {
		profiles = append(profiles, *p)
	}

	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return
	}

	os.MkdirAll(mgr.config.ConfigPath, 0755)
	os.WriteFile(filepath.Join(mgr.config.ConfigPath, "sandbox_profiles.json"), data, 0644)
}

// Reset resets the manager state.
func (mgr *SandboxLimitsManager) Reset() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.containerLimits = make(map[string]*SandboxLimits)
	mgr.usageHistory = make(map[string][]ContainerUsage)
	mgr.violations = make([]ContainerViolation, 0)
	mgr.stats = SandboxResourceStats{
		ViolationsByType: make(map[string]int),
	}
}

// GetDefaultLimits returns the default resource limits based on config.
func (mgr *SandboxLimitsManager) GetDefaultLimits() *SandboxLimits {
	return mgr.GetProfile(mgr.config.DefaultProfile)
}

// ValidateSandboxLimits validates resource limits for consistency.
func ValidateSandboxLimits(limits *SandboxLimits) error {
	if limits == nil {
		return fmt.Errorf("limits cannot be nil")
	}

	// Validate CPU shares (1-1024)
	if limits.CPUShares < 0 || limits.CPUShares > 1024 {
		return fmt.Errorf("cpu_shares must be between 0 and 1024")
	}

	// Validate CPU percent (0-100)
	if limits.CPUPercent < 0 || limits.CPUPercent > 100 {
		return fmt.Errorf("cpu_percent must be between 0 and 100")
	}

	// Validate CPUs > 0
	if limits.CPUs < 0 {
		return fmt.Errorf("cpus cannot be negative")
	}

	// Validate memory
	if limits.MemoryMB < 0 {
		return fmt.Errorf("memory_mb cannot be negative")
	}

	// Memory swap must be >= memory
	if limits.MemorySwapMB > 0 && limits.MemorySwapMB < limits.MemoryMB {
		return fmt.Errorf("memory_swap_mb must be >= memory_mb")
	}

	// Validate swappiness (0-100)
	if limits.MemorySwapiness < 0 || limits.MemorySwapiness > 100 {
		return fmt.Errorf("memory_swappiness must be between 0 and 100")
	}

	// Validate PIDs limit
	if limits.PidsLimit < 0 {
		return fmt.Errorf("pids_limit cannot be negative")
	}

	// Validate block I/O weight (10-1000)
	if limits.BlkioWeight > 0 && (limits.BlkioWeight < 10 || limits.BlkioWeight > 1000) {
		return fmt.Errorf("blkio_weight must be between 10 and 1000")
	}

	return nil
}
