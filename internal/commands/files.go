package commands

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

var (
	reHTMLTag      = regexp.MustCompile(`<[^>]+>`)
	reHTMLComment  = regexp.MustCompile(`<!--[\s\S]*?-->`)
	reScriptStyle  = regexp.MustCompile(`(?i)<(script|style)[^>]*>[\s\S]*?</(script|style)>`)
	reMultiSpace   = regexp.MustCompile(`[ \t]{2,}`)
	reMultiNewline = regexp.MustCompile(`\n{3,}`)
)

// stripHTML removes HTML tags, strips scripts/styles, and normalises whitespace.
func stripHTML(html string) string {
	s := reHTMLComment.ReplaceAllString(html, "")
	s = reScriptStyle.ReplaceAllString(s, "")
	s = reHTMLTag.ReplaceAllString(s, " ")
	s = strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", `"`, "&#39;", "'", "&nbsp;", " ",
		"&mdash;", "—", "&ndash;", "–",
	).Replace(s)
	s = reMultiSpace.ReplaceAllString(s, " ")
	s = reMultiNewline.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// RegisterFileCommands adds file and search commands.
func RegisterFileCommands(r *Registry) {
	registerFileContentCommands(r)
	registerFileSearchCommands(r)
	registerFileNavigationCommands(r)
}

func registerFileContentCommands(r *Registry) {
	r.Register(Command{
		Name:        "/add",
		Aliases:     []string{},
		Description: "inject file into context",
		Category:    "files",
		Handler:     cmdAdd,
	})

	r.Register(Command{
		Name:        "/image",
		Aliases:     []string{},
		Description: "attach image to next message (vision models)",
		Category:    "files",
		Handler:     cmdImage,
	})

	r.Register(Command{
		Name:        "/web",
		Aliases:     []string{},
		Description: "fetch URL into context",
		Category:    "files",
		Handler:     cmdWeb,
	})

	r.Register(Command{
		Name:        "/todos",
		Aliases:     []string{},
		Description: "list TODO/FIXME in codebase",
		Category:    "files",
		Handler:     cmdTodos,
	})

	r.Register(Command{
		Name:        "/deps",
		Aliases:     []string{},
		Description: "show go.mod dependencies",
		Category:    "files",
		Handler:     cmdDeps,
	})
}

func registerFileSearchCommands(r *Registry) {
	r.Register(Command{
		Name:        "/find",
		Aliases:     []string{},
		Description: "fuzzy file search",
		Category:    "files",
		Handler:     cmdFind,
	})

	r.Register(Command{
		Name:        "/grep",
		Aliases:     []string{},
		Description: "search code in repo",
		Category:    "files",
		Handler:     cmdGrep,
	})

	r.Register(Command{
		Name:        "/search",
		Aliases:     []string{},
		Description: "search web or code",
		Category:    "files",
		Handler:     cmdSearch,
	})

	r.Register(Command{
		Name:        "/search-replace",
		Aliases:     []string{},
		Description: "find and replace across repo",
		Category:    "files",
		Handler:     cmdSearchReplace,
	})
}

func registerFileNavigationCommands(r *Registry) {
	r.Register(Command{
		Name:        "/pwd",
		Aliases:     []string{},
		Description: "show current directory",
		Category:    "files",
		Handler:     cmdPwd,
	})

	r.Register(Command{
		Name:        "/cd",
		Aliases:     []string{},
		Description: "change directory",
		Category:    "files",
		Handler:     cmdCd,
	})

	r.Register(Command{
		Name:        "/ls",
		Aliases:     []string{},
		Description: "list directory",
		Category:    "files",
		Handler:     cmdLs,
	})

	r.Register(Command{
		Name:        "/paste",
		Aliases:     []string{},
		Description: "paste from clipboard",
		Category:    "files",
		Handler:     cmdPaste,
	})

	r.Register(Command{
		Name:        "/open",
		Aliases:     []string{},
		Description: "open file in editor",
		Category:    "files",
		Handler:     cmdOpen,
	})
}

func cmdAdd(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /add <filepath>")
		return Result{Handled: true}
	}
	filePath := ctx.Args()

	absPath := filePath
	if !filepath.IsAbs(filePath) {
		absPath = filepath.Join(ctx.RepoPath, filePath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		PrintError("failed to read file: %v", err)
		return Result{Handled: true}
	}

	if ctx.Agent != nil {
		content := fmt.Sprintf("Here is the content of the file `%s`:\n\n```\n%s\n```", filePath, string(data))
		ctx.Agent.Messages = append(ctx.Agent.Messages, iteragent.Message{
			Role:    "user",
			Content: content,
		})
	}

	fmt.Printf("%s✓ read %s (%d bytes) — injected into context%s\n\n", ColorLime, filePath, len(data), ColorReset)
	return Result{Handled: true}
}

func cmdFind(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /find <pattern>")
		return Result{Handled: true}
	}
	pattern := strings.ToLower(ctx.Args())

	var matches []string
	err := filepath.Walk(ctx.RepoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(strings.ToLower(info.Name()), pattern) {
			rel, _ := filepath.Rel(ctx.RepoPath, path)
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		PrintError("walk failed: %v", err)
		return Result{Handled: true}
	}

	sort.Strings(matches)
	if len(matches) == 0 {
		fmt.Println("No matches found.")
	} else {
		fmt.Printf("%s── %d matches ──%s\n", ColorDim, len(matches), ColorReset)
		for _, m := range matches {
			fmt.Printf("  %s\n", m)
		}
	}
	fmt.Println()
	return Result{Handled: true}
}

func cmdWeb(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /web <url> [prompt]")
		return Result{Handled: true}
	}
	rawURL := ctx.Arg(1)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	PrintDim("fetching %s …", rawURL)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		PrintError("invalid URL: %v", err)
		return Result{Handled: true}
	}
	req.Header.Set("User-Agent", "iterate-cli/1.0 (fetch)")
	resp, err := client.Do(req)
	if err != nil {
		PrintError("fetch failed: %v", err)
		return Result{Handled: true}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		PrintError("HTTP %d", resp.StatusCode)
		return Result{Handled: true}
	}

	const maxBodyBytes = 512 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		PrintError("read failed: %v", err)
		return Result{Handled: true}
	}

	ct := resp.Header.Get("Content-Type")
	var text string
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "xhtml") {
		text = stripHTML(string(body))
	} else {
		text = string(body)
	}

	const maxChars = 100_000
	truncated := false
	if len(text) > maxChars {
		text = text[:maxChars]
		truncated = true
	}

	header := fmt.Sprintf("[Web content from %s]\n\n", rawURL)
	if truncated {
		header += "(truncated to first 100 000 characters)\n\n"
	}
	content := header + text

	// Optional follow-up prompt after the URL.
	userPrompt := strings.TrimSpace(strings.TrimPrefix(ctx.Args(), ctx.Arg(1)))
	if userPrompt != "" {
		content = content + "\n\n" + userPrompt
	}

	if ctx.Agent == nil {
		PrintError("no agent available")
		return Result{Handled: true}
	}

	ctx.Agent.Messages = append(ctx.Agent.Messages, iteragent.Message{
		Role:    "user",
		Content: content,
	})

	suffix := ""
	if truncated {
		suffix = " (truncated)"
	}
	PrintSuccess("injected %d chars from %s%s", len(text), rawURL, suffix)

	if userPrompt != "" && ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, "", ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdGrep(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /grep <pattern>")
		return Result{Handled: true}
	}
	pattern := ctx.Args()
	fmt.Printf("%s── grep: %s ──%s\n", ColorDim, pattern, ColorReset)

	var cmd *exec.Cmd
	if _, err := exec.LookPath("rg"); err == nil {
		cmd = exec.Command("rg", "--no-heading", "-n", "-S", "--max-count", "50", pattern)
	} else {
		cmd = exec.Command("grep", "-rn", "--include=*", "-m", "50", pattern)
	}
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		fmt.Print(string(output))
	}
	if err != nil && len(output) == 0 {
		fmt.Println("No matches found.")
	}
	fmt.Println()
	return Result{Handled: true}
}

func cmdTodos(ctx Context) Result {
	fmt.Printf("%s── TODOs ──────────────────────────%s\n", ColorDim, ColorReset)

	var cmd *exec.Cmd
	if _, err := exec.LookPath("rg"); err == nil {
		cmd = exec.Command("rg", "--no-heading", "-n", "-S", "--max-count", "100", "(TODO|FIXME|HACK|XXX)")
	} else {
		cmd = exec.Command("grep", "-rn", "-E", "-m", "100", "(TODO|FIXME|HACK|XXX)")
	}
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		fmt.Print(string(output))
	}
	if err != nil && len(output) == 0 {
		fmt.Println("No TODOs found.")
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdDeps(ctx Context) Result {
	goModPath := filepath.Join(ctx.RepoPath, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		PrintError("failed to read go.mod: %v", err)
		return Result{Handled: true}
	}

	fmt.Printf("%s── Dependencies ───────────────────%s\n", ColorDim, ColorReset)
	lines := strings.Split(string(data), "\n")
	inRequire := false
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "require") {
			if strings.Contains(trimmed, "(") {
				inRequire = true
			} else {
				dep := strings.TrimPrefix(trimmed, "require ")
				if dep != trimmed && dep != "" {
					fmt.Printf("  %s\n", dep)
					count++
				}
			}
			continue
		}
		if inRequire && trimmed == ")" {
			inRequire = false
			continue
		}
		if inRequire && trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			fmt.Printf("  %s\n", trimmed)
			count++
		}
	}
	if count == 0 {
		fmt.Println("  No dependencies found in go.mod")
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdSearch(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /search <query>")
		return Result{Handled: true}
	}
	query := ctx.Args()
	fmt.Printf("%s── search: %s ──%s\n", ColorDim, query, ColorReset)

	var cmd *exec.Cmd
	if _, err := exec.LookPath("rg"); err == nil {
		cmd = exec.Command("rg", "--no-heading", "-n", "-S", "--max-count", "50", "-i", query)
	} else {
		cmd = exec.Command("grep", "-rn", "--include=*", "-i", "-m", "50", query)
	}
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		fmt.Print(string(output))
	}
	if err != nil && len(output) == 0 {
		fmt.Println("No matches found.")
	}
	fmt.Println()
	return Result{Handled: true}
}

// ---------------------------------------------------------------------------
// /image — pending image attachment for vision-capable providers
// ---------------------------------------------------------------------------

// pendingImageAttachment holds an image to be prepended to the next message.
// Access is safe because commands run sequentially in the REPL loop.
var pendingImageAttachment string // non-empty = base64 data URI

// imageMimeType returns the MIME type for common image extensions.
func imageMimeType(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

// cmdImage reads an image file, base64-encodes it, and stores it as a pending
// attachment. The next user message sent via streamAndPrint will prepend the
// image data URI so vision-capable providers can see it.
func cmdImage(ctx Context) Result {
	if !ctx.HasArg(1) {
		if pendingImageAttachment != "" {
			fmt.Printf("%s[image] pending attachment set (%d chars)%s\n\n",
				ColorDim, len(pendingImageAttachment), ColorReset)
		} else {
			fmt.Println("Usage: /image <path>")
		}
		return Result{Handled: true}
	}

	imgPath := ctx.Arg(1)
	if !filepath.IsAbs(imgPath) {
		imgPath = filepath.Join(ctx.RepoPath, imgPath)
	}

	data, err := os.ReadFile(imgPath)
	if err != nil {
		PrintError("cannot read image: %v", err)
		return Result{Handled: true}
	}

	ext := filepath.Ext(imgPath)
	mime := imageMimeType(ext)
	b64 := base64.StdEncoding.EncodeToString(data)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mime, b64)

	pendingImageAttachment = dataURI
	PrintSuccess("image loaded: %s (%s, %d bytes) — will attach to next message",
		filepath.Base(imgPath), mime, len(data))
	return Result{Handled: true}
}

// GetPendingImageAttachment returns and clears any pending image attachment.
// Called by streamAndPrint before sending each request.
func GetPendingImageAttachment() string {
	img := pendingImageAttachment
	pendingImageAttachment = ""
	return img
}
