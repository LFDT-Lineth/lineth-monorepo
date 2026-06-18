#!/usr/bin/env python3

import argparse
import re
import subprocess
import sys
from pathlib import Path


TIME_RE = re.compile(r"^(real|user|sys)\s+([0-9]+(?:[\.,][0-9]+)?)$")


def parse_args() -> argparse.Namespace:
    script = Path(__file__).resolve()
    default_makefile_dir = script.parents[1]

    parser = argparse.ArgumentParser(
        description="Compare keccak-zig-exec timings with KECCAK_ACCEL=false and true."
    )
    parser.add_argument("--intervals", type=int, default=100, help="number of intervals to run")
    parser.add_argument("--size", type=int, default=10, help="vectors per interval")
    parser.add_argument("--start", type=int, default=0, help="first vector index")
    parser.add_argument("--makefile-dir", type=Path, default=default_makefile_dir)
    parser.add_argument("--time-bin", default="/usr/bin/time")
    return parser.parse_args()


def vector_range(start: int, size: int, index: int) -> str:
    first = start + index * size
    last = first + size - 1
    return f"{first}..{last}"


def command(time_bin: str, accel: bool, selector: str) -> list[str]:
    return [
        time_bin,
        "-p",
        "make",
        "keccak-zig-exec",
        f"KECCAK_ACCEL={'true' if accel else 'false'}",
        f"KECCAK_N_VECTORS={selector}",
    ]


def run_timed(makefile_dir: Path, time_bin: str, accel: bool, selector: str) -> float:
    cmd = command(time_bin, accel, selector)
    proc = subprocess.run(cmd, cwd=makefile_dir, text=True, capture_output=True)
    if proc.returncode != 0:
        print(f"command failed: {' '.join(cmd)}", file=sys.stderr)
        print(proc.stdout, file=sys.stderr, end="")
        print(proc.stderr, file=sys.stderr, end="")
        raise SystemExit(proc.returncode)

    timings: dict[str, float] = {}
    for line in proc.stderr.splitlines():
        match = TIME_RE.match(line.strip())
        if match:
            timings[match.group(1)] = float(match.group(2).replace(",", "."))

    if "real" not in timings:
        print(f"could not parse /usr/bin/time output for: {' '.join(cmd)}", file=sys.stderr)
        print(proc.stderr, file=sys.stderr, end="")
        raise SystemExit(1)

    return timings["real"]
    
def print_row(selector: str, false_time: float, true_time: float) -> None:
    speedup = false_time / true_time if true_time else float("inf")
    print(f"| {selector:^20} | {false_time:^20.2f} | {true_time:^20.2f} | {speedup:^19.2f}x |", flush=True)

def main() -> None:
    args = parse_args()
    if args.intervals < 1:
        raise SystemExit("--intervals must be >= 1")
    if args.size < 1:
        raise SystemExit("--size must be >= 1")

    makefile_dir = args.makefile_dir.resolve()
    print(f"| {'vectors':^20} | {'false real (s)':^20} | {'true real (s)':^20} | {'speedup':^20} |")
    print(f"| {'-'*20} | {'-'*20} | {'-'*20} | {'-'*20} |")

    for index in range(args.intervals):
        selector = vector_range(args.start, args.size, index)
        false_time = run_timed(makefile_dir, args.time_bin, False, selector)
        true_time = run_timed(makefile_dir, args.time_bin, True, selector)
        print_row(selector, false_time, true_time)

if __name__ == "__main__":
    main()
