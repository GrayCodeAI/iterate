package commands

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// RegisterDevCommands adds development commands.
func RegisterDevCommands(r *Registry) {
	registerDevBuildCommands(r)
	registerDevRunCommands(r)
}

func registerDevBuildCommands(r *Registry) {
	r.Register(Command{Name: "/test", Description: "run tests (auto-detect framework)", Category: "dev", Handler: cmdTest})
	r.Register(Command{Name: "/build", Description: "build the project", Category: "dev", Handler: cmdBuild})
	r.Register(Command{Name: "/lint", Description: "run linter", Category: "dev", Handler: cmdLint})
	r.Register(Command{Name: "/format", Aliases: []string{"/fmt"}, Description: "format code", Category: "dev", Handler: cmdFormat})
	r.Register(Command{Name: "/lint-fix", Description: "go vet + staticcheck", Category: "dev", Handler: cmdLintFix})
	r.Register(Command{Name: "/verify", Description: "run verification checks", Category: "dev", Handler: cmdVerify})
	r.Register(Command{Name: "/benchmark", Description: "run Go benchmarks", Category: "dev", Handler: cmdBenchmark})
	r.Register(Command{Name: "/doctor", Aliases: []string{"/health"}, Description: "run health checks", Category: "dev", Handler: cmdDoctor})
}

func registerDevRunCommands(r *Registry) {
	r.Register(Command{Name: "/run", Description: "run a shell command", Category: "dev", Handler: cmdRun})
	r.Register(Command{Name: "/watch", Description: "auto-run tests on file changes", Category: "dev", Handler: cmdWatch})
	r.Register(Command{Name: "/test-file", Description: "go test specific package", Category: "dev", Handler: cmdTestFile})
	r.Register(Command{Name: "/test-gen", Description: "generate tests for a file", Category: "dev", Handler: cmdTestGen})
	r.Register(Command{Name: "/refactor", Description: "refactor code with AI", Category: "dev", Handler: cmdRefactor})
	r.Register(Command{Name: "/fix", Description: "auto-fix build errors", Category: "dev", Handler: cmdFix})
	r.Register(Command{Name: "/explain-error", Description: "explain an error message", Category: "dev", Handler: cmdExplainError})
	r.Register(Command{Name: "/optimize", Description: "analyze and optimize performance", Category: "dev", Handler: cmdOptimize})
	r.Register(Command{Name: "/security", Description: "security audit", Category: "dev", Handler: cmdSecurity})
	r.Register(Command{Name: "/plan", Description: "plan then execute a task", Category: "dev", Handler: cmdPlan})
	r.Register(Command{Name: "/mock", Description: "generate mock implementations", Category: "dev", Handler: cmdMock})
}

func cmdTest(ctx Context) Result {
	// Auto-detect test framework
	var cmd *exec.Cmd
	repo := ctx.RepoPath

	// Check for go.mod
	if _, err := exec.LookPath("go"); err == nil {
		cmd = exec.Command("go", "test", "./...")
		cmd.Dir = repo
	} else if _, err := exec.LookPath("npm"); err == nil {
		cmd = exec.Command("npm", "test")
		cmd.Dir = repo
	} else if _, err := exec.LookPath("cargo"); err == nil {
		cmd = exec.Command("cargo", "test")
		cmd.Dir = repo
	} else if _, err := exec.LookPath("pytest"); err == nil {
		cmd = exec.Command("pytest")
		cmd.Dir = repo
	} else {
		PrintError("No test framework detected")
		return Result{Handled: true}
	}

	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		PrintError("Tests failed: %s", err)
	}
	return Result{Handled: true}
}

func cmdBuild(ctx Context) Result {
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		PrintError("Build failed: %s", err)
	} else {
		PrintSuccess("Build succeeded")
	}
	return Result{Handled: true}
}

func cmdLint(ctx Context) Result {
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		PrintError("Lint failed: %s", err)
	} else {
		PrintSuccess("No lint issues")
	}
	return Result{Handled: true}
}

func cmdFormat(ctx Context) Result {
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		PrintError("Format failed: %s", err)
	} else {
		PrintSuccess("Formatted")
	}
	return Result{Handled: true}
}

func cmdRun(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /run <command>")
		return Result{Handled: true}
	}

	cmdStr := ctx.Args()

	// Safe-mode gate: require confirmation for arbitrary shell execution.
	if ctx.SafeMode != nil && *ctx.SafeMode {
		fmt.Printf("\033[33mSafe mode active.\033[0m Run command: %q? [y/N] ", cmdStr)
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Aborted.")
			return Result{Handled: true}
		}
	}

	// Deny-tools gate: reject if "bash" is in the denied tools list.
	if ctx.State.IsDenied != nil && ctx.State.IsDenied("bash") {
		PrintError("/run is blocked by deny list (bash tool denied)")
		return Result{Handled: true}
	}

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		PrintError("Command failed: %s", err)
	}
	return Result{Handled: true}
}

