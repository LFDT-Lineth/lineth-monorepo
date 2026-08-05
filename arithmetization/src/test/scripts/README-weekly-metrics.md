# Weekly zkc arithmetization metrics

Tracks the RISC-V arithmetization week over week for the delivery meeting. A weekly CI job
measures four things and appends one row to each markdown table in
[`arithmetization/docs/metrics/zkc-weekly-metrics.md`](../../../docs/metrics/zkc-weekly-metrics.md).

**That markdown file is the ledger** — there is no separate data store. Rows go in newest-first.

| piece | what it is |
|---|---|
| `.github/workflows/arithmetization-weekly-zkc-metrics.yml` | the workflow: Wednesday 03:00 UTC (05:00 Paris CEST), or on demand |
| `.github/actions/setup-zkc-measurement/` | shared preamble for the measurement jobs |
| `weekly_metrics/` | parses zkc's output and inserts one row per table |
| `weekly_zkc_metrics.sh` | run the same measurements locally, for testing or an ad-hoc harvest |

## What is measured

| # | command |
|---|---|
| 1 | `zkc compile --stats --order name arithmetization/src/main/riscv/main.zkc` |
| 2 | guest ELF + zkc JSON (`KECCAK_ACCEL=true`) |
| 3 | `zkc trace --stats <guest>.json main.zkc` |
| 4 | `zkc trace --stats --check <guest>.json main.zkc` |

Costs are not documented here — they are what the tables record.

The workload is the l2-execution guest against
`riscv-guests/l2-execution/test/testdata/stateless_input.ssz` — the small committed reference
block, and the guest Makefile's default input.

Steps 3 and 4 are best-effort with their own timeouts. A timeout is recorded as that week's data
point rather than failing the job: knowing the ceiling is the measurement.

## What a failure looks like

The report job runs unless the workflow was cancelled, so a failed or timed-out measurement still
produces the week's row — that outcome *is* the data point. Cells read:

| situation | cell |
|---|---|
| step succeeded | `ok <wall> / <peak RSS>` |
| hit its cap | `**TIMEOUT** <wall> / <peak RSS>` |
| killed, almost always the OOM killer | `**OOM/killed** (SIGKILL) …` |
| segfaulted | `**CRASH** (SIGSEGV) …` |
| constraints actually failed | `**FAIL** (N failing)` — grepped, since zkc still exits 0 |
| prerequisite missing, e.g. the guest build failed | `skipped` |
| other non-zero exit | `**FAIL** (rc N)` |

The peak RSS survives a kill, which is the point: it tells you what the step died wanting.

Each measurement runs in its own job and uploads its result as soon as it finishes, so losing one
runner cannot destroy work that already completed: the report assembles whatever artifacts exist and
the rest reads `skipped`. The Actions UI shows which job failed and why.

Per-measurement caps come from the dispatch inputs and are clamped in-step to stay below the job's
`timeout-minutes`, so the cap always fires where it can be recorded rather than the job being killed
with nothing to show.

## Triggering it

Weekly by cron, or by hand from the Actions tab. The manual form takes:

- **zkc-ref** — a branch, a release tag, or a commit hash (short or full); default `main`. This is
  how you answer "did a zkc change cost us?" without touching the arithmetization. Short hashes are
  expanded to full SHAs first, because the install path is `git fetch --depth=1 origin <ref>` and
  git cannot fetch an abbreviated SHA.
- **monorepo-ref** — a commit, branch or tag of *this* repo to measure; default is the ref the run
  was started from. The tooling always comes from the ref you dispatched, and the measured tree is
  checked out separately under `measured/` — so you can measure a commit that predates this
  tooling entirely, and the row still lands on the branch you dispatched from.
- **run-heavy** — off to get just the constraint stats in ~2 minutes.
- **trace-timeout-min** / **check-timeout-min** — raise the caps when chasing a completion; they are
  clamped to their job's cap (40m and 110m). The check defaults to 90m: these runners get disrupted,
  and "did not finish in 90m" is a more reliable weekly datum than an occasional heroic completion.
- **commit-results** — off to measure without writing to the file.

Both refs are recorded in the row, requested form and resolved SHA, so a measurement can always be
traced back to exactly what was built.

## Running it locally

