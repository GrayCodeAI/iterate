// Package autonomous - Task 25: Tests for network isolation
package autonomous

import (
	"testing"
)

func TestNetworkModes(t *testing.T) {
	tests := []struct {
		name     string
		mode     NetworkMode
		expected string
	}{
		{"none mode", NetworkModeNone, "none"},
		{"internal mode", NetworkModeInternal, "internal"},
		{"bridge mode", NetworkModeBridge, "bridge"},
		{"host mode", NetworkModeHost, "host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.mode) != tt.expected {
				t.Errorf("NetworkMode %s != %s", tt.mode, tt.expected)
			}
		})
	}
}

func TestDefaultNetworkPolicy(t *testing.T) {
	policy := DefaultNetworkPolicy()

	if !policy.AllowDNS {
		t.Error("Default policy should allow DNS")
	}
	if !policy.AllowHTTPS {
		t.Error("Default policy should allow HTTPS")
	}
	if policy.AllowHTTP {
		t.Error("Default policy should not allow HTTP")
	}
	if policy.AllowOutbound {
		t.Error("Default policy should not allow all outbound")
	}
	if len(policy.AllowedPorts) != 1 || policy.AllowedPorts[0] != 443 {
		t.Error("Default policy should only allow port 443")
	}
}

func TestStrictNetworkPolicy(t *testing.T) {
	policy := StrictNetworkPolicy()

	if policy.AllowDNS {
		t.Error("Strict policy should not allow DNS")
	}
	if policy.AllowHTTPS {
		t.Error("Strict policy should not allow HTTPS")
	}
	if policy.AllowHTTP {
		t.Error("Strict policy should not allow HTTP")
	}
	if policy.AllowOutbound {
		t.Error("Strict policy should not allow outbound")
	}
	if len(policy.BlockedHosts) != 1 || policy.BlockedHosts[0] != "*" {
		t.Error("Strict policy should block all hosts")
	}
}

func TestPermissiveNetworkPolicy(t *testing.T) {
	policy := PermissiveNetworkPolicy()

	if !policy.AllowDNS {
		t.Error("Permissive policy should allow DNS")
	}
	if !policy.AllowHTTPS {
		t.Error("Permissive policy should allow HTTPS")
	}
	if !policy.AllowHTTP {
		t.Error("Permissive policy should allow HTTP")
	}
	if !policy.AllowOutbound {
		t.Error("Permissive policy should allow outbound")
	}
}

func TestDevelopmentNetworkPolicy(t *testing.T) {
	policy := DevelopmentNetworkPolicy()

	if !policy.AllowInbound {
		t.Error("Development policy should allow inbound")
	}

	// Check common dev ports
	expectedPorts := map[int]bool{80: true, 443: true, 3000: true, 5000: true, 8000: true, 8080: true}
	for _, port := range policy.AllowedPorts {
		if !expectedPorts[port] {
			t.Errorf("Unexpected port %d in development policy", port)
		}
	}
}

func TestNewNetworkIsolation(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{})

	if ni.GetMode() != NetworkModeNone {
		t.Error("Default mode should be 'none'")
	}
}

func TestNewNetworkIsolationWithMode(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode: NetworkModeBridge,
	})

	if ni.GetMode() != NetworkModeBridge {
		t.Errorf("Mode should be 'bridge', got %s", ni.GetMode())
	}
}

func TestNetworkIsolation_SetMode(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{})

	ni.SetMode(NetworkModeHost)
	if ni.GetMode() != NetworkModeHost {
		t.Error("Failed to set mode to host")
	}

	ni.SetMode(NetworkModeInternal)
	if ni.GetMode() != NetworkModeInternal {
		t.Error("Failed to set mode to internal")
	}
}

func TestNetworkIsolation_SetPolicy(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{})

	newPolicy := PermissiveNetworkPolicy()
	ni.SetPolicy(newPolicy)

	if !ni.GetPolicy().AllowOutbound {
		t.Error("Failed to set permissive policy")
	}
}

func TestNetworkIsolation_PortMappings(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{})

	ni.AddPortMapping(PortMapping{
		HostPort:      8080,
		ContainerPort: 80,
		Protocol:      "tcp",
	})

	mappings := ni.GetPortMappings()
	if len(mappings) != 1 {
		t.Fatalf("Expected 1 port mapping, got %d", len(mappings))
	}

	if mappings[0].HostPort != 8080 || mappings[0].ContainerPort != 80 {
		t.Errorf("Port mapping mismatch: %+v", mappings[0])
	}

	// Remove mapping
	ni.RemovePortMapping(8080)
	mappings = ni.GetPortMappings()
	if len(mappings) != 0 {
		t.Errorf("Expected 0 port mappings after removal, got %d", len(mappings))
	}
}

