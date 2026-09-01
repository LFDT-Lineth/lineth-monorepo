# Arithmetization

This directory holds the arithmetization of RISC-V, with target = `riscv64im_zicclsm-unknown-none-elf`.

Arithmetization is written in ZkC, a simple imperative language designed primarily for writing programs whose executions can be proved. This language is written and maintained by the Linea arithmetization team, available in the [`zkc` repository](https://github.com/LFDT-Lineth/zkc/blob/main/docs/ZKC_LANGUAGE.md).

## Prerequisites for local setup

### Install target toolchain

Install toolchain `riscv64im_zicclsm-unknown-none-elf`

Note: The target riscv64im_zicclsm-unknown-none-elf is not a standard target. To install it, you can use a standard RISC‑V toolchain and just specify the architecture/extensions manually.

### Zkc

The [`zkc`](https://github.com/LFDT-Lineth/zkc) tool is pinned in [`go.mod`](go.mod) and requires no separate installation. Run it from this directory with:

```console
go tool zkc
```

Make targets use this pinned version by default and download its module dependencies before invoking the tool. To select another commit, branch, or tag, pass `ZKC_REF`, for example `make riscv-check-lint ZKC_REF=main`. This updates `go.mod` and `go.sum` with `go get` before downloading the selected dependencies and running the tool.

## CI actions and workflows

### Setup Arithmetization RISC-V Environment

All RISC-V arithmetization workflow should use the composite action **[Setup Arithmetization RISC-V Environment](../.github/actions/setup-arithmetization-riscv/action.yml)**.
It installs:
- Go (version pinned in the action)

### Tracer riscv-constraints check compilation

The workflow **[Tracer riscv-constraints check compilation](../.github/workflows/arithmetization-zkc-riscv-check-compilation.yml)** verifies that the ZkC program compiles in CI.
It runs the arithmetization setup step above.
It runs the `zkc` tool pinned in `arithmetization/go.mod` and compiles the main entrypoint under this tree. A workflow input can override the pinned version when needed.

It runs on **push** and **pull_request** to `main` when relevant paths change, including:

- `arithmetization/**`
- `.github/actions/setup-arithmetization-riscv/**`
- `.github/workflows/arithmetization-*.yml`

It is also available via **workflow_dispatch** and **workflow_call**.

### Install Sail for ACT4 host builds

The `install-sail` target downloads Sail.

```bash
make -C arithmetization install-sail
```

See [src/test/README.md](src/test/README.md) for ACT4 prerequisites, build, and run commands.
