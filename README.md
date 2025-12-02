# Commit Writer

A standalone golang program that

* Reads a staged git diff
* Sends it to a "factual summarizer model" (default: `gemma3:4B`)
* Sends a summary + tone instruction to a "styling model" (default: `mistral:7b`) to rewrite the summary in the given tone
* Prints the final commit message to stdout and optionally writes it to a commit message file (for git hook usage)

## Build

```bash
go build -o commit-writer main.go
```

## Usage Examples

### Basic Usage

```bash
# Stage your changes
git add -A

# Generate commit message with default tone
./commit-writer

# Use a specific tone
./commit-writer --tone "increasingly insane Victorian author"

# Clean output without labels (easy to copy/paste)
./commit-writer --tone "chaotic, wild, funny" --no-labels

# Use different models
./commit-writer --summ-model "llama3:8b" --style-model "mistral:7b" --tone "pirate speak"

# Timeout flag for larger models
./commit-writer --timeout 600 --tone "professional"

# Title only mode
./commit-writer --title-only

# Title only with custom tone
./commit-writer --title-only --tone "professional, concise"

# Custom Ollama URL
./commit-writer --ollama "http://192.168.1.100:11434/api/generate" --tone "professional"
```

### Direct Commit (One-liner)

```bash
# Stage and commit in one command
git add -A && git commit -m "$(./commit-writer --no-labels --tone 'professional')"

# Or with error handling
git add -A && git commit -m "$(./commit-writer --no-labels)" || echo "Commit failed"

# Using default tone
git add . && git commit -m "$(./commit-writer --no-labels)"

# Quick commit with custom tone
git commit -am "$(./commit-writer --no-labels --tone 'concise and technical')"
```

### Advanced: Save/Reuse Summary for Faster Tone Iteration

You can save the factual summary from the first LLM and reuse it to quickly try different tones:

```bash
# Step 1: Generate and save the factual summary
git add -A
./commit-writer --save-summary summary.txt --tone "professional"

# Step 2: Iterate on different tones using the saved summary (much faster!)
./commit-writer --load-summary summary.txt --tone "increasingly insane Victorian author"
./commit-writer --load-summary summary.txt --tone "pirate speak"
./commit-writer --load-summary summary.txt --tone "overly dramatic Shakespeare"

# Step 3: You can also write your own summary and apply tone to it
echo "Add user authentication\n\nImplemented JWT-based auth" > my-summary.txt
./commit-writer --load-summary my-summary.txt --tone "chaotic, wild, funny"
```

This workflow is useful for:
1. **Reviewing the factual summary** before applying tone
2. **Faster iteration** on different tones without re-analyzing the diff
3. **Manual control** - write your own summary and let the style model add flair


### Git Hook Setup

Install as a `prepare-commit-msg` hook to automatically generate commit messages:

```bash
# Copy the binary to your hooks directory
cp ./commit-writer .git/hooks/commit-writer

# Create the prepare-commit-msg hook
cat > .git/hooks/prepare-commit-msg << 'EOF'
#!/bin/bash
# Auto-generate commit messages with commit-writer
# $1 is the path to the commit message file

TONE="${COMMIT_TONE:-chaotic, wild, funny}"
.git/hooks/commit-writer --hook "$1" --tone "$TONE" --force
EOF

# Make both executable
chmod +x .git/hooks/prepare-commit-msg
chmod +x .git/hooks/commit-writer
```

Now when you run `git commit`, the message will be auto-generated:

```bash
# Stage changes and commit (message auto-generated)
git add -A
git commit

# Override the tone with environment variable
COMMIT_TONE="professional and concise" git commit

# Or edit the generated message before committing
git commit  # Opens editor with generated message
```

#### Alternative Hook: Append suggestion as comment

If you prefer to review the suggestion before using it:

```bash
cat > .git/hooks/prepare-commit-msg << 'EOF'
#!/bin/bash
# Add commit-writer suggestion as a comment
.git/hooks/commit-writer --hook "$1" --tone "chaotic, wild, funny"
EOF

chmod +x .git/hooks/prepare-commit-msg
```

This appends the suggestion as a commented section in your commit message editor

### Creative Tone Examples

```bash
# Professional & concise
./commit-writer --tone "professional, concise, technical"

# Fun and creative
./commit-writer --tone "pirate captain documenting ship modifications"
./commit-writer --tone "overly dramatic Shakespeare"
./commit-writer --tone "excited scientist making a breakthrough"
./commit-writer --tone "noir detective solving a case"
./commit-writer --tone "sports commentator narrating a game"

# Specific styles
./commit-writer --tone "haiku poetry"
./commit-writer --tone "military briefing"
./commit-writer --tone "cooking recipe instructions"

# Combining with save/load for experimentation
./commit-writer --save-summary sum.txt --tone "professional"
./commit-writer --load-summary sum.txt --tone "1920s gangster"
./commit-writer --load-summary sum.txt --tone "confused time traveler"
```

