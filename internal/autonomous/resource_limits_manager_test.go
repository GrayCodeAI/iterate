package autonomous

import (
	"testing"
	"time"
)

func TestNewSandboxLimitsManager(t *testing.T) {
	config := DefaultSandboxLimitsConfig()
	mgr := NewSandboxLimitsManager(config)
	
	if mgr == nil {
		t.Fatal("expected manager to be created")
	}
	
	// Check default profiles exist
	if len(mgr.profiles) != 4 {
		t.Errorf("expected 4 default profiles, got %d", len(mgr.profiles))
	}
}

func TestSandboxLimitsManager_GetProfile(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	tests := []struct {
		profile  SandboxLimitsProfile
		expected bool
	}{
		{SandboxLimitsProfileLow, true},
		{SandboxLimitsProfileMedium, true},
		{SandboxLimitsProfileHigh, true},
		{SandboxLimitsProfileUnlimited, true},
		{SandboxLimitsProfileCustom, false},
	}
	
	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			profile := mgr.GetProfile(tt.profile)
			if (profile != nil) != tt.expected {
				t.Errorf("profile %s: expected exists=%v, got %v", tt.profile, tt.expected, profile != nil)
			}
		})
	}
}

func TestSandboxLimitsManager_ProfileValues(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	// Check Low profile
	low := mgr.GetProfile(SandboxLimitsProfileLow)
	if low == nil {
		t.Fatal("low profile not found")
	}
	if low.MemoryMB != 256 {
		t.Errorf("low profile: expected memory 256MB, got %d", low.MemoryMB)
	}
	if low.CPUShares != 256 {
		t.Errorf("low profile: expected cpu shares 256, got %d", low.CPUShares)
	}
	
	// Check Medium profile
	medium := mgr.GetProfile(SandboxLimitsProfileMedium)
	if medium == nil {
		t.Fatal("medium profile not found")
	}
	if medium.MemoryMB != 512 {
		t.Errorf("medium profile: expected memory 512MB, got %d", medium.MemoryMB)
	}
	if medium.CPUShares != 512 {
		t.Errorf("medium profile: expected cpu shares 512, got %d", medium.CPUShares)
	}
	
	// Check High profile
	high := mgr.GetProfile(SandboxLimitsProfileHigh)
	if high == nil {
		t.Fatal("high profile not found")
	}
	if high.MemoryMB != 2048 {
		t.Errorf("high profile: expected memory 2048MB, got %d", high.MemoryMB)
	}
	if high.CPUShares != 1024 {
		t.Errorf("high profile: expected cpu shares 1024, got %d", high.CPUShares)
	}
}

func TestSandboxLimitsManager_AddCustomProfile(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	custom := SandboxLimits{
		Name:      "Custom Test Profile",
		CPUShares: 768,
		MemoryMB:  1024,
	}
	
	err := mgr.AddCustomProfile(custom)
	if err != nil {
		t.Fatalf("failed to add custom profile: %v", err)
	}
	
	// Verify it was added
	if len(mgr.customProfiles) != 1 {
		t.Errorf("expected 1 custom profile, got %d", len(mgr.customProfiles))
	}
	
	// Verify ID was assigned
	var found bool
	for _, p := range mgr.customProfiles {
		if p.Name == "Custom Test Profile" {
			found = true
			if p.ID == "" {
				t.Error("expected ID to be assigned")
			}
			if p.Profile != SandboxLimitsProfileCustom {
				t.Errorf("expected profile type custom, got %s", p.Profile)
			}
		}
	}
	if !found {
		t.Error("custom profile not found in map")
	}
}

func TestSandboxLimitsManager_RemoveCustomProfile(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	// Add then remove
	custom := SandboxLimits{
		ID:        "test-custom",
		Name:      "Test",
		CPUShares: 512,
	}
	mgr.AddCustomProfile(custom)
	
	err := mgr.RemoveCustomProfile("test-custom")
	if err != nil {
		t.Fatalf("failed to remove: %v", err)
	}
	
	if len(mgr.customProfiles) != 0 {
		t.Errorf("expected 0 custom profiles, got %d", len(mgr.customProfiles))
	}
	
	// Remove non-existent
	err = mgr.RemoveCustomProfile("non-existent")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}

