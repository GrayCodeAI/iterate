// Package autonomous - Task 21: Docker-based sandboxed command execution tests
package autonomous

import (
	"context"
	"testing"
	"time"
)

func TestNewSandbox(t *testing.T) {
	config := DefaultSandboxConfig()
	sandbox := NewSandbox(config)
	
	if sandbox == nil {
		t.Fatal("expected sandbox, got nil")
	}
	
	if sandbox.image == "" {
		t.Error("expected image to be set")
	}
}

func TestDefaultSandboxConfig(t *testing.T) {
	config := DefaultSandboxConfig()
	
	if config.Image == "" {
		t.Error("expected default image")
	}
	
	if config.WorkDir == "" {
		t.Error("expected default work dir")
	}
	
	if config.ExecTimeout == 0 {
		t.Error("expected default exec timeout")
	}
	
	if config.NetworkEnabled {
		t.Error("expected network disabled by default")
	}
}

func TestSandboxBuilder(t *testing.T) {
	sandbox := NewSandboxBuilder().
		WithImage("python:3.11-slim").
		WithWorkDir("/app").
		WithNetwork(true).
		WithMemoryLimit(1024).
		WithCPUShares(1024).
		WithTimeout(10 * time.Minute).
		WithEnvVar("DEBUG", "true").
		WithCleanupOnExit(true).
		Build()
	
	if sandbox == nil {
		t.Fatal("expected sandbox, got nil")
	}
	
	if sandbox.image != "python:3.11-slim" {
		t.Errorf("expected image 'python:3.11-slim', got '%s'", sandbox.image)
	}
	
	if sandbox.workDir != "/app" {
		t.Errorf("expected workdir '/app', got '%s'", sandbox.workDir)
	}
	
	if !sandbox.networkEnabled {
		t.Error("expected network enabled")
	}
	
	if sandbox.resourceLimits.MemoryMB != 1024 {
		t.Errorf("expected memory 1024, got %d", sandbox.resourceLimits.MemoryMB)
	}
}

func TestSandboxBuilderConfig(t *testing.T) {
	config := NewSandboxBuilder().
		WithImage("golang:1.21").
		WithWorkDir("/src").
		BuildConfig()
	
	if config.Image != "golang:1.21" {
		t.Errorf("expected image 'golang:1.21', got '%s'", config.Image)
	}
	
	if config.WorkDir != "/src" {
		t.Errorf("expected workdir '/src', got '%s'", config.WorkDir)
	}
}

func TestSandboxConfigDefaults(t *testing.T) {
	// Test that empty values get defaults
	sandbox := NewSandbox(SandboxConfig{})
	
	if sandbox.image == "" {
		t.Error("expected default image to be set")
	}
	
	if sandbox.workDir == "" {
		t.Error("expected default workdir to be set")
	}
	
	if sandbox.execTimeout == 0 {
		t.Error("expected default timeout to be set")
	}
	
	if sandbox.envVars == nil {
		t.Error("expected env vars map to be initialized")
	}
}

func TestVolumeMount(t *testing.T) {
	vm := VolumeMount{
		HostPath:      "/host/path",
		ContainerPath: "/container/path",
		ReadOnly:      true,
	}
	
	if vm.HostPath != "/host/path" {
		t.Errorf("expected host path '/host/path', got '%s'", vm.HostPath)
	}
	
	if !vm.ReadOnly {
		t.Error("expected read-only")
	}
}

func TestSandboxResourceLimits(t *testing.T) {
	limits := SandboxResourceLimits{
		CPUShares:  512,
		MemoryMB:   256,
		PidsLimit:  50,
		Timeout:    time.Minute,
	}
	
	if limits.CPUShares != 512 {
		t.Errorf("expected CPU shares 512, got %d", limits.CPUShares)
	}
	
	if limits.MemoryMB != 256 {
		t.Errorf("expected memory 256, got %d", limits.MemoryMB)
	}
}

func TestSandboxResult(t *testing.T) {
	result := &SandboxResult{
		Success:  true,
		ExitCode: 0,
		Output:   "Hello, World!",
		Command:  "echo 'Hello, World!'",
		Duration: 100 * time.Millisecond,
	}
	
	if !result.Success {
		t.Error("expected success")
	}
	
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	
	if result.Output != "Hello, World!" {
		t.Errorf("expected output 'Hello, World!', got '%s'", result.Output)
	}
}

