// Package autonomous - Task 21: Docker-based sandboxed command execution
package autonomous

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Sandbox provides isolated execution environment for commands.
type Sandbox struct {
	mu              sync.RWMutex
	config          SandboxConfig
	containerID     string
	image           string
	running         bool
	workDir         string
	volumeMounts    []VolumeMount
	envVars         map[string]string
	networkEnabled  bool
	resourceLimits  SandboxResourceLimits
	execTimeout     time.Duration
	outputCallback  func(string)
}

// SandboxConfig configures the sandbox environment.
type SandboxConfig struct {
	Image           string            `json:"image"`
	WorkDir         string            `json:"work_dir"`
	VolumeMounts    []VolumeMount     `json:"volume_mounts"`
	EnvVars         map[string]string `json:"env_vars"`
	NetworkEnabled  bool              `json:"network_enabled"`
	ResourceLimits  SandboxResourceLimits `json:"resource_limits"`
	ExecTimeout     time.Duration     `json:"exec_timeout"`
	OutputCallback  func(string)      `json:"-"`
	CleanupOnExit   bool              `json:"cleanup_on_exit"`
	PullImage       bool              `json:"pull_image"`
}

// VolumeMount represents a volume mount in the container.
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// SandboxResourceLimits defines resource constraints for Docker containers.
// This is distinct from ResourceLimits which controls autonomous operation limits.
type SandboxResourceLimits struct {
	CPUShares    int64         `json:"cpu_shares"`
	MemoryMB     int64         `json:"memory_mb"`
	MemorySwapMB int64         `json:"memory_swap_mb"`
	CPUPercent   int           `json:"cpu_percent"`
	PidsLimit    int64         `json:"pids_limit"`
	Timeout      time.Duration `json:"timeout"`
}

