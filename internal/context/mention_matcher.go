// Package context provides smart @ mention matching capabilities.
// Task 45: Smart @ Mention with fuzzy matching

package context

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MentionConfig holds configuration for mention matching.
type MentionConfig struct {
	MaxResults      int           `json:"max_results"`       // Max results per query
	MinScore        float64       `json:"min_score"`         // Minimum fuzzy match score (0-1)
	FuzzyThreshold  float64       `json:"fuzzy_threshold"`   // Threshold for fuzzy matching
	IncludeHidden   bool          `json:"include_hidden"`    // Include hidden files
	CacheTTL        time.Duration `json:"cache_ttl"`         // Cache time-to-live
	PreferRecent    bool          `json:"prefer_recent"`     // Prefer recently accessed files
	PreferOpenFiles bool          `json:"prefer_open_files"` // Prefer currently open files
}

// DefaultMentionConfig returns default configuration.
func DefaultMentionConfig() *MentionConfig {
	return &MentionConfig{
		MaxResults:      10,
		MinScore:        0.3,
		FuzzyThreshold:  0.5,
		IncludeHidden:   false,
		CacheTTL:        5 * time.Minute,
		PreferRecent:    true,
		PreferOpenFiles: true,
	}
}

// MentionMatch represents a matched mention.
type MentionMatch struct {
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	Type          string    `json:"type"` // "file", "folder", "symbol"
	Score         float64   `json:"score"`
	MatchType     string    `json:"match_type"`               // "exact", "prefix", "fuzzy", "basename"
	MatchedRanges []int     `json:"matched_ranges,omitempty"` // [start, end] pairs
	LastAccessed  time.Time `json:"last_accessed,omitempty"`
	IsOpen        bool      `json:"is_open,omitempty"`
}

// MentionResult contains the result of a mention query.
type MentionResult struct {
	Query    string          `json:"query"`
	Matches  []*MentionMatch `json:"matches"`
	Total    int             `json:"total"`
	Duration time.Duration   `json:"duration"`
}

// MentionMatcher handles fuzzy matching for @ mentions.
type MentionMatcher struct {
	config *MentionConfig
	logger *slog.Logger
	mu     sync.RWMutex

	// File index
	files       map[string]*FileInfo
	recentFiles []string
	openFiles   map[string]bool

	// Cache
	queryCache  map[string]*MentionResult
	cacheExpiry time.Time

	// Symbol index for faster lookups
	basenameIndex map[string][]string // basename -> full paths
}

// FileInfo holds information about a file for matching.
type FileInfo struct {
	Path         string
	Basename     string
	Ext          string
	Dir          string
	LastAccessed time.Time
	IsOpen       bool
	IsHidden     bool
}

// NewMentionMatcher creates a new mention matcher.
func NewMentionMatcher(config *MentionConfig, logger *slog.Logger) *MentionMatcher {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultMentionConfig()
	}

	return &MentionMatcher{
		config:        config,
		logger:        logger.With("component", "mention_matcher"),
		files:         make(map[string]*FileInfo),
		openFiles:     make(map[string]bool),
		queryCache:    make(map[string]*MentionResult),
		basenameIndex: make(map[string][]string),
	}
}

// IndexFiles indexes files for matching.
func (mm *MentionMatcher) IndexFiles(files []string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Clear existing index
	mm.files = make(map[string]*FileInfo)
	mm.basenameIndex = make(map[string][]string)

	for _, path := range files {
		info := &FileInfo{
			Path:     path,
			Basename: filepath.Base(path),
			Ext:      filepath.Ext(path),
			Dir:      filepath.Dir(path),
		}

		// Check if hidden
		info.IsHidden = strings.HasPrefix(info.Basename, ".")

		// Add to files map
		mm.files[path] = info

		// Add to basename index
		mm.basenameIndex[info.Basename] = append(mm.basenameIndex[info.Basename], path)

		// Add to recent files if applicable
		if mm.config.PreferRecent {
			mm.recentFiles = append(mm.recentFiles, path)
		}
	}
}

// SetOpenFiles sets the currently open files.
func (mm *MentionMatcher) SetOpenFiles(files []string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.openFiles = make(map[string]bool)
	for _, f := range files {
		mm.openFiles[f] = true
		if info, ok := mm.files[f]; ok {
			info.IsOpen = true
		}
	}
}

// SetRecentFiles sets the recently accessed files.
func (mm *MentionMatcher) SetRecentFiles(files []string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.recentFiles = files
}