func TestSandboxLimitsManager_AssignProfile(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	containerID := "test-container-1"
	err := mgr.AssignProfile(containerID, SandboxLimitsProfileMedium)
	if err != nil {
		t.Fatalf("failed to assign profile: %v", err)
	}
	
	limits := mgr.GetLimits(containerID)
	if limits == nil {
		t.Fatal("expected limits to be assigned")
	}
	if limits.Profile != SandboxLimitsProfileMedium {
		t.Errorf("expected medium profile, got %s", limits.Profile)
	}
	if limits.MemoryMB != 512 {
		t.Errorf("expected memory 512MB, got %d", limits.MemoryMB)
	}
	
	// Check stats
	stats := mgr.GetStats()
	if stats.TotalContainers != 1 {
		t.Errorf("expected 1 total container, got %d", stats.TotalContainers)
	}
	if stats.ActiveContainers != 1 {
		t.Errorf("expected 1 active container, got %d", stats.ActiveContainers)
	}
}

func TestSandboxLimitsManager_AssignLimits(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	limits := &SandboxLimits{
		ID:        "custom-limits",
		CPUShares: 1024,
		MemoryMB:  2048,
	}
	
	containerID := "test-container-2"
	mgr.AssignLimits(containerID, limits)
	
	retrieved := mgr.GetLimits(containerID)
	if retrieved == nil {
		t.Fatal("expected limits to be assigned")
	}
	if retrieved.CPUShares != 1024 {
		t.Errorf("expected cpu shares 1024, got %d", retrieved.CPUShares)
	}
}

func TestSandboxLimitsManager_RemoveContainer(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	containerID := "test-container-3"
	mgr.AssignProfile(containerID, SandboxLimitsProfileLow)
	
	// Remove
	mgr.RemoveContainer(containerID)
	
	limits := mgr.GetLimits(containerID)
	if limits != nil {
		t.Error("expected limits to be removed")
	}
	
	stats := mgr.GetStats()
	if stats.ActiveContainers != 0 {
		t.Errorf("expected 0 active containers, got %d", stats.ActiveContainers)
	}
}

func TestSandboxLimits_ToDockerArgs(t *testing.T) {
	tests := []struct {
		name     string
		limits   SandboxLimits
		expected []string
	}{
		{
			name: "cpu limits",
			limits: SandboxLimits{
				CPUs:      1.5,
				CPUShares: 512,
			},
			expected: []string{"--cpus", "--cpu-shares"},
		},
		{
			name: "memory limits",
			limits: SandboxLimits{
				MemoryMB:     512,
				MemorySwapMB: 1024,
			},
			expected: []string{"--memory", "--memory-swap"},
		},
		{
			name: "pids limit",
			limits: SandboxLimits{
				PidsLimit: 100,
			},
			expected: []string{"--pids-limit"},
		},
		{
			name: "all limits",
			limits: SandboxLimits{
				CPUs:          2.0,
				CPUShares:     1024,
				MemoryMB:      1024,
				MemorySwapMB:  2048,
				PidsLimit:     200,
				BlkioWeight:   500,
			},
			expected: []string{"--cpus", "--cpu-shares", "--memory", "--memory-swap", "--pids-limit", "--blkio-weight"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.limits.ToDockerArgs()
			
			for _, expected := range tt.expected {
				found := false
				for _, arg := range args {
					if arg == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find %s in args %v", expected, args)
				}
			}
		})
	}
}

