#!/usr/bin/env bash
# Shared helpers for the git-cliff step scripts.
#
# Contract for every script that sources this file:
#   - the script's *payload* goes to stdout (a value, or the changelog text),
#   - human-readable logs go to stderr (via log),
#   - when running inside GitHub Actions ($GITHUB_OUTPUT set) the step outputs
#     are also appended to $GITHUB_OUTPUT.
#
# This lets the same scripts back both the composite action (which reads
# $GITHUB_OUTPUT) and local `make` dry-runs (which read stdout).

# log MESSAGE... — write a human-readable line to stderr.
log() { printf '%s\n' "$*" >&2; }

# emit_kv KEY VALUE — single-line step output.
# Prints "KEY=VALUE" to stdout and, in CI, appends it to $GITHUB_OUTPUT.
emit_kv() {
  local key="$1" value="$2"
  printf '%s=%s\n' "$key" "$value"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    printf '%s=%s\n' "$key" "$value" >> "$GITHUB_OUTPUT"
  fi
}

# emit_block KEY — multi-line step output; payload is read from stdin.
# Prints the raw payload to stdout and, in CI, appends a heredoc-delimited
# block to $GITHUB_OUTPUT.
emit_block() {
  local key="$1"
  local delim="__${key}_EOF__"
  local payload
  payload="$(cat)"
  printf '%s\n' "$payload"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    {
      printf '%s<<%s\n' "$key" "$delim"
      printf '%s\n' "$payload"
      printf '%s\n' "$delim"
    } >> "$GITHUB_OUTPUT"
  fi
}
