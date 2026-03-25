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

// RunCommunicatePhase posts issue responses, writes the journal entry, merges PR if created, and reflects on learnings.
func (e *Engine) RunCommunicatePhase(ctx context.Context, p iteragent.Provider) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	planPath := filepath.Join(e.repoPath, "SESSION_PLAN.md")
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		e.logger.Warn("SESSION_PLAN.md not found, skipping communicate phase")
		return nil
	}

	identity, err := os.ReadFile(filepath.Join(e.repoPath, "docs/IDENTITY.md"))
	if err != nil {
		e.logger.Warn("failed to read IDENTITY.md", "err", err)
	}
	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	tools := iteragent.DefaultTools(e.repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(e.repoPath, "skills")})

	e.selfReviewAndMergePR(ctx, p, tools, systemPrompt, skills)

	// Pull latest main so journal commit doesn't conflict with merged PR.
	if _, err := e.runTool(ctx, "bash", map[string]string{"cmd": "git pull --rebase origin main 2>/dev/null || true"}); err != nil {
		e.logger.Warn("failed to pull main before journal write", "err", err)
	}

	day := e.readDayCount()
	e.writeJournalEntry(ctx, p, tools, systemPrompt, skills, day)

	// Commit journal entry directly so it's not lost if the workflow push step skips.
	if _, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf(`git add docs/JOURNAL.md memory/ && git diff --cached --quiet || git commit -m "journal: Day %s session entry"`, day),
	}); err != nil {
		e.logger.Warn("failed to commit journal entry", "err", err)
	}

	e.postIssueComments(ctx, p, tools, systemPrompt, skills, string(planBytes))
	e.recordLearnings(ctx, p, tools, systemPrompt, skills, day)

	return nil
}

func (e *Engine) readDayCount() string {
	dayBytes, err := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))
	if err != nil {
		e.logger.Warn("failed to read DAY_COUNT", "err", err)
	}
	return strings.TrimSpace(string(dayBytes))
}

// selfReviewAndMergePR reviews the PR from the implement phase and merges if approved.
func (e *Engine) selfReviewAndMergePR(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet) {
	if e.prNumber <= 0 {
		return
	}

	e.logger.Info("PR found from implement phase, running self-review", "pr", e.prNumber)

	prDiff, _ := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("gh pr diff %d --repo %s 2>/dev/null || echo ''", e.prNumber, e.repo),
	})

	// Ask for a plain text verdict only — no tool calls, just LGTM or BLOCK.
	reviewPrompt := fmt.Sprintf(`Review PR #%d diff for blocking issues only (build failures, panics, security holes).

## PR Diff:
%s

Reply with exactly one word:
- LGTM — no blocking issues, safe to merge
- BLOCK — has a blocking issue (explain briefly on the same line)

Do not use any tools. Reply with just your verdict.`, e.prNumber, util.Truncate(prDiff, 6000))

	a := e.newAgent(p, nil, systemPrompt, skills) // no tools — pure text verdict
	var reviewOutput string
	for ev := range a.Prompt(ctx, reviewPrompt) {
		if e.eventSink != nil {
			select {
			case e.eventSink <- ev:
			default:
			}
		}
		if ev.Type == string(iteragent.EventMessageEnd) {
			reviewOutput = ev.Content
		}
	}
	a.Finish()

	verdict := strings.ToUpper(strings.TrimSpace(reviewOutput))
	blocked := strings.HasPrefix(verdict, "BLOCK")

	if blocked {
		e.logger.Warn("PR self-review blocked merge", "output", util.Truncate(reviewOutput, 200))
	} else {
		// Default to merge — if the agent didn't explicitly block, proceed.
		if err := e.mergePR(ctx); err != nil {
			e.logger.Warn("PR merge failed in communicate phase", "err", err)
		} else {
			e.logger.Info("PR merged successfully in communicate phase", "pr", e.prNumber)
		}
	}

	_ = e.switchToMain(ctx)
}

// postIssueComments posts GitHub comments for each issue response in the plan,
// skipping any issue that already has a bot comment from this session's day.
func (e *Engine) postIssueComments(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet, plan string) {
	responses := parseIssueResponses(plan)
	day := e.readDayCount()
	for _, resp := range responses {
		if e.issueAlreadyCommented(ctx, resp.IssueNum, day) {
			e.logger.Info("skipping issue comment, already posted today", "issue", resp.IssueNum, "day", day)
			continue
		}
		body := fmt.Sprintf("Status: %s\nReason: %s", resp.Status, resp.Reason)
		if e.prURL != "" && (resp.Status == "implement" || resp.Status == "partial") {
			body += fmt.Sprintf("\n\nPR: %s", e.prURL)
		}
		userMsg := fmt.Sprintf(`Post a GitHub issue comment on issue #%d.

Be brief, honest, and in your own voice. Sign off with your day count.

Issue response body:
%s

Use: gh issue comment %d --repo %s --body "..."`,
			resp.IssueNum, body, resp.IssueNum, e.repo)
		a := e.newAgent(p, tools, systemPrompt, skills)
		e.forwardEvents(a.Prompt(ctx, userMsg))
		a.Finish()
	}
}

// issueAlreadyCommented checks whether the bot has already commented on the given issue
// during the current day by inspecting recent comments via gh CLI.
func (e *Engine) issueAlreadyCommented(ctx context.Context, issueNum int, day string) bool {
	out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("gh issue view %d --repo %s --comments --json comments --jq '.comments[-3:][].body' 2>/dev/null || echo ''", issueNum, e.repo),
	})
	if err != nil || out == "" {
		return false
	}
	// Match "Day <N>" as a whole word to avoid "Day 1" matching "Day 10".
	pattern := `\bDay ` + day + `\b`
	matched, _ := regexp.MatchString(pattern, out)
	return matched
}