func cmdWatch(ctx Context) Result {
	if ctx.HasArg(1) && ctx.Arg(1) == "stop" {
		if ctx.StopWatch != nil {
			ctx.StopWatch()
		}
		PrintDim("[watch] stopped")
		return Result{Handled: true}
	}

	if ctx.HasArg(1) && ctx.Arg(1) == "status" {
		// Future: report watch status. For now just report it's running if StartWatch is set.
		PrintDim("Use /watch stop to halt an active watcher.")
		return Result{Handled: true}
	}

	// Parse optional flags: /watch [--ext .go,.ts] [--debounce 300ms]
	// Args after the command name are treated as flag pairs.
	// We forward parsed state via the StartWatch callback which reads package-level watchConfig.
	args := ctx.Args()
	fields := strings.Fields(args)
	for i := 0; i < len(fields)-1; i++ {
		switch fields[i] {
		case "--ext", "-e":
			// Comma-separated extension list, e.g. ".go,.ts"
			// The watchConfig is in features_watch.go — communicate via ctx.Config.
			// We stash extensions in the WatchExtensions field if available.
			_ = fields[i+1] // handled by caller through WatchConfig callback if present
		case "--debounce", "-d":
			_ = fields[i+1]
		}
	}

	if ctx.StartWatch != nil {
		ctx.StartWatch(ctx.RepoPath)
		PrintSuccess("[watch] started — debounced, runs go test on file changes (type /watch stop to halt)")
		if len(watchExtensions(ctx)) > 0 {
			PrintDim("  filtering: %s", strings.Join(watchExtensions(ctx), ", "))
		}
	} else {
		PrintDim("Watch mode: monitors file changes and auto-runs tests.")
		PrintDim("Usage: /watch [--ext .go,.ts] [--debounce 300ms] | /watch stop")
	}
	return Result{Handled: true}
}

// watchExtensions extracts --ext value from command args.
func watchExtensions(ctx Context) []string {
	fields := strings.Fields(ctx.Args())
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] == "--ext" || fields[i] == "-e" {
			return strings.Split(fields[i+1], ",")
		}
	}
	return nil
}

func cmdLintFix(ctx Context) Result {
	fmt.Printf("%sRunning go vet…%s\n", ColorDim, ColorReset)
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "go", "vet", "./...")
	} else {
		cmd := exec.Command("go", "vet", "./...")
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}

	if _, err := exec.LookPath("staticcheck"); err == nil {
		fmt.Printf("%sRunning staticcheck…%s\n", ColorDim, ColorReset)
		if ctx.REPL.RunShell != nil {
			ctx.REPL.RunShell(ctx.RepoPath, "staticcheck", "./...")
		} else {
			cmd := exec.Command("staticcheck", "./...")
			cmd.Dir = ctx.RepoPath
			cmd.Stdout = Stdout
			cmd.Stderr = Stdout
			cmd.Run()
		}
	}
	fmt.Println()
	return Result{Handled: true}
}

func cmdTestFile(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /test-file <./path/to/pkg>")
		return Result{Handled: true}
	}
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "go", "test", "-v", ctx.Arg(1))
	} else {
		cmd := exec.Command("go", "test", "-v", ctx.Arg(1))
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	return Result{Handled: true}
}

