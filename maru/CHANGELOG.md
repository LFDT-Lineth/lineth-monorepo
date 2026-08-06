## [unreleased]

### 🚀 Features

- *(maru)* Auto-generate the Maru TOML configuration reference and publish it to the Linea docs site (#NNNN)

### 🐛 Bug Fixes

- *(maru)* Declare jar task dependency on runtimeClasspath to fix manifest race (#3694)

### ⚙️ Miscellaneous Tasks

- *(misc)* Update jackson from 2.19.4 to 2.22.1 (#3595)
- *(misc)* Cleanup redundant deps (#3596)
- *(misc)* Rename gradle plugins (#3729)
## [1.3.0] - 2026-07-14

### 🚀 Features

- *(maru)* Added a new CL phase in which Beacon block's chain identi… (#3493)

### 🐛 Bug Fixes

- *(maru)* Address PR 3126 workflow follow-ups (#3164)
- *(arithmetization)* Fix alert 514 (#3246)
- *(maru)* Making discovery retry configurable to increase the convergence speed (#3387)
- *(Maru)* Fixed a bug when simultaneous mutual connection attempts r… (#3439)
- *(coordinator)* Bound eth_getLogs in finalized-state lookup (#3519)
- *(linea-besu)* Updating Besu version (#3535)

### 🚜 Refactor

- *(maru)* Fix maru build circular dependencies (#3204)
- *(maru)* Relocate Maru JVM libs (#3236)

### ⚙️ Miscellaneous Tasks

- *(maru)* Follow up for pr 3236 (#3237)
- *(maru)* Split testing workflow (#3234)
- *(maru)* Remove unused test (#3279)
- *(maru)* Per-test log isolation + bounded integration-test concurrency (#3343)
- *(coordinator)* Update kotlin to v2.4 (#3454)
- *(coordinator)* LSP violation fix (#3386)
- *(misc)* Trying to speed up Maru integration tests and optimize resource consumption (#3462)
- *(maru)* Reuse hoplite decoders (#3559)
