package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RegisterLearningCommands adds pattern recognition and learning management commands.
// Task 51: Pattern Recognition from Active Learnings
// Task 52: Cross-Project Learning Transfer
// Task 53: Learning Categories
// Task 54: Learning Confidence Score
// Task 55: Learning Expiration
// Task 60: Manual Learning Curation
func RegisterLearningCommands(r *Registry) {
	r.Register(Command{
		Name:        "/patterns",
		Aliases:     []string{"/pat"},
		Description: "recognize patterns in learnings",
		Category:    "memory",
		Handler:     cmdPatterns,
	})
	r.Register(Command{
		Name:        "/learnings",
		Aliases:     []string{"/ln"},
		Description: "show learnings with categories and confidence",
		Category:    "memory",
		Handler:     cmdLearnings,
	})
	r.Register(Command{
		Name:        "/curate",
		Aliases:     []string{},
		Description: "review and curate learnings (approve/reject)",
		Category:    "memory",
		Handler:     cmdCurate,
	})
	r.Register(Command{
		Name:        "/learning-stats",
		Aliases:     []string{"/ls"},
		Description: "learning analytics and statistics",
		Category:    "memory",
		Handler:     cmdLearningStats,
	})
	r.Register(Command{
		Name:        "/expire-learnings",
		Aliases:     []string{},
		Description: "expire outdated learnings (>90 days)",
		Category:    "memory",
		Handler:     cmdExpireLearnings,
	})
	r.Register(Command{
		Name:        "/transfer-learnings",
		Aliases:     []string{},
		Description: "export learnings for cross-project transfer",
		Category:    "memory",
		Handler:     cmdTransferLearnings,
	})
}