func TestNetworkIsolation_BuildDockerArgs_None(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode: NetworkModeNone,
	})

	args := ni.BuildDockerArgs()

	found := false
	for i, arg := range args {
		if arg == "--network" && i+1 < len(args) && args[i+1] == "none" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected --network none in Docker args")
	}
}

func TestNetworkIsolation_BuildDockerArgs_Host(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode: NetworkModeHost,
	})

	args := ni.BuildDockerArgs()

	found := false
	for i, arg := range args {
		if arg == "--network" && i+1 < len(args) && args[i+1] == "host" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected --network host in Docker args")
	}
}

func TestNetworkIsolation_BuildDockerArgs_WithPortMapping(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode: NetworkModeBridge,
		PortMappings: []PortMapping{
			{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		},
	})

	args := ni.BuildDockerArgs()

	found := false
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) {
			expected := "0.0.0.0:8080:80/tcp"
			if args[i+1] == expected {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("Expected port mapping in Docker args, got: %v", args)
	}
}

func TestNetworkIsolation_BuildDockerArgs_WithDNS(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode: NetworkModeBridge,
		DNSConfig: &DNSConfig{
			Servers: []string{"8.8.8.8", "8.8.4.4"},
		},
	})

	args := ni.BuildDockerArgs()

	found8 := false
	found4 := false
	for i, arg := range args {
		if arg == "--dns" && i+1 < len(args) {
			if args[i+1] == "8.8.8.8" {
				found8 = true
			}
			if args[i+1] == "8.8.4.4" {
				found4 = true
			}
		}
	}

	if !found8 || !found4 {
		t.Errorf("Expected DNS servers in Docker args, got: %v", args)
	}
}

func TestNetworkIsolation_ValidateConnection_None(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode: NetworkModeNone,
	})

	err := ni.ValidateConnection("example.com", 443)
	if err == nil {
		t.Error("Expected error for connection with network mode 'none'")
	}
}

func TestNetworkIsolation_ValidateConnection_Host(t *testing.T) {
	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode: NetworkModeHost,
	})

	err := ni.ValidateConnection("example.com", 443)
	if err != nil {
		t.Errorf("Host mode should allow all connections: %v", err)
	}
}

func TestNetworkIsolation_ValidateConnection_BlockedHost(t *testing.T) {
	policy := DefaultNetworkPolicy()
	policy.BlockedHosts = []string{"blocked.com"}

	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode:   NetworkModeBridge,
		Policy: policy,
	})

	err := ni.ValidateConnection("blocked.com", 443)
	if err == nil {
		t.Error("Expected error for blocked host")
	}
}

func TestNetworkIsolation_ValidateConnection_BlockedPort(t *testing.T) {
	policy := DefaultNetworkPolicy()
	policy.BlockedPorts = []int{22}

	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode:   NetworkModeBridge,
		Policy: policy,
	})

	err := ni.ValidateConnection("example.com", 22)
	if err == nil {
		t.Error("Expected error for blocked port")
	}
}

func TestNetworkIsolation_ValidateConnection_AllowOutbound(t *testing.T) {
	policy := PermissiveNetworkPolicy()

	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode:   NetworkModeBridge,
		Policy: policy,
	})

	err := ni.ValidateConnection("example.com", 443)
	if err != nil {
		t.Errorf("Permissive policy should allow outbound: %v", err)
	}
}

func TestNetworkIsolation_ValidateConnection_DNS(t *testing.T) {
	policy := DefaultNetworkPolicy()
	policy.AllowDNS = false

	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode:   NetworkModeBridge,
		Policy: policy,
	})

	err := ni.ValidateConnection("dns.server", 53)
	if err == nil {
		t.Error("Expected error for DNS when disabled")
	}
}

func TestNetworkIsolation_ValidateConnection_HTTP(t *testing.T) {
	policy := DefaultNetworkPolicy()
	policy.AllowHTTP = false

	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode:   NetworkModeBridge,
		Policy: policy,
	})

	err := ni.ValidateConnection("example.com", 80)
	if err == nil {
		t.Error("Expected error for HTTP when disabled")
	}
}

