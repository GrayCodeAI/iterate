package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/commands"
	"github.com/GrayCodeAI/iterate/internal/ui/selector"
)

// Color variables — reassignable for /theme support.
// Protected by colorMu: applyTheme writes, signal handler goroutine reads.
var (
	colorMu     sync.RWMutex
	colorReset  = "\033[0m"
	colorLime   = "\033[38;5;154m"
	colorYellow = "\033[38;5;220m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
	colorCyan   = "\033[36m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[38;5;114m"
	colorAmber  = "\033[38;5;221m"
	colorBlue   = "\033[38;5;75m"
	colorPurple = "\033[38;5;141m"
)

// replRepoPath is the repo path used in the current REPL session (for prompt display).
var replRepoPath string

// iterateVersion is the current version string.
const iterateVersion = "dev"

func makeAgent(p iteragent.Provider, repoPath string, thinking iteragent.ThinkingLevel, logger *slog.Logger) *iteragent.Agent {
	base := iteragent.DefaultTools(repoPath)
	switch currentMode {
	case modeAsk:
		base = readOnlyTools(base)
	case modeArchitect:
		base = nil // no tools — thinking only
	}
	tools := wrapToolsWithPermissions(base)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(repoPath, "skills")})
	defaultTemp := float32(0.9)
	ag := iteragent.New(p, tools, logger).
		WithSystemPrompt(replSystemPrompt(repoPath)).
		WithSkillSet(skills).
		WithThinkingLevel(thinking).
		WithToolExecutionStrategy(iteragent.NewParallelStrategy()).
		WithHooks(replHooks()).
		WithTemperature(defaultTemp)
	if rtConfig.Temperature != nil {
		ag = ag.WithTemperature(*rtConfig.Temperature)
	}
	if rtConfig.MaxTokens != nil {
		ag = ag.WithMaxTokens(*rtConfig.MaxTokens)
	}
	if rtConfig.CacheEnabled != nil {
		ag = ag.WithCacheEnabled(*rtConfig.CacheEnabled)
	}
	return ag
}

// replHooks returns lifecycle hooks for the REPL agent.
// In debug mode: prints turn timing and tool execution duration.
func replHooks() iteragent.AgentHooks {
	var turnStart time.Time
	var toolStart time.Time
	return iteragent.AgentHooks{
		BeforeTurn: func(turn int, messages []iteragent.Message) {
			turnStart = time.Now()
			if cfg.DebugMode {
				fmt.Printf("%s[debug] turn %d — %d messages in context%s\n",
					colorDim, turn, len(messages), colorReset)
			}
		},
		AfterTurn: func(turn int, response string) {
			if cfg.DebugMode {
				elapsed := time.Since(turnStart).Round(time.Millisecond)
				fmt.Printf("%s[debug] turn %d done in %s (%d chars)%s\n",
					colorDim, turn, elapsed, len(response), colorReset)
			}
		},
		OnToolStart: func(toolName string, args map[string]string) {
			toolStart = time.Now()
			if cfg.DebugMode {
				fmt.Printf("%s[debug] → %s%s\n", colorDim, toolName, colorReset)
			}
		},
		OnToolEnd: func(toolName string, result string, err error) {
			if cfg.DebugMode {
				elapsed := time.Since(toolStart).Round(time.Millisecond)
				status := "ok"
				if err != nil {
					status = "err"
				}
				fmt.Printf("%s[debug] ← %s (%s, %s, %d chars)%s\n",
					colorDim, toolName, status, elapsed, len(result), colorReset)
			}
		},
	}
}

// initREPL loads config, applies theme, sets up signal handling and runtime state.
func setupSigintHandler() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	go func() {
		for range sigCh {
			if sess.RequestCancel != nil {
				sess.RequestCancel()
				// Snapshot colors under read lock — applyTheme writes these from the main goroutine.
				colorMu.RLock()
				y, r := colorYellow, colorReset
				colorMu.RUnlock()
				fmt.Printf("\r\033[K%s[cancelled]%s\n", y, r)
			}
		}
	}()
}

