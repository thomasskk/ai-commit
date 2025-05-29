package main

import (
	"context"
	"fmt"
	"google.golang.org/genai"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const promptTemplate = `You are an AI assistant specialized in generating concise, single-line Conventional Commit messages from git diffs.
Your **sole task** is to produce a commit message.
The **primary and strongly preferred output** is a single line adhering to this exact format:
<emoji> <type>(<scope>): <short description>
e.g., 🐛 fix(parser): Correct off-by-one error in tokenization

**Body and Footer (AVOID unless absolutely CRITICAL):**
*   Only include a body or footer if the changes are exceptionally complex AND a single subject line is **demonstrably insufficient** to convey a **vital aspect** (e.g., a significant BREAKING CHANGE that cannot be summarized or hinted at, or an essential issue link).
*   **Your default behavior must be to summarize everything into the single subject line.**
*   If unavoidable, separate the body/footer with blank lines as per the specification.

Available types and their emojis (choose one for the subject line):
- feat: ✨ (A new feature)
- fix: 🐛 (A bug fix)
- docs: 📚 (Documentation only changes)
- style: 💎 (Changes that do not affect the meaning of the code)
- refactor: ♻️ (Code change that neither fixes a bug nor adds a feature)
- perf: ⚡️ (Code change that improves performance)
- test: ✅ (Adding or correcting tests)
- build: 📦 (Changes to build system or external dependencies)
- ci: ⚙️ (Changes to CI configuration)
- chore: 🧹 (Other changes not modifying src or test files)
- revert: ⏪ (Reverts a previous commit)

Guidelines for the **single subject line**:
1.  **Summarize the Core Change**: Identify the primary purpose/goal of the entire diff.
2.  **Imperative Mood**: Start with a verb (e.g., 'Add', 'Fix', 'Update', 'Refactor').
3.  **Conciseness**: Aim for 50-72 characters. Be brief but informative.
4.  **No Period**: Do not end the subject line with a period.
5.  **Scope (Optional)**: If applicable, a noun describing the affected area (e.g., 'api', 'ui', 'auth').
6.  **Emoji & Type**: Select the most fitting type and its emoji.
7.  **Focus**: Prioritize the overall *intent* and *impact*, not granular file-by-file details. Distill the essence of the changes.%s
Here is the git diff of the changes:
\\\ diff
%s
\\\ diff
Based ONLY on the diff provided, generate the commit message.
**Your response should be ONLY the commit message itself, with NO additional text, explanation, or markdown formatting surrounding it.**
**Strive for a single line. Every time.**`

func showSpinner(ctx context.Context, message string) {
	spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	fmt.Fprintf(os.Stderr, "%s %s", message, spinnerChars[0])

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", len(message)+1+1))
			return
		case <-ticker.C:
			i = (i + 1) % len(spinnerChars)
			fmt.Fprintf(os.Stderr, "\r%s %s", message, spinnerChars[i])
		}
	}
}

const geminiModel = "gemini-2.5-flash-preview-05-20"

func main() {
	geminiApiKey := os.Getenv("GEMINI_API_KEY")

	if geminiApiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is not set.")
	}

	additionalContext := ""
	if len(os.Args) > 1 {
		input := strings.Join(os.Args[1:], " ")
		additionalContext = "\n" +
			fmt.Sprintf(`User-provided hint/context for this commit: %s
Please take this hint into account when generating the commit message.
`, input)
	}

	insideGitRepoOutput, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output()
	if err != nil {
		log.Fatalf("Error checking git repository status: %v", err)
	}
	if strings.TrimSpace(string(insideGitRepoOutput)) != "true" {
		log.Fatal("Not inside a git repository.")
	}

	stagedDiff, err := exec.Command("git", "diff", "--staged", "--patch", "--unified=5").Output()
	if err != nil {
		log.Fatalf("Error getting staged diff: %v", err)
	}
	if len(stagedDiff) == 0 {
		log.Fatal("No staged files to commit.")
	}

	promptText := fmt.Sprintf(promptTemplate, additionalContext, string(stagedDiff))

	clientAPICtx := context.Background()
	client, err := genai.NewClient(clientAPICtx, &genai.ClientConfig{
		APIKey:  geminiApiKey,
		Backend: genai.BackendGeminiAPI,
	})

	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}

	spinnerCtx, cancelSpinner := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	spinnerMessage := fmt.Sprintf("🤖 %s", geminiModel)

	wg.Add(1)
	go func() {
		defer wg.Done()
		showSpinner(spinnerCtx, spinnerMessage)
	}()

	result, err := client.Models.GenerateContent(
		clientAPICtx,
		geminiModel,
		genai.Text(promptText),
		nil,
	)

	cancelSpinner()
	wg.Wait()

	if err != nil {
		log.Fatalf("Error generating commit message : %v", err)
	}

	fmt.Println(result.Text())
}
