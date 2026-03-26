package evolution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/GrayCodeAI/iterate/internal/util"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RunCommunicatePhase writes journal, posts issue comments, records learnings.
func (e *Engine) RunCommunicatePhase(ctx context.Context, p iteragent.Provider) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	planBytes, err := os.ReadFile(filepath.Join(e.repoPath, "SESSION_PLAN.md"))
	if err != nil {
		e.logger.Warn("SESSION_PLAN.md not found, writing fallback journal entry and skipping issue comments")
		planBytes = []byte("")
	}

	day := e.readDayCount()

	// Write journal (always, even if agent returns empty)
	e.writeJournalEntry(ctx, p, day)

	// Commit journal
	if _, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf(`git add docs/JOURNAL.md memory/ && git diff --cached --quiet || git commit -m "journal: Day %s session entry"`, day),
	}); err != nil {
		e.logger.Warn("failed to commit journal", "err", err)
	}

	// Post issue comments if we have issues
	identity, _ := os.ReadFile(filepath.Join(e.repoPath, "docs/IDENTITY.md"))
	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	tools := iteragent.DefaultTools(e.repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(e.repoPath, "skills")})
	e.postIssueComments(ctx, p, tools, systemPrompt, skills, string(planBytes))

	// Record learnings
	e.recordLearnings(ctx, p, tools, systemPrompt, skills, day)

	return nil
}

func (e *Engine) readDayCount() string {
	dayBytes, err := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))
	if err != nil {
		return "0"
	}
	return strings.TrimSpace(string(dayBytes))
}

// writeJournalEntry always produces a journal entry — via agent or fallback.
func (e *Engine) writeJournalEntry(ctx context.Context, p iteragent.Provider, day string) {
	journalCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	recentCommits, _ := e.runTool(journalCtx, "bash", map[string]string{"cmd": "git log --oneline -8"})

	minimalPrompt := "You are iterate, a self-evolving coding agent. Reply with ONLY text, no tool calls."

	journalMsg := fmt.Sprintf(`Write a journal entry for this evolution session. Reply with ONLY the entry text.

Recent commits:
%s

Format:
## Day %s — HH:MM — Title

Body (2-4 honest sentences about what was done, what worked, what failed).

Rules:
- Start with "## Day %s" exactly
- If nothing happened, write "Evolution session completed." as the body`, recentCommits, day, day)

	a := e.newAgent(p, nil, minimalPrompt, nil)
	var journalEntry string
	for ev := range a.Prompt(journalCtx, journalMsg) {
		if ev.Type == string(iteragent.EventMessageEnd) {
			journalEntry = strings.TrimSpace(ev.Content)
		}
	}
	a.Finish()

	e.persistJournalEntry(journalEntry, day)
}

// persistJournalEntry writes the journal entry, generating a fallback if needed.
func (e *Engine) persistJournalEntry(journalEntry string, day string) {
	dayNum, err := strconv.Atoi(day)
	if err != nil {
		dayNum = 0
	}
	now := time.Now().UTC().Format("15:04")

	if journalEntry == "" {
		journalEntry = fmt.Sprintf("## Day %d — %s — Evolution session\n\nEvolution session completed.\n", dayNum, now)
	}

	idx := strings.Index(journalEntry, "## Day")
	if idx < 0 {
		idx = strings.Index(journalEntry, "# Day")
	}

	var extracted string
	if idx >= 0 {
		extracted = journalEntry[idx:]
		if nextIdx := strings.Index(extracted[1:], "\n## "); nextIdx >= 0 {
			extracted = extracted[:nextIdx+1]
		}
	} else {
		extracted = fmt.Sprintf("## Day %d — %s — Evolution session\n\n%s\n", dayNum, now, strings.TrimSpace(journalEntry))
	}

	extracted = strings.TrimSpace(extracted)
	// Always normalize day number — agent might output wrong day
	dayPattern := regexp.MustCompile(`^(#+)\s*Day\s*\d+`)
	extracted = dayPattern.ReplaceAllString(extracted, fmt.Sprintf("## Day %d", dayNum))

	journalPath := filepath.Join(e.repoPath, "docs/JOURNAL.md")
	_ = os.MkdirAll(filepath.Dir(journalPath), 0o755)
	journal, _ := os.ReadFile(journalPath)

	header := "# iterate Evolution Journal\n"
	if !strings.HasPrefix(string(journal), header) {
		journal = []byte(header)
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(string(journal), header), "\n")
	newContent := header + "\n" + extracted + "\n\n" + rest
	_ = os.WriteFile(journalPath, []byte(newContent), 0o644)
}

// issueAlreadyCommented checks if the bot already commented on an issue today.
func (e *Engine) issueAlreadyCommented(ctx context.Context, issueNum int, day string) bool {
	out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("gh issue view %d --repo %s --comments --json comments --jq '.comments[-3:][].body' 2>/dev/null || echo ''", issueNum, e.repo),
	})
	if err != nil || out == "" {
		return false
	}
	pattern := `\bDay ` + day + `\b`
	matched, _ := regexp.MatchString(pattern, out)
	return matched
}

// postIssueComments posts GitHub comments for addressed issues.
func (e *Engine) postIssueComments(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet, plan string) {
	responses := parseIssueResponses(plan)
	for _, resp := range responses {
		body := fmt.Sprintf("Status: %s\nReason: %s", resp.Status, resp.Reason)
		userMsg := fmt.Sprintf(`Post a GitHub issue comment on issue #%d. Be brief. Sign off with your day count.

Body: %s

Use: gh issue comment %d --repo %s --body "..."`, resp.IssueNum, body, resp.IssueNum, e.repo)
		a := e.newAgent(p, tools, systemPrompt, skills)
		e.forwardEvents(a.Prompt(ctx, userMsg))
		a.Finish()
	}
}

// recordLearnings prompts the agent to reflect on what was learned.
func (e *Engine) recordLearnings(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet, day string) {
	learnings, _ := os.ReadFile(filepath.Join(e.repoPath, "memory", "ACTIVE_LEARNINGS.md"))
	learningsMsg := fmt.Sprintf(`Did this session teach you something new?

Read memory/ACTIVE_LEARNINGS.md first. If yes, append to memory/learnings.jsonl using python3:

python3 -c "
import json, datetime
entry = {'type':'lesson','day':%s,'ts':datetime.datetime.utcnow().strftime('%%Y-%%m-%%dT%%H:%%M:%%SZ'),'source':'evolution','title':'[title]','context':'[what you tried]','takeaway':'[the lesson]'}
open('memory/learnings.jsonl','a').write(json.dumps(entry)+'\n')
"

If nothing new was learned, do nothing.

What you already know:
%s`, day, util.Truncate(string(learnings), 400))

	a := e.newAgent(p, tools, systemPrompt, skills)
	e.forwardEvents(a.Prompt(ctx, learningsMsg))
	a.Finish()
}
