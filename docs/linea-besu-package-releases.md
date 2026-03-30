# Linea Besu Package Release

This document describes the changes introduced to streamline **linea-sequencer** and **linea-tracer** releases, and local and CI support for building the linea-besu-package image with locally built besu, tracer, and sequencer plugins.

---

## 1. Introduction of `besuCommit` in `libs.versions.toml`

A new field called `besuCommit` in `libs.versions.toml` to denote the commit hash tag from `besu-eth/besu` for tracer and sequencer plugins to build against with.

### File format and location

- **Path:** `gradle/libs.versions.toml`
- **Example content:**

  ```toml
  [versions]
  besuCommit = "d68679d6887abf8c407f99456cf2502004fa2bad"
  ```

### Purpose

- **Single source of truth** for locally-built besu release version:
  - Besu artifact will be named as `besu-[release-tag]-[7-char-besu-commit-hash].tar.gz`: e.g. `besu-25.12.0-d68679d.tar.gz` where `25.12.0` is the latest release tag on `hyperleger/besu` at the given `besuCommit`, the artifact will be placed under the `tmp/besu-eth/build/distributions/folder` and the `besu` field in `libs.version.toml` will be automatically updated, e.g. `besu = 25.12.0-d68679d`, by running `gradlew buildAndUpdateBesuVersionInLibsVersions` or `cd linea-besu-package && make build`

- **Trigger workflows** when the desired besu commit, tracer/sequencer, or other plugins have been updated:
  - Build a **linea-besu-package** Docker image using **locally built** tracer and sequencer plugins with the desired `besuCommit` defined in `libs.versions.toml` (will build it from source if the corresponding besu version were not found in maven) and runs e2e tests against that image
  - Automatically trigger the maven publishing workflow for linea-besu if the besu version corresponding to the `besuCommit` in `libs.versions.toml` was not found in maven (during pull requests only)

---

## 2. Linea-Besu-Package Image with Local-Built Tracer and Sequencer in E2E (when relevant files had been changed)

When relevant files (e.g. under tracer and sequencer related folders, `gradle/libs.versions.toml`, etc.) have been changed, the CI pipeline builds the **linea-besu-package** Docker image using **locally built** tracer and sequencer plugins with the desired `besuCommit` defined in `libs.versions.toml` (will build it from source if the corresponding besu version were not found in maven), and runs e2e tests against that image. This validates integration at PR time instead of only at release time.

---

## 3. Linea-Besu-Package Makefile: Local Build with Local Tracer and Sequencer

The **`linea-besu-package/Makefile`** was revised so one can build the linea-besu-package image **locally** using besu, tracer, and sequencer plugins built from source in this repo, with the desired besu commit from `gradle/libs.versions.toml`.

### Example: full local build and e2e

From the repository root:

```bash
cd linea-besu-package
make build
make run-e2e-test
# when done
make run-e2e-test-cleanup
```

This builds tracer and sequencer from source (and besu if needed), assembles the package with those zips, builds the Docker image, and runs e2e tests.

### Overrides

- **`TAG`** – Docker image tag (default: `local`).
- **`PLATFORM`** – Optional platform for `docker buildx` (e.g. `linux/amd64`).
- **`LOCAL_TRACER_DIST_FOLDER`**, **`LOCAL_SEQUENCER_DIST_FOLDER`**, **`LOCAL_BESU_DIST_FOLDER`** – Override if your zips are in a different location.

---

## 5. Linea-besu-package release flow examples

### When the desired besu version needs to be updated

1. Create a PR to update the `besuCommit` field in [libs.versions.toml](../gradle/libs.versions.toml) which denotes the desired commit hash tag from the [besu-eth/besu](https://github.com/besu-eth/besu) repo
- The triggered Github workflows would automatically build the besu at the desired commit hash tag and publish it to cloudsmith [maven](https://cloudsmith.io/~consensys/repos/linea-besu/packages/) AND it would automatically push a commit to the PR to update the `besu` field in [libs.versions.toml](../gradle/libs.versions.toml)
- To get the corresponding `besu` version from the besu commit hash before running any Github workflow, one can run `./gradlew buildAndUpdateBesuVersionInLibsVersions` locally which shall see the update in [libs.versions.toml](../gradle/libs.versions.toml) OR run Makefile command i.e. `cd linea-besu-package && make build-besu-local-and-update-lib-version`
2. Make necessary changes in tracer and seqeuencer to accommodate the besu version change (Please note that tracer and sequencer version will be set as `linea-[7_char_commit_hash]`)
3. Once the testing workflows of tracer and sequencer are all passed, the CI pipeline will build the `linea-besu-package` image using **locally built** tracer and sequencer plugins and runs e2e tests against that image
4. When the PR merged into `main`, manually trigger the manual release [workflow](https://github.com/Consensys/linea-monorepo/actions/workflows/linea-besu-package-release.yml) and the release will be found in the linea-monorepo release [page](https://github.com/Consensys/linea-monorepo/releases)

### When arithmetization version needs to be updated

1. Create a PR to make necessary changes in tracer (and seqeuencer if needed) (Please note that tracer and sequencer version will be set as `linea-[7_char_commit_hash]`)
2. Once the testing workflows of tracer and sequencer are all passed, the CI pipeline will build the `linea-besu-package` image using **locally built** tracer and sequencer plugins and runs e2e tests against that image
3. When the PR merged into `main`, manually trigger the manual release [workflow](https://github.com/Consensys/linea-monorepo/actions/workflows/linea-besu-package-release.yml) and the release will be found in the linea-monorepo release [page](https://github.com/Consensys/linea-monorepo/releases)

### When hotfix of besu version is needed on a production image

1. Create a hotfix PR based on the **same commit tag** as the production image, and update the `besuCommit` field in [libs.versions.toml](../gradle/libs.versions.toml)
2. Make necessary changes in tracer and seqeuencer to accommodate the besu version change
3. The CI pipeline will build the `linea-besu-package` image using **locally built** tracer and sequencer plugins and runs e2e tests against that image
4. Trigger manual release [workflow](https://github.com/Consensys/linea-monorepo/actions/workflows/linea-besu-package-release.yml) on the hotfix branch and the release of linea-besu-package will be found in the linea-monorepo release [page](https://github.com/Consensys/linea-monorepo/releases)

---

## 6. Related documentations

- [Linea-besu-package README](../linea-besu-package/README.md)