func TestNetworkIsolation_ValidateConnection_HTTPS(t *testing.T) {
	policy := DefaultNetworkPolicy()
	policy.AllowHTTPS = false

	ni := NewNetworkIsolation(NetworkIsolationConfig{
		Mode:   NetworkModeBridge,
		Policy: policy,
	})

	err := ni.ValidateConnection("example.com", 443)
	if err == nil {
		t.Error("Expected error for HTTPS when disabled")
	}
}

func TestNetworkProfiles(t *testing.T) {
	profiles := ListNetworkProfiles()
	expectedProfiles := []string{"strict", "secure", "development", "isolated", "full"}

	for _, expected := range expectedProfiles {
		found := false
		for _, p := range profiles {
			if p == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected profile '%s' not found", expected)
		}
	}
}

func TestGetNetworkProfile(t *testing.T) {
	profile, ok := GetNetworkProfile("strict")
	if !ok {
		t.Error("Expected to find 'strict' profile")
	}

	if profile.Mode != NetworkModeNone {
		t.Errorf("Strict profile should have mode 'none', got %s", profile.Mode)
	}

	_, ok = GetNetworkProfile("nonexistent")
	if ok {
		t.Error("Expected not to find 'nonexistent' profile")
	}
}

func TestNetworkIsolationBuilder(t *testing.T) {
	ni := NewNetworkIsolationBuilder().
		WithMode(NetworkModeBridge).
		WithPolicy(PermissiveNetworkPolicy()).
		WithPortMapping(8080, 80, "tcp").
		WithDNS([]string{"8.8.8.8"}).
		Build()

	if ni.GetMode() != NetworkModeBridge {
		t.Error("Builder failed to set mode")
	}

	if !ni.GetPolicy().AllowOutbound {
		t.Error("Builder failed to set policy")
	}

	mappings := ni.GetPortMappings()
	if len(mappings) != 1 {
		t.Error("Builder failed to add port mapping")
	}
}

func TestNetworkIsolationBuilder_WithProfile(t *testing.T) {
	ni := NewNetworkIsolationBuilder().
		WithProfile("development").
		Build()

	if ni.GetMode() != NetworkModeBridge {
		t.Errorf("Development profile should use bridge mode, got %s", ni.GetMode())
	}

	if !ni.GetPolicy().AllowInbound {
		t.Error("Development profile should allow inbound")
	}
}

func TestApplyNetworkProfile(t *testing.T) {
	config := DefaultSandboxConfig()

	err := ApplyNetworkProfile(&config, "strict")
	if err != nil {
		t.Fatalf("Failed to apply profile: %v", err)
	}

	if config.NetworkEnabled {
		t.Error("Strict profile should disable network")
	}

	err = ApplyNetworkProfile(&config, "development")
	if err != nil {
		t.Fatalf("Failed to apply development profile: %v", err)
	}

	if !config.NetworkEnabled {
		t.Error("Development profile should enable network")
	}

	err = ApplyNetworkProfile(&config, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent profile")
	}
}

func TestTask25NetworkIsolation(t *testing.T) {
	// Comprehensive test for Task 25

	// Test 1: Create isolation with strict profile
	strict := NewNetworkIsolationBuilder().
		WithProfile("strict").
		Build()

	if strict.GetMode() != NetworkModeNone {
		t.Error("Strict profile should use 'none' network mode")
	}

	// Test 2: Connection validation
	err := strict.ValidateConnection("any.host", 443)
	if err == nil {
		t.Error("Strict profile should block all connections")
	}

	// Test 3: Create isolation with development profile
	dev := NewNetworkIsolationBuilder().
		WithProfile("development").
		WithPortMapping(3000, 3000, "tcp").
		Build()

	// Test 4: Docker args generation
	args := dev.BuildDockerArgs()
	if len(args) == 0 {
		t.Error("Development isolation should produce Docker args")
	}

	// Test 5: Connection validation for development
	err = dev.ValidateConnection("example.com", 443)
	if err != nil {
		t.Errorf("Development profile should allow HTTPS: %v", err)
	}

	// Test 6: Full profile allows everything
	full := NewNetworkIsolationBuilder().
		WithProfile("full").
		Build()

	err = full.ValidateConnection("any.host", 12345)
	if err != nil {
		t.Errorf("Full profile should allow all connections: %v", err)
	}
}
