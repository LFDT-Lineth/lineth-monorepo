#!/usr/bin/env bash
#
# weekly_zkc_metrics.sh — run the weekly arithmetization measurements locally.
#
# This is the hand-run counterpart to .github/workflows/arithmetization-weekly-zkc-metrics.yml:
# same four measurements, same weekly_metrics renderer, so a local run is a cheap way to test a
# change without waiting for Wednesday. The CI job is the source of truth for the numbers that
# go in the delivery report — a laptop's wall clocks are not comparable week to week.
#
#   1. zkc compile --stats  on main.zkc                (seconds)
#   2. guest ELF + zkc JSON from the reference block   (a minute or so)
#   3. zkc trace --stats                               (minutes, GBs)
#   4. zkc trace --stats --check                       (the expensive one)
#
# By default it writes to a LOCAL copy of the metrics file, so a test run never dirties the
# tracked one. Pass --out to target the real file.
#
# Examples, from the repository root:
#   arithmetization/src/test/scripts/weekly_zkc_metrics.sh --compile-only
#   arithmetization/src/test/scripts/weekly_zkc_metrics.sh --zkc-ref main --timebox 120
#   arithmetization/src/test/scripts/weekly_zkc_metrics.sh --zkc-src ~/dev/go/go-corset --skip-check
#
# It never touches ~/go/bin/zkc and never runs `make install-zkc` (which would overwrite it),
# and it invokes no Makefile target that rewrites a tracked file.

set -uo pipefail   # NOT -e: a step that fails or times out is data, not a reason to abort.
export LC_ALL=C    # /usr/bin/time prints "0,21 real" under a French locale otherwise.

# Must precede the argument loop: re-exec'ing after it would drop every flag.
if [ -z "${WZM_CAFFEINATED:-}" ] && command -v caffeinate >/dev/null 2>&1; then
  export WZM_CAFFEINATED=1
  exec caffeinate -i "$0" "$@"
fi

ZKC_REF="main"; ZKC_SRC=""; TIMEBOX_MIN=60
RUN_GUEST=true; RUN_TRACE=true; RUN_CHECK=true; KECCAK_ACCEL=true
OUT=""; REUSE_GUEST=false

usage() {
  cat <<'EOF'
usage: weekly_zkc_metrics.sh [options]

  --zkc-ref <ref>    zkc commit/branch/tag to measure with (default: main)
  --zkc-src <dir>    build zkc from a local checkout as-is instead (no fetch, no checkout)
  --timebox <min>    per-heavy-step wall-clock cap (default: 60)
  --compile-only     only `zkc compile --stats`
  --skip-check       skip the trace+check step
  --reuse-guest      reuse a previously generated guest JSON
  --no-keccak-accel  build the guest with stock zig keccak instead of the custom op
  --out <file>       markdown file to update
                     (default: .claude/reports/zkc-weekly-metrics.local.md)
  -h, --help         this text
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --zkc-ref) ZKC_REF="$2"; shift 2 ;;
    --zkc-src) ZKC_SRC="$2"; shift 2 ;;
    --timebox) TIMEBOX_MIN="$2"; shift 2 ;;
    --compile-only) RUN_GUEST=false; RUN_TRACE=false; RUN_CHECK=false; shift ;;
    --skip-check) RUN_CHECK=false; shift ;;
    --reuse-guest) REUSE_GUEST=true; shift ;;
    --no-keccak-accel) KECCAK_ACCEL=false; shift ;;
    --out) OUT="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/../../../.." && pwd)
cd "$REPO_ROOT" || exit 2

ZKC_MAIN="arithmetization/src/main/riscv/main.zkc"
GUEST_DIR="riscv-guests/l2-execution"
GUEST_ELF="$GUEST_DIR/zig-out/bin/evm_execution_guest"
GUEST_SSZ="$GUEST_DIR/test/testdata/stateless_input.ssz"
# Must stay a ./-relative package path: these helpers build in GOPATH mode
# (GO111MODULE=off, no go.mod in the tree), where `go run <absolute path>` is rejected.
HELPER="./${SCRIPT_DIR#"$REPO_ROOT"/}/weekly_metrics"

WORK="$REPO_ROOT/.claude/reports"
RUN="$WORK/runs/$(date -u +%Y%m%dT%H%M%SZ)"
GUEST_JSON="$WORK/guest.json"
[ -n "$OUT" ] || OUT="$WORK/zkc-weekly-metrics.local.md"
mkdir -p "$RUN" "$WORK/.toolchain" || exit 2

TIMEBOX_S=$(( TIMEBOX_MIN * 60 ))
log() { printf '\033[1m==>\033[0m %s\n' "$*" >&2; }

# ── the zkc under measurement, built privately ──────────────────────────────────

