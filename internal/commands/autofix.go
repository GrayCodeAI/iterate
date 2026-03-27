package commands

import (
	"context"
	"fmt"
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
		fmt.Printf("%s[attempt %d/%d]%s running %s %v\n",
			ColorDim, attempt, maxAttempts, ColorReset, testCmd, testArgs)

		cmd := exec.Command(testCmd, testArgs...)
		cmd.Dir = ctx.RepoPath
		output, err := cmd.CombinedOutput()

		if err == nil {
			PrintSuccess("tests passed on attempt %d/%d", attempt, maxAttempts)
			return Result{Handled: true}
		}

		// Tests failed — print failure output
		fmt.Printf("%s── Test failures ───────────────────%s\n", ColorDim, ColorReset)
		fmt.Println(string(output))
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)

		if attempt == maxAttempts {
			break
		}

		// Ask agent to fix
		prompt := fmt.Sprintf("The following tests are failing. Fix them:\n\n%s", string(output))
		ctx.REPL.StreamAndPrint(context.Background(), ctx.Agent, prompt, ctx.RepoPath)
	}

	fmt.Printf("%sautofix gave up after %d attempts%s\n\n", ColorRed, maxAttempts, ColorReset)
	return Result{Handled: true}
}
