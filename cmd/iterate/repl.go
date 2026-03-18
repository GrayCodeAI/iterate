package main

import (
	"bufio"
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

const (
	colorReset  = "\033[0m"
	colorLime   = "\033[38;5;154m"
	colorYellow = "\033[38;5;220m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
	colorCyan   = "\033[36m"
	colorRed    = "\033[31m"
)

func makeAgent(p iteragent.Provider, repoPath string, thinking iteragent.ThinkingLevel, logger *slog.Logger) *iteragent.Agent {
	tools := iteragent.DefaultTools(repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(repoPath, "skills")})
	return iteragent.New(p, tools, logger).
		WithSystemPrompt(replSystemPrompt(repoPath)).
		WithSkillSet(skills).
		WithThinkingLevel(thinking)
}

// runREPL runs an interactive session with iterate.
func runREPL(ctx context.Context, p iteragent.Provider, repoPath string, thinking iteragent.ThinkingLevel, logger *slog.Logger) {
	a := makeAgent(p, repoPath, thinking, logger)

	printHeader(p, thinking)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%s❯%s ", colorLime, colorReset)
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// /model handled here so it has scanner access and can swap provider+agent.
		if line == "/model" || strings.HasPrefix(line, "/model ") {
			newP, newThinking := selectModel(thinking)
			if newP != nil {
				p = newP
				thinking = newThinking
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

		if strings.HasPrefix(line, "/") {
			if done := handleCommand(ctx, line, a, p, repoPath, &thinking, logger); done {
				return
			}
			continue
		}

		streamAndPrint(ctx, a, line)
	}
}

func printHeader(p iteragent.Provider, thinking iteragent.ThinkingLevel) {
	fmt.Printf("\n%s iterate%s  %s%s%s", colorLime+colorBold, colorReset, colorDim, p.Name(), colorReset)
	if thinking != "" && thinking != iteragent.ThinkingLevelOff {
		fmt.Printf("  %sthinking:%s %s", colorDim, colorReset, thinking)
	}
	fmt.Println()
	fmt.Printf("%sType a message, or /help for commands. Ctrl+C to exit.%s\n", colorDim, colorReset)
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
	{name: "agx-01",  url: "http://100.102.194.103:11434/v1"},
	{name: "agx-02",  url: "http://100.87.35.70:11434/v1"},
	{name: "gb10-01", url: "http://100.93.184.1:11434/v1"},
	{name: "gb10-02", url: "http://100.87.126.2:11434/v1"},
	{name: "vps-1",   url: "http://100.79.60.48:11434/v1"},
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

// handleCommand processes a slash command. Returns true if the REPL should exit.
func handleCommand(ctx context.Context, line string, a *iteragent.Agent, p iteragent.Provider, repoPath string, thinking *iteragent.ThinkingLevel, logger *slog.Logger) bool {
	parts := strings.Fields(line)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help", "/?":
		fmt.Print(`
Available commands:
  /help               — show this help
  /clear              — reset conversation history
  /tools              — list available tools
  /skills             — list available skills
  /thinking <level>   — set thinking level: off|minimal|low|medium|high
  /model              — switch provider/model interactively
  /test               — run go test ./...
  /build              — run go build ./...
  /lint               — run go vet ./...
  /commit <msg>       — git add -A && git commit -m "<msg>"
  /status             — git status + DAY_COUNT
  /compact            — compact conversation history
  /phase <phase>      — run evolution phase: plan|implement|communicate
  /quit               — exit REPL
`)

	case "/quit", "/exit", "/q":
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

	case "/test":
		runShell(repoPath, "go", "test", "./...")

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
		cfg := iteragent.DefaultContextConfig()
		a.Messages = iteragent.CompactMessagesTiered(a.Messages, cfg)
		fmt.Printf("Compacted to %d messages.\n", len(a.Messages))

	case "/phase":
		if len(parts) < 2 {
			fmt.Println("Usage: /phase plan|implement|communicate")
			return false
		}
		phase := parts[1]
		fmt.Printf("Running phase: %s\n", phase)
		tools := iteragent.DefaultTools(repoPath)
		skills, _ := iteragent.LoadSkills([]string{filepath.Join(repoPath, "skills")})
		phaseAgent := iteragent.New(p, tools, logger).
			WithThinkingLevel(*thinking).
			WithSkillSet(skills)

		var prompt string
		switch phase {
		case "plan":
			prompt = "Read your source code, JOURNAL.md, and any ISSUES_TODAY.md. Write SESSION_PLAN.md with tasks and issue responses, then commit it. Then STOP."
		case "implement":
			prompt = "Read SESSION_PLAN.md and implement each task. Run go build && go test after each. Commit passing changes."
		case "communicate":
			prompt = "Read SESSION_PLAN.md Issue Responses section and post GitHub comments for each issue using: gh issue comment <N> --repo . --body \"...\""
		default:
			fmt.Printf("Unknown phase: %s\n", phase)
			return false
		}
		streamAndPrint(ctx, phaseAgent, prompt)

	default:
		fmt.Printf("Unknown command: %s (try /help)\n", cmd)
	}

	return false
}

// spinner runs a spinner in the terminal until stop() is called, signals done when exited.
func spinner(stop <-chan struct{}, done chan<- struct{}) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	start := time.Now()
	for {
		select {
		case <-stop:
			fmt.Print("\r\033[K")
			close(done)
			return
		default:
			elapsed := time.Since(start).Round(time.Millisecond)
			fmt.Printf("\r%s%s%s thinking… %s%s%s", colorLime, frames[i%len(frames)], colorReset, colorDim, elapsed, colorReset)
			i++
			time.Sleep(80 * time.Millisecond)
		}
	}
}

// streamAndPrint runs the agent and prints the streamed response.
func streamAndPrint(ctx context.Context, a *iteragent.Agent, prompt string) {
	events := a.Prompt(ctx, prompt)
	var lastContent string
	inProgress := false
	start := time.Now()

	stopSpinner := make(chan struct{})
	spinnerDone := make(chan struct{})
	var spinnerOnce sync.Once
	stopOnce := func() {
		spinnerOnce.Do(func() {
			close(stopSpinner)
			<-spinnerDone
		})
	}
	go spinner(stopSpinner, spinnerDone)
	defer stopOnce()

	for e := range events {
		switch iteragent.EventType(e.Type) {
		case iteragent.EventMessageUpdate:
			stopOnce()
			inProgress = true
			lastContent = e.Content

		case iteragent.EventToolExecutionStart:
			stopOnce()
			fmt.Printf("\r\033[K%s⚙ %s%s", colorYellow, e.ToolName, colorReset)

		case iteragent.EventToolExecutionEnd:
			snippet := e.Result
			if len(snippet) > 60 {
				snippet = snippet[:60] + "…"
			}
			fmt.Printf("%s → %s%s\n", colorDim, snippet, colorReset)

		case iteragent.EventContextCompacted:
			fmt.Printf("\r\033[K%s[context compacted]%s\n", colorDim, colorReset)

		case iteragent.EventMessageEnd:
			lastContent = e.Content

		case iteragent.EventError:
			fmt.Printf("\r\033[K%sError: %s%s\n", colorRed, e.Content, colorReset)
		}
	}
	a.Finish()

	if inProgress {
		fmt.Print("\r\033[K")
	}
	elapsed := time.Since(start).Round(time.Millisecond)
	if lastContent != "" {
		fmt.Printf("%s%s%s\n", colorBold, lastContent, colorReset)
	}
	fmt.Printf("%s%s%s\n\n", colorDim, elapsed, colorReset)
}

// runShell runs a command in repoPath and prints its output.
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
	return base
}
