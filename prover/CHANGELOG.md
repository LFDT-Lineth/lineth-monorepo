## [unreleased]

### 🐛 Bug Fixes

- *(prover)* Bind aggregation's initial state root to the parent shnarf via in-circuit opening (#3542)

### 🚜 Refactor

- *(prover)* Remove redundant and unsafe utility functions (#3273)

### ⚙️ Miscellaneous Tasks

- *(deps)* Bump golang.org/x/net (#3500)
## [1.0.5] - 2026-06-30

### 🐛 Bug Fixes

- *(prover)* Reserve a padding row in the state-manager modules (#3470)
## [1.0.4] - 2026-06-29

### 🐛 Bug Fixes

- *(prover)* Bit-decompose limbs by absolute row, not compact position (#3464)
- *(prover)* Filter accumulator-summary state-summary inclusions (#3465)
## [1.0.3] - 2026-06-29

### 🐛 Bug Fixes

- *(prover)* Bump BLOCKHASH module limit from 2048 to 4096 (#3426)
- *(prover)* Resolve data race in limitless prover (#3442)
- *(prover)* Index-out-of-range panic in merkle proof verification (#3453)
## [1.0.2] - 2026-06-18

### ⚙️ Miscellaneous Tasks

- Update gnark to a3ad59ad083caac7691cba84a497d4d7c1759d2a (#3402)
## [1.0.1] - 2026-06-18

### 🐛 Bug Fixes

- *(coordinator, jvm-libs, e2e, state-recovery, prover, docker, misc)* Remove state manager request version (#3099)
- *(ci)* Provide correct path to rlp_blocks.bin (#3125)
- *(prover)* Update rlp_blocks.bin path in shnarf_calculator tests (#3129)
- *(prover)* Stronger soundness binding for euclidean division and crumb decomposition (#2910)
- *(prover)* Valid-nonce-ftx (#3179)
- *(prover)* L2 Messages (#3195)
- *(prover)* Incorporate `isAllowedCircuitID` into aggregation FPI (#3194)
- *(prover)* Invalidity prover bug fixes (#3138)
- *(prover)* Post small-fields constraints (#2845)
- *(prover)* Populate isAllowedCircuitID in aggregation response (#3381)

### 🚜 Refactor

- *(misc)* Rename Linea to Lineth across documentation and codebase (#3316)
- *(prover-ray)* Export logderivativesum internals for verifier-ray codegen (#3354)

### ⚡ Performance

- *(prover)* Limitless prover performance optimization (#3362)

### ⚙️ Miscellaneous Tasks

- Update gnark (#3089)
- Update to latest gnark and gnark-crypto (#3142)
- Update gnark dependency (#3215)
- *(ci)* Migrate amd64 runners to gha-lfdt-lineth-ss scale sets (#3280)
## [1.1.0-devnet] - 2026-06-08

### 🚀 Features

- *(prover)* Add chain config sanity check for invalidity proofs (#3174)

### 🐛 Bug Fixes

- *(coordinator, jvm-libs, e2e, state-recovery, prover, docker, misc)* Remove state manager request version (#3099)
- *(ci)* Provide correct path to rlp_blocks.bin (#3125)
- *(prover)* Update rlp_blocks.bin path in shnarf_calculator tests (#3129)
- *(prover)* Remove global overwrite in FullZKEVMWithSuite  (#3114)
- *(prover)* Populate CongloVK and VKMerkleRoot in invalidity limitless circuit (#3150)
- *(prover)* Valid-nonce-ftx (#3182)
- *(prover)* Make MAX_L2_LOGS configurable via traces_limits BLOCK_L2_L1_LOGS (#3285)
- *(prover)* Stronger soundness binding for euclidean division and crumb decomposition (#2910)
- *(prover)* Valid-nonce-ftx (#3179)
- *(prover)* L2 Messages (#3195)
- *(prover)* Incorporate `isAllowedCircuitID` into aggregation FPI (#3194)
- Failing invalidity tests

### ⚙️ Miscellaneous Tasks

- Update gnark (#3089)
- Update to latest gnark and gnark-crypto (#3142)
- Update gnark dependency (#3215)
- *(ci)* Migrate amd64 runners to gha-lfdt-lineth-ss scale sets (#3280)
