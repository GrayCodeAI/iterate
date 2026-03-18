package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/GrayCodeAI/iterate/internal/community"
	"github.com/GrayCodeAI/iterate/internal/evolution"
	"github.com/GrayCodeAI/iterate/internal/session"
	"github.com/GrayCodeAI/iterate/internal/social"
	"github.com/GrayCodeAI/iterate/internal/web"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func main() {
	var (
		repoPath    = flag.String("repo", ".", "Path to the repo iterate will evolve")
		serve       = flag.Bool("serve", false, "Run the web dashboard server")
		addr        = flag.String("addr", ":8080", "Web server address")
		ghOwner     = flag.String("gh-owner", "", "GitHub repo owner")
		ghRepo      = flag.String("gh-repo", "", "GitHub repo name")
		issueMax    = flag.Int("issue-limit", 5, "Max community issues to include")
		socialOnly  = flag.Bool("social", false, "Run social loop only (no evolution)")
		replyIssues = flag.Bool("reply-issues", true, "Post bot replies to addressed issues")
		provider    = flag.String("provider", "", "Provider to use (anthropic, openai, groq, gemini)")
		model       = flag.String("model", "", "Model to use")
		apiKey      = flag.String("api-key", "", "API key (or set ANTHROPIC_API_KEY, GEMINI_API_KEY, etc.)")
		thinking    = flag.String("thinking", "off", "Extended thinking depth: off, minimal, low, medium, high")

		// Interactive REPL
		chat = flag.Bool("chat", false, "Start interactive REPL (slash commands + free-form chat)")

		// 3-phase evolution
		phase = flag.String("phase", "", "Evolution phase: plan, implement, communicate, or \"\" (all)")

		// Session management
		saveSession  = flag.String("save-session", "", "Save agent messages to JSON file after run")
		loadSession  = flag.String("load-session", "", "Load agent messages from JSON file before run")
		compactFlag  = flag.Bool("compact", false, "Compact loaded session if it exceeds 80% of context budget before running")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	absRepo, err := filepath.Abs(*repoPath)
	if err != nil {
		logger.Error("invalid repo path", "err", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(absRepo, ".iterate", "sessions.db")
	_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
	store, err := session.NewStore(dbPath)
	if err != nil {
		logger.Error("open session store", "err", err)
		os.Exit(1)
	}

	// Set model env var if provided
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

	// Interactive REPL mode.
	if *chat {
		runREPL(ctx, p, absRepo, iteragent.ThinkingLevel(*thinking), logger)
		return
	}

	// Social-only mode (used by the 4h social workflow)
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

	// Web dashboard
	webServer := web.New(store, logger)
	if *serve {
		go func() {
			logger.Info("web dashboard starting", "addr", *addr)
			if err := http.ListenAndServe(*addr, webServer.Handler()); err != nil {
				logger.Error("web server failed", "err", err)
			}
		}()
	}

	// Fetch community issues
	var rawIssues map[community.IssueType][]community.Issue
	var issues string
	if *ghOwner != "" && *ghRepo != "" {
		fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		issueTypes := []community.IssueType{
			community.IssueTypeInput,
			community.IssueTypeSelf,
			community.IssueTypeHelpWanted,
		}
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
			logger.Info("loaded community issues", "count", total)
		}
	}

	// Load session messages if requested
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

	// Compact if needed (80% of 100k token budget)
	if *compactFlag && len(sessionMessages) > 0 {
		cfg := iteragent.DefaultContextConfig()
		sessionMessages = iteragent.CompactMessagesTiered(sessionMessages, cfg)
		logger.Info("compacted session messages", "remaining", len(sessionMessages))
	}

	// Run evolution session, wiring live events to the web dashboard.
	engine := evolution.New(absRepo, store, logger).
		WithEventSink(webServer.EventBus()).
		WithThinking(iteragent.ThinkingLevel(*thinking))
	logger.Info("starting evolution session", "repo", absRepo)
	fmt.Println(banner())

	var sess *session.Session
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
		// "" — run all phases (original single-shot behaviour)
		var runErr error
		sess, runErr = engine.Run(ctx, p, issues)
		if runErr != nil {
			logger.Error("evolution session failed", "err", runErr)
			os.Exit(1)
		}
	}

	// Auto-save session messages to .iterate/last-session.json
	autoSavePath := filepath.Join(absRepo, ".iterate", "last-session.json")
	_ = os.MkdirAll(filepath.Dir(autoSavePath), 0o755)
	if err := saveSessionToFile(autoSavePath, sessionMessages); err != nil {
		logger.Warn("auto-save session failed", "err", err)
	}

	// Save session to explicit path if requested
	if *saveSession != "" {
		if err := saveSessionToFile(*saveSession, sessionMessages); err != nil {
			logger.Warn("save session failed", "path", *saveSession, "err", err)
		} else {
			logger.Info("session saved", "path", *saveSession)
		}
	}

	// Post bot replies to issues that were addressed
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

	if sess != nil {
		logger.Info("session complete",
			"status", sess.Status,
			"duration", sess.FinishedAt.Sub(sess.StartedAt).Round(time.Second),
		)
	}

	if *serve {
		logger.Info("web dashboard running, press Ctrl+C to stop", "addr", *addr)
		select {}
	}
}

func incrementDayCount(repoPath string) {
	path := filepath.Join(repoPath, "DAY_COUNT")
	data, _ := os.ReadFile(path)
	n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d", n+1)), 0o644)
}

func banner() string {
	return `
  _  _            _
 (_)| |_ ___  _ _| |_  ___
 | ||  _/ -_)| '_/ _|/ -_)
 |_| \__\___||_| \__|\___|

 self-evolving code agent
 ─────────────────────────
 `
}

// saveSessionToFile marshals messages as JSON and writes to path.
func saveSessionToFile(path string, messages []iteragent.Message) error {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// loadSessionFromFile reads JSON from path and deserializes as []Message.
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