if [ -n "$ZKC_SRC" ]; then
  sha=$(git -C "$ZKC_SRC" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
  ZKC="$WORK/.toolchain/zkc-src-$sha"
  if [ ! -x "$ZKC" ]; then
    log "building zkc from $ZKC_SRC @ $sha"
    ( cd "$ZKC_SRC" && GOWORK=off go build -o "$ZKC" ./cmd/zkc ) || exit 1
  fi
else
  tmp=$(mktemp -d) || exit 1
  log "installing zkc @ $ZKC_REF (private GOBIN; ~/go/bin untouched)"
  GOBIN="$tmp" GOWORK=off GOFLAGS= go install "github.com/LFDT-Lineth/zkc/cmd/zkc@$ZKC_REF" || exit 1
  sha=$(go version -m "$tmp/zkc" | awk '$1=="mod"{print $3}' | sed -E 's/.*-([0-9a-f]{12})$/\1/')
  ZKC="$WORK/.toolchain/zkc-${ZKC_REF//\//_}-${sha:-unknown}"
  mv -f "$tmp/zkc" "$ZKC"; rmdir "$tmp" 2>/dev/null
fi
log "zkc: $ZKC"

# ── time-boxed step runner ──────────────────────────────────────────────────────
#
# macOS has no timeout(1) and no GNU time, so the watchdog is hand-rolled. It kills the
# CHILD rather than /usr/bin/time, which is what lets `time -l` still write its rusage
# block — so a timed-out step still reports a real peak RSS.
# Always returns 0; the outcome lives in files under $d.
run_step() {           # run_step <name> <timebox_s|0> -- <cmd...>
  local name="$1" box="$2"; shift 2; [ "$1" = "--" ] && shift
  local d="$RUN/$name"; mkdir -p "$d"
  log "$name: $*"

  local t0 t1 rc=0 child="" i=0 wd=""
  t0=$(date +%s)
  /usr/bin/time -l -o "$d/rusage" \
    /bin/sh -c 'echo $$ > "$1"; shift; exec "$@"' sh "$d/pid" "$@" \
    > "$d/stdout" 2> "$d/stderr" &
  local tpid=$!
  while [ $i -lt 200 ]; do
    [ -s "$d/pid" ] && { child=$(cat "$d/pid"); break; }
    sleep 0.05; i=$((i + 1))
  done
  if [ -n "$child" ] && [ "$box" -gt 0 ] 2>/dev/null; then
    ( sleep "$box"
      kill -0 "$child" 2>/dev/null || exit 0
      : > "$d/timedout"; kill -TERM "$child" 2>/dev/null
      sleep 30; kill -KILL "$child" 2>/dev/null ) &
    wd=$!
  fi
  wait "$tpid"; rc=$?
  [ -n "$wd" ] && { kill "$wd" 2>/dev/null; wait "$wd" 2>/dev/null; }
  t1=$(date +%s)

  echo "$rc"          > "$d/rc"
  echo "$((t1 - t0))" > "$d/wall_s"
  local what="rc=$rc"; [ -f "$d/timedout" ] && what="TIMEOUT after ${box}s"
  log "$name: $what in $((t1 - t0))s"
  return 0
}

# ── the measurements ────────────────────────────────────────────────────────────

# --order name (not zkc's default --order total) keeps the table diffable week to week;
# the Total rows the renderer parses are order-independent.
run_step compile 0 -- "$ZKC" compile --stats --order name "$ZKC_MAIN"

if $RUN_GUEST; then
  if $REUSE_GUEST && [ -s "$GUEST_JSON" ]; then
    log "reusing $GUEST_JSON"
    mkdir -p "$RUN/guest"; echo 0 > "$RUN/guest/rc"; echo 0 > "$RUN/guest/wall_s"
  else
    # NB: no `make linker-script` — linker_script.ld is tracked and @embedFile'd by the
    # zig build, so regenerating it would only risk dirtying the tree.
    run_step guest 0 -- make -C "$GUEST_DIR" compile KECCAK_ACCEL="$KECCAK_ACCEL"
    if [ "$(cat "$RUN/guest/rc")" = "0" ]; then
      make -C riscv-guests/build_common elf-to-json \
        BIN_EXT="$REPO_ROOT/$GUEST_ELF" JSON_EXT="$GUEST_JSON" \
        IN_BYTES="@$REPO_ROOT/$GUEST_SSZ" >> "$RUN/guest/stdout" 2>&1
    fi
  fi
fi

if [ -s "$GUEST_JSON" ]; then
  $RUN_TRACE && run_step trace "$TIMEBOX_S" -- "$ZKC" trace --stats "$GUEST_JSON" "$ZKC_MAIN"
  $RUN_CHECK && run_step check "$TIMEBOX_S" -- "$ZKC" trace --stats --check "$GUEST_JSON" "$ZKC_MAIN"
fi

# ── provenance, then render ─────────────────────────────────────────────────────

zkc_mod=$(go version -m "$ZKC" | awk '$1=="mod"{print $3}')
zkc_commit=$(go version -m "$ZKC" | awk '$1=="build" && $2 ~ /^vcs\.revision=/{print substr($2,14)}')
[ -n "$zkc_commit" ] || zkc_commit=$(printf '%s' "$zkc_mod" | sed -E 's/.*-([0-9a-f]{12})$/\1/')
mono_head=$(git rev-parse --short=9 HEAD)
mono_branch=$(git rev-parse --abbrev-ref HEAD)
# Scoped: the untracked zkc/ clone and the go-corset submodule keep a naive
# `git status` permanently dirty, which would make the flag meaningless.
git diff --quiet --ignore-submodules=all HEAD -- && dirty="" || dirty=" (dirty)"

{
  echo "week=$(date -u +%G-W%V)"
  echo "started=$(date -u '+%Y-%m-%d %H:%M')"
  echo "trigger=local ($(whoami)@$(hostname -s))"
  echo "runner=laptop $(uname -m) $(sysctl -n hw.ncpu 2>/dev/null || nproc) cores"
  echo "arithmetization=\`$mono_head\` ($mono_branch)$dirty"
  echo "zkc=${ZKC_SRC:+local }${ZKC_REF}@\`${zkc_commit:0:9}\`"
  echo "run_url=–"
} > "$RUN/meta.env"

log "updating $OUT"
GO111MODULE=off go run "$HELPER" -run-dir "$RUN" -out "$OUT" || exit 1
log "raw artifacts: $RUN"
