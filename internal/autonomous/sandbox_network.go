// Package autonomous - Task 25: Network isolation options for sandbox
package autonomous

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// NetworkMode defines the network isolation level for the sandbox.
type NetworkMode string

const (
	// NetworkModeNone - No network access (most secure)
	NetworkModeNone NetworkMode = "none"

	// NetworkModeInternal - Internal network only (can communicate with other containers)
	NetworkModeInternal NetworkMode = "internal"

	// NetworkModeBridge - Bridge network with controlled outbound access
	NetworkModeBridge NetworkMode = "bridge"

	// NetworkModeHost - Full host network access (least secure)
	NetworkModeHost NetworkMode = "host"
)

// NetworkPolicy defines traffic rules for the sandbox.
type NetworkPolicy struct {
	// AllowedHosts is a list of hosts that can be accessed
	AllowedHosts []string `json:"allowed_hosts"`

	// BlockedHosts is a list of hosts that are explicitly blocked
	BlockedHosts []string `json:"blocked_hosts"`

	// AllowedPorts is a list of ports that can be accessed
	AllowedPorts []int `json:"allowed_ports"`

	// BlockedPorts is a list of ports that are explicitly blocked
	BlockedPorts []int `json:"blocked_ports"`

	// AllowDNS allows DNS resolution
	AllowDNS bool `json:"allow_dns"`

	// AllowHTTP allows HTTP (port 80)
	AllowHTTP bool `json:"allow_http"`

	// AllowHTTPS allows HTTPS (port 443)
	AllowHTTPS bool `json:"allow_https"`

	// AllowOutbound allows all outbound traffic
	AllowOutbound bool `json:"allow_outbound"`

	// AllowInbound allows inbound connections
	AllowInbound bool `json:"allow_inbound"`
}

// PortMapping defines a port mapping between host and container.
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // tcp, udp
	HostIP        string `json:"host_ip"`
}

// DNSConfig defines DNS configuration for the sandbox.
type DNSConfig struct {
	Servers []string `json:"servers"`
	Search  []string `json:"search"`
	Options []string `json:"options"`
}

// NetworkIsolation manages network settings for the sandbox.
type NetworkIsolation struct {
	mu sync.RWMutex

	// mode is the current network mode
	mode NetworkMode

	// policy is the active network policy
	policy NetworkPolicy

	// portMappings are active port mappings
	portMappings []PortMapping

	// dnsConfig is the DNS configuration
	dnsConfig *DNSConfig

	// customNetwork is the name of a custom Docker network
	customNetwork string

	// networkName is the name of the network to use
	networkName string

	// iptablesRules are custom iptables rules
	iptablesRules []string
}

// NetworkIsolationConfig configures network isolation.
type NetworkIsolationConfig struct {
	Mode          NetworkMode   `json:"mode"`
	Policy        NetworkPolicy `json:"policy"`
	PortMappings  []PortMapping `json:"port_mappings"`
	DNSConfig     *DNSConfig    `json:"dns_config"`
	CustomNetwork string        `json:"custom_network"`
}

// DefaultNetworkPolicy returns a secure default network policy.
func DefaultNetworkPolicy() NetworkPolicy {
	return NetworkPolicy{
		AllowDNS:      true,
		AllowHTTPS:    true,
		AllowHTTP:     false,
		AllowOutbound: false,
		AllowInbound:  false,
		AllowedHosts:  []string{},
		BlockedHosts:  []string{},
		AllowedPorts:  []int{443}, // HTTPS only by default
		BlockedPorts:  []int{},
	}
}

// StrictNetworkPolicy returns a strict network policy (no network access).
func StrictNetworkPolicy() NetworkPolicy {
	return NetworkPolicy{
		AllowDNS:      false,
		AllowHTTPS:    false,
		AllowHTTP:     false,
		AllowOutbound: false,
		AllowInbound:  false,
		AllowedHosts:  []string{},
		BlockedHosts:  []string{"*"},
		AllowedPorts:  []int{},
		BlockedPorts:  []int{},
	}
}