// writeJournalEntry generates and writes the journal entry for this session.
func (e *Engine) writeJournalEntry(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet, day string) {
	// Use a fresh context so journal writing can't be starved by earlier phase work.
	journalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Read recent commits to give the agent context without requiring a tool call.
	recentCommits, _ := e.runTool(journalCtx, "bash", map[string]string{"cmd": "git log --oneline -8"})

	journalMsg := `Write a journal entry for this evolution session. Reply with ONLY the journal entry — no explanation, no preamble, no markdown fences.

Recent commits:
` + recentCommits + `

Format (use exactly this structure):
## Day ` + day + ` — HH:MM — Title

Body paragraph here (2-4 honest sentences about what was done, what worked, what failed).

Rules:
- HH:MM = current UTC time in 24h format
- Title = specific description of what happened this session
- Be honest: mention what was tried, what worked, what failed
- If nothing was implemented, write "Evolution session completed." as the body
- Start your reply with "## Day" — nothing before it`

	a := e.newAgent(p, nil, systemPrompt, skills) // no tools — pure text response
	var journalEntry string
	for ev := range a.Prompt(journalCtx, journalMsg) {
		if e.eventSink != nil {
			select {
			case e.eventSink <- ev:
			default:
			}
		}
		if ev.Type == string(iteragent.EventMessageEnd) {
			journalEntry = strings.TrimSpace(ev.Content)
		}
	}
	a.Finish()

	e.persistJournalEntry(journalEntry, day)
}

// persistJournalEntry extracts and writes a valid journal entry to JOURNAL.md.
func (e *Engine) persistJournalEntry(journalEntry string, day string) {
	if journalEntry == "" {
		e.logger.Warn("journal entry is empty — skipping journal write")
		return
	}

	// Try to find the journal header. Agents format things differently.
	idx := strings.Index(journalEntry, "## Day")
	if idx < 0 {
		idx = strings.Index(journalEntry, "# Day")
	}

	if idx >= 0 {
		extracted := journalEntry[idx:]
		if nextIdx := strings.Index(extracted[1:], "\n## "); nextIdx >= 0 {
			extracted = extracted[:nextIdx+1]
		}
		extracted = strings.TrimSpace(extracted)
		dayNum, _ := strconv.Atoi(day)
		if dayNum > 0 {
			dayPattern := regexp.MustCompile(`^(#+)\s*Day\s*\d+`)
			extracted = dayPattern.ReplaceAllString(extracted, fmt.Sprintf("## Day %d", dayNum))
		}
		journalPath := filepath.Join(e.repoPath, "docs/JOURNAL.md")
		_ = os.MkdirAll(filepath.Dir(journalPath), 0o755)
		journal, err := os.ReadFile(journalPath)
		if err != nil {
			e.logger.Warn("failed to read JOURNAL.md for journal update", "err", err)
		}
		header := "# iterate Evolution Journal\n"
		newContent := header + "\n" + extracted + "\n\n" + strings.TrimPrefix(strings.TrimPrefix(string(journal), header), "\n")
		_ = os.WriteFile(journalPath, []byte(newContent), 0o644)
	} else {
		// Fallback: wrap the agent's output in a proper journal entry.
		e.logger.Warn("agent output does not contain '## Day' — writing fallback journal entry")
		now := time.Now().UTC().Format("15:04")
		dayNum, _ := strconv.Atoi(day)
		fallback := fmt.Sprintf("## Day %d — %s — Evolution session\n\n%s\n", dayNum, now, strings.TrimSpace(journalEntry))
		journalPath := filepath.Join(e.repoPath, "docs/JOURNAL.md")
		_ = os.MkdirAll(filepath.Dir(journalPath), 0o755)
		journal, err := os.ReadFile(journalPath)
		if err != nil {
			e.logger.Warn("failed to read JOURNAL.md for journal update", "err", err)
		}
		header := "# iterate Evolution Journal\n"
		newContent := header + "\n" + fallback + "\n" + strings.TrimPrefix(strings.TrimPrefix(string(journal), header), "\n")
		_ = os.WriteFile(journalPath, []byte(newContent), 0o644)
	}
}

// recordLearnings prompts the agent to reflect on what was learned this session.
func (e *Engine) recordLearnings(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet, day string) {
	learnings, _ := os.ReadFile(filepath.Join(e.repoPath, "memory", "ACTIVE_LEARNINGS.md"))
	learningsMsg := fmt.Sprintf(`Did this session teach you something genuinely new that would change how you act next time?

Read memory/ACTIVE_LEARNINGS.md first to avoid duplicates.
If yes, append ONE entry to memory/learnings.jsonl using python3:

python3 -c "
import json, datetime
entry = {'type':'lesson','day':%s,'ts':datetime.datetime.utcnow().strftime('%%Y-%%m-%%dT%%H:%%M:%%SZ'),'source':'evolution','title':'[title]','context':'[what you tried]','takeaway':'[the lesson]'}
open('memory/learnings.jsonl','a').write(json.dumps(entry)+'\n')
"

If nothing genuinely new was learned, do nothing.

## What you already know:
%s`,
		day,
		util.Truncate(string(learnings), 400),
	)

	a := e.newAgent(p, tools, systemPrompt, skills)
	e.forwardEvents(a.Prompt(ctx, learningsMsg))
	a.Finish()
}
