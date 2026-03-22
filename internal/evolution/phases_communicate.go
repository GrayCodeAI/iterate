package evolution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/GrayCodeAI/iterate/internal/util"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RunCommunicatePhase posts issue responses, writes the journal entry, merges PR if created, and reflects on learnings.
func (e *Engine) RunCommunicatePhase(ctx context.Context, p iteragent.Provider) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	planPath := filepath.Join(e.repoPath, "docs/docs/SESSION_PLAN.md")
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		e.logger.Warn("SESSION_PLAN.md not found, skipping communicate phase")
		return nil
	}

	identity, err := os.ReadFile(filepath.Join(e.repoPath, "docs/docs/IDENTITY.md"))
	if err != nil {
		e.logger.Warn("failed to read IDENTITY.md", "err", err)
	}
	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	tools := iteragent.DefaultTools(e.repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(e.repoPath, "skills")})

	e.selfReviewAndMergePR(ctx, p, tools, systemPrompt, skills)
	e.postIssueComments(ctx, p, tools, systemPrompt, skills, string(planBytes))

	day := e.readDayCount()
	e.writeJournalEntry(ctx, p, tools, systemPrompt, skills, day)
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

	reviewPrompt := fmt.Sprintf(`Review PR #%d changes critically. Check for bugs, security issues, missing tests, and code quality.

## PR Diff:
%s

If you find issues, fix them, amend your commit, and push. 
If changes are good, reply: "LGTM"

After your review, also merge this PR using:
gh pr merge %d --repo %s --squash --delete-branch

Or if there are issues that prevent merge, reply with details about what needs fixing.`, e.prNumber, util.Truncate(prDiff, 6000), e.prNumber, e.repo)

	a := e.newAgent(p, tools, systemPrompt, skills)
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

	if strings.Contains(strings.ToLower(reviewOutput), "lgtm") || strings.Contains(strings.ToLower(reviewOutput), "looks good") {
		if err := e.mergePR(ctx); err != nil {
			e.logger.Warn("PR merge failed in communicate phase", "err", err)
		} else {
			e.logger.Info("PR merged successfully in communicate phase", "pr", e.prNumber)
		}
	} else {
		e.logger.Warn("PR self-review found issues, not merging", "output", util.Truncate(reviewOutput, 200))
	}

	_ = e.switchToMain(ctx)
}

// postIssueComments posts GitHub comments for each issue response in the plan.
func (e *Engine) postIssueComments(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet, plan string) {
	responses := parseIssueResponses(plan)
	for _, resp := range responses {
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

// writeJournalEntry generates and writes the journal entry for this session.
func (e *Engine) writeJournalEntry(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet, day string) {
	journalMsg := `First, run this tool call to see recent commits:

` + "```tool" + `
{"tool":"bash","args":{"cmd":"git log --oneline -10"}}
` + "```" + `

Then write a journal entry based on the output. Your ENTIRE reply must start with "## Day" and contain ONLY the journal entry — no explanation, no preamble, no markdown fences.

Format:
## Day ` + day + ` — HH:MM — Title

Body paragraph here (2-4 honest sentences).

Rules:
- HH:MM = current UTC time
- Title = specific description of what was done this session
- Be honest: say what you tried, what worked, what failed
- If nothing was implemented, say "Evolution session completed." and nothing more
- Your reply MUST start with "## Day" — no text before it`

	a := e.newAgent(p, tools, systemPrompt, skills)
	var journalEntry string
	for ev := range a.Prompt(ctx, journalMsg) {
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
	if idx := strings.Index(journalEntry, "## Day"); idx >= 0 {
		extracted := journalEntry[idx:]
		if nextIdx := strings.Index(extracted[1:], "\n## "); nextIdx >= 0 {
			extracted = extracted[:nextIdx+1]
		}
		extracted = strings.TrimSpace(extracted)
		dayNum, _ := strconv.Atoi(day)
		if dayNum > 0 {
			dayPattern := regexp.MustCompile(`^## Day \d+`)
			extracted = dayPattern.ReplaceAllString(extracted, fmt.Sprintf("## Day %d", dayNum))
		}
		journal, err := os.ReadFile(filepath.Join(e.repoPath, "docs/docs/JOURNAL.md"))
		if err != nil {
			e.logger.Warn("failed to read JOURNAL.md for journal update", "err", err)
		}
		header := "# iterate Evolution Journal\n"
		newContent := header + "\n" + extracted + "\n\n" + strings.TrimPrefix(strings.TrimPrefix(string(journal), header), "\n")
		_ = os.WriteFile(filepath.Join(e.repoPath, "docs/docs/JOURNAL.md"), []byte(newContent), 0o644) // best-effort; journal is append-mostly
	} else {
		e.logger.Warn("agent output does not contain '## Day' — skipping journal write")
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