func applyLoadedConfig(loadedCfg iterConfig, thinking iteragent.ThinkingLevel) iteragent.ThinkingLevel {
	cfg.SafeMode = loadedCfg.SafeMode
	cfg.NotifyEnabled = loadedCfg.Notify
	if loadedCfg.Theme != "" {
		if t, ok := themes[loadedCfg.Theme]; ok {
			applyTheme(t)
		}
	}
	if len(loadedCfg.DeniedTools) > 0 {
		deniedToolsMu.Lock()
		deniedTools = make(map[string]bool, len(loadedCfg.DeniedTools))
		for _, t := range loadedCfg.DeniedTools {
			deniedTools[t] = true
		}
		deniedToolsMu.Unlock()
	}

	if loadedCfg.Temperature > 0 {
		t := float32(loadedCfg.Temperature)
		rtConfig.Temperature = &t
	}
	if loadedCfg.MaxTokens > 0 {
		rtConfig.MaxTokens = &loadedCfg.MaxTokens
	}
	if loadedCfg.CacheEnabled {
		enabled := true
		rtConfig.CacheEnabled = &enabled
	}
	if thinking == iteragent.ThinkingLevelOff && loadedCfg.ThinkingLevel != "" {
		thinking = iteragent.ThinkingLevel(loadedCfg.ThinkingLevel)
	}
	return thinking
}

func initREPL(repoPath string, thinking iteragent.ThinkingLevel) iteragent.ThinkingLevel {
	selector.InitHistory()
	initAuditLog()
	setupSigintHandler()

	loadedCfg := loadConfig()
	thinking = applyLoadedConfig(loadedCfg, thinking)

	replRepoPath = repoPath
	selector.RepoPath = repoPath
	selector.SafeMode = cfg.SafeMode
	return thinking
}

// runREPL runs an interactive session with iterate.
func runREPL(ctx context.Context, p iteragent.Provider, repoPath string, thinking iteragent.ThinkingLevel, logger *slog.Logger) {
	thinking = initREPL(repoPath, thinking)

	a := makeAgent(p, repoPath, thinking, logger)
	defer func() { _ = a.Close() }() // best-effort cleanup

	printHeader(p, thinking, repoPath)

	// Restore last autosave if available (but don't force — just offer info)
	if sessions := listSessions(); containsString(sessions, "autosave") {
		fmt.Printf("%s(previous session autosaved — /load autosave to restore)%s\n\n", colorDim, colorReset)
	}

	for {
		line, ok := selector.ReadInput()
		if !ok {
			break
		}
		if line == "" {
			continue
		}

		if handled := handleModelProviderSwitch(line, &p, &thinking, &a, repoPath, logger); handled {
			continue
		}

		// Resolve aliases before command dispatch
		line = resolveAlias(line)

		if strings.HasPrefix(line, "/") {
			if done := handleCommand(ctx, line, a, p, repoPath, &thinking, logger); done {
				return
			}
			continue
		}

		streamAndPrint(ctx, a, line, repoPath)
	}

	stopWatch()
	printSessionSummary(a, repoPath)
}

// handleModelProviderSwitch processes /model and /provider commands, returning true if handled.
func handleModelProviderSwitch(line string, p *iteragent.Provider, thinking *iteragent.ThinkingLevel, a **iteragent.Agent, repoPath string, logger *slog.Logger) bool {
	if line == "/model" || strings.HasPrefix(line, "/model ") {
		newP, newThinking := selectModel(*thinking)
		if newP != nil {
			*p = newP
			*thinking = newThinking
			_ = (*a).Close() // best-effort cleanup
			*a = makeAgent(*p, repoPath, *thinking, logger)
			fmt.Printf("%s✓ switched to %s%s\n\n", colorLime, (*p).Name(), colorReset)
			// Preserve all existing config — only update provider/model fields.
			updatedCfg := loadConfig()
			updatedCfg.Provider = (*p).Name()
			if model := os.Getenv("ITERATE_MODEL"); model != "" {
				updatedCfg.Model = model
			}
			saveConfig(updatedCfg)
		}
		return true
	}
	if strings.HasPrefix(line, "/provider") {
		parts := strings.Fields(line)
		if len(parts) == 1 {
			fmt.Printf("Current provider: %s%s%s\n", colorLime, (*p).Name(), colorReset)
			fmt.Println("Usage: /provider <name>  (anthropic, openai, gemini, groq, ollama, …)")
		} else {
			providerName := parts[1]
			apiKey := ""
			if len(parts) >= 3 {
				apiKey = parts[2]
			}
			var err error
			var newP iteragent.Provider
			if apiKey != "" {
				newP, err = iteragent.NewProvider(providerName, apiKey)
			} else {
				newP, err = iteragent.NewProvider(providerName)
			}
			if err != nil {
				slog.Error("provider switch failed", "provider", providerName, "error", err)
				fmt.Printf("%serror: %s%s\n\n", colorRed, err, colorReset)
			} else {
				*p = newP
				os.Setenv("ITERATE_PROVIDER", providerName)
				_ = (*a).Close() // best-effort cleanup
				*a = makeAgent(*p, repoPath, *thinking, logger)
				fmt.Printf("%s✓ switched to %s%s\n\n", colorLime, (*p).Name(), colorReset)
			}
		}
		return true
	}
	return false
}