// learningEntry represents a parsed learning with metadata.
type learningEntry struct {
	Day        int     `json:"day"`
	TS         string  `json:"ts"`
	Source     string  `json:"source"`
	Title      string  `json:"title"`
	Context    string  `json:"context"`
	Takeaway   string  `json:"takeaway"`
	Type       string  `json:"type"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Curated    bool    `json:"curated"`
}

func cmdPatterns(ctx Context) Result {
	learnings := loadLearnings(ctx.RepoPath)
	if len(learnings) == 0 {
		fmt.Println("No learnings recorded yet. Use /learn to add observations.")
		return Result{Handled: true}
	}

	// Categorize learnings
	categories := categorizeLearnings(learnings)

	// Find recurring themes
	themes := findRecurringThemes(learnings)

	fmt.Printf("%s── Patterns ───────────────────────%s\n", ColorDim, ColorReset)

	// Show category distribution
	fmt.Printf("\n  %sCategory Distribution:%s\n", ColorBold, ColorReset)
	type catCount struct {
		name  string
		count int
	}
	var sorted []catCount
	for cat, count := range categories {
		sorted = append(sorted, catCount{cat, count})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })
	total := len(learnings)
	for _, c := range sorted {
		pct := float64(c.count) / float64(total) * 100
		bar := strings.Repeat("█", int(pct/5))
		fmt.Printf("    %-18s %3d (%.0f%%) %s%s%s\n", c.name, c.count, pct, ColorLime, bar, ColorReset)
	}

	// Show recurring themes
	if len(themes) > 0 {
		fmt.Printf("\n  %sRecurring Themes:%s\n", ColorBold, ColorReset)
		for i, theme := range themes {
			if i >= 5 {
				break
			}
			fmt.Printf("    %s●%s %s\n", ColorCyan, ColorReset, theme)
		}
	}

	// Show high-confidence insights
	fmt.Printf("\n  %sHigh-Confidence Insights:%s\n", ColorBold, ColorReset)
	highConf := filterByConfidence(learnings, 0.7)
	for i, l := range highConf {
		if i >= 5 {
			break
		}
		fmt.Printf("    %s✓%s %s (day %d)\n", ColorLime, ColorReset, l.Title, l.Day)
	}
	if len(highConf) == 0 {
		fmt.Printf("    %sNo high-confidence learnings yet.%s\n", ColorDim, ColorReset)
	}

	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdLearnings(ctx Context) Result {
	learnings := loadLearnings(ctx.RepoPath)
	if len(learnings) == 0 {
		fmt.Println("No learnings recorded yet.")
		return Result{Handled: true}
	}

	// Show learnings with categories and confidence
	fmt.Printf("%s── Learnings ──────────────────────%s\n", ColorDim, ColorReset)
	for i, l := range learnings {
		confStr := fmt.Sprintf("%.0f%%", l.Confidence*100)
		catStr := l.Category
		if catStr == "" {
			catStr = "uncategorized"
		}

		confColor := ColorDim
		if l.Confidence >= 0.7 {
			confColor = ColorLime
		} else if l.Confidence >= 0.4 {
			confColor = ColorYellow
		}

		title := l.Title
		if len(title) > 60 {
			title = title[:60] + "..."
		}

		curated := ""
		if l.Curated {
			curated = fmt.Sprintf(" %s✓%s", ColorGreen, ColorReset)
		}

		fmt.Printf("  %s%d%s %s[%s]%s %s [%s]%s%s\n",
			ColorDim, i+1, ColorReset,
			confColor, confStr, ColorReset,
			title, catStr, curated, ColorReset)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdCurate(ctx Context) Result {
	learnings := loadLearnings(ctx.RepoPath)
	if len(learnings) == 0 {
		fmt.Println("No learnings to curate.")
		return Result{Handled: true}
	}

	if !ctx.HasArg(1) {
		fmt.Printf("%s── Uncurated Learnings ────────────%s\n", ColorDim, ColorReset)
		uncurated := 0
		for i, l := range learnings {
			if !l.Curated {
				title := l.Title
				if len(title) > 70 {
					title = title[:70] + "..."
				}
				confStr := fmt.Sprintf("%.0f%%", l.Confidence*100)
				fmt.Printf("  %s%d%s %s[%s]%s %s\n", ColorDim, i+1, ColorReset, ColorYellow, confStr, ColorReset, title)
				uncurated++
			}
		}
		if uncurated == 0 {
			fmt.Println("  All learnings are curated!")
		}
		fmt.Printf("\nUsage: /curate <n> [approve|reject]\n")
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	n := 0
	fmt.Sscanf(ctx.Arg(1), "%d", &n)
	if n < 1 || n > len(learnings) {
		PrintError("invalid index (1-%d)", len(learnings))
		return Result{Handled: true}
	}

	action := "approve"
	if ctx.HasArg(2) {
		action = strings.ToLower(ctx.Arg(2))
	}

	// Update the learning
	l := &learnings[n-1]
	switch action {
	case "approve":
		l.Curated = true
		l.Confidence = 1.0
		PrintSuccess("approved learning %d: %s", n, l.Title)
	case "reject":
		l.Curated = true
		l.Confidence = 0.0
		PrintSuccess("rejected learning %d: %s", n, l.Title)
	default:
		PrintError("unknown action: %s (use approve or reject)", action)
		return Result{Handled: true}
	}

	saveLearnings(ctx.RepoPath, learnings)
	return Result{Handled: true}
}

func cmdLearningStats(ctx Context) Result {
	learnings := loadLearnings(ctx.RepoPath)
	if len(learnings) == 0 {
		fmt.Println("No learnings recorded yet.")
		return Result{Handled: true}
	}

	categories := categorizeLearnings(learnings)
	confidenceScores := make([]float64, len(learnings))
	curatedCount := 0
	for i, l := range learnings {
		confidenceScores[i] = l.Confidence
		if l.Curated {
			curatedCount++
		}
	}

	fmt.Printf("%s── Learning Analytics ─────────────%s\n", ColorDim, ColorReset)
	fmt.Printf("  Total learnings:  %d\n", len(learnings))
	fmt.Printf("  Curated:          %d\n", curatedCount)
	fmt.Printf("  Categories:       %d\n", len(categories))

	// Average confidence
	avgConf := 0.0
	for _, c := range confidenceScores {
		avgConf += c
	}
	avgConf /= float64(len(confidenceScores))
	fmt.Printf("  Avg confidence:   %.1f%%\n", avgConf*100)

	// Time range
	if len(learnings) > 0 {
		fmt.Printf("  Earliest:         Day %d\n", learnings[0].Day)
		fmt.Printf("  Latest:           Day %d\n", learnings[len(learnings)-1].Day)
	}

	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdExpireLearnings(ctx Context) Result {
	learnings := loadLearnings(ctx.RepoPath)
	if len(learnings) == 0 {
		fmt.Println("No learnings to expire.")
		return Result{Handled: true}
	}

	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	var kept []learningEntry
	expired := 0
	for _, l := range learnings {
		if l.TS != "" {
			if ts, err := time.Parse(time.RFC3339, l.TS); err == nil && ts.Before(cutoff) && !l.Curated {
				expired++
				continue
			}
		}
		kept = append(kept, l)
	}

	if expired == 0 {
		fmt.Println("No outdated learnings to expire.")
		return Result{Handled: true}
	}

	saveLearnings(ctx.RepoPath, kept)
	PrintSuccess("expired %d learnings older than 90 days (kept %d)", expired, len(kept))
	return Result{Handled: true}
}

func cmdTransferLearnings(ctx Context) Result {
	learnings := loadLearnings(ctx.RepoPath)
	if len(learnings) == 0 {
		fmt.Println("No learnings to transfer.")
		return Result{Handled: true}
	}

	// Export high-confidence learnings as a portable JSON file
	var exportable []learningEntry
	for _, l := range learnings {
		if l.Confidence >= 0.5 || l.Curated {
			exportable = append(exportable, l)
		}
	}

	if len(exportable) == 0 {
		fmt.Println("No transferable learnings (need confidence >= 50% or curated).")
		return Result{Handled: true}
	}

	exportPath := filepath.Join(ctx.RepoPath, "memory", "export_learnings.json")
	data, err := json.MarshalIndent(exportable, "", "  ")
	if err != nil {
		PrintError("failed to marshal: %v", err)
		return Result{Handled: true}
	}

	if err := os.WriteFile(exportPath, data, 0o644); err != nil {
		PrintError("failed to write: %v", err)
		return Result{Handled: true}
	}

	PrintSuccess("exported %d learnings to %s", len(exportable), exportPath)
	fmt.Printf("%sImport in another project: /learnings import %s%s\n", ColorDim, exportPath, ColorReset)
	return Result{Handled: true}
}

// loadLearnings reads and parses learnings.jsonl
func loadLearnings(repoPath string) []learningEntry {
	path := filepath.Join(repoPath, "memory", "learnings.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var entries []learningEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry learningEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		// Auto-categorize if not set
		if entry.Category == "" {
			entry.Category = inferCategory(entry)
		}
		// Auto-set confidence if zero
		if entry.Confidence == 0 {
			entry.Confidence = inferConfidence(entry)
		}
		entries = append(entries, entry)
	}
	return entries
}

// saveLearnings writes learnings back to learnings.jsonl
func saveLearnings(repoPath string, entries []learningEntry) {
	path := filepath.Join(repoPath, "memory", "learnings.jsonl")
	f, err := os.Create(path)
	if err != nil {
		PrintError("failed to save learnings: %v", err)
		return
	}
	defer f.Close()

	for _, e := range entries {
		data, _ := json.Marshal(e)
		f.WriteString(string(data) + "\n")
	}
}

// categorizeLearnings counts entries by category.
func categorizeLearnings(entries []learningEntry) map[string]int {
	counts := make(map[string]int)
	for _, e := range entries {
		cat := e.Category
		if cat == "" {
			cat = "uncategorized"
		}
		counts[cat]++
	}
	return counts
}

// findRecurringThemes finds common words/topics across learnings.
func findRecurringThemes(entries []learningEntry) []string {
	wordCounts := make(map[string]int)
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "it": true,
		"to": true, "in": true, "for": true, "of": true, "and": true,
		"or": true, "but": true, "not": true, "on": true, "at": true,
		"with": true, "by": true, "from": true, "as": true, "was": true,
		"were": true, "been": true, "be": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true,
		"would": true, "could": true, "should": true, "may": true, "might": true,
		"that": true, "this": true, "these": true, "those": true, "i": true,
		"we": true, "they": true, "you": true, "he": true, "she": true,
	}

	for _, e := range entries {
		words := strings.Fields(strings.ToLower(e.Title + " " + e.Takeaway))
		seen := make(map[string]bool)
		for _, w := range words {
			w = strings.Trim(w, ".,;:!?\"'()[]{}")
			if len(w) > 3 && !stopWords[w] && !seen[w] {
				wordCounts[w]++
				seen[w] = true
			}
		}
	}

	type wordFreq struct {
		word  string
		count int
	}
	var sorted []wordFreq
	for w, c := range wordCounts {
		if c >= 2 {
			sorted = append(sorted, wordFreq{w, c})
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	var themes []string
	for _, wf := range sorted {
		themes = append(themes, fmt.Sprintf("%s (%d occurrences)", wf.word, wf.count))
	}
	return themes
}

// filterByConfidence returns learnings above a confidence threshold.
func filterByConfidence(entries []learningEntry, threshold float64) []learningEntry {
	var result []learningEntry
	for _, e := range entries {
		if e.Confidence >= threshold {
			result = append(result, e)
		}
	}
	return result
}

// inferCategory auto-categorizes a learning based on its content.
func inferCategory(entry learningEntry) string {
	text := strings.ToLower(entry.Title + " " + entry.Takeaway + " " + entry.Context)

	keywords := map[string][]string{
		"testing":      {"test", "coverage", "assert", "mock", "fuzz"},
		"architecture": {"design", "pattern", "interface", "struct", "module"},
		"performance":  {"fast", "slow", "optimize", "cache", "memory", "speed"},
		"git":          {"git", "commit", "branch", "merge", "rebase", "push"},
		"ux":           {"user", "prompt", "display", "output", "message"},
		"safety":       {"safe", "protect", "danger", "revert", "error"},
		"learning":     {"learn", "insight", "pattern", "recognize", "remember"},
		"api":          {"api", "provider", "request", "response", "token"},
	}

	bestCat := "general"
	bestScore := 0
	for cat, words := range keywords {
		score := 0
		for _, w := range words {
			if strings.Contains(text, w) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestCat = cat
		}
	}
	return bestCat
}

// inferConfidence assigns a confidence score based on entry quality.
func inferConfidence(entry learningEntry) float64 {
	score := 0.3 // base confidence

	// Has a specific takeaway = higher confidence
	if entry.Takeaway != "" && len(entry.Takeaway) > 20 {
		score += 0.2
	}
	// Has context = higher confidence
	if entry.Context != "" && len(entry.Context) > 20 {
		score += 0.1
	}
	// Has a meaningful title
	if entry.Title != "" && len(entry.Title) > 10 {
		score += 0.1
	}
	// Recent entries get a slight boost
	if entry.Day > 0 {
		score += 0.1
	}
	// Source is evolution (agent's own insight)
	if entry.Source == "evolution" {
		score += 0.1
	}

	if score > 1.0 {
		score = 1.0
	}
	return score
}
