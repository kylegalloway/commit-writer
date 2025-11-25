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

## Run

### Manual

```bash
git add -A
./commit-writer --tone "increasingly insane Victorian author"

# For easy copy/paste without labels
./commit-writer --tone "chaotic, wild, funny" --no-labels
```

### Git Hook

```bash
# Move to .git/hooks/commit-writer
cp ./commit-writer ./.git/hooks/commit-writer
```

Then create the message prep script (`.git/hooks/prepare-commit-msg`)

```bash
#!/bin/bash
# $1 is the path to the commit message file; pass it to the tool so it writes the message
# $2 is the tone description
.git/hooks/commit-writer --hook "$1" --tone "$2"
```

```bash
# Make them executable
chmod +x .git/hooks/prepare-commit-msg
chmod +x .git/hooks/commit-writer

## Quick flags & notes

- `--ollama` : Ollama API URL (or set `OLLAMA_URL` env var). Default: `http://localhost:11434/api/generate`
- `--summ-model` : Summarizer model (default `gemma3:4B`)
- `--style-model` : Styling model (default `mistral:7b`)
- `--tone` : Tone description passed to the stylistic model
- `--hook` : Path to commit message file to write/append the suggestion
- `--force` : Overwrite the `--hook` file instead of appending
- `--debug` : Enable debug logging (prints additional info to stderr)
- `--no-labels` : Remove "Title:" and "Body:" labels from output for easier copy/paste

Build with modules enabled (there is a minimal `go.mod` included). Run
`go vet` and `golangci-lint run` during development to catch issues.

```


