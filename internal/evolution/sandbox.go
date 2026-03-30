package evolution

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type SandboxConfig struct {
	AllowedCommands []string
	BlockedPatterns []string
	AllowedPaths    []string
	Timeout         time.Duration
	MaxOutputSize   int
	EnvWhitelist    []string
}

var DefaultSandboxConfig = SandboxConfig{
	AllowedCommands: []string{
		"go", "git", "ls", "cat", "grep", "find", "echo", "pwd",
		"mkdir", "rm", "cp", "mv", "chmod", "chown",
		"npm", "yarn", "pnpm", "pip", "python", "python3",
		"cargo", "rustc", "make", "cmake",
		"docker", "docker-compose",
		"curl", "wget",
	},
	BlockedPatterns: []string{
		`rm\s+-rf\s+/(?!\.)`,
		`dd\s+of=`,
		`mkfs`,
		`curl.*\|.*sh`,
		`wget.*\|.*sh`,
		`:(){ :|:& };:`,
		`fork\(\)`,
		`chmod\s+777`,
		`>\s*/dev/sd`,
		`curl\s+.+\s+-o\s+/etc`,
		`sudo\s+rm`,
		`git\s+push\s+--force`,
		`git\s+push\s+-f`,
	},
	AllowedPaths: []string{
		"/home/runner/work",
		"/tmp",
	},
	Timeout:       5 * time.Minute,
	MaxOutputSize: 1024 * 1024,
	EnvWhitelist: []string{
		"HOME", "PATH", "USER", "GITHUB_TOKEN", "GITHUB_REPOSITORY",
		"GO111MODULE", "GOPROXY", "GOSUMDB",
	},
}

type Sandbox struct {
	config SandboxConfig
	dir    string
}

func NewSandbox(dir string) *Sandbox {
	return &Sandbox{
		config: DefaultSandboxConfig,
		dir:    dir,
	}
}

func (s *Sandbox) Execute(ctx context.Context, cmd string) (string, error) {
	if err := s.validateCommand(cmd); err != nil {
		return "", fmt.Errorf("command blocked: %w", err)
	}

	if err := s.validatePath(cmd); err != nil {
		return "", fmt.Errorf("path blocked: %w", err)
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	bin := parts[0]
	if !s.isAllowedCommand(bin) {
		return "", fmt.Errorf("command not allowed: %s", bin)
	}

	timeout := s.config.Timeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < timeout {
			timeout = remaining
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	execCmd := exec.CommandContext(ctx, bin, parts[1:]...)
	execCmd.Dir = s.dir

	execCmd.Env = s.filterEnv(os.Environ())

	output, err := execCmd.CombinedOutput()

	if len(output) > s.config.MaxOutputSize {
		output = output[:s.config.MaxOutputSize]
		output = append(output, []byte("\n\n[Output truncated due to size]")...)
	}

	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

func (s *Sandbox) validateCommand(cmd string) error {
	for _, pattern := range s.config.BlockedPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(cmd) {
			return fmt.Errorf("blocked pattern: %s", pattern)
		}
	}

	return nil
}

func (s *Sandbox) validatePath(cmd string) error {
	pathPattern := regexp.MustCompile(`(/[a-zA-Z0-9._-]+)+`)
	paths := pathPattern.FindAllString(cmd, -1)

	for _, path := range paths {
		if strings.HasPrefix(path, "/etc") ||
			strings.HasPrefix(path, "/usr/bin") && !strings.HasPrefix(path, "/usr/bin/") ||
			strings.HasPrefix(path, "/usr/local/bin") ||
			strings.HasPrefix(path, "/sbin") ||
			strings.HasPrefix(path, "/bin") && !strings.HasPrefix(path, "/bin/") ||
			strings.HasPrefix(path, "/var") ||
			strings.HasPrefix(path, "/root") {
			return fmt.Errorf("blocked path: %s", path)
		}
	}

	return nil
}

func (s *Sandbox) isAllowedCommand(cmd string) bool {
	for _, allowed := range s.config.AllowedCommands {
		if cmd == allowed {
			return true
		}
		if strings.HasSuffix("/"+cmd, "/"+allowed) {
			return true
		}
	}
	return false
}

func (s *Sandbox) filterEnv(env []string) []string {
	var filtered []string
	for _, e := range env {
		key := strings.Split(e, "=")[0]
		for _, allowed := range s.config.EnvWhitelist {
			if key == allowed {
				filtered = append(filtered, e)
				break
			}
		}
	}
	return filtered
}

func (s *Sandbox) ExecuteWithRetry(ctx context.Context, cmd string, maxRetries int) (string, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		output, err := s.Execute(ctx, cmd)
		if err == nil {
			return output, nil
		}
		lastErr = err

		if strings.Contains(err.Error(), "blocked") {
			return "", err
		}

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(i+1) * time.Second):
			}
		}
	}
	return "", lastErr
}

type DryRunSandbox struct {
	Sandbox
	Commands []string
}

func (d *DryRunSandbox) Execute(ctx context.Context, cmd string) (string, error) {
	d.Commands = append(d.Commands, cmd)
	return fmt.Sprintf("[DRY RUN] Would execute: %s", cmd), nil
}

type VerboseSandbox struct {
	Sandbox
	Log func(string)
}

func (v *VerboseSandbox) Execute(ctx context.Context, cmd string) (string, error) {
	v.Log(fmt.Sprintf("[SANDBOX] Executing: %s", cmd))
	output, err := v.Sandbox.Execute(ctx, cmd)
	if err != nil {
		v.Log(fmt.Sprintf("[SANDBOX] Error: %v", err))
	}
	return output, err
}
