## [unreleased]

### 🚀 Features

- *(riscv-guest)* L2 execution guest rollup (#3783)
- *(riscv-guest)* Rollup guest program stub with shared SSZ package (#3894)
- *(riscv-guest)* Adding fallback precompile implementations (#3931)
- *(riscv-guest)* RiscV guests release workflow (#4102)
- *(riscv-guest)* Setting up the release workflow (#4119)
- *(riscv-guest)* Refactoring to enable further Rollup GP development (#4342)
- *(docker,misc)* [**breaking**] Harden Dockerfiles — non-root users, digest pinning, re-enable KICS (#4471)
- *(riscv-guest)* Refactoring for the Aggregation guest program (#4630)

### 🐛 Bug Fixes

- *(riscv-guest)* Renaming the kebab-case dirs to sneak case, which is more conventional in Zig world (#4320)
- *(riscv-guest)* Build script refactoring. Introducing a convention … (#4347)
- *(riscv-guest, misc, prover-ray)* Riscv guest/shared zkevm fixtures (#4368)
- *(riscv-guest)* Zesu update with some small breaking change (#4586)

### 🚜 Refactor

- *(coordinator, prover-ray, riscv-guest)* Rename the DA carry-over fields to match DaStreamPosition (#4556)
