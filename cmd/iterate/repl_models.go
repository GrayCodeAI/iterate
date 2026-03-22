package main

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

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
	fmt.Printf("%sdiscovering Ollama hosts…%s\r\n", colorDim, colorReset)
	hosts := discoverOllamaHosts()

	url := promptOllamaHost(hosts)
	if url == "cancel" {
		return nil, currentThinking
	}
	if url == "" {
		url = promptOllamaURL()
		if url == "" {
			return nil, currentThinking
		}
	}

	os.Setenv("OLLAMA_BASE_URL", url)
	if !promptOllamaModelSelection(url) {
		return nil, currentThinking
	}

	p, err := iteragent.NewProvider("ollama")
	if err != nil {
		fmt.Printf("%serror: %s%s\n\n", colorRed, err, colorReset)
		return nil, currentThinking
	}
	return p, currentThinking
}

// promptOllamaHost shows a selector for discovered Ollama hosts.
// Returns the selected URL, empty string for manual entry, or "cancel" if dismissed.
func promptOllamaHost(hosts []ollamaHost) string {
	if len(hosts) == 0 {
		return ""
	}
	labels := make([]string, len(hosts))
	for i, h := range hosts {
		labels[i] = fmt.Sprintf("%-20s  %s", h.name, h.url)
	}
	labels = append(labels, "enter URL manually")

	choice, ok := selectItem("Select Ollama host", labels)
	if !ok {
		return "cancel"
	}
	if choice == "enter URL manually" {
		return ""
	}
	for _, h := range hosts {
		if strings.HasPrefix(choice, h.name) {
			return h.url
		}
	}
	return ""
}

// promptOllamaURL asks the user for a manual Ollama URL.
// Returns empty string if the user cancels.
func promptOllamaURL() string {
	currentURL := os.Getenv("OLLAMA_BASE_URL")
	if currentURL == "" {
		currentURL = "http://localhost:11434/v1"
	}
	url, ok := promptLine(fmt.Sprintf("Ollama URL (Enter to keep %s):", currentURL))
	if !ok {
		return ""
	}
	if url == "" {
		url = currentURL
	}
	return url
}

// promptOllamaModelSelection fetches models from the given URL and prompts the user to pick one.
// Returns false if the user cancels.
func promptOllamaModelSelection(baseURL string) bool {
	tagsURL := strings.TrimSuffix(strings.TrimSuffix(baseURL, "/v1"), "/") + "/api/tags"
	fmt.Printf("%sfetching models…%s\r\n", colorDim, colorReset)

	models, err := fetchOllamaModels(tagsURL)
	if err != nil || len(models) == 0 {
		modelName, ok := promptLine("Enter model name:")
		if !ok || modelName == "" {
			return false
		}
		os.Setenv("ITERATE_MODEL", modelName)
		return true
	}
	modelName, ok := selectItem("Select model", models)
	if !ok {
		return false
	}
	os.Setenv("ITERATE_MODEL", modelName)
	return true
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