### Debugging and Development

```bash
# Enable debug logging to see what's happening
./commit-writer --debug --tone "professional"

# Test with different model combinations
./commit-writer --summ-model "codellama:7b" --style-model "llama3:8b" --debug

# Save summary for manual review before styling
./commit-writer --save-summary review.txt --tone "professional"
cat review.txt  # Review the factual summary
# Edit review.txt manually if needed
./commit-writer --load-summary review.txt --tone "chaotic, wild, funny"
```

## Quick flags & notes

- `--ollama` : Ollama API URL (or set `OLLAMA_URL` env var). Default: `http://localhost:11434/api/generate`
- `--summ-model` : Summarizer model (default `gemma3:4B`)
- `--style-model` : Styling model (default `mistral:7b`)
- `--tone` : Tone description passed to the stylistic model
- `--hook` : Path to commit message file to write/append the suggestion
- `--force` : Overwrite the `--hook` file instead of appending
- `--debug` : Enable debug logging (prints additional info to stderr)
- `--no-labels` : Remove "Title:" and "Body:" labels from output for easier copy/paste
- `--save-summary` : Save the factual summary to a file (useful for review or reuse with different tones)
- `--load-summary` : Load a previously saved summary and skip the first LLM (fast tone iteration)
- `--timeout` : Sets the timeout in seconds for the HTTP call to the Ollama API. Default: 300 (5 minutes).
- `--title-only` : Outputs only a title instead of the normal title+body commit message. Default: false

## Practical Workflows

### Workflow 1: Quick one-liner commits
Perfect for small changes when you want speed:
```bash
git commit -am "$(./commit-writer --no-labels --tone 'concise')"
```

### Workflow 2: Review before committing
Generate, review, then commit manually:
```bash
./commit-writer --tone "professional"
# Review the output, then copy/paste or:
./commit-writer --no-labels | pbcopy  # macOS
./commit-writer --no-labels | xclip -selection clipboard  # Linux
git commit  # Paste the message
```

### Workflow 3: Iterate on tone
When you want to experiment with different tones:
```bash
# Generate and save factual summary once
./commit-writer --save-summary .commit-summary.txt --tone "professional"

# Try different tones quickly
./commit-writer --load-summary .commit-summary.txt --tone "pirate speak" --no-labels
./commit-writer --load-summary .commit-summary.txt --tone "haiku" --no-labels
./commit-writer --load-summary .commit-summary.txt --tone "Shakespearean" --no-labels

# Pick your favorite and commit
git commit -m "$(./commit-writer --load-summary .commit-summary.txt --tone 'pirate speak' --no-labels)"
```

### Workflow 4: Git hook automation
Set up the hook once, then commits are automatic:
```bash
# One-time setup (see Git Hook Setup section above)
# ...

# Daily usage - just commit normally
git add -A
git commit  # Message auto-generated!

# Override tone when needed
COMMIT_TONE="very serious and professional" git commit
```

### Workflow 5: Manual summary + AI styling
Write your own factual summary, then let AI add flair:
```bash
# Write your own summary
cat > my-commit.txt << 'EOF'
Add user authentication

Implemented JWT-based authentication with refresh tokens.
Added login and logout endpoints. Updated user model.
EOF

# Apply creative tone to your summary
./commit-writer --load-summary my-commit.txt --tone "excited product launch announcement" --no-labels
```

### Workflow 6: Interactive confirmation with `gum`
For interactive workflows where you want to review and approve before committing (requires [gum](https://github.com/charmbracelet/gum)):
```bash
# Generate message, display it, confirm, then commit
MSG="$(./commit-writer --tone "chaotic devops engineer" --no-labels --save-summary summary.txt)" && echo "$MSG" && gum confirm "Accept?" && git commit -m "$MSG"

# Reuse summary with different tone
MSG="$(./commit-writer --tone "Eeyore from Winnie the Pooh" --no-labels --load-summary summary.txt)" && echo "$MSG" && gum confirm "Accept?" && git commit -m "$MSG"
```

This workflow:
- Generates the commit message with your chosen tone
- Displays it for review (`echo "$MSG"`)
- Prompts for confirmation (`gum confirm`)
- Only commits if you approve
- Can quickly iterate on different tones by reusing saved summaries

## Development Notes

Build with modules enabled (there is a minimal `go.mod` included). Run
`go vet` and `golangci-lint run` during development to catch issues.

```