// Match finds files matching the query.
func (mm *MentionMatcher) Match(ctx context.Context, query string) (*MentionResult, error) {
	start := time.Now()

	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Check cache
	if result, ok := mm.queryCache[query]; ok && time.Now().Before(mm.cacheExpiry) {
		return result, nil
	}

	result := &MentionResult{
		Query:   query,
		Matches: make([]*MentionMatch, 0),
	}

	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return result, nil
	}

	// Collect all matches
	var matches []*MentionMatch

	for _, info := range mm.files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Skip hidden files unless configured
		if info.IsHidden && !mm.config.IncludeHidden {
			continue
		}

		match := mm.matchFile(query, info)
		if match != nil && match.Score >= mm.config.MinScore {
			matches = append(matches, match)
		}
	}

	// Sort by score (descending)
	sort.Slice(matches, func(i, j int) bool {
		// Prefer open files
		if mm.config.PreferOpenFiles {
			if matches[i].IsOpen != matches[j].IsOpen {
				return matches[i].IsOpen
			}
		}
		// Then by score
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		// Then by recent
		if mm.config.PreferRecent {
			return matches[i].LastAccessed.After(matches[j].LastAccessed)
		}
		return matches[i].Path < matches[j].Path
	})

	// Limit results
	if len(matches) > mm.config.MaxResults {
		matches = matches[:mm.config.MaxResults]
	}

	result.Matches = matches
	result.Total = len(matches)
	result.Duration = time.Since(start)

	// Cache result
	mm.queryCache[query] = result
	mm.cacheExpiry = time.Now().Add(mm.config.CacheTTL)

	return result, nil
}

// matchFile matches a file against the query.
func (mm *MentionMatcher) matchFile(query string, info *FileInfo) *MentionMatch {
	basename := strings.ToLower(info.Basename)
	path := strings.ToLower(info.Path)

	match := &MentionMatch{
		Path:   info.Path,
		Name:   info.Basename,
		Type:   "file",
		IsOpen: mm.openFiles[info.Path],
	}

	// 1. Exact match on basename
	if basename == query {
		match.Score = 1.0
		match.MatchType = "exact"
		return match
	}

	// 2. Exact match on full path
	if path == query {
		match.Score = 0.95
		match.MatchType = "exact"
		return match
	}

	// 3. Prefix match on basename
	if strings.HasPrefix(basename, query) {
		match.Score = 0.9
		match.MatchType = "prefix"
		match.MatchedRanges = []int{0, len(query)}
		return match
	}

	// 4. Contains match on basename
	if strings.Contains(basename, query) {
		match.Score = 0.8
		match.MatchType = "contains"
		idx := strings.Index(basename, query)
		match.MatchedRanges = []int{idx, idx + len(query)}
		return match
	}

	// 5. Prefix match on path
	if strings.HasPrefix(path, query) {
		match.Score = 0.7
		match.MatchType = "prefix"
		return match
	}

	// 6. Contains match on path
	if strings.Contains(path, query) {
		match.Score = 0.6
		match.MatchType = "contains"
		return match
	}

	// 7. Fuzzy match on basename
	if score, ranges := mm.fuzzyMatch(query, basename); score >= mm.config.FuzzyThreshold {
		match.Score = score * 0.5 // Scale down fuzzy matches
		match.MatchType = "fuzzy"
		match.MatchedRanges = ranges
		return match
	}

	return nil
}

// fuzzyMatch performs fuzzy matching using a simple algorithm.
// Returns score (0-1) and matched character ranges.
func (mm *MentionMatcher) fuzzyMatch(pattern, text string) (float64, []int) {
	if len(pattern) == 0 {
		return 1.0, nil
	}
	if len(pattern) > len(text) {
		return 0, nil
	}

	// Find consecutive matches for bonus scoring
	patternIdx := 0
	matchedIndices := make([]int, 0)
	consecutive := 0
	maxConsecutive := 0

	for i := 0; i < len(text) && patternIdx < len(pattern); i++ {
		if text[i] == pattern[patternIdx] {
			matchedIndices = append(matchedIndices, i)
			patternIdx++
			consecutive++
			if consecutive > maxConsecutive {
				maxConsecutive = consecutive
			}
		} else {
			consecutive = 0
		}
	}

	// Check if all pattern characters were matched
	if patternIdx != len(pattern) {
		return 0, nil
	}

	// Calculate score
	// Base score: proportion of pattern matched
	baseScore := float64(len(matchedIndices)) / float64(len(pattern))

	// Bonus for consecutive matches
	consecutiveBonus := float64(maxConsecutive) / float64(len(pattern)) * 0.3

	// Bonus for matching at start
	startBonus := 0.0
	if len(matchedIndices) > 0 && matchedIndices[0] == 0 {
		startBonus = 0.2
	}

	// Penalty for spread-out matches
	spreadPenalty := 0.0
	if len(matchedIndices) > 1 {
		spread := matchedIndices[len(matchedIndices)-1] - matchedIndices[0]
		maxSpread := len(text) - 1
		if maxSpread > 0 {
			spreadPenalty = float64(spread) / float64(maxSpread) * 0.1
		}
	}

	score := baseScore + consecutiveBonus + startBonus - spreadPenalty

	// Clamp score to [0, 1]
	if score > 1.0 {
		score = 1.0
	}
	if score < 0 {
		score = 0
	}

	// Build ranges from matched indices
	ranges := make([]int, 0)
	if len(matchedIndices) > 0 {
		start := matchedIndices[0]
		end := matchedIndices[0]
		for i := 1; i < len(matchedIndices); i++ {
			if matchedIndices[i] == end+1 {
				end = matchedIndices[i]
			} else {
				ranges = append(ranges, start, end+1)
				start = matchedIndices[i]
				end = matchedIndices[i]
			}
		}
		ranges = append(ranges, start, end+1)
	}

	return score, ranges
}

