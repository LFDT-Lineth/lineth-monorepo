# ACT4 / `riscv-arch-test` driver for Linea

Two scripts to drive the upstream RISC-V Architectural Compliance Tests
(ACT4) against our zkc, end-to-end:

| Script | Purpose |
|---|---|
| `build_linea_elfs.sh` | Compiles the ACT4 self-checking ELFs for the Linea (`linea-rv64im-zicclsm`) config, inside the official `riscv-act4` docker image. |
| `run_linea_elfs.sh`   | Converts each ELF to a zkc JSON and runs `zkc exec` on it. Writes a PASS/FAIL summary plus per-test logs. |

Both scripts resolve all paths relative to their own location (no
absolute paths baked in). External inputs (config dir, ELF dir, image
tag) are overridable via environment variables; defaults assume the
sibling-clone layout described below.

## One-time setup

1. **Clone [`riscv-arch-test`](https://github.com/riscv/riscv-arch-test)
   alongside `linea-monorepo`** (the script for the docker build assumes
   this exact layout — used once):

   ```
   <some-parent>/
   ├── linea-monorepo/       # this repo  (script defaults resolve relative to here)
   └── riscv-arch-test/      # OPTIONAL — only needed once, to `docker build` the image
   ```

   **`riscv-arch-test`** is only consulted by step 2 below — once
   you've built the docker image, the scripts never touch this clone
   again (everything they need is baked into the image). You can
   delete the clone afterwards if disk space matters; it's also the
   easiest way to read the upstream `.S` test sources during
   debugging (e.g. `tests/rv64i/I/I-add-00.S`).

   No other peer repo is needed at run time: the Linea ACT4 config
   (`link.ld`, `sail.json`, `linea-rv64im-zicclsm.yaml`,
   `rvmodel_macros.h`, `rvtest_config.h`, `test_config.yaml`) lives in
   this repo under `../config/linea-rv64im-zicclsm/` and is what the
   build script mounts into the container.

2. **Build the ACT4 docker image** (one-shot; can take ~30 min the first time):

   ```bash
   cd ../../../../../../riscv-arch-test       # from this scripts/ dir
   docker build -t riscv-act4 .
   ```

3. **zkc** must be on `PATH`. The simplest install path is
   `go install github.com/consensys/go-corset/cmd/zkc@v1.2.14`.

## Reproduce the run

With the layout above, from this directory:

```bash
./build_linea_elfs.sh       # compile RV64I + RV64M ELFs (~2 min)
./run_linea_elfs.sh         # run each through zkc, write ../bin/results.txt (~5 min for I+M)
```

No env vars need to be set when the cloned repos are in
their default positions. Output:

- `../bin/results.txt` — one `PASS` / `FAIL` line per test, plus
  totals on the last line.
- `../bin/logs/<test>.out` — filtered zkc terminal output per test.
  On FAIL, also includes a `--- WRITE_STR ---` block with the
  framework's diagnostic message (failing label, PC, expected vs
  actual) — emitted verbatim by the `syscall 64` handler via the
  `%c` printf format.
- `../bin/logs/<test>.full` — full unfiltered zkc output, kept only
  when a test fails (useful when the `WRITE_STR` block is not
  enough to root-cause).
- `../bin/elf2json` — cached Go-built ELF→JSON helper.
- `../bin/work/linea-rv64im-zicclsm/elfs/` — the compiled ACT4 ELFs.

## Environment variables (only needed to override defaults)

| Variable | Default | Used by | Notes |
|---|---|---|---|
| `ACT4_CONFIG_DIR` | `../config` | `build` | Directory that contains `linea-rv64im-zicclsm/`. Defaults to the in-repo copy under `act4/config/`. |
| `ACT4_WORK_DIR`   | `../bin/work` | both | Receives the build output. `run_linea_elfs.sh` derives `ELF_DIR` from it. |
| `ELF_DIR`         | derived from `ACT4_WORK_DIR` | `run` | Point at a different ELF tree. |
| `ACT4_IMAGE`      | `riscv-act4:latest` | `build` | Docker image tag. |
| `ACT4_EXTENSIONS` | `I,M` | `build` | Comma-separated list (e.g. `I` for just RV64I). |
| `ACT4_JOBS`       | `4` | `build` | Parallel `make` jobs inside the container. |
| `ACT4_FAST`       | `True` | `build` | When `True`, skip per-test `objdump` generation for faster builds. Set to empty (`ACT4_FAST=`) to get a `<test>.objdump` next to every ELF — useful when debugging. |
| `ACT4_DEBUG`      | _(empty)_ | `build` | Set to `True` to enable the framework's full debug output: per-test `.objdump` + Sail `<test>.sig.log` instruction trace + `<test>.sig.trap_report`. Slowest, most verbose. Mutually exclusive with `ACT4_FAST` — `ACT4_DEBUG` wins if both are set. |
| `ELF2JSON`        | `../bin/elf2json` (auto-built) | `run` | Path to the Go helper that converts ELF → zkc JSON. |
| `ZKC_MAIN`        | `../../../../main/riscv/main.zkc` (relative to scripts dir) | `run` | The zkc program that interprets the ELF. |
| `IN_BYTES`        | `""` (empty — ACT4 tests are self-contained) | `run` | Forwarded to `elf2json`; same meaning as in `../../Makefile`. |
| `PROGRAM_OFFSET`  | `0x00000000` | `run` | Forwarded to `elf2json`. |
| `IN_BYTES_OFFSET` | `0x08800000` | `run` | Forwarded to `elf2json` (unused by ACT4 but kept consistent with the Makefile). |
| `ENTRY_POINT`     | `$PROGRAM_OFFSET` | `run` | Forwarded to `elf2json` — ACT4 places `rvtest_entry_point` at the origin. |

