# Arithmetization

This directory holds the arithmetization of RISC-V, with target = `riscv64im_zicclsm-unknown-none-elf`.

Arithmetization is written in ZkC, a simple imperative language designed primarily for writing programs whose executions can be proved. This language is written and maintained by the Linea arithmetization team, available in the [`go-corset`repository](https://github.com/Consensys/go-corset/blob/main/ZKC_LANGUAGE.md)

## Prerequisites for local setup

### Install target toolchain

Install toolchain `riscv64im_zicclsm-unknown-none-elf`

Note: The target riscv64im_zicclsm-unknown-none-elf is not a standard target. To install it, you can use a standard RISC‑V toolchain and just specify the architecture/extensions manually.

### Install Zkc

No official releases yet
Clone repo [go-corset](https://github.com/Consensys/go-corset)

`go install .cmd/zkc`

## CI actions and workflows

### Setup Arithmetization RISC-V Environment

All RISC-V arithmetization workflow should use the composite action **[Setup Arithmetization RISC-V Environment](../.github/actions/setup-arithmetization-riscv/action.yml)**.
It installs:
- Go (version pinned in the action)
- the `go-corset` CLI from `github.com/consensys/go-corset`

### Tracer riscv-constraints check compilation

The workflow **[Tracer riscv-constraints check compilation](../.github/workflows/arithmetization-zkc-riscv-check-compilation.yml)** verifies that the ZkC program compiles in CI.
It runs the arithmetization setup step above .
It checks out [go-corset](https://github.com/Consensys/go-corset), installs the `zkc` CLI, and runs `zkc compile` on the main entrypoint under this tree.

As there are no official releases for `zkc` CLI yet, the install is done from main. If you wish to install the version from a branch, the flow can be modified at step `Checkout go-corset repo`, see comment.

It runs on **push** and **pull_request** to `main` when relevant paths change, including:

- `arithmetization/**`
- `.github/actions/setup-arithmetization-riscv/**`
- `.github/workflows/arithmetization-*.yml`

It is also available via **workflow_dispatch** and **workflow_call**.
