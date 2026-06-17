#!/usr/bin/env sh
# Shared terminal output helpers for the Lineth quickstart scripts.
# POSIX sh: this file is sourced by both Alpine and Debian-based containers.

: "${LINETH_LOG_CONTEXT:=lineth}"

if [ -z "${NO_COLOR:-}" ] && { [ -t 1 ] || [ "${LINETH_COLOR:-auto}" = "always" ]; }; then
  LINETH_COLOR_ENABLED="1"
  LINETH_RESET="$(printf '\033[0m')"
  LINETH_BOLD="$(printf '\033[1m')"
  LINETH_DIM="$(printf '\033[2m')"
  LINETH_BLUE="$(printf '\033[38;2;97;223;255m')"
  LINETH_BLUE_1="$(printf '\033[38;2;97;223;255m')"
  LINETH_BLUE_2="$(printf '\033[38;2;90;203;233m')"
  LINETH_BLUE_3="$(printf '\033[38;2;84;186;213m')"
  LINETH_BLUE_4="$(printf '\033[38;2;68;146;170m')"
  LINETH_BLUE_5="$(printf '\033[38;2;57;115;137m')"
  LINETH_BLUE_6="$(printf '\033[38;2;57;115;137m')"
  LINETH_SHADOW="$(printf '\033[38;5;24m')"
  LINETH_BADGE="$(printf '\033[38;5;16m\033[48;5;122m')"
  LINETH_GREEN="$(printf '\033[38;5;82m')"
  LINETH_YELLOW="$(printf '\033[38;5;220m')"
  LINETH_RED="$(printf '\033[38;5;203m')"
else
  LINETH_COLOR_ENABLED=""
  LINETH_RESET=""
  LINETH_BOLD=""
  LINETH_DIM=""
  LINETH_BLUE=""
  LINETH_BLUE_1=""
  LINETH_BLUE_2=""
  LINETH_BLUE_3=""
  LINETH_BLUE_4=""
  LINETH_BLUE_5=""
  LINETH_BLUE_6=""
  LINETH_SHADOW=""
  LINETH_BADGE=""
  LINETH_GREEN=""
  LINETH_YELLOW=""
  LINETH_RED=""
fi

LINETH_SECTION_INDEX="${LINETH_SECTION_INDEX:-0}"

lineth_logo_line() {
  color="$1"
  text="$2"

  if [ -n "$LINETH_COLOR_ENABLED" ]; then
    # Draw a one-column shadow, then return to column 0 and draw the foreground line.
    printf ' %s%s\r%s%s%s%s \n' "$LINETH_SHADOW" "$text" "$color" "$LINETH_BOLD" "$text" "$LINETH_RESET"
  else
    printf '%s\n' "$text"
  fi
}

lineth_banner() {
  subtitle="${1:-quickstart (Sepolia L1)}"

  printf '\n'
  lineth_logo_line "$LINETH_BLUE_1" "██╗     ██╗███╗   ██╗███████╗████████╗██╗  ██╗"
  lineth_logo_line "$LINETH_BLUE_2" "██║     ██║████╗  ██║██╔════╝╚══██╔══╝██║  ██║"
  lineth_logo_line "$LINETH_BLUE_3" "██║     ██║██╔██╗ ██║█████╗     ██║   ███████║"
  lineth_logo_line "$LINETH_BLUE_4" "██║     ██║██║╚██╗██║██╔══╝     ██║   ██╔══██║"
  lineth_logo_line "$LINETH_BLUE_5" "███████╗██║██║ ╚████║███████╗   ██║   ██║  ██║"
  lineth_logo_line "$LINETH_BLUE_6" "╚══════╝╚═╝╚═╝  ╚═══╝╚══════╝   ╚═╝   ╚═╝  ╚═╝"
  printf '  %s lineth stack %s  %s%s%s\n' "$LINETH_BADGE" "$LINETH_RESET" "$LINETH_DIM" "$subtitle" "$LINETH_RESET"
}

lineth_section() {
  LINETH_SECTION_INDEX=$((LINETH_SECTION_INDEX + 1))
  printf '\n%s[%s] %s%s\n' "$LINETH_BLUE" "$LINETH_SECTION_INDEX" "$*" "$LINETH_RESET"
}

lineth_line() {
  color="$1"
  label="$2"
  shift 2
  printf '  %s%-5s%s %s\n' "$color" "$label" "$LINETH_RESET" "$*"
}

lineth_ok() { lineth_line "$LINETH_GREEN" "ok" "$*"; }
lineth_info() { lineth_line "$LINETH_DIM" "info" "$*"; }
lineth_warn() { lineth_line "$LINETH_YELLOW" "warn" "$*"; }
lineth_error() { lineth_line "$LINETH_RED" "error" "$*"; }

lineth_kv() {
  key="$1"
  shift
  printf '  %-32s %s\n' "$key" "$*"
}

lineth_indent() {
  sed 's/^/  /'
}

lineth_clean_prefixes() {
  sed -E 's/^\[[^]]+\][[:space:]]*(ERROR:[[:space:]]*)?//'
}

lineth_child_output() {
  lineth_clean_prefixes | lineth_indent
}

lineth_run_stream() {
  tmp="$(mktemp)"
  if "$@" > "$tmp" 2>&1; then
    status=0
  else
    status=$?
  fi
  lineth_child_output < "$tmp"
  rm -f "$tmp"
  return "$status"
}

lineth_die() {
  lineth_error "$*"
  exit 1
}