```bash
arithmetization/src/test/scripts/weekly_zkc_metrics.sh --compile-only        # seconds
arithmetization/src/test/scripts/weekly_zkc_metrics.sh --zkc-ref v1.2.26     # a specific zkc
arithmetization/src/test/scripts/weekly_zkc_metrics.sh --zkc-src ~/dev/go/go-corset
```

`--help` lists the flags. By default it writes to `.claude/reports/zkc-weekly-metrics.local.md`
(gitignored) so a test run never dirties the tracked file; pass `--out` to target the real one.
Raw stdout/stderr/rusage per step land in `.claude/reports/runs/<timestamp>/`.

Local numbers are for smoke-testing, not for the report: a laptop's wall clocks are not
comparable week to week (thermals, other load). The CI rows are the ones to quote.

The script never touches `~/go/bin/zkc` and never runs `make install-zkc`, which would overwrite
it — the zkc under measurement is built into a private cache and passed explicitly.

## Shared plumbing

The zkc invocation is owned by `arithmetization/src/test/Makefile`, and this job goes through it
rather than shelling out to `zkc` itself:

| target | what it does |
|---|---|
| `make -C arithmetization install-zkc-to ZKC_BIN=<path> [ZKC_REF=<ref>]` | builds zkc to an explicit path instead of GOBIN, leaving the `zkc` on your PATH alone |
| `make -f arithmetization/src/test/Makefile zkc-trace JSON=<json> [ZKC=<bin>] [ZKC_TRACE_FLAGS=…]` | runs `zkc trace`; the counterpart to the existing `zkc-exec` |

`ZKC_TRACE_FLAGS` defaults to `--stats`; the weekly job passes `--stats --check` for the second
heavy step. Both targets are usable from any other workflow that needs a pinned zkc or a trace.

## Doing it by hand

```bash
zkc compile --stats --order name arithmetization/src/main/riscv/main.zkc

make -C riscv-guests/l2-execution compile KECCAK_ACCEL=true
make -C riscv-guests/build_common elf-to-json \
     BIN_EXT="$PWD/riscv-guests/l2-execution/zig-out/bin/evm_execution_guest" \
     JSON_EXT=/tmp/guest.json \
     IN_BYTES="@$PWD/riscv-guests/l2-execution/test/testdata/stateless_input.ssz"

zkc trace --stats         /tmp/guest.json arithmetization/src/main/riscv/main.zkc
zkc trace --stats --check /tmp/guest.json arithmetization/src/main/riscv/main.zkc
```

## Gotchas this tooling encodes

- **A constraint failure does not change zkc's exit code.** It reports the failing constraints and
  still exits 0, so pass/fail is read from the `failing …` lines, never from `$?`.
- **`zkc --version` is not a reliable identity.** Use `go version -m <binary>`; the `mod` line
  embeds the commit and its timestamp. A version string that looks old can be *newer* than the
  repo's pinned `ZKC_REF`.
- **`make -C arithmetization install-zkc` overwrites `~/go/bin/zkc`** with the pinned `ZKC_REF`,
  and every `make …-exec` target depends on it.
- **`-q` no longer exists in zkc**, so `riscv-guests/l2-execution/Makefile`'s default
  `ZKC_EXEC_FLAGS ?= -q` fails. CI never notices because it always overrides the variable. This
  tooling calls `zkc` directly rather than going through `exec`.
- **Max degree needs the column label, not just the counts.** zkc leaves the final bucket labelled
  `d8+` while nothing has reached degree 8 and closes it to `d8-d<max>` once something does. The
  renderer also asserts Σ cₖ·k² equals the reported complexity, which catches a mis-mapped column
  for free.
- **Do not run `make linker-script`:** `linker_script.ld` is tracked and `@embedFile`d by the zig
  build, so regenerating it only risks dirtying the tree.
- **`time` must wrap `timeout`, not the reverse.** Otherwise the cap kills `time` and the peak-RSS
  measurement is lost in exactly the case worth recording. On macOS there is no `timeout` and no
  GNU `time`, so the local script hand-rolls a watchdog that kills the child rather than `time`.
- **macOS "peak memory footprint" is not a peak RSS.** It is a cumulative allocation figure; use
  "maximum resident set size".
