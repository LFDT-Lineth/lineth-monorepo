## [unreleased]

### 🚀 Features

- *(coordinator)* Wire config-docs via a buildSrc plugin and declarative spec (#3607)
- *(coordinator)* Document all TOML config keys with @ConfigDoc/@ConfigSection (#3568)
- *(coordinator)* GasPriceCapProviderV2 and DRY (#3624)

### 🐛 Bug Fixes

- *(coordinator)* Small fix on start/stop handlers (#3621)

### 🚜 Refactor

- *(coordinator)* Decompose L1DependentApp into smaller scoped apps (#3615)

### ⚙️ Miscellaneous Tasks

- *(coordinator)* Add riscv enablement config (#3617)
- *(coordinator)* Adds initial scafold for contract v9 RISC-V (#3618)
- *(coordinator)* Cleanup replaced apps in PR #3615 (#3622)
## [1.0.0] - 2026-07-22

### 🚀 Features

- *(coordinator)* [**breaking**] Web3j upgrade to onboard 7594 support (#3514)
- *(coordinator)* Add inital block number config for finalized state search (#3534)
- *(coordinator)* Add extension seam (#3532)

### ⚙️ Miscellaneous Tasks

- *(misc)* Refactor hoplite decoders to it's own module (#3557)
- *(misc)* Update jackson from 2.19.4 to 2.22.1 (#3595)
- *(misc)* Cleanup redundant deps (#3596)
## [0.3.0] - 2026-07-07

### 🚀 Features

- *(coordinator)* Add config schema walker for docs generation (#3488)

### 🐛 Bug Fixes

- *(coordinator)* Pretty-print startup config logs (#3203)
- *(coordinator)* Bound eth_getLogs in finalized-state lookup (#3519)
- *(coordinator)* Bound eth_getLogs in deployment-block lookup (#3520)
- *(coordinator)* L1FinalizationPriorityFeeCalculator feeLowerBound config (#3517)
## [0.2.0] - 2026-07-03

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
