# Linea Besu Package Release

This document describes the changes introduced to streamline **linea-sequencer** and **linea-tracer** releases, including a dedicated version catalog, automatic releases on merge to `main`, and local and CI support for building the linea-besu-package image with locally built tracer and sequencer plugins.

---

## 1. Introduction of `releases.versions.toml`

A separate Gradle version catalog file **`gradle/releases.versions.toml`** was introduced to hold release versions for the tracer (arithmetization) plugin, distinct from the main dependency catalog `gradle/libs.versions.toml`.

### File format and location

- **Path:** `gradle/releases.versions.toml`
- **Example content:** (Same versioning semantics as in `linea-besu-package/versions.env`)

  ```toml
  [versions]
  arithmetization = "beta-v4.4-rc7.4"
  ```

### Purpose

- **Single source of truth** for tracer and sequencer release version (when built from source):
  - tracer artifacts will be named as `linea-tracer-[arithmetization]-[7-char-commit-hash].*`: e.g. `linea-tracer-beta-v4.4-rc7.4-9f55c53.zip`, `linea-tracer-beta-v4.4-rc7.4-9f55c53.jar`, `arithmetization-beta-v4.4-rc7.4-9f55c53.jar`, etc.
  - Sequencer artifact will be named as `linea-sequencer-[arithmetization]-[7-char-commit-hash].*`: e.g. `linea-sequencer-beta-v4.4-rc7.4-9f55c53.zip`, `linea-sequencer-beta-v4.4-rc7.4-9f55c53.jar`, etc.
  - `arithmetization` version will serve as the `expected-traces-api-version` for tracer nodes

- **Trigger workflows** when tracer release version have been updated:
  - Build a **linea-besu-package** Docker image using **locally built** tracer and sequencer plugins (from source) with the desired besu version defined in `libs.versions.toml` and runs e2e tests against that image
  - Automatically trigger the release workflow of linea-besu-package with **locally built** tracer and sequencer plugins when a PR merged into `main`

- **Gradle usage:** The `releases` catalog is registered in **root `settings.gradle`** so that all projects (including `:tracer` and `besu-plugins:linea-sequencer`) can reference it, e.g. `releases.versions.arithmetization.get()`.

---

## 2. Linea-Besu-Package Image with Local-Built Tracer and Sequencer in E2E (when `releases.versions.toml` changes)

When revelant files (e.g. under tracer and sequencer related folders, `gradle/releases.versions.toml`, `gradle/libs.versions.toml`, etc.) have been changed, the CI pipeline builds the **linea-besu-package** Docker image using **locally built** tracer and sequencer plugins (from source) with the desired besu version in `libs.versions.toml` and runs e2e tests against that image. This validates integration at PR time instead of only at release time.

---

## 3. Automatically release Linea-Besu-Package Image with Local-Built Tracer and Sequencer when push to main (when `releases.versions.toml` changes)

On push to `main`, the CI pipeline builds and publishes the **linea-besu-package** Docker image using **locally built** tracer and sequencer plugins (from source) with the desired besu version in `libs.versions.toml`, the release would be found in https://github.com/Consensys/linea-monorepo/releases

---

## 4. Linea-Besu-Package Makefile: Local Build with Local Tracer and Sequencer

The **`linea-besu-package/Makefile`** was revised so one can build the linea-besu-package image **locally** using tracer and sequencer plugins built from source in this repo, with versions read from `gradle/releases.versions.toml`.

### Example: full local build and e2e

From the repository root:

```bash
cd linea-besu-package
make build-local
make run-e2e-test-with-arithmetization-version
# when done
make run-e2e-test-cleanup
```

This builds tracer and sequencer from source, assembles the package with those zips, builds the Docker image, and runs e2e with the arithmetization version as the expected traces API version.

### Overrides

- **`TAG`** – Docker image tag (default: `local`).
- **`PLATFORM`** – Optional platform for `docker buildx` (e.g. `linux/amd64`).
- **`LOCAL_TRACER_ZIP_FOLDER`**, **`LOCAL_SEQUENCER_ZIP_FOLDER`** – Override if your zips are in a different location.
- **`EXPECTED_TRACES_API_VERSION`** - Override for a specific expected traces-api version used by Coordinator

---

## 5. Linea-besu-package release flow examples

### When linea-besu-upstream version needs to be updated

1. Create a PR to update the `besu` field in [libs.versions.toml](../gradle/libs.versions.toml)
2. Make necessary changes in tracer and seqeuencer to accommodate the besu version change, and update the corresponding `arithmetization` version in [releases.versions.toml](../gradle/releases.versions.toml) (Please note that sequencer version will be set as `[arithmetization_version]-[7_char_commit_hash]`)
3. Once the testing workflows of tracer and sequencer are all passed, the CI pipeline will build the `linea-besu-package` image using **locally built** tracer and sequencer plugins (from source) and runs e2e tests against that image
4. When the PR merged into `main`, the releases of linea-besu-package will be triggered automatically

### When arithmetization version needs to be updated

1. Create a PR to make necessary changes in tracer (and seqeuencer if needed), and update the corresponding `arithmetization` version in [releases.versions.toml](../gradle/releases.versions.toml) (Please note that sequencer version will be set as `[arithmetization_version]-[7_char_commit_hash]`)
2. Once the testing workflows of tracer and sequencer are all passed, the CI pipeline will build the `linea-besu-package` image using **locally built** tracer and sequencer plugins (from source) and runs e2e tests against that image
3. When the PR merged into `main`, the release of linea-besu-package will be triggered automatically

### When hotfix of linea-besu-upstream is needed on a production image

1. Create a hotfix PR based on the **same commit tag** as the production image, and update the `besu` field in [libs.versions.toml](../gradle/libs.versions.toml)
2. Make necessary changes in tracer and seqeuencer to accommodate the besu version change, and update the corresponding hotfix `arithmetization` versions in [releases.versions.toml](../gradle/releases.versions.toml)
3. The CI pipeline will build the `linea-besu-package` image using **locally built** tracer and sequencer plugins (from source) and runs e2e tests against that image
4. Trigger manual release workflow on the hotfix branch (Remember not to enable the "download tracer and sequencer artifacts" option)

---

## 6. Related documentations

- [Linea-besu-package README](./linea-besu-package/README.md) – how to release linea-besu-package with tracer and sequecer plugins (built from source OR download from `linea-monorepo` releases)
- [Linea-besu-package HowTo/Release Notion doc](https://www.notion.so/consensys/Linea-besu-package-HowTo-Release-2effc61a326e80f2b384cbd635e507c6) - current release process of linea-besu-package in face of besu version update
