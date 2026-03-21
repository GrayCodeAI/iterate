package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/GrayCodeAI/iterate/internal/community"
	"github.com/GrayCodeAI/iterate/internal/evolution"
	"github.com/GrayCodeAI/iterate/internal/social"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func main() {
	var (
		repoPath    = flag.String("repo", ".", "Path to the repo iterate will evolve")
		ghOwner     = flag.String("gh-owner", "", "GitHub repo owner")
		ghRepo      = flag.String("gh-repo", "", "GitHub repo name")
		issueMax    = flag.Int("issue-limit", 5, "Max community issues to include")
		socialOnly  = flag.Bool("social", false, "Run social loop only (no evolution)")
		replyIssues = flag.Bool("reply-issues", true, "Post bot replies to addressed issues")
		provider    = flag.String("provider", "gemini", "Provider to use (anthropic, openai, groq, gemini)")
		model       = flag.String("model", "", "Model to use")
		apiKey      = flag.String("api-key", "", "API key (or set OPENCODE_API_KEY, GEMINI_API_KEY, etc.)")
		thinking    = flag.String("thinking", "off", "Extended thinking depth: off, minimal, low, medium, high")
		chat        = flag.Bool("chat", false, "Start interactive REPL (default when no other mode set)")
		evolve      = flag.Bool("evolve", false, "Run one evolution session (non-interactive)")
		phase       = flag.String("phase", "", "Evolution phase: plan, implement, communicate, or \"\" (all)")
		saveSession = flag.String("save-session", "", "Save agent messages to JSON file after run")
		loadSession = flag.String("load-session", "", "Load agent messages from JSON file before run")
		compactFlag = flag.Bool("compact", false, "Compact loaded session before running")
	)
	flag.Parse()

	// Suppress logs in REPL mode for clean UI
	logLevel := slog.LevelInfo
	if *chat || (!*evolve && !*socialOnly && *phase == "") {
		logLevel = slog.LevelError // Hide info/warn in REPL mode
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Debug: log environment
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken != "" {
		logger.Info("GITHUB_TOKEN is set", "len", len(githubToken))
	} else {
		logger.Warn("GITHUB_TOKEN is NOT set")
	}

	absRepo, err := filepath.Abs(*repoPath)
	if err != nil {
		logger.Error("invalid repo path", "err", err)
		os.Exit(1)
	}

	// Load persisted config (overridden by explicit flags)
	cfg := loadConfig()
	if *provider == "gemini" && cfg.Provider != "" && cfg.Provider != "gemini" {
		*provider = cfg.Provider
	}
	if *model == "" && cfg.Model != "" {
		*model = cfg.Model
	}
	if cfg.OllamaBaseURL != "" && os.Getenv("OLLAMA_BASE_URL") == "" {
		os.Setenv("OLLAMA_BASE_URL", cfg.OllamaBaseURL)
	}

	if *model != "" {
		os.Setenv("ITERATE_MODEL", *model)
	}

	p, err := iteragent.NewProvider(*provider, *apiKey)
	if err != nil {
		logger.Error("provider init failed", "err", err)
		os.Exit(1)
	}
	logger.Info("using provider", "name", p.Name())

	ctx := context.Background()

	// Apply persisted ThinkingLevel when the flag is at its default "off".
	if *thinking == "off" && cfg.ThinkingLevel != "" {
		*thinking = cfg.ThinkingLevel
	}

	// Default to REPL unless --evolve or --social is explicitly set.
	if *chat || (!*evolve && !*socialOnly && *phase == "") {
		runREPL(ctx, p, absRepo, iteragent.ThinkingLevel(*thinking), logger)
		return
	}

	if *socialOnly {
		if *ghOwner == "" || *ghRepo == "" {
			logger.Error("--gh-owner and --gh-repo required for social mode")
			os.Exit(1)
		}
		socialEngine := social.New(absRepo, *ghOwner, *ghRepo, logger)
		logger.Info("starting social session")
		if err := socialEngine.Run(ctx, p); err != nil {
			logger.Error("social session failed", "err", err)
			os.Exit(1)
		}
		logger.Info("social session complete")
		return
	}

	var rawIssues map[community.IssueType][]community.Issue
	var issues string
	if *ghOwner != "" && *ghRepo != "" {
		fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		issueTypes := []community.IssueType{
			community.IssueTypeInput,
			community.IssueTypeSelf,
			community.IssueTypeHelpWanted,
		}
		logger.Info("fetching issues", "owner", *ghOwner, "repo", *ghRepo)
		rawIssues, err = community.FetchIssues(fetchCtx, *ghOwner, *ghRepo, issueTypes, *issueMax)
		cancel()
		if err != nil {
			logger.Warn("failed to fetch community issues", "err", err)
		} else {
			issues = community.FormatIssuesByType(rawIssues)
			var total int
			for _, v := range rawIssues {
				total += len(v)
			}
			logger.Info("loaded community issues", "count", total, "issues_len", len(issues))
			if len(issues) == 0 {
				logger.Warn("issues string is empty despite fetching some")
			}
		}
	}

	var sessionMessages []iteragent.Message
	if *loadSession != "" {
		msgs, err := loadSessionFromFile(*loadSession)
		if err != nil {
			logger.Warn("failed to load session", "path", *loadSession, "err", err)
		} else {
			sessionMessages = msgs
			logger.Info("loaded session", "path", *loadSession, "messages", len(msgs))
		}
	}

	if *compactFlag && len(sessionMessages) > 0 {
		cfg := iteragent.DefaultContextConfig()
		sessionMessages = iteragent.CompactMessagesTiered(sessionMessages, cfg)
		logger.Info("compacted session messages", "remaining", len(sessionMessages))
	}

	iteragent.SetProtectedPaths([]string{
		filepath.Join(absRepo, "scripts/evolve.sh"),
		filepath.Join(absRepo, ".github/workflows"),
		filepath.Join(absRepo, "skills"),
		filepath.Join(absRepo, "IDENTITY.md"),
		filepath.Join(absRepo, "PERSONALITY.md"),
		filepath.Join(absRepo, "CLAUDE.md"),
	})

	engine := evolution.New(absRepo, logger).
		WithThinking(iteragent.ThinkingLevel(*thinking))

	logger.Info("starting evolution session", "repo", absRepo)
	fmt.Println(banner())

	var result *evolution.RunResult
	switch *phase {
	case "plan":
		logger.Info("running plan phase")
		if err := engine.RunPlanPhase(ctx, p, issues); err != nil {
			logger.Error("plan phase failed", "err", err)
			os.Exit(1)
		}
		logger.Info("plan phase complete")
		return
	case "implement":
		logger.Info("running implement phase")
		if err := engine.RunImplementPhase(ctx, p); err != nil {
			logger.Error("implement phase failed", "err", err)
			os.Exit(1)
		}
		logger.Info("implement phase complete")
		return
	case "communicate":
		logger.Info("running communicate phase")
		if err := engine.RunCommunicatePhase(ctx, p); err != nil {
			logger.Error("communicate phase failed", "err", err)
			os.Exit(1)
		}
		logger.Info("communicate phase complete")
		return
	default:
		var runErr error
		result, runErr = engine.Run(ctx, p, issues)
		if runErr != nil {
			logger.Error("evolution session failed", "err", runErr)
			os.Exit(1)
		}
	}

	autoSavePath := filepath.Join(absRepo, ".iterate", "last-session.json")
	_ = os.MkdirAll(filepath.Dir(autoSavePath), 0o755)
	if err := saveSessionToFile(autoSavePath, sessionMessages); err != nil {
		logger.Warn("auto-save session failed", "err", err)
	}

	if *saveSession != "" {
		if err := saveSessionToFile(*saveSession, sessionMessages); err != nil {
			logger.Warn("save session failed", "path", *saveSession, "err", err)
		} else {
			logger.Info("session saved", "path", *saveSession)
		}
	}

	if *replyIssues && *ghOwner != "" && *ghRepo != "" && len(rawIssues) > 0 {
		issueNumbers := make([]int, 0)
		for _, issues := range rawIssues {
			for _, issue := range issues {
				issueNumbers = append(issueNumbers, issue.Number)
			}
		}
		socialEngine := social.New(absRepo, *ghOwner, *ghRepo, logger)
		replyCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		if err := socialEngine.ReplyToIssues(replyCtx, p, issueNumbers); err != nil {
			logger.Warn("issue replies failed", "err", err)
		}
		cancel()
	}

	incrementDayCount(absRepo)

	if result != nil {
		logger.Info("session complete",
			"status", result.Status,
			"duration", result.FinishedAt.Sub(result.StartedAt).Round(time.Second),
		)
	}
}

func incrementDayCount(repoPath string) {
	path := filepath.Join(repoPath, "DAY_COUNT")
	data, _ := os.ReadFile(path)
	n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d", n+1)), 0o644)
}

func banner() string {
	// ASCII logo moved to printHeader in repl.go
	return ""
}

func saveSessionToFile(path string, messages []iteragent.Message) error {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func loadSessionFromFile(path string) ([]iteragent.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	var messages []iteragent.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("unmarshal messages: %w", err)
	}
	return messages, nil
}