// PermissiveNetworkPolicy returns a permissive network policy.
func PermissiveNetworkPolicy() NetworkPolicy {
	return NetworkPolicy{
		AllowDNS:      true,
		AllowHTTPS:    true,
		AllowHTTP:     true,
		AllowOutbound: true,
		AllowInbound:  false,
		AllowedHosts:  []string{},
		BlockedHosts:  []string{},
		AllowedPorts:  []int{},
		BlockedPorts:  []int{},
	}
}

// DevelopmentNetworkPolicy returns a policy suitable for development.
func DevelopmentNetworkPolicy() NetworkPolicy {
	return NetworkPolicy{
		AllowDNS:      true,
		AllowHTTPS:    true,
		AllowHTTP:     true,
		AllowOutbound: true,
		AllowInbound:  true,
		AllowedHosts:  []string{},
		BlockedHosts:  []string{},
		AllowedPorts:  []int{80, 443, 3000, 5000, 8000, 8080},
		BlockedPorts:  []int{},
	}
}

// NewNetworkIsolation creates a new network isolation manager.
func NewNetworkIsolation(config NetworkIsolationConfig) *NetworkIsolation {
	ni := &NetworkIsolation{
		mode:          config.Mode,
		policy:        config.Policy,
		portMappings:  config.PortMappings,
		dnsConfig:     config.DNSConfig,
		customNetwork: config.CustomNetwork,
		networkName:   "",
		iptablesRules: make([]string, 0),
	}

	// Set default mode if not specified
	if ni.mode == "" {
		ni.mode = NetworkModeNone
	}

	// Set default policy if empty
	if ni.policy.AllowedPorts == nil && !ni.policy.AllowOutbound {
		ni.policy = DefaultNetworkPolicy()
	}

	return ni
}

// GetMode returns the current network mode.
func (ni *NetworkIsolation) GetMode() NetworkMode {
	ni.mu.RLock()
	defer ni.mu.RUnlock()
	return ni.mode
}

// SetMode sets the network mode.
func (ni *NetworkIsolation) SetMode(mode NetworkMode) {
	ni.mu.Lock()
	defer ni.mu.Unlock()
	ni.mode = mode
}

// GetPolicy returns the current network policy.
func (ni *NetworkIsolation) GetPolicy() NetworkPolicy {
	ni.mu.RLock()
	defer ni.mu.RUnlock()
	return ni.policy
}

// SetPolicy sets the network policy.
func (ni *NetworkIsolation) SetPolicy(policy NetworkPolicy) {
	ni.mu.Lock()
	defer ni.mu.Unlock()
	ni.policy = policy
}

// AddPortMapping adds a port mapping.
func (ni *NetworkIsolation) AddPortMapping(mapping PortMapping) {
	ni.mu.Lock()
	defer ni.mu.Unlock()
	ni.portMappings = append(ni.portMappings, mapping)
}

// RemovePortMapping removes a port mapping.
func (ni *NetworkIsolation) RemovePortMapping(hostPort int) {
	ni.mu.Lock()
	defer ni.mu.Unlock()

	var newMappings []PortMapping
	for _, m := range ni.portMappings {
		if m.HostPort != hostPort {
			newMappings = append(newMappings, m)
		}
	}
	ni.portMappings = newMappings
}

// GetPortMappings returns all port mappings.
func (ni *NetworkIsolation) GetPortMappings() []PortMapping {
	ni.mu.RLock()
	defer ni.mu.RUnlock()
	return append([]PortMapping{}, ni.portMappings...)
}

// SetDNSConfig sets the DNS configuration.
func (ni *NetworkIsolation) SetDNSConfig(config *DNSConfig) {
	ni.mu.Lock()
	defer ni.mu.Unlock()
	ni.dnsConfig = config
}

