package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultOllamaURL = "http://localhost:11434/api/generate"

type OllamaReq struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt,omitempty"`
	Stream  bool                   `json:"stream,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type OllamaResp struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

func callOllama(url string, req OllamaReq) (string, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	client := &http.Client{Timeout: 60 * time.Second}

	r, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	r.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(r)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("warning: failed to close response body: %v", cerr)
		}
	}()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result string
	decoder := json.NewDecoder(resp.Body)
	for {
		var o OllamaResp
		if err := decoder.Decode(&o); err == io.EOF {
			break
		} else if err != nil {
			return "", fmt.Errorf("failed to decode response: %w", err)
		}
		result += o.Response
	}

	// Clean the response: unquote JSON string if necessary and strip code fences.
	result = cleanModelOutput(result)

	return strings.TrimSpace(result), nil
}

// cleanModelOutput normalizes model text by removing code fences, unquoting
// JSON-encoded strings and normalizing newlines.
func cleanModelOutput(s string) string {
	s = strings.TrimSpace(s)
	// If the entire body is a JSON string like: "...\n...", try to unquote it.
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		if unq, err := strconv.Unquote(s); err == nil {
			s = unq
		}
	}

	// Remove triple-backtick fenced blocks, keeping the inner content if present.
	// Replace any ```lang\n...``` occurrences with the inner text.
	fenceRe := regexp.MustCompile("(?s)```[a-zA-Z0-9_-]*\\n(.*?)```")
	if fenceRe.MatchString(s) {
		s = fenceRe.ReplaceAllString(s, "$1")
	}
	// Also remove any remaining ``` markers
	s = strings.ReplaceAll(s, "```", "")

	// Normalize CRLF
	s = strings.ReplaceAll(s, "\r\n", "\n")

	return strings.TrimSpace(s)
}

func checkOllama(ollamaURL string) error {
	u, err := neturl.Parse(ollamaURL)
	if err != nil {
		return fmt.Errorf("invalid ollama URL: %w", err)
	}
	u.Path = "/api/tags"

	client := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest("GET", u.String(), nil)
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("ollama does not appear to be running; start it with 'ollama serve'")
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("warning: failed to close tags response body: %v", cerr)
		}
	}()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama tags endpoint returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func getStagedDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--staged")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git --staged failed: %w; output=%s", err, string(out))
	}
	if strings.TrimSpace(string(out)) == "" {
		cmd2 := exec.Command("git", "diff")
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return "", fmt.Errorf("git diff failed: %w; output=%s", err2, string(out2))
		}
		return string(out2), nil
	}
	return string(out), nil
}

