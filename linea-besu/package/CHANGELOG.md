## [2.1.0-rc1] - 2026-08-07

### 🚀 Features

- *(linea-besu)* Update to v26.7.0 (#3623)
- *(linea-besu)* Update to 26.7.1 (#3699)

### 🐛 Bug Fixes

- *(linea-besu)* Align Besu Kotlin runtime (#3734)
- *(linea-besu)* Add the missing web3j crypto jar in dist and fix ci to run e2e with locally built linea-besu-package

### ⚙️ Miscellaneous Tasks

- *(linea-besu)* Tidy v2.0.0 changelog entries (#3574)
- *(misc)* Update jackson from 2.19.4 to 2.22.1 (#3595)
- *(misc)* Rename gradle plugins (#3729)
## [2.0.0] - 2026-07-14

### 🐛 Bug Fixes

- *(linea-besu)* Updating Besu version (#3535)

### ⚙️ Miscellaneous Tasks

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
