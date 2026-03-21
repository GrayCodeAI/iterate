package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/agent"
	"github.com/GrayCodeAI/iterate/internal/commands"
	"github.com/GrayCodeAI/iterate/internal/evolution"
)

// fetchOllamaModels fetches the list of model names from an Ollama /api/tags endpoint.
func fetchOllamaModels(tagsURL string) ([]string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(tagsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	names := make([]string, len(result.Models))
	for i, m := range result.Models {
		names[i] = m.Name
	}
	return names, nil
}

// Color variables — reassignable for /theme support
var (
	colorReset  = "\033[0m"
	colorLime   = "\033[38;5;154m"
	colorYellow = "\033[38;5;220m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
	colorCyan   = "\033[36m"
	colorRed    = "\033[31m"
)

// replRepoPath is the repo path used in the current REPL session (for prompt display).
var replRepoPath string

// sessionTokens tracks approximate tokens used this session.
var sessionTokens int

// Detailed session token counters (from Usage metadata when available).
var sessionInputTokens int
var sessionOutputTokens int
var sessionCacheRead int
var sessionCacheWrite int

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
	ag := iteragent.New(p, tools, logger).
		WithSystemPrompt(replSystemPrompt(repoPath)).
		WithSkillSet(skills).
		WithThinkingLevel(thinking).
		WithToolExecutionStrategy(iteragent.NewParallelStrategy()).
		WithHooks(replHooks())
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
			if debugMode {
				fmt.Printf("%s[debug] turn %d — %d messages in context%s\n",
					colorDim, turn, len(messages), colorReset)
			}
		},
		AfterTurn: func(turn int, response string) {
			if debugMode {
				elapsed := time.Since(turnStart).Round(time.Millisecond)
				fmt.Printf("%s[debug] turn %d done in %s (%d chars)%s\n",
					colorDim, turn, elapsed, len(response), colorReset)
			}
		},
		OnToolStart: func(toolName string, args map[string]string) {
			toolStart = time.Now()
			if debugMode {
				fmt.Printf("%s[debug] → %s%s\n", colorDim, toolName, colorReset)
			}
		},
		OnToolEnd: func(toolName string, result string, err error) {
			if debugMode {
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

// runREPL runs an interactive session with iterate.
func runREPL(ctx context.Context, p iteragent.Provider, repoPath string, thinking iteragent.ThinkingLevel, logger *slog.Logger) {
	initHistory()
	initAuditLog()
	cfg := loadConfig()
	safeMode = cfg.SafeMode
	notifyEnabled = cfg.Notify
	if cfg.Theme != "" {
		if t, ok := themes[cfg.Theme]; ok {
			applyTheme(t)
		}
	}
	if len(cfg.DeniedTools) > 0 {
		deniedTools = make(map[string]bool, len(cfg.DeniedTools))
		for _, t := range cfg.DeniedTools {
			deniedTools[t] = true
		}
	}

	// Seed runtime config from persisted values so /set overrides start from saved state.
	if cfg.Temperature > 0 {
		t := float32(cfg.Temperature)
		rtConfig.Temperature = &t
	}
	if cfg.MaxTokens > 0 {
		rtConfig.MaxTokens = &cfg.MaxTokens
	}
	if cfg.CacheEnabled {
		enabled := true
		rtConfig.CacheEnabled = &enabled
	}
	// Apply persisted ThinkingLevel when the caller passed the default "off".
	if thinking == iteragent.ThinkingLevelOff && cfg.ThinkingLevel != "" {
		thinking = iteragent.ThinkingLevel(cfg.ThinkingLevel)
	}

	replRepoPath = repoPath

	a := makeAgent(p, repoPath, thinking, logger)
	defer func() { _ = a.Close() }()

	printHeader(p, thinking, repoPath)

	// Restore last autosave if available (but don't force — just offer info)
	if sessions := listSessions(); containsString(sessions, "autosave") {
		fmt.Printf("%s(previous session autosaved — /load autosave to restore)%s\n\n", colorDim, colorReset)
	}

	for {
		line, ok := readInput()
		if !ok {
			break
		}
		if line == "" {
			continue
		}

		// /model and /provider handled here so they can swap provider+agent.
		if line == "/model" || strings.HasPrefix(line, "/model ") {
			newP, newThinking := selectModel(thinking)
			if newP != nil {
				p = newP
				thinking = newThinking
				_ = a.Close()
				a = makeAgent(p, repoPath, thinking, logger)
				fmt.Printf("%s✓ switched to %s%s\n\n", colorLime, p.Name(), colorReset)
				saveConfig(iterConfig{
					Provider:      os.Getenv("ITERATE_PROVIDER"),
					Model:         os.Getenv("ITERATE_MODEL"),
					OllamaBaseURL: os.Getenv("OLLAMA_BASE_URL"),
				})
			}
			continue
		}
		if strings.HasPrefix(line, "/provider") {
			parts := strings.Fields(line)
			if len(parts) == 1 {
				fmt.Printf("Current provider: %s%s%s\n", colorLime, p.Name(), colorReset)
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
					fmt.Printf("%serror: %s%s\n\n", colorRed, err, colorReset)
				} else {
					p = newP
					os.Setenv("ITERATE_PROVIDER", providerName)
					_ = a.Close()
					a = makeAgent(p, repoPath, thinking, logger)
					fmt.Printf("%s✓ switched to %s%s\n\n", colorLime, p.Name(), colorReset)
				}
			}
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

	// Ctrl+C exit — auto-save and stop watch
	stopWatch()
	if len(a.Messages) > 0 {
		_ = saveSession("autosave", a.Messages)
		fmt.Printf("\n%s✓ session saved · /load autosave to restore%s\n", colorDim, colorReset)
	}
	fmt.Printf("%s  iterate%s · see you next time\n", colorLime, colorReset)
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

	// Working directory (shortened)
	home, _ := os.UserHomeDir()
	cwd := repoPath
	if strings.HasPrefix(cwd, home) {
		cwd = "~" + cwd[len(home):]
	}
	fmt.Printf("  %s%s%s", colorBold, cwd, colorReset)

	// Git branch
	if out, err := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output(); err == nil {
		branch := strings.TrimSpace(string(out))
		if branch != "" {
			fmt.Printf("  %s(%s)%s", colorLime, branch, colorReset)
		}
	}
	fmt.Println()

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

// selectModel shows an interactive provider+model picker. Returns new provider or nil on cancel.
func selectModel(currentThinking iteragent.ThinkingLevel) (iteragent.Provider, iteragent.ThinkingLevel) {
	providers := []string{"anthropic", "openai", "gemini", "groq", "ollama"}

	fmt.Println()
	providerName, ok := selectItem("Select provider", providers)
	if !ok {
		return nil, currentThinking
	}

	if providerName == "ollama" {
		os.Setenv("ITERATE_PROVIDER", "ollama")
		return selectOllamaModel(currentThinking)
	}

	apiKey, ok := promptLine("API key (Enter to use env var, ESC to cancel):")
	if !ok {
		return nil, currentThinking
	}

	var newP iteragent.Provider
	var err error
	if apiKey != "" {
		newP, err = iteragent.NewProvider(providerName, apiKey)
	} else {
		newP, err = iteragent.NewProvider(providerName)
	}
	if err != nil {
		fmt.Printf("%serror: %s%s\n\n", colorRed, err, colorReset)
		return nil, currentThinking
	}
	os.Setenv("ITERATE_PROVIDER", providerName)
	return newP, currentThinking
}

// selectOllamaModel discovers Tailscale Ollama hosts, lets user pick host then model.
func selectOllamaModel(currentThinking iteragent.ThinkingLevel) (iteragent.Provider, iteragent.ThinkingLevel) {
	// Discover Tailscale machines with Ollama
	fmt.Printf("%sdiscovering Ollama hosts…%s\r\n", colorDim, colorReset)
	hosts := discoverOllamaHosts()

	var url string
	if len(hosts) > 0 {
		labels := make([]string, len(hosts))
		for i, h := range hosts {
			labels[i] = fmt.Sprintf("%-20s  %s", h.name, h.url)
		}
		labels = append(labels, "enter URL manually")

		choice, ok := selectItem("Select Ollama host", labels)
		if !ok {
			return nil, currentThinking
		}
		if choice == "enter URL manually" {
			url = ""
		} else {
			for _, h := range hosts {
				if strings.HasPrefix(choice, h.name) {
					url = h.url
					break
				}
			}
		}
	}

	if url == "" {
		currentURL := os.Getenv("OLLAMA_BASE_URL")
		if currentURL == "" {
			currentURL = "http://localhost:11434/v1"
		}
		var ok bool
		url, ok = promptLine(fmt.Sprintf("Ollama URL (Enter to keep %s):", currentURL))
		if !ok {
			return nil, currentThinking
		}
		if url == "" {
			url = currentURL
		}
	}

	os.Setenv("OLLAMA_BASE_URL", url)
	tagsURL := strings.TrimSuffix(strings.TrimSuffix(url, "/v1"), "/") + "/api/tags"
	fmt.Printf("%sfetching models…%s\r\n", colorDim, colorReset)

	models, err := fetchOllamaModels(tagsURL)
	if err != nil || len(models) == 0 {
		modelName, ok := promptLine("Enter model name:")
		if !ok || modelName == "" {
			return nil, currentThinking
		}
		os.Setenv("ITERATE_MODEL", modelName)
	} else {
		modelName, ok := selectItem("Select model", models)
		if !ok {
			return nil, currentThinking
		}
		os.Setenv("ITERATE_MODEL", modelName)
	}

	p, err := iteragent.NewProvider("ollama")
	if err != nil {
		fmt.Printf("%serror: %s%s\n\n", colorRed, err, colorReset)
		return nil, currentThinking
	}
	return p, currentThinking
}

type ollamaHost struct {
	name string
	url  string
}

// knownHosts are the fixed Tailscale machines to check for Ollama.
var knownHosts = []ollamaHost{
	{name: "agx-01", url: "http://100.102.194.103:11434/v1"},
	{name: "agx-02", url: "http://100.87.35.70:11434/v1"},
	{name: "gb10-01", url: "http://100.93.184.1:11434/v1"},
	{name: "gb10-02", url: "http://100.87.126.2:11434/v1"},
	{name: "vps-1", url: "http://100.79.60.48:11434/v1"},
}

// discoverOllamaHosts checks known Tailscale machines for running Ollama.
func discoverOllamaHosts() []ollamaHost {
	client := &http.Client{Timeout: 2 * time.Second}
	var mu sync.Mutex
	var hosts []ollamaHost
	var wg sync.WaitGroup

	for _, h := range knownHosts {
		h := h
		wg.Add(1)
		go func() {
			defer wg.Done()
			baseURL := strings.TrimSuffix(h.url, "/v1")
			resp, err := client.Get(baseURL + "/")
			if err != nil {
				return
			}
			resp.Body.Close()
			if resp.StatusCode == 200 {
				mu.Lock()
				hosts = append(hosts, h)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	sort.Slice(hosts, func(i, j int) bool { return hosts[i].name < hosts[j].name })
	return hosts
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
	cmdCtx := commands.Context{
		RepoPath:            repoPath,
		Line:                line,
		Parts:               parts,
		Provider:            p,
		Agent:               a,
		Thinking:            thinking,
		SafeMode:            &safeMode,
		DeniedTools:         deniedTools,
		SessionInputTokens:  &sessionInputTokens,
		SessionOutputTokens: &sessionOutputTokens,
		SessionCacheRead:    &sessionCacheRead,
		SessionCacheWrite:   &sessionCacheWrite,
		InputHistory:        &inputHistory,
		StopWatch:           stopWatch,
		Pool:                agentPool,

		// Session callbacks
		SaveSession:   saveSession,
		LoadSession:   loadSession,
		ListSessions:  listSessions,
		AddBookmark:   addBookmark,
		LoadBookmarks: loadBookmarksWrapper,
		SelectItem:    selectItem,
	}

	if result := commands.DefaultRegistry().Execute(cmd, cmdCtx); result.Handled {
		return result.Done
	}

	// Fall back to legacy switch for unmigrated commands
	switch cmd {
	case "/help", "/?":
		fmt.Print(`
Available commands:
  /help                  — show this help
  /clear                 — reset conversation history
  /compact               — compact conversation history
  /tools                 — list available tools
  /skills                — list available skills
  /thinking <level>      — set thinking level: off|minimal|low|medium|high
  /model                 — switch provider/model interactively
  /cost                  — show approximate token usage this session

  /save [name]           — save session to ~/.iterate/sessions/<name>.json
  /load [name]           — load a saved session (interactive picker)
  /bookmark [name]       — save current conversation as a bookmark
  /bookmarks             — list and restore bookmarks
  /history               — show recent input history

  ── Agent Modes ──────────────────────────────────────────────────────────
  /code                  — full mode with all tools (default)
  /ask                   — read-only mode (no bash/write/edit)
  /architect             — planning only, no tools at all
  /summarize             — summarize current conversation
  /review                — code review of current changes
  /explain [path]        — explain code in a file or directory

  ── Context ──────────────────────────────────────────────────────────────
  /context               — show message count + token stats
  /compact               — compact conversation history
  /rewind [n]            — remove last n exchanges (default 1)
  /fork                  — save + start fresh conversation
  /inject <text>         — inject raw text into context
  /set [key] [val]       — set temperature, max_tokens (or /set to show)
  /pin / /unpin          — pin messages to survive /compact

  ── Files & Search ───────────────────────────────────────────────────────
  /add <file>            — inject file into context
  /find <pattern>        — fuzzy file search → pick to inject
  /web <url>             — fetch URL → inject into context
  /grep <pattern>        — search code content in repo
  /todos                 — list all TODO/FIXME/HACK in codebase
  /deps                  — show go.mod dependencies

  ── Sessions & Memory ────────────────────────────────────────────────────
  /save [name]           — save session
  /load [name]           — load saved session (picker)
  /export [file]         — export conversation to markdown
  /bookmark [name]       — snapshot current conversation
  /bookmarks             — restore a bookmark
  /history               — show recent input history
  /copy                  — copy last response to clipboard
  /retry                 — retry last message
  /memo <text>           — append note to JOURNAL.md
  /learn <fact>          — add to memory/learnings.jsonl
  /memories              — show active_learnings.md

  ── Safety & Config ──────────────────────────────────────────────────────
  /safe / /trust         — enable/disable safe mode
  /allow <tool>          — remove from deny list
  /deny <tool>           — add to deny list
  /config                — show all configuration
  /cost                  — session token usage

  ── Git ──────────────────────────────────────────────────────────────────
  /diff                  — show current diff
  /log [n]               — show last n commits (default 15)
  /status                — git status + DAY_COUNT
  /branch [name]         — list branches or create new one
  /checkout [branch]     — checkout branch (interactive picker)
  /stash / /stash pop    — git stash / pop
  /merge [branch]        — merge branch into current
  /tag [name]            — list or create tag
  /revert-file <file>    — revert a file to HEAD
  /undo                  — git reset HEAD~1
  /commit <msg>          — git add -A && git commit

  ── Code Quality ─────────────────────────────────────────────────────────
  /test                  — go test ./...
  /test-file <pkg>       — go test -v <pkg>
  /build                 — go build ./...
  /lint                  — go vet ./...
  /lint-fix              — go vet + staticcheck
  /format                — go fmt ./...

  ── Dev ──────────────────────────────────────────────────────────────────
  /watch / /watch stop   — auto-run tests on file changes
  /run <cmd>             — run any shell command
  /pr                    — create pull request
  /issues                — list open GitHub issues
  /plan <task>           — plan before execute
  /phase <phase>         — run evolution phase
  /spawn <task>          — delegate to subagent (context-efficient)
  /swarm <n> <task>      — launch N agents with rate limiting (max 100)
  /model                 — switch provider/model
  /thinking <level>      — set thinking level
  /skills / /tools       — list available skills/tools

  /quit                  — exit (auto-saves session)
  Tab                    — autocomplete · ↑↓ history
`)

	case "/quit", "/exit", "/q":
		stopWatch()
		if len(a.Messages) > 0 {
			_ = saveSession("autosave", a.Messages)
		}
		fmt.Printf("%sbye 🌱%s\n", colorLime, colorReset)
		return true

	case "/clear":
		a.Reset()
		fmt.Println("Conversation cleared.")

	case "/tools":
		tools := a.GetTools()
		fmt.Printf("%d tools:\n", len(tools))
		for _, t := range tools {
			desc := strings.SplitN(t.Description, "\n", 2)[0]
			fmt.Printf("  %-20s %s\n", t.Name, desc)
		}

	case "/skills":
		skills, _ := iteragent.LoadSkills([]string{filepath.Join(repoPath, "skills")})
		if len(skills.Skills) == 0 {
			fmt.Println("No skills found.")
		} else {
			fmt.Printf("%d skills:\n", len(skills.Skills))
			for _, s := range skills.Skills {
				fmt.Printf("  %-20s %s\n", s.Name, s.Description)
			}
		}

	case "/thinking":
		if len(parts) < 2 {
			fmt.Printf("Current thinking level: %s\n", *thinking)
			fmt.Println("Usage: /thinking off|minimal|low|medium|high")
			return false
		}
		*thinking = iteragent.ThinkingLevel(parts[1])
		a.WithThinkingLevel(*thinking)
		fmt.Printf("Thinking set to %s.\n", *thinking)

	case "/diff":
		showGitDiffEnhanced(repoPath)

	case "/cost":
		model := p.Name()
		fmt.Printf("%s── Cost estimate ───────────────────%s\n", colorDim, colorReset)
		fmt.Print(formatCostTable(sessionInputTokens, sessionOutputTokens, sessionCacheWrite, sessionCacheRead, model))
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	case "/tokens":
		const windowSize = 200000
		fmt.Printf("%s── Token usage ─────────────────────%s\n", colorDim, colorReset)
		fmt.Printf("  %s\n", contextBar(a.Messages, windowSize))
		fmt.Printf("  Session input:  ~%d tokens\n", sessionInputTokens)
		fmt.Printf("  Session output: ~%d tokens\n", sessionOutputTokens)
		if sessionCacheRead > 0 || sessionCacheWrite > 0 {
			fmt.Printf("  Cache read:     ~%d tokens\n", sessionCacheRead)
			fmt.Printf("  Cache write:    ~%d tokens\n", sessionCacheWrite)
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	case "/safe":
		safeMode = true
		cfg := loadConfig()
		cfg.SafeMode = true
		saveConfig(cfg)
		fmt.Printf("%s✓ Safe mode on — will ask before bash/write_file/edit_file%s\n\n", colorLime, colorReset)

	case "/trust":
		safeMode = false
		cfg := loadConfig()
		cfg.SafeMode = false
		saveConfig(cfg)
		fmt.Printf("%s✓ Trust mode — tools run without confirmation%s\n\n", colorLime, colorReset)

	case "/allow":
		if len(parts) < 2 {
			fmt.Println("Usage: /allow <tool>")
			return false
		}
		delete(deniedTools, parts[1])
		cfg := loadConfig()
		var list []string
		for t := range deniedTools {
			list = append(list, t)
		}
		cfg.DeniedTools = list
		saveConfig(cfg)
		fmt.Printf("%s✓ %s removed from deny list%s\n\n", colorLime, parts[1], colorReset)

	case "/deny":
		if len(parts) < 2 {
			fmt.Println("Usage: /deny <tool>")
			return false
		}
		deniedTools[parts[1]] = true
		cfg := loadConfig()
		var list []string
		for t := range deniedTools {
			list = append(list, t)
		}
		cfg.DeniedTools = list
		saveConfig(cfg)
		fmt.Printf("%s✓ %s added to deny list%s\n\n", colorLime, parts[1], colorReset)

	case "/save":
		name := "default"
		if len(parts) >= 2 {
			name = parts[1]
		}
		if err := saveSession(name, a.Messages); err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ session saved as \"%s\"%s\n\n", colorLime, name, colorReset)
		}

	case "/load":
		sessions := listSessions()
		if len(sessions) == 0 {
			fmt.Println("No saved sessions. Use /save to create one.")
			return false
		}
		var pick string
		if len(parts) >= 2 {
			pick = parts[1]
		} else {
			var ok bool
			pick, ok = selectItem("Select session to load", sessions)
			if !ok {
				return false
			}
		}
		msgs, err := loadSession(pick)
		if err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
			return false
		}
		a.Messages = msgs
		fmt.Printf("%s✓ loaded session \"%s\" (%d messages)%s\n\n", colorLime, pick, len(msgs), colorReset)

	case "/bookmark":
		name := time.Now().Format("2006-01-02T15:04")
		if len(parts) >= 2 {
			name = strings.Join(parts[1:], " ")
		}
		addBookmark(name, a.Messages)
		fmt.Printf("%s✓ bookmark \"%s\" saved%s\n\n", colorLime, name, colorReset)

	case "/bookmarks":
		bms := loadBookmarks()
		if len(bms) == 0 {
			fmt.Println("No bookmarks. Use /bookmark [name] to save one.")
			return false
		}
		labels := make([]string, len(bms))
		for i, b := range bms {
			labels[i] = fmt.Sprintf("%-30s  %s  (%d msgs)", b.Name, b.CreatedAt.Format("01-02 15:04"), len(b.Messages))
		}
		choice, ok := selectItem("Select bookmark to restore", labels)
		if !ok {
			return false
		}
		for i, label := range labels {
			if label == choice {
				a.Messages = bms[i].Messages
				fmt.Printf("%s✓ restored bookmark \"%s\"%s\n\n", colorLime, bms[i].Name, colorReset)
				break
			}
		}

	case "/history":
		if len(inputHistory) == 0 {
			fmt.Println("No history yet.")
			return false
		}
		start := 0
		if len(inputHistory) > 20 {
			start = len(inputHistory) - 20
		}
		for i, h := range inputHistory[start:] {
			fmt.Printf("  %s%3d%s  %s\n", colorDim, start+i+1, colorReset, h)
		}
		fmt.Println()

	case "/add":
		if len(parts) < 2 {
			fmt.Println("Usage: /add <filepath>")
			return false
		}
		filePath := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		// Allow both absolute and repo-relative paths
		absPath := filePath
		if !filepath.IsAbs(filePath) {
			absPath = filepath.Join(repoPath, filePath)
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
			return false
		}
		ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
		msg := fmt.Sprintf("File: %s\n```%s\n%s\n```", filePath, ext, string(data))
		streamAndPrint(ctx, a, msg, repoPath)

	case "/find":
		if len(parts) < 2 {
			fmt.Println("Usage: /find <pattern>")
			return false
		}
		pattern := strings.Join(parts[1:], " ")
		results := findFiles(repoPath, pattern)
		if len(results) == 0 {
			fmt.Printf("No files matching %q\n\n", pattern)
			return false
		}
		// Let user pick a file to /add
		choice, ok := selectItem(fmt.Sprintf("Files matching %q", pattern), results)
		if !ok {
			return false
		}
		data, err := os.ReadFile(filepath.Join(repoPath, choice))
		if err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
			return false
		}
		ext := strings.TrimPrefix(filepath.Ext(choice), ".")
		msg := fmt.Sprintf("File: %s\n```%s\n%s\n```", choice, ext, string(data))
		fmt.Printf("%s+ added %s to context%s\n\n", colorLime, choice, colorReset)
		streamAndPrint(ctx, a, msg, repoPath)

	case "/pr":
		handlePR(ctx, line, a, repoPath)

	case "/web":
		if len(parts) < 2 {
			fmt.Println("Usage: /web <url>")
			return false
		}
		url := parts[1]
		fmt.Printf("%sfetching %s…%s\n", colorDim, url, colorReset)
		text, err := fetchURL(url)
		if err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
			return false
		}
		if len(text) > 8000 {
			text = text[:8000] + "\n…[truncated]"
		}
		msg := fmt.Sprintf("Web page content from %s:\n\n%s", url, text)
		fmt.Printf("%s✓ fetched %d chars — injecting into context%s\n\n", colorLime, len(text), colorReset)
		streamAndPrint(ctx, a, msg, repoPath)

	case "/grep":
		if len(parts) < 2 {
			fmt.Println("Usage: /grep <pattern>")
			return false
		}
		pattern := strings.Join(parts[1:], " ")
		result, err := grepRepo(repoPath, pattern)
		if err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
			return false
		}
		fmt.Printf("%s── grep: %s ──%s\n%s\n\n", colorDim, pattern, colorReset, result)

	case "/context":
		const windowSize = 200000
		fmt.Printf("%s── Context ─────────────────────────%s\n", colorDim, colorReset)
		fmt.Printf("  %s\n", contextStats(a.Messages))
		fmt.Printf("  %s\n", contextBar(a.Messages, windowSize))
		if len(pinnedMessages) > 0 {
			fmt.Printf("  Pinned: %d messages\n", len(pinnedMessages))
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	case "/export":
		name := fmt.Sprintf("iterate-export-%s.md", time.Now().Format("2006-01-02-150405"))
		if len(parts) >= 2 {
			name = parts[1]
		}
		if err := exportConversation(a.Messages, filepath.Join(repoPath, name)); err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ exported to %s%s\n\n", colorLime, name, colorReset)
		}

	case "/retry":
		if lastPrompt == "" {
			fmt.Println("No previous message to retry.")
			return false
		}
		// Remove last exchange from history so we don't duplicate
		if len(a.Messages) >= 2 {
			a.Messages = a.Messages[:len(a.Messages)-2]
		}
		fmt.Printf("%sRetrying: %s%s\n\n", colorDim, lastPrompt, colorReset)
		streamAndPrint(ctx, a, lastPrompt, repoPath)

	case "/copy":
		if lastResponse == "" {
			fmt.Println("No response to copy.")
			return false
		}
		if err := copyToClipboard(lastResponse); err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ copied to clipboard (%d chars)%s\n\n", colorLime, len(lastResponse), colorReset)
		}

	case "/todos":
		todos := findTodos(repoPath)
		if len(todos) == 0 {
			fmt.Println("No TODO/FIXME/HACK comments found.")
			return false
		}
		fmt.Printf("%s── TODOs ──────────────────────────%s\n", colorDim, colorReset)
		for _, t := range todos {
			fmt.Printf("  %s\n", t)
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	case "/watch":
		if len(parts) >= 2 && parts[1] == "stop" {
			stopWatch()
			fmt.Printf("%s[watch] stopped%s\n\n", colorDim, colorReset)
		} else {
			startWatch(repoPath)
			fmt.Printf("%s[watch] started — runs go test on every file change (type /watch stop to halt)%s\n\n", colorLime, colorReset)
		}

	case "/issues":
		limit := 10
		result, err := listGitHubIssues(repoPath, limit)
		if err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
			return false
		}
		if result == "" {
			fmt.Println("No open issues.")
			return false
		}
		fmt.Printf("%s── Open Issues ────────────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
			colorDim, colorReset, result, colorDim, colorReset)

	case "/pin":
		if len(a.Messages) == 0 {
			fmt.Println("No messages to pin.")
			return false
		}
		// Pin last assistant message
		last := a.Messages[len(a.Messages)-1]
		pinnedMessages = append(pinnedMessages, last)
		fmt.Printf("%s✓ message pinned (%d pinned total) — will survive /compact%s\n\n",
			colorLime, len(pinnedMessages), colorReset)

	case "/unpin":
		pinnedMessages = nil
		fmt.Printf("%s✓ all pins cleared%s\n\n", colorLime, colorReset)

	case "/config":
		cfg := loadConfig()
		safe := "off"
		if safeMode {
			safe = "on"
		}
		var denied []string
		for t := range deniedTools {
			denied = append(denied, t)
		}
		fmt.Printf("%s── Config ─────────────────────────%s\n", colorDim, colorReset)
		fmt.Printf("  Provider:    %s\n", cfg.Provider)
		fmt.Printf("  Model:       %s\n", cfg.Model)
		if cfg.OllamaBaseURL != "" {
			fmt.Printf("  Ollama URL:  %s\n", cfg.OllamaBaseURL)
		}
		fmt.Printf("  Safe mode:   %s\n", safe)
		if len(denied) > 0 {
			fmt.Printf("  Denied:      %s\n", strings.Join(denied, ", "))
		}
		fmt.Printf("  Config file: %s\n", configPath())
		fmt.Printf("  History:     %s\n", historyFile)
		fmt.Printf("  Sessions:    %s\n", sessionsDir())
		fmt.Printf("  Audit log:   %s\n", auditLogPath)
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	case "/run":
		if len(parts) < 2 {
			fmt.Println("Usage: /run <shell command>")
			return false
		}
		shellCmd := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		cmd := exec.Command("bash", "-c", shellCmd)
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("%sexit: %v%s\n", colorRed, err, colorReset)
		}
		fmt.Println()

	// ── Agent modes ─────────────────────────────────────────────────────────

	case "/ask":
		currentMode = modeAsk
		_ = a.Close()
		a = makeAgent(p, repoPath, *thinking, logger)
		fmt.Printf("%s✓ Ask mode — read-only (no bash/write). /code to exit.%s\n\n", colorCyan, colorReset)

	case "/architect":
		currentMode = modeArchitect
		_ = a.Close()
		a = makeAgent(p, repoPath, *thinking, logger)
		fmt.Printf("%s✓ Architect mode — planning only, no tools. /code to exit.%s\n\n", colorPurple, colorReset)

	case "/code":
		currentMode = modeNormal
		_ = a.Close()
		a = makeAgent(p, repoPath, *thinking, logger)
		fmt.Printf("%s✓ Code mode — all tools enabled.%s\n\n", colorLime, colorReset)

	case "/summarize":
		streamAndPrint(ctx, a, buildSummarizePrompt(a.Messages), repoPath)

	case "/review":
		streamAndPrint(ctx, a, buildReviewPrompt(repoPath), repoPath)

	// ── Context control ──────────────────────────────────────────────────────

	case "/rewind":
		n := 1
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &n)
		}
		remove := n * 2 // each exchange = user + assistant
		if remove > len(a.Messages) {
			remove = len(a.Messages)
		}
		a.Messages = a.Messages[:len(a.Messages)-remove]
		fmt.Printf("%s✓ rewound %d exchange(s) — %d messages remain%s\n\n",
			colorLime, n, len(a.Messages), colorReset)

	case "/fork":
		_ = saveSession(fmt.Sprintf("fork-%s", time.Now().Format("20060102-150405")), a.Messages)
		a.Reset()
		fmt.Printf("%s✓ conversation forked (saved) — starting fresh%s\n\n", colorLime, colorReset)

	case "/inject":
		text := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if text == "" {
			fmt.Println("Usage: /inject <text to add to context>")
			return false
		}
		a.Messages = append(a.Messages, iteragent.Message{
			Role:    "user",
			Content: text,
		})
		fmt.Printf("%s✓ injected into context%s\n\n", colorLime, colorReset)

	case "/set":
		if len(parts) < 3 {
			temp := "default"
			if rtConfig.Temperature != nil {
				temp = fmt.Sprintf("%.2f", *rtConfig.Temperature)
			}
			maxt := "default"
			if rtConfig.MaxTokens != nil {
				maxt = fmt.Sprintf("%d", *rtConfig.MaxTokens)
			}
			fmt.Printf("%s── Runtime config ──────────────────%s\n", colorDim, colorReset)
			fmt.Printf("  temperature:  %s\n", temp)
			fmt.Printf("  max_tokens:   %s\n", maxt)
			fmt.Printf("%sUsage: /set temperature 0.7 | /set max_tokens 4096 | /set reset%s\n\n", colorDim, colorReset)
			return false
		}
		switch parts[1] {
		case "temperature", "temp":
			var v float64
			fmt.Sscanf(parts[2], "%f", &v)
			f := float32(v)
			rtConfig.Temperature = &f
			_ = a.Close()
			a = makeAgent(p, repoPath, *thinking, logger)
			fmt.Printf("%s✓ temperature = %.2f%s\n\n", colorLime, f, colorReset)
		case "max_tokens", "max-tokens", "tokens":
			var v int
			fmt.Sscanf(parts[2], "%d", &v)
			rtConfig.MaxTokens = &v
			_ = a.Close()
			a = makeAgent(p, repoPath, *thinking, logger)
			fmt.Printf("%s✓ max_tokens = %d%s\n\n", colorLime, v, colorReset)
		case "reset":
			rtConfig = runtimeConfig{}
			_ = a.Close()
			a = makeAgent(p, repoPath, *thinking, logger)
			fmt.Printf("%s✓ runtime config reset to defaults%s\n\n", colorLime, colorReset)
		default:
			fmt.Printf("Unknown setting: %s (try: temperature, max_tokens, reset)\n", parts[1])
		}

	// ── Memory & journal ─────────────────────────────────────────────────────

	case "/memo":
		text := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if text == "" {
			fmt.Println("Usage: /memo <text>")
			return false
		}
		if err := appendMemo(repoPath, text); err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ memo added to JOURNAL.md%s\n\n", colorLime, colorReset)
		}

	case "/learn":
		fact := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if fact == "" {
			fmt.Println("Usage: /learn <fact or lesson>")
			return false
		}
		if err := appendLearning(repoPath, fact); err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ added to memory/learnings.jsonl%s\n\n", colorLime, colorReset)
		}

	case "/memories":
		// Show project notes first
		printProjectMemory(repoPath)
		// Then show evolution learnings
		if content := readActiveLearnings(repoPath); content != "" {
			fmt.Printf("%s── Evolution Learnings ────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
				colorDim, colorReset, content, colorDim, colorReset)
		}

	// ── Git workflow ─────────────────────────────────────────────────────────

	case "/log":
		n := 15
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &n)
		}
		fmt.Println(gitLog(repoPath, n))
		fmt.Println()

	case "/branch":
		if len(parts) < 2 {
			branches := gitBranches(repoPath)
			cur := gitCurrentBranch(repoPath)
			fmt.Printf("%s── Branches (current: %s) ─────────%s\n", colorDim, cur, colorReset)
			for _, b := range branches {
				if b == cur {
					fmt.Printf("  %s* %s%s\n", colorLime, b, colorReset)
				} else {
					fmt.Printf("    %s\n", b)
				}
			}
			fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)
			return false
		}
		runShell(repoPath, "git", "checkout", "-b", parts[1])

	case "/checkout":
		branches := gitBranches(repoPath)
		if len(branches) == 0 {
			fmt.Println("No branches found.")
			return false
		}
		var target string
		if len(parts) >= 2 {
			target = parts[1]
		} else {
			var ok bool
			target, ok = selectItem("Checkout branch", branches)
			if !ok {
				return false
			}
		}
		runShell(repoPath, "git", "checkout", target)

	case "/merge":
		branches := gitBranches(repoPath)
		var target string
		if len(parts) >= 2 {
			target = parts[1]
		} else {
			var ok bool
			target, ok = selectItem("Merge branch into current", branches)
			if !ok {
				return false
			}
		}
		runShell(repoPath, "git", "merge", target)

	case "/stash":
		pop := len(parts) >= 2 && parts[1] == "pop"
		if err := gitStash(repoPath, pop); err != nil {
			fmt.Printf("%sexit: %v%s\n", colorRed, err, colorReset)
		}
		fmt.Println()

	case "/tag":
		if len(parts) < 2 {
			tags := gitTags(repoPath)
			if len(tags) == 0 {
				fmt.Println("No tags.")
			} else {
				fmt.Printf("%s── Tags ───────────────────────────%s\n", colorDim, colorReset)
				for _, t := range tags {
					fmt.Printf("  %s\n", t)
				}
				fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)
			}
			return false
		}
		runShell(repoPath, "git", "tag", parts[1])
		fmt.Printf("%s✓ tag %s created%s\n\n", colorLime, parts[1], colorReset)

	case "/revert-file":
		if len(parts) < 2 {
			fmt.Println("Usage: /revert-file <filepath>")
			return false
		}
		file := parts[1]
		fmt.Printf("%sRevert %s to HEAD? (y/N): %s", colorYellow, file, colorReset)
		confirm, _ := promptLine("")
		if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
			runShell(repoPath, "git", "checkout", "HEAD", "--", file)
		} else {
			fmt.Println("Cancelled.")
		}

	// ── Code quality ─────────────────────────────────────────────────────────

	case "/format":
		runShell(repoPath, "go", "fmt", "./...")
		fmt.Printf("%s✓ formatted%s\n\n", colorLime, colorReset)

	case "/lint-fix":
		fmt.Printf("%sRunning go vet…%s\n", colorDim, colorReset)
		runShell(repoPath, "go", "vet", "./...")
		if commandExists("staticcheck") {
			fmt.Printf("%sRunning staticcheck…%s\n", colorDim, colorReset)
			runShell(repoPath, "staticcheck", "./...")
		}
		fmt.Println()

	case "/test-file":
		if len(parts) < 2 {
			fmt.Println("Usage: /test-file <./path/to/pkg>")
			return false
		}
		runShell(repoPath, "go", "test", "-v", parts[1])

	case "/explain":
		target := "."
		if len(parts) >= 2 {
			target = strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		}
		prompt := fmt.Sprintf("Explain the code in %s clearly and concisely. "+
			"Cover: purpose, key abstractions, data flow, and anything non-obvious.", target)
		streamAndPrint(ctx, a, prompt, repoPath)

	case "/deps":
		data, err := os.ReadFile(filepath.Join(repoPath, "go.mod"))
		if err != nil {
			fmt.Printf("%serror reading go.mod: %v%s\n", colorRed, err, colorReset)
			return false
		}
		fmt.Printf("%s── go.mod ─────────────────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
			colorDim, colorReset, string(data), colorDim, colorReset)

	// ── Aliases ──────────────────────────────────────────────────────────────

	case "/alias":
		aliases := loadAliases()
		if len(parts) == 1 {
			// List all aliases (same as /aliases)
			if len(aliases) == 0 {
				fmt.Println("No aliases. Use: /alias <name> <command>")
				return false
			}
			fmt.Printf("%s── Aliases ────────────────────────%s\n", colorDim, colorReset)
			for k, v := range aliases {
				fmt.Printf("  %-16s → %s\n", k, v)
			}
			fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)
			return false
		}
		if len(parts) < 3 {
			fmt.Println("Usage: /alias <name> <command>  or  /alias <name> delete")
			return false
		}
		name := parts[1]
		if parts[2] == "delete" {
			delete(aliases, name)
			saveAliases(aliases)
			fmt.Printf("%s✓ alias %q removed%s\n\n", colorLime, name, colorReset)
			return false
		}
		expansion := strings.Join(parts[2:], " ")
		aliases[name] = expansion
		saveAliases(aliases)
		fmt.Printf("%s✓ alias %q → %s%s\n\n", colorLime, name, expansion, colorReset)

	case "/aliases":
		aliases := loadAliases()
		if len(aliases) == 0 {
			fmt.Println("No aliases defined. Use: /alias <name> <command>")
			return false
		}
		fmt.Printf("%s── Aliases ────────────────────────%s\n", colorDim, colorReset)
		for k, v := range aliases {
			fmt.Printf("  %-16s → %s\n", k, v)
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	// ── Stats & theme ─────────────────────────────────────────────────────

	case "/stats":
		fmt.Printf("%s── Session stats ──────────────────%s\n", colorDim, colorReset)
		fmt.Printf("  %s\n", sessionStats())
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	case "/theme":
		themeNames := []string{"default", "nord", "monokai", "minimal"}
		if len(parts) >= 2 {
			t, ok := themes[parts[1]]
			if !ok {
				fmt.Printf("Unknown theme %q. Available: %s\n", parts[1], strings.Join(themeNames, ", "))
				return false
			}
			applyTheme(t)
			cfg := loadConfig()
			cfg.Theme = parts[1]
			saveConfig(cfg)
			fmt.Printf("%s✓ theme %s applied%s\n\n", colorLime, parts[1], colorReset)
		} else {
			choice, ok := selectItem("Select theme", themeNames)
			if !ok {
				return false
			}
			applyTheme(themes[choice])
			cfg := loadConfig()
			cfg.Theme = choice
			saveConfig(cfg)
			fmt.Printf("%s✓ theme %s applied%s\n\n", colorLime, choice, colorReset)
		}

	case "/notify":
		notifyEnabled = !notifyEnabled
		cfg := loadConfig()
		cfg.Notify = notifyEnabled
		saveConfig(cfg)
		state := "off"
		if notifyEnabled {
			state = "on"
		}
		fmt.Printf("%s✓ notifications %s (terminal bell on completion)%s\n\n", colorLime, state, colorReset)

	// ── Doctor & coverage ────────────────────────────────────────────────

	case "/doctor", "/health":
		pt := detectProjectType(repoPath)
		fmt.Printf("%sRunning health checks for %s project…%s\n", colorDim, pt.String(), colorReset)
		var results []healthResult
		if pt == projectTypeGo || pt == projectTypeUnknown {
			results = runDoctor(repoPath)
		} else {
			results = runProjectHealthChecks(repoPath, pt)
		}
		fmt.Printf("%s── Health ──────────────────────────%s\n", colorDim, colorReset)
		allOk := true
		for _, r := range results {
			status := colorLime + "✓" + colorReset
			if !r.ok {
				status = colorRed + "✗" + colorReset
				allOk = false
			}
			fmt.Printf("  %s  %-20s  %s%s%s\n", status, r.check, colorDim, r.detail, colorReset)
		}
		fmt.Printf("%s──────────────────────────────────%s\n", colorDim, colorReset)
		if allOk {
			fmt.Printf("%s✓ all checks pass%s\n\n", colorLime, colorReset)
		} else {
			fmt.Printf("%s✗ some checks failed — use /fix to auto-repair%s\n\n", colorRed, colorReset)
		}

	case "/coverage":
		fmt.Printf("%sRunning tests with coverage…%s\n", colorDim, colorReset)
		out, err := runCoverage(repoPath)
		if err != nil {
			fmt.Printf("%sTests failed:%s\n%s\n\n", colorRed, colorReset, out)
		} else {
			fmt.Printf("%s── Coverage ────────────────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
				colorDim, colorReset, out, colorDim, colorReset)
		}

	case "/mutants":
		fmt.Printf("%sRunning mutation tests…%s\n", colorDim, colorReset)
		fmt.Printf("%sThis finds untested code paths by mutating code and checking if tests catch it.%s\n\n", colorDim, colorReset)
		// Use the agent to run mutation testing tool
		mutantPrompt := "Run the mutation_test tool to analyze test quality. Report which mutations survived (indicating weak tests) and suggest improvements."
		streamAndPrint(ctx, a, mutantPrompt, repoPath)
		fmt.Printf("\n")

	case "/day":
		dayFile := filepath.Join(repoPath, "DAY_COUNT")
		currentDay := "1"
		if data, err := os.ReadFile(dayFile); err == nil && len(data) > 0 {
			currentDay = strings.TrimSpace(string(data))
		}
		if len(parts) < 2 {
			// Show current day
			fmt.Printf("%sCurrent day: %s%s\n\n", colorLime, currentDay, colorReset)
			return false
		}
		// Set new day
		newDay := parts[1]
		if _, err := fmt.Sscanf(newDay, "%s", &newDay); err != nil {
			fmt.Printf("%sUsage: /day [number]%s\n\n", colorRed, colorReset)
			return false
		}
		if err := os.WriteFile(dayFile, []byte(newDay), 0644); err != nil {
			fmt.Printf("%sFailed to update day: %v%s\n\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%sDay updated: %s → %s%s\n\n", colorLime, currentDay, newDay, colorReset)
		}

	// ── Templates ────────────────────────────────────────────────────────

	case "/templates":
		ts := loadTemplates()
		if len(ts) == 0 {
			fmt.Println("No templates. Use: /save-template <name> (saves last prompt)")
			return false
		}
		labels := make([]string, len(ts))
		for i, t := range ts {
			labels[i] = fmt.Sprintf("%-20s  %s", t.Name, t.Created.Format("01-02"))
		}
		choice, ok := selectItem("Select template", labels)
		if !ok {
			return false
		}
		for i, label := range labels {
			if label == choice {
				fmt.Printf("%sUsing template: %s%s\n\n", colorDim, ts[i].Name, colorReset)
				streamAndPrint(ctx, a, ts[i].Prompt, repoPath)
				break
			}
		}

	case "/save-template":
		if lastPrompt == "" {
			fmt.Println("No previous prompt to save.")
			return false
		}
		name := time.Now().Format("2006-01-02-150405")
		if len(parts) >= 2 {
			name = strings.Join(parts[1:], " ")
		}
		addTemplate(name, lastPrompt)
		fmt.Printf("%s✓ template %q saved%s\n\n", colorLime, name, colorReset)

	case "/template":
		if len(parts) < 2 {
			fmt.Println("Usage: /template <name>  (use /templates to browse)")
			return false
		}
		name := strings.Join(parts[1:], " ")
		ts := loadTemplates()
		for _, t := range ts {
			if strings.EqualFold(t.Name, name) {
				streamAndPrint(ctx, a, t.Prompt, repoPath)
				return false
			}
		}
		fmt.Printf("Template %q not found. Use /templates to browse.\n", name)

	// ── Multi-line input ──────────────────────────────────────────────────

	case "/multi":
		text, ok := readMultiLine()
		if !ok || strings.TrimSpace(text) == "" {
			fmt.Println("Cancelled.")
			return false
		}
		streamAndPrint(ctx, a, text, repoPath)

	// ── Search & replace ─────────────────────────────────────────────────

	case "/search-replace":
		if len(parts) < 3 {
			fmt.Println("Usage: /search-replace <old> <new>")
			return false
		}
		oldText := parts[1]
		newText := parts[2]
		fmt.Printf("%sReplace all occurrences of %q with %q? (y/N): %s", colorYellow, oldText, newText, colorReset)
		confirm, _ := promptLine("")
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			fmt.Println("Cancelled.")
			return false
		}
		n, err := searchReplace(repoPath, oldText, newText)
		if err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ replaced in %d file(s)%s\n\n", colorLime, n, colorReset)
		}

	// ── More git ──────────────────────────────────────────────────────────

	case "/blame":
		if len(parts) < 2 {
			fmt.Println("Usage: /blame <file>")
			return false
		}
		runShell(repoPath, "git", "blame", "--color-lines", parts[1])

	case "/show":
		ref := "HEAD"
		if len(parts) >= 2 {
			ref = parts[1]
		}
		runShell(repoPath, "git", "show", "--stat", "--color", ref)

	case "/cherry-pick":
		if len(parts) < 2 {
			fmt.Println("Usage: /cherry-pick <commit-hash>")
			return false
		}
		runShell(repoPath, "git", "cherry-pick", parts[1])

	// ── GitHub PR ─────────────────────────────────────────────────────────

	case "/pr-list":
		// Alias for /pr list
		handlePR(ctx, "/pr list", a, repoPath)

	case "/pr-review":
		// Alias for /pr review [N]
		arg := ""
		if len(parts) >= 2 {
			arg = parts[1]
		}
		handlePR(ctx, "/pr review "+arg, a, repoPath)

	// ── AI helpers ────────────────────────────────────────────────────────

	case "/changelog":
		since := ""
		if len(parts) >= 2 {
			since = parts[1]
		}
		streamAndPrint(ctx, a, buildChangelogPrompt(repoPath, since), repoPath)

	case "/docs":
		target := "."
		if len(parts) >= 2 {
			target = strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		}
		prompt := fmt.Sprintf(
			"Generate comprehensive documentation for %s. Include: overview, function signatures, "+
				"parameters, return values, and usage examples. Format as markdown.", target)
		streamAndPrint(ctx, a, prompt, repoPath)

	case "/test-gen":
		if len(parts) < 2 {
			fmt.Println("Usage: /test-gen <file>")
			return false
		}
		target := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		prompt := fmt.Sprintf(
			"Read %s and generate comprehensive Go tests for it. Use table-driven tests where appropriate. "+
				"Cover happy paths, edge cases, and error conditions. Write the tests to the correct _test.go file.", target)
		streamAndPrint(ctx, a, prompt, repoPath)

	case "/refactor":
		desc := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if desc == "" {
			fmt.Println("Usage: /refactor <description of what to refactor>")
			return false
		}
		prompt := fmt.Sprintf(
			"Refactor the code as described. Make minimal changes, preserve behavior, run tests after.\n\nTask: %s", desc)
		streamAndPrint(ctx, a, prompt, repoPath)

	// ── Git network ──────────────────────────────────────────────────────────

	case "/fetch":
		runShell(repoPath, "git", "fetch", "--all", "--prune")

	case "/pull":
		runShell(repoPath, "git", "pull", "--rebase")

	case "/push":
		pushArgs := []string{"push"}
		if len(parts) >= 2 {
			pushArgs = append(pushArgs, parts[1:]...)
		}
		runShell(repoPath, "git", pushArgs...)

	case "/remote":
		runShell(repoPath, "git", "remote", "-v")

	case "/rebase":
		target := "main"
		if len(parts) >= 2 {
			target = parts[1]
		}
		runShell(repoPath, "git", "rebase", target)

	case "/amend":
		msg := ""
		if len(parts) >= 2 {
			msg = strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		}
		if err := gitAmend(repoPath, msg); err != nil {
			fmt.Printf("%sexit: %v%s\n", colorRed, err, colorReset)
		}
		fmt.Println()

	case "/squash":
		n := 2
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &n)
		}
		msg := fmt.Sprintf("squash: combine last %d commits", n)
		if len(parts) >= 3 {
			msg = strings.Join(parts[2:], " ")
		}
		fmt.Printf("%sSquash last %d commits into %q? (y/N): %s", colorYellow, n, msg, colorReset)
		if confirm, _ := promptLine(""); strings.ToLower(strings.TrimSpace(confirm)) == "y" {
			if err := squashCommits(repoPath, n, msg); err != nil {
				fmt.Printf("%sexit: %v%s\n", colorRed, err, colorReset)
			}
		} else {
			fmt.Println("Cancelled.")
		}
		fmt.Println()

	case "/diff-staged":
		runShell(repoPath, "git", "diff", "--cached", "--color")

	case "/stash-list":
		if list := gitStashList(repoPath); list != "" {
			fmt.Printf("%s── Stash ──────────────────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
				colorDim, colorReset, list, colorDim, colorReset)
		} else {
			fmt.Println("Stash is empty.")
		}

	case "/clean":
		out, _ := exec.Command("git", "-C", repoPath, "clean", "-nd").Output()
		if strings.TrimSpace(string(out)) == "" {
			fmt.Println("Nothing to clean.")
			return false
		}
		fmt.Print(string(out))
		fmt.Printf("%sRemove these files? (y/N): %s", colorYellow, colorReset)
		if confirm, _ := promptLine(""); strings.ToLower(strings.TrimSpace(confirm)) == "y" {
			runShell(repoPath, "git", "clean", "-fd")
		} else {
			fmt.Println("Cancelled.")
		}

	// ── Repo insights ─────────────────────────────────────────────────────

	case "/count-lines":
		fmt.Printf("%sCounting lines…%s\n", colorDim, colorReset)
		fmt.Printf("%s── Lines of Code ───────────────────%s\n", colorDim, colorReset)
		fmt.Println(languageBreakdown(repoPath))
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	case "/hotspots":
		n := 15
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &n)
		}
		result := gitHotspots(repoPath, n)
		if result == "" {
			fmt.Println("No git history found.")
			return false
		}
		fmt.Printf("%s── Most Changed Files ──────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
			colorDim, colorReset, result, colorDim, colorReset)

	case "/contributors":
		result := gitContributors(repoPath)
		if result == "" {
			fmt.Println("No contributors found.")
			return false
		}
		fmt.Printf("%s── Contributors ────────────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
			colorDim, colorReset, result, colorDim, colorReset)

	case "/languages":
		fmt.Printf("%s── Languages ───────────────────────%s\n", colorDim, colorReset)
		fmt.Println(languageBreakdown(repoPath))
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	// ── Context improvements ──────────────────────────────────────────────

	case "/forget":
		// /forget <n>  → remove project memory entry n (1-indexed)
		// /forget msg <n> → remove conversation message n
		if len(parts) >= 2 && parts[1] == "msg" {
			// Remove from conversation
			n := len(a.Messages)
			if len(parts) >= 3 {
				fmt.Sscanf(parts[2], "%d", &n)
				n--
			}
			if n < 0 || n >= len(a.Messages) {
				fmt.Printf("Invalid index. Context has %d messages (1-%d).\n", len(a.Messages), len(a.Messages))
				return false
			}
			removed := a.Messages[n]
			a.Messages = append(a.Messages[:n], a.Messages[n+1:]...)
			snippet := removed.Content
			if len(snippet) > 60 {
				snippet = snippet[:60] + "…"
			}
			fmt.Printf("%s✓ removed message %d [%s]: %s%s\n\n", colorLime, n+1, removed.Role, snippet, colorReset)
		} else {
			// Remove from project memory
			n := 0
			if len(parts) >= 2 {
				fmt.Sscanf(parts[1], "%d", &n)
				n-- // 1-indexed → 0-indexed
			}
			entry, ok := removeProjectMemoryEntry(repoPath, n)
			if !ok {
				m := loadProjectMemory(repoPath)
				if len(m.Entries) == 0 {
					fmt.Println("No project notes. Use /remember <note> to add one.")
				} else {
					fmt.Printf("Invalid index. Use /memories to list entries (1-%d).\n", len(m.Entries))
				}
				return false
			}
			fmt.Printf("%s✓ removed note: %s%s\n\n", colorLime, entry.Note, colorReset)
		}

	case "/compact-hard":
		keep := 6
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &keep)
		}
		before := len(a.Messages)
		a.Messages = compactHard(a.Messages, keep)
		fmt.Printf("%s✓ hard compacted: %d → %d messages%s\n\n", colorLime, before, len(a.Messages), colorReset)

	case "/pin-list":
		fmt.Printf("%s── Pinned Messages ─────────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
			colorDim, colorReset, formatPinnedMessages(pinnedMessages), colorDim, colorReset)

	// ── Dev tools ─────────────────────────────────────────────────────────

	case "/benchmark":
		pkg := ""
		if len(parts) >= 2 {
			pkg = parts[1]
		}
		fmt.Printf("%sRunning benchmarks…%s\n", colorDim, colorReset)
		out, err := runBenchmark(repoPath, pkg)
		if err != nil {
			fmt.Printf("%s%s%s\n\n", colorRed, out, colorReset)
		} else {
			fmt.Printf("%s── Benchmarks ──────────────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
				colorDim, colorReset, out, colorDim, colorReset)
		}

	case "/env":
		if len(parts) == 1 {
			// Show iterate-relevant env vars
			filter := "ITERATE\nOLLAMA\nANTHROPIC\nOPENAI\nGEMINI\nGROQ\nGITHUB\nGO"
			var lines []string
			for _, f := range strings.Split(filter, "\n") {
				result := showEnv(f)
				if result != "" {
					lines = append(lines, result)
				}
			}
			fmt.Printf("%s── Environment ─────────────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
				colorDim, colorReset, strings.Join(lines, "\n"), colorDim, colorReset)
		} else if len(parts) == 2 {
			fmt.Println(os.Getenv(parts[1]))
		} else {
			os.Setenv(parts[1], parts[2])
			fmt.Printf("%s✓ %s=%s%s\n\n", colorLime, parts[1], parts[2], colorReset)
		}

	case "/debug":
		debugMode = !debugMode
		state := "off"
		if debugMode {
			state = "on"
		}
		fmt.Printf("%s✓ debug mode %s%s\n\n", colorLime, state, colorReset)

	// ── Clipboard & file ops ──────────────────────────────────────────────

	case "/paste":
		text, err := pasteFromClipboard()
		if err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
			return false
		}
		if strings.TrimSpace(text) == "" {
			fmt.Println("Clipboard is empty.")
			return false
		}
		fmt.Printf("%s✓ pasting %d chars from clipboard%s\n\n", colorLime, len(text), colorReset)
		streamAndPrint(ctx, a, text, repoPath)

	case "/open":
		if len(parts) < 2 {
			fmt.Println("Usage: /open <file>")
			return false
		}
		filePath := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		absPath := filePath
		if !filepath.IsAbs(filePath) {
			absPath = filepath.Join(repoPath, filePath)
		}
		if err := openInEditor(absPath); err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
		}

	case "/pwd":
		fmt.Printf("%s\n\n", repoPath)

	case "/cd":
		if len(parts) < 2 {
			fmt.Println("Usage: /cd <directory>")
			return false
		}
		newDir := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if !filepath.IsAbs(newDir) {
			newDir = filepath.Join(repoPath, newDir)
		}
		info, err := os.Stat(newDir)
		if err != nil || !info.IsDir() {
			fmt.Printf("%snot a directory: %s%s\n", colorRed, newDir, colorReset)
			return false
		}
		// Rebuild agent with new repoPath — we return true to signal the caller to update
		// Since we can't change repoPath from here, we inform the user
		fmt.Printf("%s⚠ /cd changes context for file tools — use /run cd or restart with --repo%s\n\n", colorYellow, colorReset)
		runShell(newDir, "ls", "-la")

	// ── Project workflow ──────────────────────────────────────────────────

	case "/journal":
		n := 50
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &n)
		}
		content := viewJournal(repoPath, n)
		fmt.Printf("%s── JOURNAL.md (last %d lines) ───────%s\n%s\n%s──────────────────────────────────%s\n\n",
			colorDim, n, colorReset, content, colorDim, colorReset)

	case "/skill-create":
		name := ""
		desc := "A new iterate skill."
		if len(parts) >= 2 {
			name = parts[1]
		}
		if len(parts) >= 3 {
			desc = strings.Join(parts[2:], " ")
		}
		if name == "" {
			var ok bool
			name, ok = promptLine("Skill name:")
			if !ok || name == "" {
				return false
			}
		}
		path, err := scaffoldSkill(repoPath, name, desc)
		if err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
			return false
		}
		fmt.Printf("%s✓ skill scaffolded at %s%s\n", colorLime, path, colorReset)
		fmt.Printf("%sRefining skill with AI…%s\n\n", colorDim, colorReset)
		prompt := fmt.Sprintf(
			"Read the skill file at %s and improve it: fill in realistic steps, "+
				"add good examples, and make the description compelling. Save the improved version.", path)
		streamAndPrint(ctx, a, prompt, repoPath)

	case "/self-improve":
		prompt := "Analyze your own source code in cmd/iterate/. Identify the top 3 improvements you could make " +
			"to your UX, performance, or capabilities. Then implement the single most impactful one, " +
			"run go build && go test, and commit if passing."
		streamAndPrint(ctx, a, prompt, repoPath)

	case "/evolve-now":
		prompt := "Run your full evolution loop: read SESSION_PLAN.md (or create one from current issues), " +
			"implement one improvement, run tests, commit passing changes, update JOURNAL.md."
		streamAndPrint(ctx, a, prompt, repoPath)

	// ── Error helpers ─────────────────────────────────────────────────────

	case "/fix":
		errText := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if errText == "" {
			// Detect project type and run the right build command
			pt := detectProjectType(repoPath)
			prompt := buildFixPromptForProject(repoPath, pt)
			if prompt == "" {
				fmt.Printf("%sNo errors found — build is clean.%s\n\n", colorLime, colorReset)
				return false
			}
			streamAndPrint(ctx, a, prompt, repoPath)
		} else {
			streamAndPrint(ctx, a, buildFixPrompt(errText), repoPath)
		}

	case "/explain-error":
		errText := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if errText == "" {
			fmt.Println("Usage: /explain-error <error message>  or paste the error text")
			return false
		}
		streamAndPrint(ctx, a, buildExplainErrorPrompt(errText), repoPath)

	case "/optimize":
		target := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		prompt := "Analyze the code for performance bottlenecks. Profile hot paths, " +
			"reduce allocations, and improve algorithmic complexity where possible."
		if target != "" {
			prompt = fmt.Sprintf("Optimize %s for performance. Reduce allocations, improve algorithms, "+
				"run benchmarks before and after.", target)
		}
		streamAndPrint(ctx, a, prompt, repoPath)

	case "/security":
		target := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		prompt := "Perform a security audit of this codebase. Check for: input validation, " +
			"injection vulnerabilities, hardcoded secrets, insecure defaults, path traversal, " +
			"and dependency vulnerabilities. Report findings with severity and fix suggestions."
		if target != "" {
			prompt = fmt.Sprintf("Security audit of %s: check for vulnerabilities, hardcoded secrets, "+
				"unsafe operations, and insecure patterns.", target)
		}
		streamAndPrint(ctx, a, prompt, repoPath)

	case "/plan":
		task := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if task == "" {
			fmt.Println("Usage: /plan <task description>")
			return false
		}
		planPrompt := fmt.Sprintf(
			"Think step by step about how to accomplish the following task. "+
				"Write out a numbered plan with each step clearly described. "+
				"Do NOT execute anything yet — only produce the plan.\n\nTask: %s", task)
		planAgent := iteragent.New(p, iteragent.DefaultTools(repoPath), logger).
			WithSystemPrompt(replSystemPrompt(repoPath)).
			WithThinkingLevel(*thinking)
		streamAndPrint(ctx, planAgent, planPrompt, repoPath)
		fmt.Printf("%sProceed with this plan? (y/N): %s", colorYellow, colorReset)
		confirm, _ := promptLine("")
		if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
			streamAndPrint(ctx, a, "Now execute the plan above step by step.", repoPath)
		} else {
			fmt.Println("Cancelled.")
		}

	case "/undo":
		fmt.Printf("%sUndo last commit? (y/N): %s", colorYellow, colorReset)
		confirm, _ := promptLine("")
		if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
			runShell(repoPath, "git", "reset", "HEAD~1")
			fmt.Printf("%s✓ undone%s\n", colorLime, colorReset)
		} else {
			fmt.Println("Cancelled.")
		}

	case "/test":
		// Auto-detect test command based on project type (yoyo-evolve style)
		testCmd := detectTestCommand(repoPath)
		fmt.Printf("%sRunning: %s%s\n", colorDim, testCmd, colorReset)
		runShell(repoPath, "bash", "-c", testCmd)

	case "/build":
		runShell(repoPath, "go", "build", "./...")

	case "/lint":
		runShell(repoPath, "go", "vet", "./...")

	case "/commit":
		msg := strings.TrimPrefix(line, parts[0])
		msg = strings.TrimSpace(msg)
		if msg == "" {
			msg = "iterate: manual commit"
		}
		runShell(repoPath, "git", "add", "-A")
		runShell(repoPath, "git", "commit", "-m", msg)

	case "/status":
		runShell(repoPath, "git", "status", "--short")
		if day, err := os.ReadFile(filepath.Join(repoPath, "DAY_COUNT")); err == nil {
			fmt.Printf("Day: %s\n", strings.TrimSpace(string(day)))
		}

	case "/compact":
		before := len(a.Messages)
		if before == 0 {
			fmt.Println("Nothing to compact.")
			return false
		}
		// Ask the agent to summarise what we've done before we drop messages.
		summaryPrompt := fmt.Sprintf(
			"Summarise this conversation in 5 bullet points. Focus on: what was asked, "+
				"what was implemented, key decisions, and any open questions. Be concise.\n\n"+
				"(Conversation has %d messages)", before)
		fmt.Printf("%sCompacting — summarising %d messages…%s\n", colorDim, before, colorReset)
		streamAndPrint(ctx, a, summaryPrompt, repoPath)
		// Now compact, preserving the summary (last assistant message) + system context.
		cfg := iteragent.DefaultContextConfig()
		a.Messages = iteragent.CompactMessagesTiered(a.Messages, cfg)
		fmt.Printf("%sCompacted: %d → %d messages%s\n", colorDim, before, len(a.Messages), colorReset)

	case "/phase":
		if len(parts) < 2 {
			fmt.Println("Usage: /phase plan|implement|communicate")
			return false
		}
		phase := parts[1]
		fmt.Printf("Running phase: %s\n", phase)

		// Fetch issues for plan phase
		issues := ""
		if phase == "plan" {
			var err error
			issues, err = listGitHubIssues(repoPath, 20)
			if err != nil {
				fmt.Printf("%sWarning: could not fetch issues: %s%s\n", colorYellow, err, colorReset)
			}
		}

		// Create evolution engine with event forwarding
		eventCh := make(chan iteragent.Event, 100)
		eng := evolution.New(repoPath, logger).
			WithThinking(*thinking).
			WithEventSink(eventCh)

		// Forward events to streamAndPrint-like output
		go func() {
			for ev := range eventCh {
				switch iteragent.EventType(ev.Type) {
				case iteragent.EventTokenUpdate:
					fmt.Print(ev.Content)
				case iteragent.EventToolExecutionStart:
					fmt.Printf("%s⚙ %s%s", colorYellow, ev.ToolName, colorReset)
				case iteragent.EventToolExecutionEnd:
					snippet := ev.Result
					if len(snippet) > 60 {
						snippet = snippet[:60] + "…"
					}
					fmt.Printf("%s → %s%s\n", colorDim, snippet, colorReset)
				case iteragent.EventError:
					fmt.Printf("%sError: %s%s\n", colorRed, ev.Content, colorReset)
				}
			}
		}()

		var err error
		switch phase {
		case "plan":
			err = eng.RunPlanPhase(ctx, p, issues)
		case "implement":
			err = eng.RunImplementPhase(ctx, p)
		case "communicate":
			err = eng.RunCommunicatePhase(ctx, p)
		default:
			fmt.Printf("Unknown phase: %s\n", phase)
			return false
		}

		if err != nil {
			fmt.Printf("%sPhase failed: %s%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ Phase %s completed%s\n", colorLime, phase, colorReset)
		}

	case "/generate-commit":
		prompt := buildGenerateCommitPrompt(repoPath)
		streamAndPrint(ctx, a, prompt, repoPath)

	case "/release":
		arg := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		from, to := "", "HEAD"
		if arg != "" {
			toks := strings.Fields(arg)
			from = toks[0]
			if len(toks) > 1 {
				to = toks[1]
			}
		}
		notes := buildReleaseNotes(repoPath, from, to)
		streamAndPrint(ctx, a, notes, repoPath)

	case "/ci":
		status, err := getCIStatus(repoPath)
		if err != nil {
			fmt.Printf("%s%s%s\n", colorRed, err.Error(), colorReset)
		} else {
			fmt.Print(status)
		}

	case "/view":
		path := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if path == "" {
			fmt.Println("Usage: /view <file>")
			return false
		}
		content, err := viewFile(filepath.Join(repoPath, path))
		if err != nil {
			fmt.Printf("%s%s%s\n", colorRed, err.Error(), colorReset)
		} else {
			renderResponse(content)
		}

	case "/verify":
		results := runVerify(repoPath)
		if len(results) == 0 {
			fmt.Printf("%s✓ all checks passed%s\n", colorLime, colorReset)
		} else {
			for _, r := range results {
				icon := "✓"
				col := colorLime
				if !r.ok {
					icon = "✗"
					col = colorRed
				}
				fmt.Printf("%s%s %s%s", col, icon, r.name, colorReset)
				if r.output != "" {
					fmt.Printf(": %s", r.output)
				}
				fmt.Println()
			}
		}

	case "/snapshot":
		name := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if name == "" {
			name = time.Now().Format("20060102-150405")
		}
		if err := saveSnapshot(repoPath, name, a.Messages); err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err.Error(), colorReset)
		} else {
			fmt.Printf("%s✓ snapshot saved: %s%s\n", colorLime, name, colorReset)
		}

	case "/snapshots":
		snaps := listSnapshots()
		if len(snaps) == 0 {
			fmt.Println("No snapshots found.")
		} else {
			for _, s := range snaps {
				fmt.Printf("  %s%s%s  — %s  (%d messages)\n",
					colorYellow, s.Name, colorReset,
					s.CreatedAt.Format("2006-01-02 15:04"), len(s.Messages))
			}
		}

	case "/pair":
		currentMode = modeNormal
		_ = a.Close()
		a = makeAgent(p, repoPath, *thinking, logger)
		a.Messages = append(a.Messages, iteragent.Message{
			Role:    "user",
			Content: pairModePrompt,
		})
		fmt.Printf("%s⟳ pair programming mode activated%s\n", colorLime, colorReset)

	case "/auto-commit":
		autoCommitEnabled = !autoCommitEnabled
		state := "disabled"
		if autoCommitEnabled {
			state = "enabled"
		}
		fmt.Printf("%s✓ auto-commit %s%s\n", colorLime, state, colorReset)

	case "/mcp-add":
		arg := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		toks := strings.Fields(arg)
		if len(toks) < 2 {
			fmt.Println("Usage: /mcp-add <name> <url-or-command> [args...]")
			return false
		}
		srv := mcpServer{Name: toks[0]}
		if strings.HasPrefix(toks[1], "http") {
			srv.URL = toks[1]
		} else {
			srv.Command = toks[1]
			if len(toks) > 2 {
				srv.Args = toks[2:]
			}
		}
		servers := loadMCPServers()
		servers = append(servers, srv)
		saveMCPServers(servers)
		fmt.Printf("%s✓ MCP server added: %s%s\n", colorLime, srv.Name, colorReset)

	case "/mcp-list":
		servers := loadMCPServers()
		if len(servers) == 0 {
			fmt.Println("No MCP servers configured.")
		} else {
			for _, s := range servers {
				loc := s.URL
				if loc == "" {
					loc = s.Command + " " + strings.Join(s.Args, " ")
				}
				fmt.Printf("  %s%s%s → %s\n", colorYellow, s.Name, colorReset, strings.TrimSpace(loc))
			}
		}

	case "/mcp-remove":
		name := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if name == "" {
			fmt.Println("Usage: /mcp-remove <name>")
			return false
		}
		servers := loadMCPServers()
		var kept []mcpServer
		for _, s := range servers {
			if s.Name != name {
				kept = append(kept, s)
			}
		}
		saveMCPServers(kept)
		fmt.Printf("%s✓ removed: %s%s\n", colorLime, name, colorReset)

	case "/diagram":
		prompt := buildDiagramPrompt(repoPath)
		streamAndPrint(ctx, a, prompt, repoPath)

	case "/generate-readme":
		prompt := buildReadmePrompt(repoPath)
		fmt.Printf("%sGenerate and write README.md? (y/N): %s", colorYellow, colorReset)
		confirm, _ := promptLine("")
		if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
			streamAndPrint(ctx, a, prompt+" Write the result directly to README.md.", repoPath)
		} else {
			streamAndPrint(ctx, a, prompt, repoPath)
		}

	case "/mock":
		filePath := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if filePath == "" {
			fmt.Println("Usage: /mock <file.go>")
			return false
		}
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(repoPath, filePath)
		}
		prompt := buildMockPrompt(filePath)
		streamAndPrint(ctx, a, prompt, repoPath)

	case "/pr-checkout":
		prNum := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if prNum == "" {
			fmt.Println("Usage: /pr-checkout <PR-number>")
			return false
		}
		if err := prCheckout(repoPath, prNum); err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err.Error(), colorReset)
		} else {
			fmt.Printf("%s✓ checked out PR #%s%s\n", colorLime, prNum, colorReset)
		}

	case "/gist":
		content := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if content == "" && lastResponse != "" {
			content = lastResponse
		}
		if content == "" {
			fmt.Println("Usage: /gist <content>  (or run after a response to gist it)")
			return false
		}
		fmt.Printf("%sFilename (e.g. output.md): %s", colorDim, colorReset)
		fname, _ := promptLine("")
		fname = strings.TrimSpace(fname)
		if fname == "" {
			fname = "iterate-output.md"
		}
		fmt.Printf("%sDescription: %s", colorDim, colorReset)
		desc, _ := promptLine("")
		url, err := createGist(content, fname, strings.TrimSpace(desc), false)
		if err != nil {
			fmt.Printf("%serror creating gist: %s%s\n", colorRed, err.Error(), colorReset)
		} else {
			fmt.Printf("%s✓ gist created: %s%s\n", colorLime, url, colorReset)
		}

	case "/init":
		name := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if name == "" {
			name = filepath.Base(repoPath)
		}
		created := initProject(repoPath, name)
		if len(created) == 0 {
			fmt.Printf("%s✓ project already initialized%s\n", colorLime, colorReset)
		} else {
			fmt.Printf("%s✓ initialized project '%s'%s\n", colorLime, name, colorReset)
			for _, f := range created {
				fmt.Printf("  %s+%s %s\n", colorLime, colorReset, f)
			}
		}

	case "/search":
		query := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if query == "" {
			fmt.Println("Usage: /search <query>")
			return false
		}
		matches := 0
		for i, msg := range a.Messages {
			if strings.Contains(strings.ToLower(msg.Content), strings.ToLower(query)) {
				role := msg.Role
				if role == "assistant" {
					role = colorLime + "AI" + colorReset
				} else {
					role = colorYellow + "You" + colorReset
				}
				// Show first 80 chars of match
				snippet := msg.Content
				if len(snippet) > 80 {
					snippet = snippet[:80] + "..."
				}
				fmt.Printf("  [%d] %s: %s\n", i, role, snippet)
				matches++
				if matches >= 5 {
					break
				}
			}
		}
		if matches == 0 {
			fmt.Println("No messages match that query.")
		} else {
			fmt.Printf("Found %d match(es). Use message index with /jump or context commands.\n", matches)
		}

	case "/spawn":
		task := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if task == "" {
			fmt.Println("Usage: /spawn <task description>")
			fmt.Println("")
			fmt.Println("Spawns a subagent to handle a focused, independent task.")
			fmt.Println("Example: /spawn optimize this code for performance")
			return false
		}
		subPrompt := fmt.Sprintf(
			"You are a focused subagent. Complete this task:\n\n%s\n\n"+
				"Provide a complete, standalone solution. Do not ask for clarification.",
			task)
		subAgent := makeAgent(p, repoPath, *thinking, logger)
		defer func() { _ = subAgent.Close() }()
		fmt.Printf("%sSpawning subagent for: %s%s\n\n", colorCyan, task, colorReset)
		streamAndPrint(ctx, subAgent, subPrompt, repoPath)
		fmt.Printf("\n%sSubagent completed.%s\n", colorCyan, colorReset)

	case "/swarm":
		// /swarm <n> <task> — safely launch N agents with rate limiting
		if len(parts) < 3 {
			fmt.Println("Usage: /swarm <count> <task template>")
			fmt.Println("")
			fmt.Println("Launches multiple agents with rate limiting and connection pooling.")
			fmt.Println("Example: /swarm 10 review file #1.md")
			fmt.Println("         /swarm 100 analyze issues in internal/")
			fmt.Println("")
			fmt.Println("Safety: 10 max concurrent, 5 req/sec rate limit")
			return false
		}
		count := 10
		fmt.Sscanf(parts[1], "%d", &count)
		if count > 100 {
			fmt.Printf("%sWarning: capping swarm at 100 agents%s\n", colorYellow, colorReset)
			count = 100
		}
		taskTemplate := strings.TrimSpace(strings.TrimPrefix(line, parts[0]+" "+parts[1]))

		// Generate tasks by replacing {N} with index
		tasks := make([]string, count)
		for i := 0; i < count; i++ {
			tasks[i] = strings.ReplaceAll(taskTemplate, "{N}", fmt.Sprintf("%d", i+1))
			tasks[i] = strings.ReplaceAll(tasks[i], "{n}", fmt.Sprintf("%d", i))
		}

		fmt.Printf("%sLaunching swarm: %d agents%s\n", colorCyan, count, colorReset)
		fmt.Printf("%sTask: %s%s\n", colorDim, taskTemplate, colorReset)
		fmt.Printf("%sRate limit: 5 req/sec, Max concurrent: 10%s\n\n", colorDim, colorReset)

		// Create agent pool with rate limiting
		pool := agent.NewPool(p, iteragent.DefaultTools(repoPath), logger, 10, 5)
		defer pool.Close()

		// Track results
		var success, failed int
		var resultsMu sync.Mutex

		start := time.Now()
		errs := pool.SpawnAll(ctx, tasks, func(a *iteragent.Agent, task string) error {
			prompt := fmt.Sprintf("You are a focused subagent. Complete this task:\n\n%s\n\nBe concise.", task)
			var result string
			for ev := range a.Prompt(ctx, prompt) {
				if ev.Type == string(iteragent.EventMessageEnd) {
					result = ev.Content
				}
			}
			resultsMu.Lock()
			if result != "" {
				success++
			} else {
				failed++
			}
			resultsMu.Unlock()
			return nil
		})

		// Report results
		elapsed := time.Since(start).Round(time.Second)
		fmt.Printf("\n%s── Swarm Complete ─────────────────%s\n", colorDim, colorReset)
		fmt.Printf("  Total:     %d agents\n", count)
		fmt.Printf("  Succeeded: %s%d%s\n", colorLime, success, colorReset)
		fmt.Printf("  Failed:    %s%d%s\n", colorRed, failed, colorReset)
		fmt.Printf("  Duration:  %s\n", elapsed)
		fmt.Printf("  Rate:      %.1f agents/sec\n", float64(count)/elapsed.Seconds())

		// Report errors
		for i, err := range errs {
			if err != nil {
				fmt.Printf("%sAgent %d error: %s%s\n", colorRed, i+1, err, colorReset)
			}
		}

	// ── Project tooling ───────────────────────────────────────────────────

	case "/tree":
		maxDepth := 4
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &maxDepth)
		}
		fmt.Printf("%sProject tree (git ls-files):%s\n", colorDim, colorReset)
		tree := buildProjectTree(repoPath, maxDepth)
		if tree == "" {
			fmt.Println("  (no files found)")
		} else {
			fmt.Println(tree)
		}
		fmt.Println()

	case "/index":
		fmt.Printf("%sBuilding project index…%s\n", colorDim, colorReset)
		entries := buildProjectIndex(repoPath)
		if len(entries) == 0 {
			fmt.Println("No files found.")
			return false
		}
		fmt.Printf("%s── Project Index (%d files) ─────────%s\n", colorDim, len(entries), colorReset)
		fmt.Print(formatProjectIndex(entries))
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	case "/pkgdoc":
		if len(parts) < 2 {
			fmt.Println("Usage: /pkgdoc <package>  e.g. /pkgdoc encoding/json")
			return false
		}
		pkg := strings.Join(parts[1:], "/")
		fmt.Printf("%sfetching pkg.go.dev/%s…%s\n", colorDim, pkg, colorReset)
		doc, err := fetchGoPkgDoc(pkg)
		if err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
			return false
		}
		fmt.Printf("%s── pkg.go.dev/%s ────────────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
			colorDim, pkg, colorReset, doc, colorDim, colorReset)

	// ── Provider & version ────────────────────────────────────────────────

	case "/version":
		fmt.Printf("iterate  version %s\n", iterateVersion)
		fmt.Printf("iteragent (SDK embedded)\n")
		fmt.Printf("provider: %s\n\n", p.Name())

	// ── In-memory conversation marks ──────────────────────────────────────

	case "/mark":
		name := fmt.Sprintf("mark-%d", len(conversationMarks)+1)
		if len(parts) >= 2 {
			name = strings.Join(parts[1:], " ")
		}
		conversationMarks[name] = len(a.Messages)
		fmt.Printf("%s✓ marked position %d as %q%s\n\n", colorLime, len(a.Messages), name, colorReset)

	case "/marks":
		if len(conversationMarks) == 0 {
			fmt.Println("No marks. Use /mark [name] to set one.")
			return false
		}
		fmt.Printf("%s── Marks ───────────────────────────%s\n", colorDim, colorReset)
		for name, idx := range conversationMarks {
			fmt.Printf("  %-20s → message %d\n", name, idx)
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	case "/jump":
		if len(parts) < 2 {
			fmt.Println("Usage: /jump <mark-name>")
			// Show available marks
			for name := range conversationMarks {
				fmt.Printf("  %s\n", name)
			}
			return false
		}
		name := strings.Join(parts[1:], " ")
		idx, ok := conversationMarks[name]
		if !ok {
			fmt.Printf("Mark %q not found. Use /marks to list.\n", name)
			return false
		}
		if idx > len(a.Messages) {
			idx = len(a.Messages)
		}
		a.Messages = a.Messages[:idx]
		fmt.Printf("%s✓ jumped to mark %q (message %d)%s\n\n", colorLime, name, idx, colorReset)

	// ── Per-project memory (/remember, /forget override) ──────────────────

	case "/remember":
		note := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		if note == "" {
			fmt.Println("Usage: /remember <note>")
			return false
		}
		if err := addProjectMemoryNote(repoPath, note); err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ note saved to .iterate/memory.json%s\n\n", colorLime, colorReset)
		}

	// ── /git passthrough ──────────────────────────────────────────────────

	case "/git":
		if len(parts) < 2 {
			runShell(repoPath, "git", "status")
			return false
		}
		gitArgs := parts[1:]
		runShell(repoPath, "git", gitArgs...)

	// ── Session changes ───────────────────────────────────────────────────

	case "/changes":
		fmt.Printf("%s── File changes this session ────────%s\n", colorDim, colorReset)
		fmt.Println(sessionChanges.format())
		fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)

	// ── ITERATE.md init ───────────────────────────────────────────────────

	case "/iterate-init":
		iterateMDPath := filepath.Join(repoPath, "ITERATE.md")
		if _, err := os.Stat(iterateMDPath); err == nil {
			fmt.Printf("%sITERATE.md already exists. Overwrite? (y/N): %s", colorYellow, colorReset)
			confirm, _ := promptLine("")
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				fmt.Println("Cancelled.")
				return false
			}
		}
		content := generateIterateMD(repoPath)
		if err := os.WriteFile(iterateMDPath, []byte(content), 0o644); err != nil {
			fmt.Printf("%serror: %s%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ ITERATE.md generated — edit it to add project-specific context%s\n\n", colorLime, colorReset)
		}

	default:
		fmt.Printf("Unknown command: %s (try /help)\n", cmd)
	}

	return false
}

