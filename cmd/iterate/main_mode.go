package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/GrayCodeAI/iterate/internal/community"
	"github.com/GrayCodeAI/iterate/internal/evolution"
	"github.com/GrayCodeAI/iterate/internal/social"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func runMode(ctx context.Context, f mainFlags, absRepo string, logger *slog.Logger) {
	cfg := loadConfig()
	providerName, modelName, apiKey := resolveProviderConfig(f.provider, f.model, f.apiKey, cfg)
	f.provider = providerName
	f.model = modelName
	f.apiKey = apiKey

	p, err := initProvider(f.provider, f.apiKey, logger)
	if err != nil {
		logger.Error("provider init failed", "err", err)
		os.Exit(1)
	}

	f.thinking = resolveThinkingLevel(f.thinking, cfg)

	isREPL := f.chat || (!f.evolve && !f.socialOnly && f.phase == "")
	if isREPL {
		runREPL(ctx, p, absRepo, iteragent.ThinkingLevel(f.thinking), logger)
		return
	}

	if f.socialOnly {
		runSocialMode(ctx, f, absRepo, p, logger)
		return
	}

	result, sessionMessages, rawIssues := runEvolutionMode(ctx, f, absRepo, p, logger)
	handleEvolutionCleanup(ctx, f, absRepo, result, sessionMessages, rawIssues, p, logger)
}

func runSocialMode(ctx context.Context, f mainFlags, absRepo string, p iteragent.Provider, logger *slog.Logger) {
	if f.ghOwner == "" || f.ghRepo == "" {
		logger.Error("--gh-owner and --gh-repo required for social mode")
		os.Exit(1)
	}
	socialEngine := social.New(absRepo, f.ghOwner, f.ghRepo, logger)
	logger.Info("starting social session")
	if err := socialEngine.Run(ctx, p); err != nil {
		logger.Error("social session failed", "err", err)
		os.Exit(1)
	}
	logger.Info("social session complete")
}

func runEvolutionMode(ctx context.Context, f mainFlags, absRepo string, p iteragent.Provider, logger *slog.Logger) (*evolution.RunResult, []iteragent.Message, map[community.IssueType][]community.Issue) {
	rawIssues, issues := fetchCommunityIssues(ctx, f, logger)
	sessionMessages := loadEvolutionSession(f, logger)

	iteragent.SetProtectedPaths([]string{
		filepath.Join(absRepo, "scripts/evolution/evolve.sh"),
		filepath.Join(absRepo, ".github/workflows"),
		filepath.Join(absRepo, "skills"),
		filepath.Join(absRepo, "docs/IDENTITY.md"),
		filepath.Join(absRepo, "docs/PERSONALITY.md"),
		filepath.Join(absRepo, "CLAUDE.md"),
	})

	engine := evolution.New(absRepo, logger).
		WithThinking(iteragent.ThinkingLevel(f.thinking))

	logger.Info("starting evolution session", "repo", absRepo)

	result := runEvolutionPhase(ctx, f, p, engine, issues, logger)
	return result, sessionMessages, rawIssues
}