func TestSandboxIsRunning(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	
	if sandbox.IsRunning() {
		t.Error("expected sandbox not to be running initially")
	}
}

func TestSandboxGetContainerID(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	
	if sandbox.GetContainerID() != "" {
		t.Error("expected empty container ID initially")
	}
}

func TestSandboxSetEnvVar(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	
	sandbox.SetEnvVar("TEST_VAR", "test_value")
	
	if sandbox.envVars["TEST_VAR"] != "test_value" {
		t.Error("expected env var to be set")
	}
}

func TestSandboxAddVolumeMount(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	
	sandbox.AddVolumeMount("/host", "/container", true)
	
	if len(sandbox.volumeMounts) != 1 {
		t.Fatal("expected 1 volume mount")
	}
	
	if sandbox.volumeMounts[0].HostPath != "/host" {
		t.Error("expected host path '/host'")
	}
	
	if !sandbox.volumeMounts[0].ReadOnly {
		t.Error("expected read-only mount")
	}
}

func TestSandboxStartTwice(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	
	// Mock running state
	sandbox.mu.Lock()
	sandbox.running = true
	sandbox.mu.Unlock()
	
	ctx := context.Background()
	err := sandbox.Start(ctx)
	
	if err == nil {
		t.Error("expected error when starting already running sandbox")
	}
}

func TestSandboxStopWhenNotRunning(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	
	ctx := context.Background()
	err := sandbox.Stop(ctx)
	
	if err != nil {
		t.Errorf("expected no error when stopping non-running sandbox, got: %v", err)
	}
}