// toolStyle returns the display icon, label, and ANSI color for a tool name.
func toolStyle(name string) (icon, label, col string) {
	switch name {
	case "bash", "run_command", "run_terminal_cmd":
		return "❯", "bash", colorLime
	case "read_file", "read":
		return "◎", "read", colorCyan
	case "write_file", "create_file":
		return "✎", "write", colorYellow
	case "edit_file":
		return "✎", "edit", colorAmber
	case "search_files", "grep_search", "find_files":
		return "⌕", "search", colorCyan
	case "list_dir", "list_directory":
		return "◈", "list", colorBlue
	case "web_fetch", "fetch_url":
		return "↓", "fetch", colorBlue
	default:
		return "⚙", name, colorDim
	}
}

// spinner runs a spinner in the terminal until stop() is called, signals done when exited.
func spinner(stop <-chan struct{}, done chan<- struct{}) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	start := time.Now()
	spinnerActive.Store(1)
	for {
		select {
		case <-stop:
			spinnerActive.Store(0)
			fmt.Print("\r\033[K")
			close(done)
			return
		default:
			elapsed := time.Since(start).Round(time.Millisecond)
			fmt.Printf("\r%s%s%s  %sthinking%s  %s%s%s", colorLime, frames[i%len(frames)], colorReset, colorBold, colorReset, colorDim, elapsed, colorReset)
			i++
			time.Sleep(80 * time.Millisecond)
		}
	}
}