func fetchCommunityIssues(ctx context.Context, f mainFlags, logger *slog.Logger) (map[community.IssueType][]community.Issue, string) {
	if f.ghOwner == "" || f.ghRepo == "" {
		return nil, ""
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	issueTypes := []community.IssueType{
		community.IssueTypeInput,
		community.IssueTypeSelf,
		community.IssueTypeHelpWanted,
	}
	logger.Info("fetching issues", "owner", f.ghOwner, "repo", f.ghRepo)
	rawIssues, err := community.FetchIssues(fetchCtx, f.ghOwner, f.ghRepo, issueTypes, f.issueMax)
	cancel()
	if err != nil {
		logger.Warn("failed to fetch community issues", "err", err)
		return nil, ""
	}

	issues := community.FormatIssuesByType(rawIssues)
	var total int
	for _, v := range rawIssues {
		total += len(v)
	}
	logger.Info("loaded community issues", "count", total, "issues_len", len(issues))
	if len(issues) == 0 {
		logger.Warn("issues string is empty despite fetching some")
	}
	return rawIssues, issues
}

func loadEvolutionSession(f mainFlags, logger *slog.Logger) []iteragent.Message {
	var sessionMessages []iteragent.Message
	if f.loadSession != "" {
		msgs, err := loadSessionFromFile(f.loadSession)
		if err != nil {
			logger.Warn("failed to load session", "path", f.loadSession, "err", err)
		} else {
			sessionMessages = msgs
			logger.Info("loaded session", "path", f.loadSession, "messages", len(msgs))
		}
	}

	if f.compactFlag && len(sessionMessages) > 0 {
		ctxCfg := iteragent.DefaultContextConfig()
		sessionMessages = iteragent.CompactMessagesTiered(sessionMessages, ctxCfg)
		logger.Info("compacted session messages", "remaining", len(sessionMessages))
	}
	return sessionMessages
}

func runEvolutionPhase(ctx context.Context, f mainFlags, p iteragent.Provider, engine *evolution.Engine, issues string, logger *slog.Logger) *evolution.RunResult {
	switch f.phase {
	case "plan":
		logger.Info("running plan phase")
		if err := engine.RunPlanPhase(ctx, p, issues); err != nil {
			logger.Error("plan phase failed", "err", err)
			os.Exit(1)
		}
		logger.Info("plan phase complete")
		return nil
	case "implement":
		logger.Info("running implement phase")
		if err := engine.RunImplementPhase(ctx, p); err != nil {
			logger.Error("implement phase failed", "err", err)
			os.Exit(1)
		}
		logger.Info("implement phase complete")
		return nil
	case "pr":
		logger.Info("running PR phase")
		if err := engine.RunPRPhase(ctx); err != nil {
			logger.Error("PR phase failed", "err", err)
			os.Exit(1)
		}
		logger.Info("PR phase complete")
		return nil
	case "review":
		logger.Info("running review phase")
		if err := engine.RunReviewPhase(ctx, p); err != nil {
			logger.Error("review phase failed", "err", err)
			os.Exit(1)
		}
		logger.Info("review phase complete")
		return nil
	case "merge":
		logger.Info("running merge phase")
		if err := engine.RunMergePhase(ctx); err != nil {
			logger.Error("merge phase failed", "err", err)
			os.Exit(1)
		}
		logger.Info("merge phase complete")
		return nil
	case "communicate":
		logger.Info("running communicate phase")
		if err := engine.RunCommunicatePhase(ctx, p); err != nil {
			logger.Error("communicate phase failed", "err", err)
			os.Exit(1)
		}
		logger.Info("communicate phase complete")
		return nil
	default:
		result, runErr := engine.Run(ctx, p, issues)
		if runErr != nil {
			logger.Error("evolution session failed", "err", runErr)
			os.Exit(1)
		}
		return result
	}
}

func handleEvolutionCleanup(ctx context.Context, f mainFlags, absRepo string, result *evolution.RunResult, sessionMessages []iteragent.Message, rawIssues map[community.IssueType][]community.Issue, p iteragent.Provider, logger *slog.Logger) {
	autoSavePath := filepath.Join(absRepo, ".iterate", "last-session.json")
	_ = os.MkdirAll(filepath.Dir(autoSavePath), 0o755) // best-effort cleanup
	if err := saveSessionToFile(autoSavePath, sessionMessages); err != nil {
		logger.Warn("auto-save session failed", "err", err)
	}

	if f.saveSession != "" {
		if err := saveSessionToFile(f.saveSession, sessionMessages); err != nil {
			logger.Warn("save session failed", "path", f.saveSession, "err", err)
		} else {
			logger.Info("session saved", "path", f.saveSession)
		}
	}

	if f.replyIssues && f.ghOwner != "" && f.ghRepo != "" && len(rawIssues) > 0 {
		issueNumbers := make([]int, 0)
		for _, issues := range rawIssues {
			for _, issue := range issues {
				issueNumbers = append(issueNumbers, issue.Number)
			}
		}
		socialEngine := social.New(absRepo, f.ghOwner, f.ghRepo, logger)
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
