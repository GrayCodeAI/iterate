package commands

import (
	"fmt"
	"os/exec"
)

// RegisterDevCommands adds development commands.
func RegisterDevCommands(r *Registry) {
	r.Register(Command{
		Name:        "/test",
		Aliases:     []string{},
		Description: "run tests (auto-detect framework)",
		Category:    "dev",
		Handler:     cmdTest,
	})

	r.Register(Command{
		Name:        "/build",
		Aliases:     []string{},
		Description: "build the project",
		Category:    "dev",
		Handler:     cmdBuild,
	})

	r.Register(Command{
		Name:        "/lint",
		Aliases:     []string{},
		Description: "run linter",
		Category:    "dev",
		Handler:     cmdLint,
	})

	r.Register(Command{
		Name:        "/format",
		Aliases:     []string{"/fmt"},
		Description: "format code",
		Category:    "dev",
		Handler:     cmdFormat,
	})

	r.Register(Command{
		Name:        "/run",
		Aliases:     []string{},
		Description: "run a shell command",
		Category:    "dev",
		Handler:     cmdRun,
	})

	r.Register(Command{
		Name:        "/watch",
		Aliases:     []string{},
		Description: "auto-run tests on file changes",
		Category:    "dev",
		Handler:     cmdWatch,
	})
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
	// TODO: wire up file watcher
	if ctx.HasArg(1) && ctx.Arg(1) == "stop" {
		PrintSuccess("Watch stopped")
		return Result{Handled: true}
	}
	fmt.Println("Watch mode not yet wired in modular commands.")
	return Result{Handled: true}
}