// streamAndPrint runs the agent and prints the streamed response.
func streamAndPrint(ctx context.Context, a *iteragent.Agent, prompt string, repoPath string) {
	lastPrompt = prompt
	recordMessage()
	events := a.Prompt(ctx, prompt)
	start := time.Now()

	var (
		stopSpinner chan struct{}
		spinnerDone chan struct{}
		spinnerOnce sync.Once
		stopOnce    func()
	)
	newSpinner := func() {
		stopSpinner = make(chan struct{})
		spinnerDone = make(chan struct{})
		spinnerOnce = sync.Once{}
		stopOnce = func() {
			spinnerOnce.Do(func() {
				close(stopSpinner)
				<-spinnerDone
			})
		}
		go spinner(stopSpinner, spinnerDone)
	}
	newSpinner()
	defer func() { stopOnce() }()

	var fullContent string
	var streamingTokens bool // true once we receive the first EventTokenUpdate

	for e := range events {
		switch iteragent.EventType(e.Type) {
		case iteragent.EventTokenUpdate:
			// First token: stop spinner and clear line, then print tokens live.
			if !streamingTokens {
				stopOnce()
				fmt.Print("\r\033[K")
				streamingTokens = true
			}
			fmt.Print(e.Content)
			fullContent += e.Content

		case iteragent.EventMessageUpdate:
			if streamingTokens {
				// Already printed token-by-token; just keep fullContent in sync.
				fullContent = e.Content
			} else {
				// Non-streaming provider: buffer and render at the end.
				fullContent = e.Content
			}

		case iteragent.EventToolExecutionStart:
			recordToolCall()
			stopOnce()
			if fullContent != "" {
				if !streamingTokens {
					fmt.Print("\r\033[K")
					renderResponse(fullContent)
				}
				fmt.Println()
				fullContent = ""
				streamingTokens = false
			}
			icon, label, col := toolStyle(e.ToolName)
			fmt.Printf("%s%s %s%s%s", col, icon, label, colorReset, colorDim)

		case iteragent.EventToolExecutionEnd:
			snippet := e.Result
			if len(snippet) > 60 {
				snippet = snippet[:60] + "…"
			}
			snippet = strings.ReplaceAll(snippet, "\n", " ")
			fmt.Printf(" → %s%s\n", snippet, colorReset)
			// Show git diff after file edits
			if e.ToolName == "write_file" || e.ToolName == "edit_file" || e.ToolName == "create_file" {
				showGitDiff(repoPath)
			}
			// Restart spinner for next agent step
			newSpinner()

		case iteragent.EventContextCompacted:
			fmt.Printf("\r\033[K%s[context compacted]%s\n", colorDim, colorReset)

		case iteragent.EventError:
			fmt.Printf("\r\033[K%sError: %s%s\n", colorRed, e.Content, colorReset)
		}
	}
	a.Finish()
	stopOnce()

	if fullContent != "" {
		lastResponse = fullContent
		if streamingTokens {
			// Tokens were printed live; just add a newline and syntax-render if markdown.
			fmt.Println()
		} else {
			fmt.Print("\r\033[K")
			renderResponse(fullContent)
			fmt.Println()
		}
	}
	maybeNotify()
	elapsed := time.Since(start).Round(time.Millisecond)

	// Try to get accurate usage from the last assistant message.
	usageStr := ""
	if len(a.Messages) > 0 {
		last := a.Messages[len(a.Messages)-1]
		if last.Usage != nil {
			sessionInputTokens += last.Usage.InputTokens
			sessionOutputTokens += last.Usage.OutputTokens
			sessionCacheRead += last.Usage.CacheRead
			sessionCacheWrite += last.Usage.CacheWrite
			sessionTokens += last.Usage.TotalTokens
			usageStr = fmt.Sprintf(" · %d in / %d out tokens", last.Usage.InputTokens, last.Usage.OutputTokens)
		}
	}
	if usageStr == "" {
		approxTokens := len(fullContent) / 4
		sessionTokens += approxTokens
		sessionOutputTokens += approxTokens
		if approxTokens > 0 {
			usageStr = fmt.Sprintf(" · ~%d tokens", approxTokens)
		}
	}
	fmt.Printf("\n%s●%s %s%s%s%s\n\n", colorLime, colorReset, colorDim, elapsed, usageStr, colorReset)
}