func main() {
	var (
		ollamaURL       string
		summarizerModel string
		styleModel      string
		tone            string
		hookFile        string
		forceWrite      bool
		debug           bool
	)

	flag.StringVar(&ollamaURL, "ollama", os.Getenv("OLLAMA_URL"), "Ollama URL")
	flag.StringVar(&summarizerModel, "summ-model", "gemma3:4B", "Summarizer model")
	flag.StringVar(&styleModel, "style-model", "mistral:7b", "Styling model")
	flag.StringVar(&tone, "tone", "chaotic, wild, funny", "Tone for stylistic rewrite")
	flag.StringVar(&hookFile, "hook", "", "Path for git hook commit message file")
	flag.BoolVar(&forceWrite, "force", false, "Overwrite existing commit message in hook file")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")
	flag.Parse()

	if ollamaURL == "" {
		ollamaURL = defaultOllamaURL
	}

	if debug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Printf("debug: ollamaURL=%s summarizerModel=%s styleModel=%s tone=%s hookFile=%s force=%v",
			ollamaURL, summarizerModel, styleModel, tone, hookFile, forceWrite)
	}

	// helper to print progress status to stderr (keeps stdout reserved for the final message)
	statusf := func(format string, args ...interface{}) {
		fmt.Fprintf(os.Stderr, "[status] "+format+"\n", args...)
	}

	statusf("Checking Ollama availability at %s", ollamaURL)
	if err := checkOllama(ollamaURL); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if debug {
			log.Printf("checkOllama error: %v", err)
		}
		os.Exit(1)
	}
	statusf("Ollama reachable")

	statusf("Gathering git diff (staged or unstaged)")
	diff, err := getStagedDiff()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading git diff: %v\n", err)
		if debug {
			log.Printf("getStagedDiff error: %v", err)
		}
		os.Exit(2)
	}
	statusf("Diff collected (%d bytes)", len(diff))

	summaryPrompt := fmt.Sprintf(`Summarize the following git diff with strict factual accuracy.
Produce TWO sections:
1. A short commit title (max 60 chars)
2. A 2-4 line commit body describing the key changes.

Rules:
- Title should be imperative tense.
- Body should describe files, functions, and intent.
- Do NOT invent or hallucinate.
- Keep it concise.

Diff:
%s
`, diff)

	summaryPrompt = summaryPrompt + "\n\nOUTPUT FORMAT:\nTITLE (one line)\nBLANK LINE\nBODY (2-4 lines)\n"

	statusf("Calling summarizer model '%s'", summarizerModel)
	// Try the summarizer and validate the output; retry once with a stricter
	// prompt if the result doesn't match the expected "title + body" format.
	var sum string
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		sum, lastErr = callOllama(ollamaURL, OllamaReq{
			Model:  summarizerModel,
			Prompt: summaryPrompt,
			Stream: false,
			Options: map[string]interface{}{
				"temperature": 0.0,
			},
		})
		if lastErr != nil {
			if debug {
				log.Printf("summarizer call error (attempt %d): %v", attempt, lastErr)
			}
			continue
		}

		statusf("Summary received (attempt %d)", attempt)
	}
	if lastErr != nil {
		fmt.Fprintf(os.Stderr, "Summarizer error: %v\n", lastErr)
		os.Exit(3)
	}

	stylePrompt := fmt.Sprintf(`Rewrite the following commit (title + body) but:
- KEEP the factual content *exactly*.
- Apply this tone: %s
- Make it wild/funny/chaotic while readable.
- Maintain title + body structure.
- 1 title line, 2-4 body lines.

Original commit:
%s
`, tone, sum)

	statusf("Calling style model '%s' with tone: %s", styleModel, tone)
	finalMsg, err := callOllama(ollamaURL, OllamaReq{
		Model:  styleModel,
		Prompt: stylePrompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.9,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Styling model error: %v\n", err)
		if debug {
			log.Printf("styling call error: %v", err)
		}
		os.Exit(4)
	}
	statusf("Final message generated")

	finalMsg = strings.TrimSpace(finalMsg)
	fmt.Println(finalMsg)

	if hookFile != "" {
		if forceWrite {
			statusf("Writing suggested message to %s (overwrite)", hookFile)
		} else {
			if _, err := os.Stat(hookFile); err == nil {
				statusf("Appending suggested message to %s", hookFile)
			} else {
				statusf("Writing suggested message to %s", hookFile)
			}
		}
		if _, err := os.Stat(hookFile); err == nil && !forceWrite {
			f, err := os.OpenFile(hookFile, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open hook file for append: %v\n", err)
				if debug {
					log.Printf("openfile error: %v", err)
				}
				os.Exit(5)
			}
			defer func() {
				if cerr := f.Close(); cerr != nil {
					log.Printf("warning: failed to close hook file: %v", cerr)
				}
			}()
			if _, err := f.WriteString("\n# Suggested commit message (auto-generated):\n" + finalMsg + "\n"); err != nil {
				fmt.Fprintf(os.Stderr, "failed to write to hook file: %v\n", err)
				if debug {
					log.Printf("write error: %v", err)
				}
				os.Exit(6)
			}
		} else {
			if err := os.WriteFile(hookFile, []byte(finalMsg+"\n"), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "failed to write hook file: %v\n", err)
				if debug {
					log.Printf("writefile error: %v", err)
				}
				os.Exit(7)
			}
		}
		statusf("Hook file updated: %s", hookFile)
	}
	statusf("Done")
}