// BuildDockerArgs builds Docker arguments for network configuration.
func (ni *NetworkIsolation) BuildDockerArgs() []string {
	ni.mu.RLock()
	defer ni.mu.RUnlock()

	var args []string

	// Network mode
	switch ni.mode {
	case NetworkModeNone:
		args = append(args, "--network", "none")
	case NetworkModeHost:
		args = append(args, "--network", "host")
	case NetworkModeInternal:
		// Create/use an internal network
		if ni.customNetwork != "" {
			args = append(args, "--network", ni.customNetwork)
		} else {
			args = append(args, "--network", "internal")
		}
	case NetworkModeBridge:
		if ni.customNetwork != "" {
			args = append(args, "--network", ni.customNetwork)
		}
		// Bridge is the default, no explicit flag needed
	}

	// Port mappings (only for bridge mode)
	if ni.mode == NetworkModeBridge || ni.mode == NetworkModeInternal {
		for _, pm := range ni.portMappings {
			protocol := pm.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			hostIP := pm.HostIP
			if hostIP == "" {
				hostIP = "0.0.0.0"
			}
			arg := fmt.Sprintf("%s:%d:%d/%s", hostIP, pm.HostPort, pm.ContainerPort, protocol)
			args = append(args, "-p", arg)
		}
	}

	// DNS configuration
	if ni.dnsConfig != nil {
		for _, server := range ni.dnsConfig.Servers {
			args = append(args, "--dns", server)
		}
		for _, search := range ni.dnsConfig.Search {
			args = append(args, "--dns-search", search)
		}
		for _, opt := range ni.dnsConfig.Options {
			args = append(args, "--dns-opt", opt)
		}
	}

	// Host entries for allowed hosts
	if len(ni.policy.AllowedHosts) > 0 && ni.mode != NetworkModeNone {
		for _, host := range ni.policy.AllowedHosts {
			// Resolve host and add to /etc/hosts
			if ip := resolveHost(host); ip != "" {
				args = append(args, "--add-host", host+":"+ip)
			}
		}
	}

	return args
}

// ValidateConnection checks if a connection is allowed by the policy.
func (ni *NetworkIsolation) ValidateConnection(host string, port int) error {
	ni.mu.RLock()
	defer ni.mu.RUnlock()

	// No network mode blocks all connections
	if ni.mode == NetworkModeNone {
		return fmt.Errorf("network access disabled (mode: none)")
	}

	// Host network mode allows all
	if ni.mode == NetworkModeHost {
		return nil
	}

	// Check blocked hosts
	for _, blocked := range ni.policy.BlockedHosts {
		if blocked == "*" || blocked == host {
			return fmt.Errorf("host '%s' is blocked by policy", host)
		}
	}

	// Check blocked ports
	for _, blocked := range ni.policy.BlockedPorts {
		if blocked == port {
			return fmt.Errorf("port %d is blocked by policy", port)
		}
	}

	// If outbound allowed, permit
	if ni.policy.AllowOutbound {
		return nil
	}

	// Check allowed hosts
	hostAllowed := len(ni.policy.AllowedHosts) == 0 // Empty means all allowed
	for _, allowed := range ni.policy.AllowedHosts {
		if allowed == host || allowed == "*" {
			hostAllowed = true
			break
		}
	}
	if !hostAllowed {
		return fmt.Errorf("host '%s' is not in allowed list", host)
	}

	// Check allowed ports
	portAllowed := len(ni.policy.AllowedPorts) == 0 // Empty means all allowed
	for _, allowed := range ni.policy.AllowedPorts {
		if allowed == port {
			portAllowed = true
			break
		}
	}
	if !portAllowed {
		return fmt.Errorf("port %d is not in allowed list", port)
	}

	// Check specific protocol permissions
	if port == 53 && !ni.policy.AllowDNS {
		return fmt.Errorf("DNS access is disabled by policy")
	}
	if port == 80 && !ni.policy.AllowHTTP {
		return fmt.Errorf("HTTP access is disabled by policy")
	}
	if port == 443 && !ni.policy.AllowHTTPS {
		return fmt.Errorf("HTTPS access is disabled by policy")
	}

	return nil
}