// printSessionSummary prints final session statistics and goodbye message.
func printSessionSummary(a *iteragent.Agent, repoPath string) {
	elapsed := time.Since(sess.Start).Round(time.Second)
	if len(a.Messages) > 0 {
		_ = saveSession("autosave", a.Messages) // best-effort cleanup
	}
	fmt.Println()
	fmt.Printf("%s  session:%s %s%s%s %s·%s %s%d messages%s %s·%s %s%d tokens%s\n",
		colorDim, colorReset,
		colorCyan, elapsed, colorReset,
		colorDim, colorReset,
		colorDim, sess.Messages, colorReset,
		colorDim, colorReset,
		colorPurple, sess.InputTokens+sess.OutputTokens, colorReset)
	if len(a.Messages) > 0 {
		fmt.Printf("%s  autosaved · /load autosave to restore%s\n", colorDim, colorReset)
	}
	fmt.Printf("%s  Goodbye! %sSee you next time%s 🌱\n\n", colorLime, colorCyan, colorReset)
}

func printHeader(p iteragent.Provider, thinking iteragent.ThinkingLevel, repoPath string) {
	fmt.Println()

	// ASCII logo
	fmt.Printf("%s", colorLime)
	fmt.Println("   ___ _                 _       ")
	fmt.Println("  |_ _| |_ ___ _ __ __ _| |_ ___ ")
	fmt.Println("   | || __/ _ \\ '__/ _` | __/ _ \\")
	fmt.Println("   | || ||  __/ | | (_| | ||  __/")
	fmt.Println("  |___|\\_\\___|_|  \\__,_|\\__\\___|")
	fmt.Printf("%s", colorReset)
	fmt.Println()

	// Tagline
	fmt.Printf("  %sSelf-Evolving Coding Agent%s\n", colorBold, colorReset)
	fmt.Println()

	printHeaderGit(repoPath)
	printHeaderConfig(p, thinking)

	fmt.Println()

	// Keyboard hints
	fmt.Printf("  %s/help%s · %sTab%s complete · %s↑↓%s history · %sCtrl+R%s search · %sCtrl+C%s exit\n",
		colorCyan, colorDim,
		colorCyan, colorDim,
		colorCyan, colorDim,
		colorCyan, colorDim,
		colorCyan, colorReset)
	fmt.Println()
}

func printHeaderGit(repoPath string) {
	home, _ := os.UserHomeDir()
	cwd := repoPath
	if strings.HasPrefix(cwd, home) {
		cwd = "~" + cwd[len(home):]
	}
	fmt.Printf("  %s%s%s", colorBold, cwd, colorReset)

	if out, err := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output(); err == nil {
		branch := strings.TrimSpace(string(out))
		if branch != "" {
			fmt.Printf("  %s(%s)%s", colorLime, branch, colorReset)
		}
	}

	staged, unstaged := selector.GitStatus()
	if staged+unstaged > 0 {
		if staged > 0 && unstaged > 0 {
			fmt.Printf("  %s+%d staged%s  %s±%d unstaged%s", colorGreen, staged, colorReset, colorYellow, unstaged, colorReset)
		} else if staged > 0 {
			fmt.Printf("  %s+%d staged%s", colorGreen, staged, colorReset)
		} else {
			fmt.Printf("  %s±%d unstaged%s", colorYellow, unstaged, colorReset)
		}
	}

	fmt.Println()
}

