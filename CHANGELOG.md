## [unreleased]

### 🐛 Bug Fixes

- *(sequencer)* Bypass background scheduler collision in buildNewBlockAndWait(Long) (#3072)
- *(tracer)* No parallelism (#2957)
- *(coordinator, jvm-libs, e2e, state-recovery, prover, docker, misc)* Remove state manager request version (#3099)
- *(coordinator)* Remove traces version from requests (#3110)
- *(ci)* Provide correct path to rlp_blocks.bin (#3125)
- *(prover)* Update rlp_blocks.bin path in shnarf_calculator tests (#3129)
- *(coordinator)* Export FTX number metrics (#3165)

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
