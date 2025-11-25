#!/bin/bash
# Git hook: prepare-commit-msg
# $1 = path to commit message file

TOOL=".git/hooks/commit-writer"
TONE="increasingly insane Victorian author"

$TOOL --hook "$1" --tone "$TONE" --force