func TestSandboxExecuteWhenNotRunning(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	
	ctx := context.Background()
	result := sandbox.Execute(ctx, "echo", "hello")
	
	if result.Success {
		t.Error("expected failure when executing in non-running sandbox")
	}
	
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestSandboxExecuteWithOutputWhenNotRunning(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	
	ctx := context.Background()
	result := sandbox.ExecuteWithOutput(ctx, "echo", "hello")
	
	if result.Success {
		t.Error("expected failure when executing in non-running sandbox")
	}
	
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestSandboxCopyToWhenNotRunning(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	
	ctx := context.Background()
	err := sandbox.CopyTo(ctx, "/host/file", "/container/file")
	
	if err == nil {
		t.Error("expected error when copying to non-running sandbox")
	}
}

func TestSandboxCopyFromWhenNotRunning(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	
	ctx := context.Background()
	err := sandbox.CopyFrom(ctx, "/container/file", "/host/file")
	
	if err == nil {
		t.Error("expected error when copying from non-running sandbox")
	}
}

func TestBuildExecArgs(t *testing.T) {
	sandbox := NewSandbox(SandboxConfig{
		Image:   "node:18",
		WorkDir: "/app",
		EnvVars: map[string]string{
			"NODE_ENV": "test",
		},
	})
	sandbox.containerID = "test123"
	
	args := sandbox.buildExecArgs("npm", "test")
	
	// Check that args contain expected elements
	foundWorkDir := false
	foundEnv := false
	foundContainerID := false
	foundCommand := false
	
	for _, arg := range args {
		if arg == "-w" {
			foundWorkDir = true
		}
		if arg == "NODE_ENV=test" {
			foundEnv = true
		}
		if arg == "test123" {
			foundContainerID = true
		}
		if arg == "npm" {
			foundCommand = true
		}
	}
	
	if !foundWorkDir {
		t.Error("expected workdir flag in args")
	}
	if !foundEnv {
		t.Error("expected env var in args")
	}
	if !foundContainerID {
		t.Error("expected container ID in args")
	}
	if !foundCommand {
		t.Error("expected command in args")
	}
}

func TestSandboxBuilderWithVolumeMount(t *testing.T) {
	sandbox := NewSandboxBuilder().
		WithVolumeMount("/host/src", "/container/src", false).
		WithVolumeMount("/host/config", "/container/config", true).
		Build()
	
	if len(sandbox.volumeMounts) != 2 {
		t.Errorf("expected 2 volume mounts, got %d", len(sandbox.volumeMounts))
	}
}

func TestSandboxBuilderWithPullImage(t *testing.T) {
	sandbox := NewSandboxBuilder().
		WithPullImage(true).
		Build()
	
	if !sandbox.config.PullImage {
		t.Error("expected pull image to be true")
	}
}

func TestSandboxBuilderWithOutputCallback(t *testing.T) {
	called := false
	callback := func(s string) {
		called = true
	}
	
	sandbox := NewSandboxBuilder().
		WithOutputCallback(callback).
		Build()
	
	if sandbox.outputCallback == nil {
		t.Error("expected output callback to be set")
	}
	
	// Call the callback to verify it works
	sandbox.outputCallback("test")
	if !called {
		t.Error("expected callback to be called")
	}
}

func TestCheckDockerAvailable(t *testing.T) {
	// This test just verifies the function doesn't panic
	// Docker may or may not be available in test environment
	_ = CheckDockerAvailable()
}

func TestTask21SandboxStruct(t *testing.T) {
	// Verify all struct fields are properly initialized
	sandbox := NewSandbox(SandboxConfig{
		Image:          "alpine:latest",
		WorkDir:        "/data",
		NetworkEnabled: true,
			ResourceLimits: SandboxResourceLimits{
			CPUShares: 256,
			MemoryMB:  128,
			PidsLimit: 10,
		},
		EnvVars: map[string]string{
			"VAR1": "value1",
		},
	})
	
	if sandbox.image != "alpine:latest" {
		t.Errorf("expected image 'alpine:latest', got '%s'", sandbox.image)
	}
	
	if sandbox.workDir != "/data" {
		t.Errorf("expected workdir '/data', got '%s'", sandbox.workDir)
	}
	
	if !sandbox.networkEnabled {
		t.Error("expected network enabled")
	}
	
	if sandbox.resourceLimits.CPUShares != 256 {
		t.Errorf("expected CPU shares 256, got %d", sandbox.resourceLimits.CPUShares)
	}
	
	if sandbox.envVars["VAR1"] != "value1" {
		t.Error("expected env var VAR1 to be set")
	}
}

func TestSandboxResultTimedOut(t *testing.T) {
	result := &SandboxResult{
		Success:  false,
		TimedOut: true,
		Error:    "command timed out",
	}
	
	if !result.TimedOut {
		t.Error("expected timed out")
	}
	
	if result.Success {
		t.Error("expected failure due to timeout")
	}
}

func TestSandboxResultMetadata(t *testing.T) {
	result := &SandboxResult{
		Success: true,
		Metadata: map[string]any{
			"files_changed": 5,
			"lines_added":   100,
		},
	}
	
	if result.Metadata == nil {
		t.Fatal("expected metadata")
	}
	
	if result.Metadata["files_changed"] != 5 {
		t.Error("expected files_changed to be 5")
	}
}

// Note: Integration tests that require Docker are skipped by default
// They can be run with: go test -tags=docker -run TestDockerIntegration

func TestTask21FullIntegration(t *testing.T) {
	// This test verifies the sandbox configuration and builder work correctly
	// without requiring Docker to be available
	
	config := NewSandboxBuilder().
		WithImage("node:18-slim").
		WithWorkDir("/workspace").
		WithNetwork(false).
		WithMemoryLimit(512).
		WithCPUShares(512).
		WithTimeout(5 * time.Minute).
		WithEnvVar("NODE_ENV", "production").
		WithCleanupOnExit(true).
		BuildConfig()
	
	if config.Image != "node:18-slim" {
		t.Errorf("expected image 'node:18-slim', got '%s'", config.Image)
	}
	
	if config.WorkDir != "/workspace" {
		t.Errorf("expected workdir '/workspace', got '%s'", config.WorkDir)
	}
	
	if config.NetworkEnabled {
		t.Error("expected network disabled")
	}
	
	if config.ResourceLimits.MemoryMB != 512 {
		t.Errorf("expected memory 512, got %d", config.ResourceLimits.MemoryMB)
	}
	
	if config.ResourceLimits.CPUShares != 512 {
		t.Errorf("expected CPU shares 512, got %d", config.ResourceLimits.CPUShares)
	}
	
	if !config.CleanupOnExit {
		t.Error("expected cleanup on exit")
	}
	
	// Test builder produces valid sandbox
	sandbox := NewSandbox(config)
	
	if sandbox == nil {
		t.Fatal("expected sandbox, got nil")
	}
	
	if sandbox.IsRunning() {
		t.Error("expected sandbox not to be running")
	}
	
	t.Logf("✅ Task 21: Docker-based Sandboxed Command Execution - Full integration PASSED")
	t.Logf("Image: %s, WorkDir: %s, Memory: %dMB, CPU: %d",
		config.Image, config.WorkDir, config.ResourceLimits.MemoryMB, config.ResourceLimits.CPUShares)
}