// SandboxResult contains the result of a sandboxed execution.
type SandboxResult struct {
	Success   bool              `json:"success"`
	ExitCode  int               `json:"exit_code"`
	Output    string            `json:"output"`
	Error     string            `json:"error,omitempty"`
	Duration  time.Duration     `json:"duration"`
	Command   string            `json:"command"`
	TimedOut  bool              `json:"timed_out"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
}

// DefaultSandboxConfig returns default sandbox configuration.
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		Image:          "node:18-slim",
		WorkDir:        "/workspace",
		VolumeMounts:   []VolumeMount{},
		EnvVars:        make(map[string]string),
		NetworkEnabled: false,
		ResourceLimits: SandboxResourceLimits{
			CPUShares:  512,
			MemoryMB:   512,
			PidsLimit:  100,
			Timeout:    5 * time.Minute,
		},
		ExecTimeout:   5 * time.Minute,
		CleanupOnExit: true,
		PullImage:     false,
	}
}

// NewSandbox creates a new sandbox instance.
func NewSandbox(config SandboxConfig) *Sandbox {
	if config.Image == "" {
		config.Image = DefaultSandboxConfig().Image
	}
	if config.WorkDir == "" {
		config.WorkDir = DefaultSandboxConfig().WorkDir
	}
	if config.ExecTimeout == 0 {
		config.ExecTimeout = DefaultSandboxConfig().ExecTimeout
	}
	if config.EnvVars == nil {
		config.EnvVars = make(map[string]string)
	}
	
	return &Sandbox{
		config:         config,
		image:          config.Image,
		workDir:        config.WorkDir,
		volumeMounts:   config.VolumeMounts,
		envVars:        config.EnvVars,
		networkEnabled: config.NetworkEnabled,
		resourceLimits: config.ResourceLimits,
		execTimeout:    config.ExecTimeout,
		outputCallback: config.OutputCallback,
	}
}

// Start creates and starts the sandbox container.
func (s *Sandbox) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.running {
		return fmt.Errorf("sandbox already running")
	}
	
	// Check if Docker is available
	if !s.isDockerAvailable() {
		return fmt.Errorf("docker is not available")
	}
	
	// Pull image if needed
	if s.config.PullImage {
		if err := s.pullImage(ctx); err != nil {
			return fmt.Errorf("failed to pull image: %w", err)
		}
	}
	
	// Create container
	containerID, err := s.createContainer(ctx)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	
	s.containerID = containerID
	
	// Start container
	if err := s.startContainer(ctx); err != nil {
		s.cleanupContainer(ctx)
		return fmt.Errorf("failed to start container: %w", err)
	}
	
	s.running = true
	return nil
}

// Stop stops and removes the sandbox container.
func (s *Sandbox) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.running || s.containerID == "" {
		return nil
	}
	
	// Stop container
	stopCmd := exec.CommandContext(ctx, "docker", "stop", s.containerID)
	stopCmd.Run() // Ignore error, container may already be stopped
	
	// Remove container if cleanup enabled
	if s.config.CleanupOnExit {
		s.cleanupContainer(ctx)
	}
	
	s.running = false
	s.containerID = ""
	return nil
}

// Execute runs a command inside the sandbox.
func (s *Sandbox) Execute(ctx context.Context, command string, args ...string) *SandboxResult {
	s.mu.RLock()
	containerID := s.containerID
	running := s.running
	s.mu.RUnlock()
	
	result := &SandboxResult{
		Command:  strings.Join(append([]string{command}, args...), " "),
		Metadata: make(map[string]any),
	}
	
	if !running || containerID == "" {
		result.Error = "sandbox not running"
		return result
	}
	
	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()
	
	// Build docker exec command
	execArgs := s.buildExecArgs(command, args...)
	
	// Set timeout
	timeout := s.execTimeout
	if s.resourceLimits.Timeout > 0 && s.resourceLimits.Timeout < timeout {
		timeout = s.resourceLimits.Timeout
	}
	
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	cmd := exec.CommandContext(execCtx, "docker", execArgs...)
	
	// Capture output
	output, err := cmd.CombinedOutput()
	result.Output = string(output)
	
	if execCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.Error = "command timed out"
		result.ExitCode = -1
		return result
	}
	
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err.Error()
			result.ExitCode = -1
		}
		return result
	}
	
	result.Success = true
	result.ExitCode = 0
	return result
}

// ExecuteWithOutput runs a command and streams output.
func (s *Sandbox) ExecuteWithOutput(ctx context.Context, command string, args ...string) *SandboxResult {
	s.mu.RLock()
	containerID := s.containerID
	running := s.running
	s.mu.RUnlock()
	
	result := &SandboxResult{
		Command:  strings.Join(append([]string{command}, args...), " "),
		Metadata: make(map[string]any),
	}
	
	if !running || containerID == "" {
		result.Error = "sandbox not running"
		return result
	}
	
	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()
	
	execArgs := s.buildExecArgs(command, args...)
	
	timeout := s.execTimeout
	if s.resourceLimits.Timeout > 0 && s.resourceLimits.Timeout < timeout {
		timeout = s.resourceLimits.Timeout
	}
	
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	cmd := exec.CommandContext(execCtx, "docker", execArgs...)
	
	// Get pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	
	if err := cmd.Start(); err != nil {
		result.Error = err.Error()
		return result
	}
	
	// Stream output
	var outputBuilder strings.Builder
	outputChan := make(chan string, 100)
	
	go s.streamOutput(stdout, outputChan)
	go s.streamOutput(stderr, outputChan)
	
	go func() {
		for line := range outputChan {
			outputBuilder.WriteString(line)
			if s.outputCallback != nil {
				s.outputCallback(line)
			}
		}
	}()
	
	err = cmd.Wait()
	result.Output = outputBuilder.String()
	
	if execCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.Error = "command timed out"
		result.ExitCode = -1
		return result
	}
	
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err.Error()
			result.ExitCode = -1
		}
		return result
	}
	
	result.Success = true
	result.ExitCode = 0
	return result
}

// CopyTo copies files from host to the sandbox.
func (s *Sandbox) CopyTo(ctx context.Context, hostPath, containerPath string) error {
	s.mu.RLock()
	containerID := s.containerID
	s.mu.RUnlock()
	
	if containerID == "" {
		return fmt.Errorf("sandbox not running")
	}
	
	// Use docker cp
	cmd := exec.CommandContext(ctx, "docker", "cp", hostPath, containerID+":"+containerPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cp failed: %w, output: %s", err, string(output))
	}
	
	return nil
}

// CopyFrom copies files from the sandbox to host.
func (s *Sandbox) CopyFrom(ctx context.Context, containerPath, hostPath string) error {
	s.mu.RLock()
	containerID := s.containerID
	s.mu.RUnlock()
	
	if containerID == "" {
		return fmt.Errorf("sandbox not running")
	}
	
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(hostPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}
	
	// Use docker cp
	cmd := exec.CommandContext(ctx, "docker", "cp", containerID+":"+containerPath, hostPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cp failed: %w, output: %s", err, string(output))
	}
	
	return nil
}

// IsRunning returns whether the sandbox is currently running.
func (s *Sandbox) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetContainerID returns the container ID.
func (s *Sandbox) GetContainerID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.containerID
}

// SetEnvVar sets an environment variable in the sandbox.
func (s *Sandbox) SetEnvVar(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.envVars[key] = value
}

// AddVolumeMount adds a volume mount to the sandbox.
func (s *Sandbox) AddVolumeMount(hostPath, containerPath string, readOnly bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.volumeMounts = append(s.volumeMounts, VolumeMount{
		HostPath:      hostPath,
		ContainerPath: containerPath,
		ReadOnly:      readOnly,
	})
}

// buildExecArgs builds arguments for docker exec.
func (s *Sandbox) buildExecArgs(command string, args ...string) []string {
	execArgs := []string{"exec"}
	
	// Add environment variables
	for k, v := range s.envVars {
		execArgs = append(execArgs, "-e", k+"="+v)
	}
	
	// Set working directory
	execArgs = append(execArgs, "-w", s.workDir)
	
	// Add container ID
	execArgs = append(execArgs, s.containerID)
	
	// Add command and args
	execArgs = append(execArgs, command)
	execArgs = append(execArgs, args...)
	
	return execArgs
}

// isDockerAvailable checks if Docker is available.
func (s *Sandbox) isDockerAvailable() bool {
	cmd := exec.Command("docker", "--version")
	return cmd.Run() == nil
}

// pullImage pulls the Docker image.
func (s *Sandbox) pullImage(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", s.image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker pull failed: %w, output: %s", err, string(output))
	}
	return nil
}

// createContainer creates a new container.
func (s *Sandbox) createContainer(ctx context.Context) (string, error) {
	args := []string{"create"}
	
	// Add volume mounts
	for _, vm := range s.volumeMounts {
		mount := vm.HostPath + ":" + vm.ContainerPath
		if vm.ReadOnly {
			mount += ":ro"
		}
		args = append(args, "-v", mount)
	}
	
	// Set working directory
	args = append(args, "-w", s.workDir)
	
	// Network settings
	if !s.networkEnabled {
		args = append(args, "--network", "none")
	}
	
	// Resource limits
	if s.resourceLimits.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", s.resourceLimits.MemoryMB))
	}
	if s.resourceLimits.CPUShares > 0 {
		args = append(args, "--cpu-shares", fmt.Sprintf("%d", s.resourceLimits.CPUShares))
	}
	if s.resourceLimits.PidsLimit > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", s.resourceLimits.PidsLimit))
	}
	
	// Add environment variables
	for k, v := range s.envVars {
		args = append(args, "-e", k+"="+v)
	}
	
	// Image
	args = append(args, s.image)
	
	// Keep container running
	args = append(args, "tail", "-f", "/dev/null")
	
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker create failed: %w", err)
	}
	
	return strings.TrimSpace(string(output)), nil
}

// startContainer starts the container.
func (s *Sandbox) startContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "start", s.containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker start failed: %w, output: %s", err, string(output))
	}
	return nil
}

// cleanupContainer removes the container.
func (s *Sandbox) cleanupContainer(ctx context.Context) {
	if s.containerID == "" {
		return
	}
	
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", s.containerID)
	cmd.Run() // Ignore errors
}

// streamOutput streams output from a reader.
func (s *Sandbox) streamOutput(reader io.Reader, outputChan chan<- string) {
	defer close(outputChan)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		outputChan <- scanner.Text() + "\n"
	}
}

// SandboxBuilder helps create Sandbox configurations.
type SandboxBuilder struct {
	config SandboxConfig
}

// NewSandboxBuilder creates a new sandbox builder.
func NewSandboxBuilder() *SandboxBuilder {
	return &SandboxBuilder{
		config: DefaultSandboxConfig(),
	}
}

// WithImage sets the Docker image.
func (b *SandboxBuilder) WithImage(image string) *SandboxBuilder {
	b.config.Image = image
	return b
}

// WithWorkDir sets the working directory.
func (b *SandboxBuilder) WithWorkDir(dir string) *SandboxBuilder {
	b.config.WorkDir = dir
	return b
}

// WithVolumeMount adds a volume mount.
func (b *SandboxBuilder) WithVolumeMount(hostPath, containerPath string, readOnly bool) *SandboxBuilder {
	b.config.VolumeMounts = append(b.config.VolumeMounts, VolumeMount{
		HostPath:      hostPath,
		ContainerPath: containerPath,
		ReadOnly:      readOnly,
	})
	return b
}

// WithEnvVar sets an environment variable.
func (b *SandboxBuilder) WithEnvVar(key, value string) *SandboxBuilder {
	if b.config.EnvVars == nil {
		b.config.EnvVars = make(map[string]string)
	}
	b.config.EnvVars[key] = value
	return b
}

// WithNetwork enables or disables network.
func (b *SandboxBuilder) WithNetwork(enabled bool) *SandboxBuilder {
	b.config.NetworkEnabled = enabled
	return b
}

// WithMemoryLimit sets the memory limit in MB.
func (b *SandboxBuilder) WithMemoryLimit(mb int64) *SandboxBuilder {
	b.config.ResourceLimits.MemoryMB = mb
	return b
}

// WithCPUShares sets the CPU shares.
func (b *SandboxBuilder) WithCPUShares(shares int64) *SandboxBuilder {
	b.config.ResourceLimits.CPUShares = shares
	return b
}

// WithTimeout sets the execution timeout.
func (b *SandboxBuilder) WithTimeout(timeout time.Duration) *SandboxBuilder {
	b.config.ExecTimeout = timeout
	b.config.ResourceLimits.Timeout = timeout
	return b
}

// WithOutputCallback sets the output callback.
func (b *SandboxBuilder) WithOutputCallback(callback func(string)) *SandboxBuilder {
	b.config.OutputCallback = callback
	return b
}

// WithCleanupOnExit sets whether to cleanup on exit.
func (b *SandboxBuilder) WithCleanupOnExit(cleanup bool) *SandboxBuilder {
	b.config.CleanupOnExit = cleanup
	return b
}

// WithPullImage sets whether to pull the image.
func (b *SandboxBuilder) WithPullImage(pull bool) *SandboxBuilder {
	b.config.PullImage = pull
	return b
}

// Build creates the Sandbox.
func (b *SandboxBuilder) Build() *Sandbox {
	return NewSandbox(b.config)
}

// BuildConfig returns the configuration.
func (b *SandboxBuilder) BuildConfig() SandboxConfig {
	return b.config
}

// CheckDockerAvailable checks if Docker is available on the system.
func CheckDockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// ListImages lists available Docker images.
func ListImages(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "images", "--format", "{{.Repository}}:{{.Tag}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	var images []string
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			images = append(images, line)
		}
	}
	return images, nil
}

// GetContainerStats gets stats for a container.
func (s *Sandbox) GetContainerStats(ctx context.Context) (map[string]any, error) {
	s.mu.RLock()
	containerID := s.containerID
	s.mu.RUnlock()
	
	if containerID == "" {
		return nil, fmt.Errorf("no container running")
	}
	
	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", 
		`{"cpu":"{{.CPUPerc}}","memory":"{{.MemUsage}}","net_io":"{{.NetIO}}"}`, containerID)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	return map[string]any{
		"raw":    string(output),
		"container_id": containerID,
	}, nil
}