// CreateInternalNetwork creates a Docker internal network.
func (ni *NetworkIsolation) CreateInternalNetwork(ctx context.Context, name string) error {
	ni.mu.Lock()
	defer ni.mu.Unlock()

	// Check if network exists
	checkCmd := exec.CommandContext(ctx, "docker", "network", "inspect", name)
	if checkCmd.Run() == nil {
		ni.networkName = name
		return nil
	}

	// Create internal network
	createCmd := exec.CommandContext(ctx, "docker", "network", "create",
		"--internal",
		"--driver", "bridge",
		name,
	)
	output, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create internal network: %w, output: %s", err, string(output))
	}

	ni.networkName = name
	ni.customNetwork = name

	return nil
}

// RemoveNetwork removes a Docker network.
func (ni *NetworkIsolation) RemoveNetwork(ctx context.Context) error {
	ni.mu.Lock()
	defer ni.mu.Unlock()

	if ni.networkName == "" {
		return nil
	}

	cmd := exec.CommandContext(ctx, "docker", "network", "rm", ni.networkName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove network: %w, output: %s", err, string(output))
	}

	ni.networkName = ""
	return nil
}

// GetNetworkStats returns network statistics.
func (ni *NetworkIsolation) GetNetworkStats(ctx context.Context, containerID string) (map[string]interface{}, error) {
	ni.mu.RLock()
	defer ni.mu.RUnlock()

	if containerID == "" {
		return nil, fmt.Errorf("no container ID provided")
	}

	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format",
		`{"net_io":"{{.NetIO}}"}`, containerID)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"raw":          string(output),
		"mode":         string(ni.mode),
		"container_id": containerID,
	}, nil
}

// TestConnectivity tests network connectivity from inside the sandbox.
func (ni *NetworkIsolation) TestConnectivity(ctx context.Context, sandbox *Sandbox, host string, port int) (bool, error) {
	ni.mu.RLock()
	mode := ni.mode
	ni.mu.RUnlock()

	if mode == NetworkModeNone {
		return false, fmt.Errorf("network disabled")
	}

	// Use curl or wget inside container
	result := sandbox.Execute(ctx, "sh", "-c",
		fmt.Sprintf("timeout 5 sh -c 'cat < /dev/null > /dev/tcp/%s/%d' 2>/dev/null && echo 'connected' || echo 'failed'",
			host, port))

	if result.Success && strings.Contains(result.Output, "connected") {
		return true, nil
	}

	return false, nil
}

// resolveHost resolves a hostname to an IP address.
func resolveHost(host string) string {
	// Skip wildcard
	if host == "*" {
		return ""
	}

	// Check if already an IP
	if net.ParseIP(host) != nil {
		return host
	}

	// Resolve DNS
	ips, err := net.LookupHost(host)
	if err != nil || len(ips) == 0 {
		return ""
	}

	return ips[0]
}

// NetworkProfile represents a predefined network configuration.
type NetworkProfile struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Mode        NetworkMode   `json:"mode"`
	Policy      NetworkPolicy `json:"policy"`
}

// Predefined network profiles
var NetworkProfiles = map[string]NetworkProfile{
	"strict": {
		Name:        "strict",
		Description: "No network access - maximum security",
		Mode:        NetworkModeNone,
		Policy:      StrictNetworkPolicy(),
	},
	"secure": {
		Name:        "secure",
		Description: "HTTPS only with DNS",
		Mode:        NetworkModeBridge,
		Policy:      DefaultNetworkPolicy(),
	},
	"development": {
		Name:        "development",
		Description: "Permissive for development",
		Mode:        NetworkModeBridge,
		Policy:      DevelopmentNetworkPolicy(),
	},
	"isolated": {
		Name:        "isolated",
		Description: "Internal network only",
		Mode:        NetworkModeInternal,
		Policy:      StrictNetworkPolicy(),
	},
	"full": {
		Name:        "full",
		Description: "Full network access",
		Mode:        NetworkModeHost,
		Policy:      PermissiveNetworkPolicy(),
	},
}

