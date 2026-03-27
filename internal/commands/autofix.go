package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
)

// RegisterAutofixCommands adds the /autofix command.
func RegisterAutofixCommands(r *Registry) {
	r.Register(Command{
		Name:        "/autofix",
		Aliases:     []string{"/af"},
		Description: "run tests, ask agent to fix failures, repeat up to N times (default 3)",
		Category:    "dev",
		Handler:     cmdAutofix,
	})
}

// detectTestCmd returns the test command and args appropriate for the repo.
func detectTestCmd(repoPath string) (string, []string) {
	// Check for go.mod (most common for this project)
	if _, err := exec.LookPath("go"); err == nil {
		gomod := exec.Command("go", "env", "GOMOD")
		gomod.Dir = repoPath
		if out, err := gomod.Output(); err == nil && len(out) > 0 {
			return "go", []string{"test", "./..."}
		}
	}
	if _, err := exec.LookPath("npm"); err == nil {
		pkg := exec.Command("npm", "run", "test", "--if-present")
		pkg.Dir = repoPath
		return "npm", []string{"test"}
	}
	if _, err := exec.LookPath("pytest"); err == nil {
		return "pytest", []string{}
	}
	// Default fallback
	return "go", []string{"test", "./..."}
}

func cmdAutofix(ctx Context) Result {
	maxAttempts := 3
	if ctx.HasArg(1) {
		if v, err := strconv.Atoi(ctx.Arg(1)); err == nil && v > 0 {
			maxAttempts = v
		}
	}

	if ctx.REPL.StreamAndPrint == nil {
		PrintError("agent not available for autofix")
		return Result{Handled: true}
	}

	testCmd, testArgs := detectTestCmd(ctx.RepoPath)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		fmt.Printf("%s[attempt %d/%d]%s  %s %s\n",
			ColorDim, attempt, maxAttempts, ColorReset, testCmd, joinArgs(testArgs))

		// Run with live output — capture into buf AND stream to terminal simultaneously.
		var buf bytes.Buffer
		cmd := exec.Command(testCmd, testArgs...)
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &buf)

		err := cmd.Run()

		if err == nil {
			PrintSuccess("all tests pass (%d/%d)", attempt, maxAttempts)
			return Result{Handled: true}
		}

		fmt.Println() // blank line after test output
		if attempt == maxAttempts {
			break
		}

		// Truncate failure output sent to agent (keep last 200 lines — most relevant).
		failureOutput := tailLines(buf.String(), 200)
		prompt := fmt.Sprintf(
			"Tests failed (attempt %d/%d). Fix the failures shown below, then ensure all tests pass.\n\n```\n%s\n```",
			attempt, maxAttempts, failureOutput)
		ctx.REPL.StreamAndPrint(context.Background(), ctx.Agent, prompt, ctx.RepoPath)
	}

	fmt.Printf("%sautofix gave up after %d attempts%s\n\n", ColorRed, maxAttempts, ColorReset)
	return Result{Handled: true}
}

func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}

// tailLines returns the last n lines of s.
func tailLines(s string, n int) string {
	lines := splitLines(s)
	if len(lines) <= n {
		return s
	}
	dropped := len(lines) - n
	return fmt.Sprintf("[... %d lines omitted ...]\n", dropped) + joinLines(lines[dropped:])
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	var b bytes.Buffer
	for _, l := range lines {
		b.WriteString(l)
	}
	return b.String()
}