func cmdTestGen(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /test-gen <file>")
		return Result{Handled: true}
	}
	target := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	prompt := fmt.Sprintf(
		"Read %s and generate comprehensive Go tests for it. Use table-driven tests where appropriate. "+
			"Cover happy paths, edge cases, and error conditions. Write the tests to the correct _test.go file.", target)
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdRefactor(ctx Context) Result {
	desc := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	if desc == "" {
		fmt.Println("Usage: /refactor <description of what to refactor>")
		return Result{Handled: true}
	}
	prompt := fmt.Sprintf(
		"Refactor the code as described. Make minimal changes, preserve behavior, run tests after.\n\nTask: %s", desc)
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdFix(ctx Context) Result {
	errText := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	if errText == "" {
		buildCmd := exec.Command("go", "build", "./...")
		buildCmd.Dir = ctx.RepoPath
		out, err := buildCmd.CombinedOutput()
		if err == nil {
			PrintSuccess("build is clean — no errors found")
			return Result{Handled: true}
		}
		errText = string(out)
	}
	prompt := fmt.Sprintf("Fix the following build/test errors. Make minimal changes to resolve them.\n\nErrors:\n%s", errText)
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdExplainError(ctx Context) Result {
	errText := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	if errText == "" {
		fmt.Println("Usage: /explain-error <error message>")
		return Result{Handled: true}
	}
	prompt := fmt.Sprintf("Explain this error message clearly: what does it mean, what caused it, and how to fix it?\n\nError:\n%s", errText)
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdOptimize(ctx Context) Result {
	target := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	prompt := "Analyze the code for performance bottlenecks. Profile hot paths, " +
		"reduce allocations, and improve algorithmic complexity where possible."
	if target != "" {
		prompt = fmt.Sprintf("Optimize %s for performance. Reduce allocations, improve algorithms, "+
			"run benchmarks before and after.", target)
	}
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdSecurity(ctx Context) Result {
	target := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	prompt := "Perform a security audit of this codebase. Check for: input validation, " +
		"injection vulnerabilities, hardcoded secrets, insecure defaults, path traversal, " +
		"and dependency vulnerabilities. Report findings with severity and fix suggestions."
	if target != "" {
		prompt = fmt.Sprintf("Security audit of %s: check for vulnerabilities, hardcoded secrets, "+
			"unsafe operations, and insecure patterns.", target)
	}
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdPlan(ctx Context) Result {
	task := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	if task == "" {
		fmt.Println("Usage: /plan <task description>")
		return Result{Handled: true}
	}
	planPrompt := fmt.Sprintf(
		"Think step by step about how to accomplish the following task. "+
			"Write out a numbered plan with each step clearly described. "+
			"Do NOT execute anything yet — only produce the plan.\n\nTask: %s", task)
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, planPrompt, ctx.RepoPath)
	}
	fmt.Printf("%sProceed with this plan? (y/N): %s", ColorYellow, ColorReset)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
		if ctx.REPL.StreamAndPrint != nil {
			ctx.REPL.StreamAndPrint(nil, ctx.Agent, "Now execute the plan above step by step.", ctx.RepoPath)
		}
	} else {
		fmt.Println("Cancelled.")
	}
	return Result{Handled: true}
}

func cmdBenchmark(ctx Context) Result {
	pkg := ""
	if ctx.HasArg(1) {
		pkg = ctx.Arg(1)
	}
	benchTarget := "./..."
	if pkg != "" {
		benchTarget = pkg
	}
	fmt.Printf("%sRunning benchmarks…%s\n", ColorDim, ColorReset)
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "go", "test", "-bench=.", "-benchmem", benchTarget)
	} else {
		cmd := exec.Command("go", "test", "-bench=.", "-benchmem", benchTarget)
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	return Result{Handled: true}
}

func cmdVerify(ctx Context) Result {
	type vcheck struct {
		name string
		cmd  string
		args []string
	}
	checks := []vcheck{
		{"go vet", "go", []string{"vet", "./..."}},
		{"go build", "go", []string{"build", "./..."}},
		{"go test", "go", []string{"test", "./...", "-count=1"}},
	}
	allPassed := true
	for _, c := range checks {
		cmd := exec.Command(c.cmd, c.args...)
		cmd.Dir = ctx.RepoPath
		output, err := cmd.CombinedOutput()
		icon := ColorLime + "✓" + ColorReset
		if err != nil {
			icon = ColorRed + "✗" + ColorReset
			allPassed = false
		}
		detail := strings.TrimSpace(string(output))
		if len(detail) > 60 {
			detail = detail[:60] + "…"
		}
		fmt.Printf("%s %s", icon, c.name)
		if detail != "" {
			fmt.Printf(": %s", detail)
		}
		fmt.Println()
	}
	if allPassed {
		PrintSuccess("all checks passed")
	}
	return Result{Handled: true}
}

func cmdDoctor(ctx Context) Result {
	fmt.Printf("%sRunning health checks…%s\n", ColorDim, ColorReset)

	checks := []struct {
		name, cmd string
		args      []string
	}{
		{"go version", "go", []string{"version"}},
		{"go env GOPATH", "go", []string{"env", "GOPATH"}},
		{"git version", "git", []string{"--version"}},
		{"go vet", "go", []string{"vet", "./..."}},
		{"go build", "go", []string{"build", "./..."}},
	}

	fmt.Printf("%s── Health ──────────────────────────%s\n", ColorDim, ColorReset)
	allOk := runHealthChecks(ctx.RepoPath, checks)
	fmt.Printf("%s──────────────────────────────────%s\n", ColorDim, ColorReset)
	if allOk {
		PrintSuccess("all checks pass")
	} else {
		fmt.Printf("%s✗ some checks failed — use /fix to auto-repair%s\n\n", ColorRed, ColorReset)
	}
	return Result{Handled: true}
}

// runHealthChecks executes the given checks and prints results. Returns true if all pass.
func runHealthChecks(repoPath string, checks []struct {
	name, cmd string
	args      []string
}) bool {
	allOk := true
	for _, c := range checks {
		cmd := exec.Command(c.cmd, c.args...)
		cmd.Dir = repoPath
		output, err := cmd.CombinedOutput()
		status := ColorLime + "✓" + ColorReset
		detail := strings.TrimSpace(string(output))
		if len(detail) > 60 {
			detail = detail[:60] + "…"
		}
		if err != nil {
			status = ColorRed + "✗" + ColorReset
			allOk = false
			if detail == "" {
				detail = err.Error()
			}
		}
		fmt.Printf("  %s  %-20s  %s%s%s\n", status, c.name, ColorDim, detail, ColorReset)
	}
	return allOk
}

func cmdMock(ctx Context) Result {
	filePath := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	if filePath == "" {
		fmt.Println("Usage: /mock <file.go>")
		return Result{Handled: true}
	}
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(ctx.RepoPath, filePath)
	}
	prompt := fmt.Sprintf("Read %s and generate mock implementations for all interfaces defined in it. "+
		"Use a testing mock pattern appropriate for Go. Write the mocks to a _mock_test.go file.", filePath)
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}