// GetNetworkProfile returns a predefined network profile.
func GetNetworkProfile(name string) (NetworkProfile, bool) {
	profile, ok := NetworkProfiles[name]
	return profile, ok
}

// ListNetworkProfiles lists all available network profiles.
func ListNetworkProfiles() []string {
	var profiles []string
	for name := range NetworkProfiles {
		profiles = append(profiles, name)
	}
	return profiles
}

// ApplyNetworkProfile applies a predefined profile to the sandbox config.
func ApplyNetworkProfile(config *SandboxConfig, profileName string) error {
	profile, ok := GetNetworkProfile(profileName)
	if !ok {
		return fmt.Errorf("unknown network profile: %s", profileName)
	}

	// Update sandbox config
	config.NetworkEnabled = profile.Mode != NetworkModeNone

	return nil
}

// NetworkIsolationBuilder helps create NetworkIsolation configurations.
type NetworkIsolationBuilder struct {
	config NetworkIsolationConfig
}

// NewNetworkIsolationBuilder creates a new builder.
func NewNetworkIsolationBuilder() *NetworkIsolationBuilder {
	return &NetworkIsolationBuilder{
		config: NetworkIsolationConfig{
			Mode:   NetworkModeNone,
			Policy: DefaultNetworkPolicy(),
		},
	}
}

// WithMode sets the network mode.
func (b *NetworkIsolationBuilder) WithMode(mode NetworkMode) *NetworkIsolationBuilder {
	b.config.Mode = mode
	return b
}

// WithPolicy sets the network policy.
func (b *NetworkIsolationBuilder) WithPolicy(policy NetworkPolicy) *NetworkIsolationBuilder {
	b.config.Policy = policy
	return b
}

// WithProfile applies a predefined profile.
func (b *NetworkIsolationBuilder) WithProfile(profileName string) *NetworkIsolationBuilder {
	if profile, ok := NetworkProfiles[profileName]; ok {
		b.config.Mode = profile.Mode
		b.config.Policy = profile.Policy
	}
	return b
}

// WithPortMapping adds a port mapping.
func (b *NetworkIsolationBuilder) WithPortMapping(hostPort, containerPort int, protocol string) *NetworkIsolationBuilder {
	b.config.PortMappings = append(b.config.PortMappings, PortMapping{
		HostPort:      hostPort,
		ContainerPort: containerPort,
		Protocol:      protocol,
	})
	return b
}

// WithDNS sets DNS servers.
func (b *NetworkIsolationBuilder) WithDNS(servers []string) *NetworkIsolationBuilder {
	if b.config.DNSConfig == nil {
		b.config.DNSConfig = &DNSConfig{}
	}
	b.config.DNSConfig.Servers = servers
	return b
}

// WithCustomNetwork sets a custom Docker network.
func (b *NetworkIsolationBuilder) WithCustomNetwork(network string) *NetworkIsolationBuilder {
	b.config.CustomNetwork = network
	return b
}

// Build creates the NetworkIsolation.
func (b *NetworkIsolationBuilder) Build() *NetworkIsolation {
	return NewNetworkIsolation(b.config)
}

// BuildConfig returns the configuration.
func (b *NetworkIsolationBuilder) BuildConfig() NetworkIsolationConfig {
	return b.config
}

// currentTimeNano returns current time in nanoseconds.
var currentTimeNano = func() int64 {
	return time.Now().UnixNano()
}