// showGitDiff runs git diff and prints colored output if there are changes.
func showGitDiff(repoPath string) {
	cmd := exec.Command("git", "diff", "--color=always", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		// Try unstaged diff
		cmd2 := exec.Command("git", "diff", "--color=always")
		cmd2.Dir = repoPath
		out, err = cmd2.Output()
	}
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return
	}
	fmt.Printf("\n%s── diff ──────────────────────────%s\n", colorDim, colorReset)
	fmt.Print(string(out))
	fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)
}

// runShell runs a command in repoPath and prints its output.
func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// detectTestCommand returns the appropriate test command based on project type.
// Supports: Go, Rust, Python, Node.js, and falls back to 'make test'.
func detectTestCommand(repoPath string) string {
	// Check for Go project
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); err == nil {
		return "go test ./... -v"
	}
	// Check for Rust project
	if _, err := os.Stat(filepath.Join(repoPath, "Cargo.toml")); err == nil {
		return "cargo test"
	}
	// Check for Python project
	if _, err := os.Stat(filepath.Join(repoPath, "pyproject.toml")); err == nil {
		return "pytest"
	}
	if _, err := os.Stat(filepath.Join(repoPath, "setup.py")); err == nil {
		return "python -m pytest"
	}
	// Check for Node.js project
	if _, err := os.Stat(filepath.Join(repoPath, "package.json")); err == nil {
		// Check for npm test script
		pkgJSON, _ := os.ReadFile(filepath.Join(repoPath, "package.json"))
		if bytes.Contains(pkgJSON, []byte(`"test"`)) {
			return "npm test"
		}
		return "node --test"
	}
	// Check for Makefile
	if _, err := os.Stat(filepath.Join(repoPath, "Makefile")); err == nil {
		return "make test"
	}
	// Default fallback
	return "go test ./..."
}

