package main

import (
	"flag"
	"log/slog"
	"os"
)

type mainFlags struct {
	repoPath    string
	ghOwner     string
	ghRepo      string
	issueMax    int
	socialOnly  bool
	replyIssues bool
	provider    string
	model       string
	apiKey      string
	thinking    string
	chat        bool
	evolve      bool
	phase       string
	saveSession string
	loadSession string
	compactFlag bool
	noTools     bool
	budget      float64
	sandbox     bool
	sandboxImage string
}

func parseFlags() mainFlags {
	var f mainFlags
	flag.StringVar(&f.repoPath, "repo", ".", "Path to the repo iterate will evolve")
	flag.StringVar(&f.ghOwner, "gh-owner", "", "GitHub repo owner")
	flag.StringVar(&f.ghRepo, "gh-repo", "", "GitHub repo name")
	flag.IntVar(&f.issueMax, "issue-limit", 5, "Max community issues to include")
	flag.BoolVar(&f.socialOnly, "social", false, "Run social loop only (no evolution)")
	flag.BoolVar(&f.replyIssues, "reply-issues", true, "Post bot replies to addressed issues")
	flag.StringVar(&f.provider, "provider", "gemini",
		"LLM provider: anthropic, openai, gemini, groq, ollama, azure, vertex, opencode (default: gemini)")
	flag.StringVar(&f.model, "model", "", "Model name override (e.g. claude-opus-4-6, gpt-4o, gemini-2.0-flash)")
	flag.StringVar(&f.apiKey, "api-key", "", "API key override (or set ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, GROQ_API_KEY, etc.)")
	flag.StringVar(&f.thinking, "thinking", "off", "Extended thinking depth: off, minimal, low, medium, high")
	flag.BoolVar(&f.chat, "chat", false, "Start interactive REPL (default when no other mode set)")
	flag.BoolVar(&f.evolve, "evolve", false, "Run one evolution session (non-interactive)")
	flag.StringVar(&f.phase, "phase", "", "Evolution phase: plan, implement, communicate, or \"\" (all)")
	flag.StringVar(&f.saveSession, "save-session", "", "Save agent messages to JSON file after run")
	flag.StringVar(&f.loadSession, "load-session", "", "Load agent messages from JSON file before run")
	flag.BoolVar(&f.compactFlag, "compact", false, "Compact loaded session before running")
	flag.BoolVar(&f.noTools, "no-tools", false, "Disable all tools — pure chat mode (no file reads/writes/bash)")
	flag.Float64Var(&f.budget, "budget", 0, "Spending limit in USD (0 = no limit, e.g. --budget 5.00)")
	flag.BoolVar(&f.sandbox, "sandbox", false, "Enable Docker sandbox for command execution (isolates file system and network)")
	flag.StringVar(&f.sandboxImage, "sandbox-image", "node:18-slim", "Docker image to use for sandbox (default: node:18-slim)")
	flag.Parse()
	return f
}

func setupLogging(isREPL bool) *slog.Logger {
	logLevel := slog.LevelInfo
	if isREPL {
		logLevel = slog.LevelError
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken != "" {
		logger.Info("GITHUB_TOKEN is set", "len", len(githubToken))
	} else {
		logger.Warn("GITHUB_TOKEN is NOT set")
	}
	return logger
}