## Required tools

- Docker (for `build_linea_elfs.sh`)
- `go` (auto-builds `elf2json` from `../../main.go` the first time)
- `zkc` on `PATH`
- bash, GNU coreutils

## Building just one extension

```bash
ACT4_EXTENSIONS=M ./build_linea_elfs.sh
ELF_DIR="../bin/work/linea-rv64im-zicclsm/elfs/rv64i/M" ./run_linea_elfs.sh
```

## Running a single test by hand

`run_linea_elfs.sh` caches each `elf2json` result at
`../bin/logs/<test>.json`. After at least one sweep you can re-run any
single test directly through `zkc`, bypassing the wrapper:

```bash
zkc exec ../bin/logs/<test>.json ../../../../main/riscv/main.zkc
```

## Build speed vs introspection

`build_linea_elfs.sh` defaults to `ACT4_FAST=True` because most reproduce
runs only need the ELFs. Two ways to ask for more output, in increasing
order of slowness:

```bash
# generate a human-readable disassembly next to every ELF
ACT4_FAST= ./build_linea_elfs.sh
# look at ../bin/work/linea-rv64im-zicclsm/elfs/rv64i/I/I-add-00.objdump

# additionally generate Sail instruction-by-instruction traces
ACT4_DEBUG=True ./build_linea_elfs.sh
# .sig.log  — full Sail trace
# .sig.trap_report — human-readable trap summary
```

Both options are forwarded as the framework's own `FAST=` / `DEBUG=`
make-args; the two are mutually exclusive and `ACT4_DEBUG` wins if both
are set.

## Inspecting the generated tests

The docker image generates the tests, but the work directory it writes to
is the host's `../bin/work/`, so every artefact is visible on the host
once the build finishes. Per test you get two stages of output:

```
../bin/work/linea-rv64im-zicclsm/
├── build/<arch>/<ext>/<test>.sig.elf      first-pass ELF with placeholder signatures
├── build/<arch>/<ext>/<test>.sig          binary signature blob from the Sail run
├── build/<arch>/<ext>/<test>.sig.log      Sail's stdout for this test
├── build/<arch>/<ext>/<test>.results      unpacked signature (`.quad` lines)
└── elfs/<arch>/<ext>/<test>.elf           the final self-checking ELF (what zkc runs)
```

`<arch>` is the **base ISA** (e.g. `rv64i` = the 64-bit integer base),
not a march string. The per-extension subdirectories — `<ext>` —
contain tests for the extension on top of that base. So `rv64i/M/`
holds the M-extension tests for the RV64I base, even though the
parent directory name doesn't mention M. This mirrors the upstream
`riscv-arch-test/tests/` layout exactly; renaming our copy would
diverge from every other zkVM's ACT4 setup.

Useful inspection commands:

```bash
# disassemble a final ELF
riscv64-unknown-elf-objdump -d ../bin/work/linea-rv64im-zicclsm/elfs/rv64i/I/I-add-00.elf | less

# section layout & symbols
riscv64-unknown-elf-readelf -SW ../bin/work/linea-rv64im-zicclsm/elfs/rv64i/I/I-add-00.elf
riscv64-unknown-elf-nm        ../bin/work/linea-rv64im-zicclsm/elfs/rv64i/I/I-add-00.elf | head

# what Sail expected, vs. the raw signature bytes
head -5 ../bin/work/linea-rv64im-zicclsm/build/rv64i/I/I-add-00.results
xxd        ../bin/work/linea-rv64im-zicclsm/build/rv64i/I/I-add-00.sig | head
```

The upstream `.S` test sources are not in `bin/work/` — they live in
your `riscv-arch-test` clone (e.g. `tests/rv64i/I/I-add-00.S`), and the
docker image picks them up from its own copy at build time.