func runShell(repoPath string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("exit: %v\n", err)
	}
}

func replSystemPrompt(repoPath string) string {
	personality, _ := os.ReadFile(filepath.Join(repoPath, "PERSONALITY.md"))

	base := "You are iterate, a self-evolving Go coding agent in an interactive REPL.\n"
	base += "Help the user with coding tasks, answer questions, and use tools when needed.\n"
	base += "Keep responses concise and direct. Do not add journals, logs, or internal monologue.\n"
	if len(personality) > 0 {
		base += "\n## Personality\n" + string(personality)
	}

	// Inject project memory notes (per-project .iterate/memory.json)
	if mem := loadProjectMemory(repoPath); len(mem.Entries) > 0 {
		base += "\n" + formatProjectMemoryForPrompt(mem)
	}

	// Inject active learnings (evolution memory)
	if learnings := readActiveLearnings(repoPath); learnings != "" {
		base += "\n## Active Learnings\n" + learnings + "\n"
	}

	// Inject ITERATE.md if present
	if iterateMD, err := os.ReadFile(filepath.Join(repoPath, "ITERATE.md")); err == nil {
		base += "\n## Project Context (ITERATE.md)\n" + string(iterateMD)
	}

	if index := buildRepoIndex(repoPath); index != "" {
		base += "\n## Repo structure\n```\n" + index + "\n```\n"
	}
	return base
}