func TestSandboxLimits_ToSandboxResourceLimits(t *testing.T) {
	limits := &SandboxLimits{
		CPUShares:    512,
		MemoryMB:     1024,
		MemorySwapMB: 2048,
		CPUPercent:   50,
		PidsLimit:    100,
		ExecTimeout:  5 * time.Minute,
	}
	
	sandbox := limits.ToSandboxResourceLimits()
	
	if sandbox.CPUShares != 512 {
		t.Errorf("expected cpu shares 512, got %d", sandbox.CPUShares)
	}
	if sandbox.MemoryMB != 1024 {
		t.Errorf("expected memory 1024, got %d", sandbox.MemoryMB)
	}
	if sandbox.PidsLimit != 100 {
		t.Errorf("expected pids limit 100, got %d", sandbox.PidsLimit)
	}
}

func TestParseDockerStats(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectCPU  float64
		expectMem  int64
	}{
		{
			name:      "basic stats",
			input:     `{"cpu":"15.50%","memory":"256MiB / 512MiB","net_io":"1.2kB / 3.4kB"}`,
			expectCPU: 15.50,
			expectMem: 256,
		},
		{
			name:      "high usage",
			input:     `{"cpu":"85.00%","memory":"450MiB / 512MiB","net_io":"0B / 0B"}`,
			expectCPU: 85.00,
			expectMem: 450,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage, err := ParseDockerStats(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			
			if usage.CPUPercent != tt.expectCPU {
				t.Errorf("expected cpu %.2f, got %.2f", tt.expectCPU, usage.CPUPercent)
			}
			if usage.MemoryMB != tt.expectMem {
				t.Errorf("expected memory %d, got %d", tt.expectMem, usage.MemoryMB)
			}
		})
	}
}

func TestSandboxLimitsManager_CheckViolation(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	containerID := "test-violation"
	mgr.AssignProfile(containerID, SandboxLimitsProfileMedium)
	
	// Memory violation
	usage := &ContainerUsage{
		ContainerID:  containerID,
		MemoryMB:     600, // Limit is 512
		CPUPercent:   50,
		PidsCurrent:  50,
	}
	
	violation := mgr.CheckViolation(containerID, usage)
	if violation == nil {
		t.Fatal("expected memory violation")
	}
	if violation.Type != "memory" {
		t.Errorf("expected memory violation, got %s", violation.Type)
	}
	if violation.Severity != "high" {
		t.Errorf("expected high severity, got %s", violation.Severity)
	}
	
	// Check stats updated
	stats := mgr.GetStats()
	if stats.TotalViolations != 1 {
		t.Errorf("expected 1 violation, got %d", stats.TotalViolations)
	}
}

func TestSandboxLimitsManager_CheckViolation_CPU(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	// Create limits with CPU percent
	limits := &SandboxLimits{
		ID:         "cpu-test",
		CPUPercent: 50,
		MemoryMB:   512,
	}
	
	containerID := "test-cpu-violation"
	mgr.AssignLimits(containerID, limits)
	
	// CPU violation (usage > 1.5x limit)
	usage := &ContainerUsage{
		ContainerID: containerID,
		CPUPercent:  80, // 50 * 1.5 = 75, so 80 > 75
		MemoryMB:    256,
		PidsCurrent: 10,
	}
	
	violation := mgr.CheckViolation(containerID, usage)
	if violation == nil {
		t.Fatal("expected CPU violation")
	}
	if violation.Type != "cpu" {
		t.Errorf("expected cpu violation, got %s", violation.Type)
	}
}

func TestSandboxLimitsManager_CheckViolation_Pids(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	containerID := "test-pids-violation"
	mgr.AssignProfile(containerID, SandboxLimitsProfileLow) // PidsLimit: 50
	
	// Pids violation
	usage := &ContainerUsage{
		ContainerID: containerID,
		MemoryMB:    128,
		CPUPercent:  20,
		PidsCurrent: 60, // Limit is 50
	}
	
	violation := mgr.CheckViolation(containerID, usage)
	if violation == nil {
		t.Fatal("expected pids violation")
	}
	if violation.Type != "pids" {
		t.Errorf("expected pids violation, got %s", violation.Type)
	}
	if violation.Action != "kill" {
		t.Errorf("expected kill action, got %s", violation.Action)
	}
}

