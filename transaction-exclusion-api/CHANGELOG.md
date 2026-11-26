## [2.0.0] - 2026-11-26

### 🚀 Features

- *(docker,misc)* [**breaking**] Harden Dockerfiles — non-root users, digest pinning, re-enable KICS (#4471)

### ⚙️ Miscellaneous Tasks

- *(maru, coordinator, tx-exclusion-api)* Pin 25-noble image digest for the cve fix and remove the temp workaround (#4593)
## [1.0.1] - 2026-11-19

### 🚜 Refactor

- *(misc)* Rename Linea to Lineth across documentation and codebase (#3316)

### ⚙️ Miscellaneous Tasks

- *(misc)* Update jackson from 2.19.4 to 2.22.1 (#3595)
- *(misc)* Rename gradle plugins (#3729)
- *(misc)* Fix gradle build correctness bugs (#4044)
- *(misc)* Decrease vertx threadpool sizes for testing (#4047)
- *(misc)* Ship license files and metadata in all published artifacts (#4336)
- *(maru, coordinator, tx-exclusion-api)* JAVA_TOOL_OPTIONS="-XX:-UseBASE64Intrinsics"  (#4540)
