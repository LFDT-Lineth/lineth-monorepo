#!/bin/bash
# Compile and execute every op_*.s test under ../src/, then report PASS/FAIL.
#
# Layout (relative to this script):
#   ../src/op_*.s    — test sources (committed)
#   ../bin/          — build artefacts (gitignored)
#   ../bin/results.txt
#   ../bin/logs/     — per-test zkc trace
#
# Tools required: bash, python3, go, riscv64-unknown-elf-gcc, zkc.
# Run from any cwd; paths are resolved relative to the script's location.

set -u

SCRIPT_DIR=$(cd -- "$(dirname -- "$0")" && pwd)
ASM_DIR=$(cd -- "$SCRIPT_DIR/.." && pwd)
EXAMPLES_DIR=$(cd -- "$ASM_DIR/.." && pwd)
ZKC_MAIN=$(cd -- "$EXAMPLES_DIR/../../main/riscv" && pwd)/main.zkc

LD="$EXAMPLES_DIR/linker_script.ld"
BIN_DIR="$ASM_DIR/bin"
LOGS="$BIN_DIR/logs"
RESULTS="$BIN_DIR/results.txt"
ELF2JSON="$BIN_DIR/elf2json"

# Memory layout — same defaults as ../../Makefile. Override via env, e.g.
# `IN_BYTES_OFFSET=0x10000000 ./run_op_tests.sh`. Keep these in sync with
# the Makefile if you change them there.
IN_BYTES="${IN_BYTES:-}"
PROGRAM_OFFSET="${PROGRAM_OFFSET:-0x00000000}"
IN_BYTES_OFFSET="${IN_BYTES_OFFSET:-0x08800000}"
ENTRY_POINT="${ENTRY_POINT:-$PROGRAM_OFFSET}"

# Pick GNU make. macOS ships BSD make as `make`; Homebrew installs GNU as `gmake`.
if command -v gmake >/dev/null 2>&1; then
    MAKE_CMD=gmake
else
    MAKE_CMD=make
fi

# Sanity-check required tools up front.
missing=()
for t in go riscv64-unknown-elf-gcc zkc "$MAKE_CMD"; do
    command -v "$t" >/dev/null 2>&1 || missing+=("$t")
done
if [ ${#missing[@]} -gt 0 ]; then
    echo "error: missing required tool(s): ${missing[*]}" >&2
    echo "see asm/scripts/README.md for install hints." >&2
    exit 127
fi

mkdir -p "$BIN_DIR" "$LOGS"
> "$RESULTS"

# Build the ELF→JSON helper once (instead of `go run main.go` per test).
( cd "$EXAMPLES_DIR" && go build -o "$ELF2JSON" main.go )

# Generate the linker script. Pass the same memory-layout overrides through
# so the linker and the JSON agree.
"$MAKE_CMD" -C "$EXAMPLES_DIR" \
    PROGRAM_OFFSET="$PROGRAM_OFFSET" \
    IN_BYTES_OFFSET="$IN_BYTES_OFFSET" \
    ${SP:+SP="$SP"} \
    linker-script > /dev/null 2>&1

count=0
pass=0
fail=0
for f in "$ASM_DIR/src/op"/op_*.s; do
    name=$(basename "$f" .s)
    count=$((count+1))
    bin="$BIN_DIR/${name}"
    json="$BIN_DIR/${name}.json"

    if ! riscv64-unknown-elf-gcc -march=rv64im -mabi=lp64 -nostdlib -T"$LD" -o "$bin" "$f" \
            2> "$LOGS/${name}.gcc.err"; then
        echo "COMPILE_FAIL $name" >> "$RESULTS"
        fail=$((fail+1))
        continue
    fi

    "$ELF2JSON" "$bin" "$IN_BYTES" "$PROGRAM_OFFSET" "$IN_BYTES_OFFSET" "$ENTRY_POINT" \
        > "$json" 2> "$LOGS/${name}.json.err"

    zkc exec --ir "$json" "$ZKC_MAIN" > "$LOGS/${name}.out" 2>&1
    zkc_exit=$?

    if grep -q "Program exited successfully" "$LOGS/${name}.out"; then
        echo "PASS  $name" >> "$RESULTS"
        pass=$((pass+1))
    else
        code=$(grep -oE "exit with code -?[0-9]+" "$LOGS/${name}.out" | head -1 \
               | grep -oE "\-?[0-9]+")
        [ -z "$code" ] && code="?"
        echo "FAIL  $name  exit=$code  zkc_exit=$zkc_exit" >> "$RESULTS"
        fail=$((fail+1))
    fi
done

echo "" >> "$RESULTS"
echo "DONE: $count tests   PASS: $pass   FAIL: $fail" >> "$RESULTS"

# Echo summary to stdout for interactive runs.
echo
tail -1 "$RESULTS"
echo "results: $RESULTS"
echo "logs:    $LOGS/"