func TestSandboxLimitsManager_RecordUsage(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	containerID := "test-usage"
	mgr.AssignProfile(containerID, SandboxLimitsProfileMedium)
	
	// Record multiple usage points
	for i := 0; i < 5; i++ {
		usage := &ContainerUsage{
			ContainerID: containerID,
			CPUPercent:  float64(10 * (i + 1)),
			MemoryMB:    int64(100 * (i + 1)),
		}
		mgr.RecordUsage(containerID, usage)
	}
	
	history := mgr.GetUsageHistory(containerID)
	if len(history) != 5 {
		t.Errorf("expected 5 history entries, got %d", len(history))
	}
	
	// Check peak stats
	stats := mgr.GetStats()
	if stats.PeakCPUUsage != 50.0 {
		t.Errorf("expected peak cpu 50, got %.1f", stats.PeakCPUUsage)
	}
	if stats.PeakMemoryUsage != 500.0 {
		t.Errorf("expected peak memory 500, got %.1f", stats.PeakMemoryUsage)
	}
}

func TestSandboxLimitsManager_UsageHistoryLimit(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	containerID := "test-history-limit"
	mgr.AssignProfile(containerID, SandboxLimitsProfileMedium)
	
	// Record more than 100 entries
	for i := 0; i < 150; i++ {
		usage := &ContainerUsage{
			ContainerID: containerID,
			CPUPercent:  float64(i),
		}
		mgr.RecordUsage(containerID, usage)
	}
	
	history := mgr.GetUsageHistory(containerID)
	if len(history) > 100 {
		t.Errorf("expected max 100 history entries, got %d", len(history))
	}
}

func TestSandboxLimitsManager_ListProfiles(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	profiles := mgr.ListProfiles()
	if len(profiles) != 4 {
		t.Errorf("expected 4 profiles, got %d", len(profiles))
	}
	
	// Add custom and check again
	mgr.AddCustomProfile(SandboxLimits{Name: "Custom"})
	profiles = mgr.ListProfiles()
	if len(profiles) != 5 {
		t.Errorf("expected 5 profiles, got %d", len(profiles))
	}
}

func TestSandboxLimitsManager_CreateLimitsFromProfile(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	overrides := map[string]interface{}{
		"cpu_shares": int64(768),
		"memory_mb":  int64(1024),
	}
	
	limits := mgr.CreateLimitsFromProfile(SandboxLimitsProfileMedium, overrides)
	
	if limits == nil {
		t.Fatal("expected limits to be created")
	}
	if limits.CPUShares != 768 {
		t.Errorf("expected cpu shares 768, got %d", limits.CPUShares)
	}
	if limits.MemoryMB != 1024 {
		t.Errorf("expected memory 1024, got %d", limits.MemoryMB)
	}
	// Should inherit from medium profile
	if limits.PidsLimit != 100 {
		t.Errorf("expected pids limit 100 from medium, got %d", limits.PidsLimit)
	}
}

