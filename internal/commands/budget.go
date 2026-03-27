package commands

import (
	"fmt"
	"strconv"
)

// RegisterBudgetCommands adds the /budget spending-limit command.
func RegisterBudgetCommands(r *Registry) {
	r.Register(Command{
		Name:        "/budget",
		Aliases:     []string{},
		Description: "show or set spending budget (e.g. /budget 5.00)",
		Category:    "session",
		Handler:     cmdBudget,
	})
}

func cmdBudget(ctx Context) Result {
	if ctx.BudgetLimit == nil || ctx.SessionCostUSD == nil {
		PrintError("budget tracking not available")
		return Result{Handled: true}
	}

	// /budget <amount> — set a new limit.
	if ctx.HasArg(1) {
		val, err := strconv.ParseFloat(ctx.Arg(1), 64)
		if err != nil || val < 0 {
			PrintError("invalid amount: %s (use e.g. /budget 5.00)", ctx.Arg(1))
			return Result{Handled: true}
		}
		*ctx.BudgetLimit = val
		if val == 0 {
			PrintSuccess("budget limit cleared (no limit)")
		} else {
			PrintSuccess("budget limit set to $%.2f", val)
		}
		return Result{Handled: true}
	}

	// No arg — show current spend vs limit.
	spent := *ctx.SessionCostUSD
	limit := *ctx.BudgetLimit

	fmt.Printf("%s── Budget ──────────────────────────%s\n", ColorDim, ColorReset)
	if limit <= 0 {
		fmt.Printf("  Spent:  %s$%.4f%s\n", ColorLime, spent, ColorReset)
		fmt.Printf("  Limit:  %snone%s (use /budget <amount> to set)\n", ColorDim, ColorReset)
	} else {
		pct := 0.0
		if limit > 0 {
			pct = (spent / limit) * 100
		}
		pctColor := ColorLime
		if pct >= 90 {
			pctColor = ColorRed
		} else if pct >= 70 {
			pctColor = ColorYellow
		}
		fmt.Printf("  Spent:  %s$%.4f%s / %s$%.2f%s limit  %s(%.1f%%)%s\n",
			ColorLime, spent, ColorReset,
			ColorBold, limit, ColorReset,
			pctColor, pct, ColorReset)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}
