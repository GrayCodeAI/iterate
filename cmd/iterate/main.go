package main

import (
	"context"
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

	p, err := iteragent.NewProvider("")
	if err != nil {
		logger.Error("provider init failed", "err", err)
		os.Exit(1)
	}
	logger.Info("using provider", "name", p.Name())

	ctx := context.Background()

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

	// Run evolution session
	engine := evolution.New(absRepo, store, logger)
	logger.Info("starting evolution session", "repo", absRepo)
	fmt.Println(banner())

	sess, err := engine.Run(ctx, p, issues)
	if err != nil {
		logger.Error("evolution session failed", "err", err)
		os.Exit(1)
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

	logger.Info("session complete",
		"status", sess.Status,
		"duration", sess.FinishedAt.Sub(sess.StartedAt).Round(time.Second),
	)

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