func TestValidateSandboxLimits(t *testing.T) {
	tests := []struct {
		name    string
		limits  *SandboxLimits
		wantErr bool
	}{
		{
			name: "valid limits",
			limits: &SandboxLimits{
				CPUShares: 512,
				MemoryMB:  512,
				PidsLimit: 100,
			},
			wantErr: false,
		},
		{
			name:    "nil limits",
			limits:  nil,
			wantErr: true,
		},
		{
			name: "cpu shares too high",
			limits: &SandboxLimits{
				CPUShares: 2000,
			},
			wantErr: true,
		},
		{
			name: "cpu percent too high",
			limits: &SandboxLimits{
				CPUPercent: 150,
			},
			wantErr: true,
		},
		{
			name: "negative memory",
			limits: &SandboxLimits{
				MemoryMB: -100,
			},
			wantErr: true,
		},
		{
			name: "swap less than memory",
			limits: &SandboxLimits{
				MemoryMB:     1024,
				MemorySwapMB: 512,
			},
			wantErr: true,
		},
		{
			name: "invalid swappiness",
			limits: &SandboxLimits{
				MemorySwapiness: 150,
			},
			wantErr: true,
		},
		{
			name: "invalid blkio weight low",
			limits: &SandboxLimits{
				BlkioWeight: 5,
			},
			wantErr: true,
		},
		{
			name: "invalid blkio weight high",
			limits: &SandboxLimits{
				BlkioWeight: 2000,
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSandboxLimits(tt.limits)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSandboxLimits() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSandboxLimitsManager_Export(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	data, err := mgr.Export()
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	
	if len(data) == 0 {
		t.Error("expected non-empty export data")
	}
}

func TestSandboxLimitsManager_Reset(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	// Add some data
	containerID := "test-reset"
	mgr.AssignProfile(containerID, SandboxLimitsProfileMedium)
	mgr.RecordUsage(containerID, &ContainerUsage{CPUPercent: 50})
	mgr.CheckViolation(containerID, &ContainerUsage{MemoryMB: 600})
	
	// Reset
	mgr.Reset()
	
	// Verify reset
	if len(mgr.containerLimits) != 0 {
		t.Error("expected container limits to be cleared")
	}
	if len(mgr.usageHistory) != 0 {
		t.Error("expected usage history to be cleared")
	}
	if len(mgr.violations) != 0 {
		t.Error("expected violations to be cleared")
	}
	
	stats := mgr.GetStats()
	if stats.TotalContainers != 0 {
		t.Error("expected stats to be reset")
	}
}

func TestSandboxLimitsManager_GetDefaultLimits(t *testing.T) {
	config := DefaultSandboxLimitsConfig()
	config.DefaultProfile = SandboxLimitsProfileHigh
	mgr := NewSandboxLimitsManager(config)
	
	limits := mgr.GetDefaultLimits()
	if limits == nil {
		t.Fatal("expected default limits")
	}
	if limits.Profile != SandboxLimitsProfileHigh {
		t.Errorf("expected high profile, got %s", limits.Profile)
	}
}

func TestSandboxLimitsManager_GetViolations(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	containerID := "test-violations"
	mgr.AssignProfile(containerID, SandboxLimitsProfileMedium)
	
	// Create multiple violations
	mgr.CheckViolation(containerID, &ContainerUsage{MemoryMB: 600})
	mgr.CheckViolation(containerID, &ContainerUsage{MemoryMB: 700})
	
	violations := mgr.GetViolations()
	if len(violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(violations))
	}
}

func TestParseMemoryValue(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"256MiB", 256},
		{"1.5GiB", 1536},
		{"512MB", 512},
		{"2GiB", 2048},
		{"0B", 0},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseMemoryValue(tt.input)
			if result != tt.expected {
				t.Errorf("parseMemoryValue(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseBytesValue(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1kB", 1024},
		{"2MB", 2 * 1024 * 1024},
		{"1.5GB", int64(1.5 * 1024 * 1024 * 1024)},
		{"0B", 0},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBytesValue(tt.input)
			if result != tt.expected {
				t.Errorf("parseBytesValue(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSandboxLimitsManager_AssignInvalidProfile(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	err := mgr.AssignProfile("test", SandboxLimitsProfileCustom)
	if err == nil {
		t.Error("expected error for invalid profile")
	}
}

func TestSandboxLimitsManager_CheckViolation_NoLimits(t *testing.T) {
	mgr := NewSandboxLimitsManager(DefaultSandboxLimitsConfig())
	
	// Container without assigned limits
	usage := &ContainerUsage{
		ContainerID: "no-limits",
		MemoryMB:    9999,
	}
	
	violation := mgr.CheckViolation("no-limits", usage)
	if violation != nil {
		t.Error("expected no violation for container without limits")
	}
}
