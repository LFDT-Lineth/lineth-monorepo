## [2.0.0] - 2026-07-14

### 🚀 Features

- *(coordinator)* [**breaking**] Web3j upgrade to onboard 7594 support (#3514)

### 🐛 Bug Fixes

- *(misc)* Web3j-solc solidity resolution workaround (#3516)
- *(linea-besu)* Updating Besu version (#3535)

### ⚙️ Miscellaneous Tasks

- *(coordinator)* Update kotlin to v2.4 (#3454)
- *(ci)* Update image tags (#3553)
- *(deps)* Align bouncycastle catalog pin to 1.84 (#3562)
## [1.1.1] - 2026-06-26

### 🐛 Bug Fixes

- *(e2e)* Trying to fix rare profitability issues, FTX fix (#3388)
- *(besu-plugins)* Removed mutual exclusiveness of the txpool and blo… (#3416)
- *(Maru)* Fixed a bug when simultaneous mutual connection attempts r… (#3439)
## [1.1.0] - 2026-06-11

### 🚀 Features

- *(linea-besu)* Add interfaces module: security and LineaTransactionSelectionResult (#3238)
- *(linea-besu)* Forced transactions integration with security policy transaction selector (#3295)

### 🐛 Bug Fixes

- *(sequencer)* Bypass background scheduler collision in buildNewBlockAndWait(Long) (#3072)
- *(tracer)* No parallelism (#2957)
- *(arithmetization)* Fix alert 514 (#3246)
- *(arithmetization)* Security alert 513 on loop condition in BlockDataInstruction (#3253)

### 🚜 Refactor

- *(misc)* Rename Linea to Lineth across documentation and codebase (#3316)

### ⚙️ Miscellaneous Tasks

- *(linea-besu)* Move besu project under nested directory (#3063)
- *(linea-besu)* Move linea-besu-package into linea-besu/package (#3069)
- *(linea-besu)* Move besu-plugins into linea-besu/plugins (#3075)
- *(misc)* Besu-plugin acceptance test cleanup of deadcode (#3152)
- *(maru)* Remove spring dependency management (#3205)
- *(misc)* Rename Consensys/linea-monorepo references to LFDT-Lineth/lineth-monorepo (#3297)
- *(misc)* Point references at in-tree paths for previously-external repos (#3309)
