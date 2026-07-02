## [unreleased]

### 🚀 Features

- *(coordinator)* Add config documentation annotations (#3463)
- *(coordinator)* First draft of risc-v prover client (#3269)

### 🐛 Bug Fixes

- *(coordinator, jvm-libs, e2e, state-recovery, prover, docker, misc)* Remove state manager request version (#3099)
- *(coordinator)* Remove traces version from requests (#3110)
- *(coordinator)* Export FTX number metrics (#3165)
- *(coordinator)* Drop insecure FileWriter temp-file fallback (CodeQL 21207) (#3413)
- *(coordinator)* Fixed a flacky test by closing the ftxInvalidityProofService in the ForcedTransactionsApp (#3438)
- *(coordinator)* Forced transactions concurrency update (#3444)
- *(coordinator)* Bound eth_getLogs queries to provider block-range limits (#3473)

### 🚜 Refactor

- *(misc)* Rename Linea to Lineth across documentation and codebase (#3316)

### ⚙️ Miscellaneous Tasks

- *(coordinator)* Make persistence module flat (#3066)
- *(coordinator)* Rename package net.consensys.zkevm.persistence to linea.persistence #3073
- *(2876)* Rename catch variable from it to e in GoBackedBlobShnarfCalculator (#2889)
- *(2876)* Coordinator review fixes — dead code, null safety, exception handling, dedup (#2882)
- *(coordinator)* Move Web3SignerTxSignService into web3j-extensions lib (#3091)
- *(coordinator)* Remove "build" prefix from package names
- *(coordinator)* Rename packages net.consensys.zkevm.* -> linea.* (#3105)
- *(coordinator)* Log and message error improvements (#3193)
- *(coordinator)* Favour generic name ChainSecurityRuleViolation over specific implementation Phylax (#3330)
- *(coordinator)* Update kotlin to v2.4 (#3454)
- *(coordinator)* LSP violation fix (#3386)