func printHeaderConfig(p iteragent.Provider, thinking iteragent.ThinkingLevel) {
	modelName := os.Getenv("ITERATE_MODEL")
	if modelName == "" {
		modelName = p.Name()
	}
	thinkingStr := ""
	if thinking != iteragent.ThinkingLevelOff && thinking != "" {
		thinkingStr = fmt.Sprintf("  %sthinking:%s%s%s", colorDim, colorCyan, thinking, colorReset)
	}
	safeModeStr := ""
	if cfg.SafeMode {
		safeModeStr = fmt.Sprintf("  %s🔒 safe mode%s", colorCyan, colorReset)
	}
	fmt.Printf("  %s%s%s%s%s\n", colorDim, modelName, thinkingStr, safeModeStr, colorReset)
}

// loadBookmarksWrapper converts main package Bookmarks to commands.Bookmarks.
func loadBookmarksWrapper() []commands.Bookmark {
	bms := loadBookmarks()
	result := make([]commands.Bookmark, len(bms))
	for i, b := range bms {
		result[i] = commands.Bookmark{
			Name:      b.Name,
			CreatedAt: b.CreatedAt,
			Messages:  b.Messages,
		}
	}
	return result
}

// handleCommand processes a slash command. Returns true if the REPL should exit.
func handleCommand(ctx context.Context, line string, a *iteragent.Agent, p iteragent.Provider, repoPath string, thinking *iteragent.ThinkingLevel, logger *slog.Logger) bool {
	parts := strings.Fields(line)
	cmd := strings.ToLower(parts[0])

	// Try modular command registry first
	cmdCtx := buildCommandContext(repoPath, line, parts, p, a, thinking)

	if result := commands.DefaultRegistry().Execute(cmd, cmdCtx); result.Handled {
		return result.Done
	}

	slog.Debug("unknown command", "command", cmd, "line", line)
	fmt.Printf("Unknown command: %s (try /help)\n", cmd)
	return false
}

func buildCommandContext(repoPath, line string, parts []string, p iteragent.Provider, a *iteragent.Agent, thinking *iteragent.ThinkingLevel) commands.Context {
	return commands.Context{
		RepoPath:            repoPath,
		Line:                line,
		Parts:               parts,
		Version:             iterateVersion,
		Provider:            p,
		Agent:               a,
		Thinking:            thinking,
		SafeMode:            &cfg.SafeMode,
		AutoCommitEnabled:   &cfg.AutoCommitEnabled,
		DeniedTools:         nil,
		SessionInputTokens:  &sess.InputTokens,
		SessionOutputTokens: &sess.OutputTokens,
		SessionCacheRead:    &sess.CacheRead,
		SessionCacheWrite:   &sess.CacheWrite,
		InputHistory:        selector.InputHistoryRef,
		StopWatch:           stopWatch,
		Pool:                agentPool,
		Session: commands.SessionCallbacks{
			SaveSession:   saveSession,
			LoadSession:   loadSession,
			ListSessions:  listSessions,
			AddBookmark:   addBookmark,
			LoadBookmarks: loadBookmarksWrapper,
			SelectItem:    selector.SelectItem,
		},
		REPL: commands.REPLCallbacks{
			StreamAndPrint: streamAndPrint,
			RunShell:       runShell,
			PromptLine:     selector.PromptLine,
		},
		State: commands.StateAccessors{
			IsDenied:             isDenied,
			DenyTool:             denyTool,
			AllowTool:            allowTool,
			GetDeniedList:        getDeniedList,
			GetConversationMark:  getConversationMark,
			SetConversationMark:  setConversationMark,
			GetConversationMarks: getConversationMarks,
			ConversationMarksLen: conversationMarksLen,
			GetPinnedMessages:    getPinnedMessages,
			SetPinnedMessages:    setPinnedMessages,
		},
		Templates: commands.TemplateCallbacks{
			FormatSessionChanges: sessionChanges.format,
		},
		PersistConfig: func() {
			existing := loadConfig()
			existing.SafeMode = cfg.SafeMode
			existing.DeniedTools = getDeniedList()
			saveConfig(existing)
		},
	}
}
