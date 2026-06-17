## [unreleased]

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
