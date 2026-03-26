package main

import (
	"fmt"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Unified /pr subcommand dispatcher
// ---------------------------------------------------------------------------

type prSubcommand int

const (
	prSubList prSubcommand = iota
	prSubView
	prSubDiff
	prSubReview // AI code review using PR diff
	prSubComment
	prSubCheckout
	prSubCreate
	prSubHelp
)

type parsedPR struct {
	sub    prSubcommand
	number string
	text   string
	draft  bool
}

func parsePRNumberedArg(parts []string, sub prSubcommand) parsedPR {
	num := ""
	if len(parts) > 1 {
		num = parts[1]
	}
	return parsedPR{sub: sub, number: num}
}

func parsePRArgs(args string) parsedPR {
	parts := strings.Fields(strings.TrimSpace(args))
	if len(parts) == 0 {
		return parsedPR{sub: prSubList}
	}
	switch parts[0] {
	case "list", "ls":
		return parsedPR{sub: prSubList}
	case "view":
		return parsePRNumberedArg(parts, prSubView)
	case "diff":
		return parsePRNumberedArg(parts, prSubDiff)
	case "review":
		return parsePRNumberedArg(parts, prSubReview)
	case "comment":
		num, body := "", ""
		if len(parts) > 1 {
			num = parts[1]
		}
		if len(parts) > 2 {
			body = strings.Join(parts[2:], " ")
		}
		return parsedPR{sub: prSubComment, number: num, text: body}
	case "checkout", "co":
		return parsePRNumberedArg(parts, prSubCheckout)
	case "create", "new":
		draft := false
		for _, p := range parts[1:] {
			if p == "--draft" || p == "-d" {
				draft = true
			}
		}
		return parsedPR{sub: prSubCreate, draft: draft}
	default:
		if _, err := strconv.Atoi(parts[0]); err == nil {
			return parsedPR{sub: prSubView, number: parts[0]}
		}
		return parsedPR{sub: prSubHelp}
	}
}

// buildPRReviewPrompt constructs the AI review prompt for the given PR number and diff.
func buildPRReviewPrompt(number, diff string) string {
	return fmt.Sprintf(
		"Review PR #%s. Focus on: correctness, edge cases, security, performance, and style.\n\n```diff\n%s\n```",
		number, diff)
}