// MatchByType finds matches filtered by type.
func (mm *MentionMatcher) MatchByType(ctx context.Context, query string, mentionType string) (*MentionResult, error) {
	result, err := mm.Match(ctx, query)
	if err != nil {
		return nil, err
	}

	// Filter by type
	filtered := make([]*MentionMatch, 0)
	for _, m := range result.Matches {
		if m.Type == mentionType {
			filtered = append(filtered, m)
		}
	}

	result.Matches = filtered
	result.Total = len(filtered)
	return result, nil
}

// GetSuggestions returns quick suggestions for a prefix.
func (mm *MentionMatcher) GetSuggestions(prefix string, limit int) []string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	suggestions := make([]string, 0)
	prefix = strings.ToLower(prefix)

	// Check basename index first
	for basename, paths := range mm.basenameIndex {
		if strings.HasPrefix(strings.ToLower(basename), prefix) {
			if len(paths) > 0 {
				suggestions = append(suggestions, paths[0])
			}
		}
		if len(suggestions) >= limit {
			break
		}
	}

	return suggestions
}

// ClearCache clears the query cache.
func (mm *MentionMatcher) ClearCache() {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.queryCache = make(map[string]*MentionResult)
}

// GetStats returns statistics about the matcher.
func (mm *MentionMatcher) GetStats() map[string]interface{} {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	return map[string]interface{}{
		"indexed_files":    len(mm.files),
		"open_files":       len(mm.openFiles),
		"recent_files":     len(mm.recentFiles),
		"cached_queries":   len(mm.queryCache),
		"unique_basenames": len(mm.basenameIndex),
	}
}

// UpdateConfig updates the matcher configuration.
func (mm *MentionMatcher) UpdateConfig(config *MentionConfig) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.config = config
}

// ResolveMention resolves a mention string to a file path.
func (mm *MentionMatcher) ResolveMention(mention string) (string, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Direct path match
	if _, ok := mm.files[mention]; ok {
		return mention, true
	}

	// Check basename index
	if paths, ok := mm.basenameIndex[mention]; ok && len(paths) > 0 {
		return paths[0], true
	}

	// Try case-insensitive basename match
	lowerMention := strings.ToLower(mention)
	for basename, paths := range mm.basenameIndex {
		if strings.ToLower(basename) == lowerMention && len(paths) > 0 {
			return paths[0], true
		}
	}

	return "", false
}

// ToMarkdown generates a markdown representation of the mention result.
func (r *MentionResult) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString("### @ Mention Results\n\n")
	sb.WriteString("Query: `")
	sb.WriteString(r.Query)
	sb.WriteString("` (")
	sb.WriteString(r.Duration.String())
	sb.WriteString(")\n\n")

	if len(r.Matches) == 0 {
		sb.WriteString("No matches found.\n")
		return sb.String()
	}

	for _, m := range r.Matches {
		sb.WriteString("- **")
		sb.WriteString(m.Name)
		sb.WriteString("** (")
		sb.WriteString(m.MatchType)
		sb.WriteString(", score: ")
		scoreStr := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", m.Score), "0"), ".")
		sb.WriteString(scoreStr)
		sb.WriteString(")\n  `")
		sb.WriteString(m.Path)
		sb.WriteString("`\n")
		if m.IsOpen {
			sb.WriteString("  📄 *open*\n")
		}
	}

	return sb.String()
}
