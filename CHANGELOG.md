## [unreleased]

### 🚀 Features

- *(linea-besu)* Add interfaces module: security and LineaTransactionSelectionResult (#3238)
- *(linea-besu)* Forced transactions integration with security policy transaction selector (#3295)

### 🐛 Bug Fixes

- *(sequencer)* Bypass background scheduler collision in buildNewBlockAndWait(Long) (#3072)
- *(tracer)* No parallelism (#2957)
- *(coordinator, jvm-libs, e2e, state-recovery, prover, docker, misc)* Remove state manager request version (#3099)
- *(coordinator)* Remove traces version from requests (#3110)
- *(ci)* Provide correct path to rlp_blocks.bin (#3125)
- *(prover)* Update rlp_blocks.bin path in shnarf_calculator tests (#3129)
- *(coordinator)* Export FTX number metrics (#3165)
- *(prover)* Stronger soundness binding for euclidean division and crumb decomposition (#2910)
- *(prover)* Valid-nonce-ftx (#3179)
- *(arithmetization)* Fix alert 514 (#3246)
- *(prover)* L2 Messages (#3195)
- *(arithmetization)* Security alert 513 on loop condition in BlockDataInstruction (#3253)
- *(prover)* Incorporate `isAllowedCircuitID` into aggregation FPI (#3194)
- *(prover)* Invalidity prover bug fixes (#3138)

### 🚜 Refactor

- *(misc)* Rename Linea to Lineth across documentation and codebase (#3316)

### ⚙️ Miscellaneous Tasks

- *(linea-besu)* Move besu project under nested directory (#3063)
- *(coordinator)* Make persistence module flat (#3066)
- *(linea-besu)* Move linea-besu-package into linea-besu/package (#3069)
- *(deps)* Refresh monorepo dependencies (#3061)
- *(coordinator)* Rename package net.consensys.zkevm.persistence to linea.persistence #3073
- *(linea-besu)* Move besu-plugins into linea-besu/plugins (#3075)
- *(2876)* Rename catch variable from it to e in GoBackedBlobShnarfCalculator (#2889)
- *(2876)* Coordinator review fixes — dead code, null safety, exception handling, dedup (#2882)
- *(coordinator)* Move Web3SignerTxSignService into web3j-extensions lib (#3091)
- *(coordinator)* Remove "build" prefix from package names
- Update gnark (#3089)
- *(coordinator)* Rename packages net.consensys.zkevm.* -> linea.* (#3105)
- *(deps)* Update Jest to 30.4 (#3077)
- Update to latest gnark and gnark-crypto (#3142)
- *(misc)* Besu-plugin acceptance test cleanup of deadcode (#3152)
- *(coordinator)* Log and message error improvements (#3193)
- *(maru)* Remove spring dependency management (#3205)
- Update gnark dependency (#3215)
- *(ci)* Migrate amd64 runners to gha-lfdt-lineth-ss scale sets (#3280)
- *(misc)* Rename Consensys/linea-monorepo references to LFDT-Lineth/lineth-monorepo (#3297)
- *(misc)* Point references at in-tree paths for previously-external repos (#3309)
- *(coordinator)* Favour generic name ChainSecurityRuleViolation over specific implementation Phylax (#3330)
- *(packages)* Rename npm package scope (#3324)
