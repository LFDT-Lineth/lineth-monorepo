#!/bin/bash
# Run every ACT4 ELF through our zkc and tally PASS/FAIL.
#
# Inputs:
#   ELF_DIR             directory tree containing `*.elf` (default: derived
#                       from ACT4_WORK_DIR if set, else exit)
#   ACT4_WORK_DIR       alternate way to point at the build output —
#                       ELF_DIR is then `$ACT4_WORK_DIR/linea-rv64im-zicclsm/elfs`
#   ELF2JSON            path to the elf-to-json binary (auto-built if missing)
#   ZKC_MAIN            path to main.zkc (default: ../../../main/riscv/main.zkc
#                       relative to this script)
#
# Output:
#   $RESULTS            per-test PASS/FAIL summary  (default: ../bin/results.txt)
#   $LOGS/<name>.out    per-test zkc terminal lines (default: ../bin/logs/)

# Ensures the script crashes immediately if it attempts to read any unassigned variable
set -u

SCRIPT_DIR=$(cd -- "$(dirname -- "$0")" && pwd)
ACT4_DIR=$(cd -- "$SCRIPT_DIR/.." && pwd)
EXAMPLES_DIR=$(cd -- "$ACT4_DIR/.." && pwd)
ZKC_MAIN_DEFAULT="$EXAMPLES_DIR/../../main/riscv/main.zkc"

ZKC_MAIN="${ZKC_MAIN:-$ZKC_MAIN_DEFAULT}"
ELF2JSON="${ELF2JSON:-$ACT4_DIR/bin/elf2json}"

# Memory layout — same defaults as ../../Makefile. Override via env, e.g.
# `IN_BYTES_OFFSET=0x10000000 ./run_linea_elfs.sh`. ACT4 tests are self-
# contained, so IN_BYTES is empty by default; ENTRY_POINT defaults to 0
# because ACT4's `link.ld` places `rvtest_entry_point` at the origin.
IN_BYTES="${IN_BYTES:-}"
PROGRAM_OFFSET="${PROGRAM_OFFSET:-0x00000000}"
IN_BYTES_OFFSET="${IN_BYTES_OFFSET:-0x08800000}"
ENTRY_POINT="${ENTRY_POINT:-$PROGRAM_OFFSET}"

# Resolve ELF_DIR. Default uses the work directory that `build_linea_elfs.sh`
# writes by default (under this script's tree).
DEFAULT_WORK_DIR="$ACT4_DIR/bin/work"
ACT4_WORK_DIR="${ACT4_WORK_DIR:-$DEFAULT_WORK_DIR}"
ELF_DIR="${ELF_DIR:-$ACT4_WORK_DIR/linea-rv64im-zicclsm/elfs}"

if [ ! -d "$ELF_DIR" ]; then
    echo "error: '$ELF_DIR' does not exist." >&2
    echo "Build the ELFs first via build_linea_elfs.sh." >&2
    exit 2
fi

BIN_DIR="$ACT4_DIR/bin"
LOGS="${LOGS:-$BIN_DIR/logs}"
RESULTS="${RESULTS:-$BIN_DIR/results.txt}"
mkdir -p "$BIN_DIR" "$LOGS"
> "$RESULTS"

# Clean logs from previous run
rm -f "$LOGS"/*

# Auto-build elf2json from ../../main.go if not provided.
if [ ! -x "$ELF2JSON" ]; then
    if ! command -v go >/dev/null 2>&1; then
        echo "error: ELF2JSON ($ELF2JSON) missing and 'go' not on PATH." >&2
        exit 2
    fi
    ( cd "$EXAMPLES_DIR" && go build -o "$ELF2JSON" main.go )
fi

if ! command -v zkc >/dev/null 2>&1; then
    echo "error: 'zkc' not on PATH." >&2
    exit 2
fi

count=0
pass=0
fail=0
for elf in $(find "$ELF_DIR" -name '*.elf' | sort); do
    name=$(basename "$elf" .elf)
    count=$((count+1))
    json="$LOGS/${name}.json"
    out="$LOGS/${name}.out"
    full="$LOGS/${name}.full"

    "$ELF2JSON" "$elf" "$IN_BYTES" "$PROGRAM_OFFSET" "$IN_BYTES_OFFSET" "$ENTRY_POINT" \
        > "$json" 2> "$LOGS/${name}.json.err"

    zkc exec "$json" "$ZKC_MAIN" 2>&1 | tee "$full" \
        | grep -E "Program exited successfully|machine panic|exit with code|fail|ERROR" \
        | head -3 > "$out"

    if grep -q "Program exited successfully" "$out"; then
        echo "PASS  $name" >> "$RESULTS"
        pass=$((pass+1))
        rm -f "$full"
    else
        # Append the framework's ACT4 diagnostic block (each
        # `ECALL for write` emits one syscall-64 string verbatim via
        # the %c format). Strip the interpreter's `-----------`
        # divider that gets concatenated onto the last byte of each
        # WRITE_STR call (since the message ends without a newline).
        # TODO: make this more robust by changing the framework to emit a more unique marker before/after each write.
        rvcp=$(awk '
            /^ECALL for write$/ { capture = 1; next }
            capture && /-----------/ {
                sub(/-+.*$/, "", $0)
                if (length($0)) print
                capture = 0
                next
            }
            capture { print }
      ' "$full")
        if [ -n "$rvcp" ]; then
            {
                echo ""
                echo "--- WRITE_STR ---"
                printf '%s\n' "$rvcp"
            } >> "$out"
        fi
        code=$(grep -oE "exit with code -?[0-9]+" "$out" | head -1 | grep -oE "\-?[0-9]+")
        [ -z "$code" ] && code="?"
        first=$(head -1 "$out" | tr -d '\n' | cut -c1-120)
        echo "FAIL  $name  code=$code  '${first}'" >> "$RESULTS"
        fail=$((fail+1))
    fi
done

echo "" >> "$RESULTS"
echo "DONE: $count tests   PASS: $pass   FAIL: $fail" >> "$RESULTS"

echo
tail -1 "$RESULTS"
echo "results: $RESULTS"
echo "logs:    $LOGS/"
