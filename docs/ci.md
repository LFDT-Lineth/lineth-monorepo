# CI Workflow

**Source:** `.github/workflows/main.yml`
**Triggers:** Every pull request (any branch) and every push to `main`.

> **E2E tests require manual approval on PRs.**
> Docker build and E2E will not run automatically on a pull request. A maintainer must approve the `docker-build-and-e2e` GitHub Environment deployment before those stages proceed. If your PR is waiting on E2E results, ask a maintainer to approve it.

---

## Pipeline at a Glance

```
┌─────────────────────────┐
│  filter-commit-changes  │  Which components actually changed?
└──────────┬──────────────┘
           │
     ┌─────┴──────────────────────────────┐
     │                                    │
     ▼                                    ▼
┌──────────────────────┐    ┌────────────────────────────────┐
│  check-and-tag-images│    │  get-has-changes-requiring-    │
│  (image cache check) │    │  e2e-testing                   │
└──────────┬───────────┘    └──────────────┬─────────────────┘
           │                               │
           │             ┌─────────────────┘
           ▼             ▼
┌──────────────────────────────────┐
│  code-analysis (CodeQL)          │  parallel, always runs
└──────────────────────────────────┘

┌──────────────────────────────────┐
│  testing                         │  unit/integration per component
│  (skipped if docs-only change)   │
└──────────────────────────────────┘

┌──────────────────────────────────┐
│  manual-docker-build-and-e2e-    │  manual approval gate (PRs only)
│  tests                           │
└──────────────┬───────────────────┘
               ▼
┌──────────────────────────────────┐
│  docker-build                    │  build images, no push yet
└──────────────┬───────────────────┘
               ▼
┌──────────────────────────────────┐
│  run-e2e-tests                   │  full local stack E2E
└──────────────┬───────────────────┘
               │
       ┌───────┴────────────────────────────────┐
       ▼                                        ▼
┌────────────────────┐              ┌───────────────────────────────┐
│ publish-linea-besu │ (PR only,    │ publish-images-on-main        │
│ after e2e success  │  same-repo)  │ (main only, push_image: true) │
└────────────────────┘              └───────────────────────────────┘

┌──────────────────────────────────┐
│  cleanup-deployments             │  remove GitHub deployment record
└──────────────────────────────────┘

┌──────────────────────────────────┐
│  notify (Slack)                  │  main-branch failures only
└──────────────────────────────────┘
```

---

## Stage-by-Stage Breakdown

### 1. `filter-commit-changes`

**What it does:** Runs `dorny/paths-filter` twice to produce two sets of flags:

- **Per-component flags** (`coordinator`, `prover`, `postman`, `smart-contracts`, `tracer`, `linea-sequencer-plugin`, `transaction-exclusion-api`, `linea-besu`, `linea-besu-package`, `tracer-constraints`, `native-yield-automation-service`, `lido-governance-monitor`, `staterecovery`) - `true` when any file belonging to that component changed.
- **`has-changes-requiring-build`** - `false` when every changed file is on the exclusion list (`*.md`, `*.mdx`, `docs/**`, `contracts/token-generation-event/**`, `codeql.yml`), meaning the PR contains nothing that requires compilation or testing. `true` as soon as any changed file falls outside that list. This is the main gate that determines whether most downstream jobs run at all.
- **`has-changes-requiring-linea-besu-package-build`** - `true` when the sequencer plugin, tracer, tracer-constraints, linea-besu, or linea-besu-package changed.

**New contributor tip:** If your PR only touches markdown or docs, every job after this point except CodeQL is skipped. This is intentional.

---

### 2. `check-and-tag-images` (reuse-check-images-tags-and-push.yml)

**What it does:** For each Docker image (`coordinator`, `prover`, `postman`, `transaction-exclusion-api`, `native-yield-automation-service`, `lido-governance-monitor`), checks whether an image tagged with the current commit SHA already exists in the registry. If it does, that image is reused in all subsequent steps. If not, the image tag is reserved for later.

**Outputs:** `commit_tag`, `develop_tag`, and per-image `image_tagged_*` flags that tell `docker-build` whether to skip rebuilding a particular image.

**Why:** Avoids redundant builds when multiple workflow runs reference the same commit.

---

### 3. `code-analysis` (codeql.yml)

Runs CodeQL static analysis across Go, Java/Kotlin, JavaScript/TypeScript, Python, and GitHub Actions. Always runs regardless of what changed.

---

### 4. `testing` (testing.yml)

**Condition:** `has-changes-requiring-build == 'true'`.

Fans out to a set of per-component test workflows in parallel. Each receives a boolean flag from step 1 and skips itself if that component did not change:

| Job | Workflow |
|-----|----------|
| coordinator | `coordinator-testing.yml` |
| prover | `prover-testing.yml` |
| postman | `postman-testing.yml` |
| smart-contracts | `run-smc-tests.yml` |
| transaction-exclusion-api | `transaction-exclusion-api-testing.yml` |
| linea-sequencer | `linea-sequencer-plugin-testing.yml` |
| tracer-constraints | `tracer-constraints-check-compilation.yml` |
| native-yield-automation-service | `native-yield-automation-service-testing.yml` |
| lido-governance-monitor | `lido-governance-monitor-testing.yml` |
| staterecovery | disabled (pending fixes) |

After all component jobs finish, a `jacoco-report` job aggregates JVM coverage data (`coordinator` + `linea-sequencer` + `staterecovery` + `transaction-exclusion-api`) and uploads to Codecov under the `kotlin` flag.

When the postman job runs, `postman-testing.yml` uploads Jest LCOV for `ts-libs/linea-native-libs/`, `postman/`, and `sdk/sdk-viem/` (artifact `ts-coverage-*` for forks). Staging uses a non-hidden `ci-ts-coverage/` directory because `upload-artifact@v4` excludes hidden paths by default.

The native-yield and lido jobs upload LCOV for `ts-libs/linea-shared-utils/` plus their service packages (`native-yield-automation-service`, `lido-governance-monitor`) with matching artifacts for forks. The standalone `sdk-testing` workflow uploads `sdk-core`, `sdk-viem`, and `sdk-ethers` LCOV and an `sdk-ts-coverage-*` artifact. Tracer unit tests (`reusable-tracer-unit-tests.yml`, via `tracer-gradle-tests.yml` / release workflow) upload Jacoco XML under the `tracer` flag (separate from merged JVM `kotlin`). [`.github/workflows/codecov-external-pr.yml`](.github/workflows/codecov-external-pr.yml) listens for `main` and `sdk-testing` completion to re-upload fork PR coverage.

---

### 5. `get-has-changes-requiring-e2e-testing` (get-has-changes-requiring-e2e-testing.yml)

Runs a stricter path filter: changed files must match `coordinator/**/src/main/**` or `jvm-libs/**/src/main/**` (production source, not tests). Only coordinator production-source changes justify the cost of spinning up the full protocol stack for E2E. The output `has-changes-requiring-e2e-testing` drives whether docker-build and E2E actually execute.

---

### 6. `manual-docker-build-and-e2e-tests` (gate step)

**On PRs only:** This job targets the `docker-build-and-e2e` GitHub Environment, which has a required reviewer configured. A maintainer must manually approve this step before Docker images are built and E2E tests run. This prevents untrusted or expensive workflows from running automatically on every PR commit.

**On `main`:** No environment is set, so this step is a no-op pass-through.

---

### 7. `docker-build` (build-and-publish.yml)

**Condition:** `has-changes-requiring-e2e-testing == 'true'` (gate step must have passed).

Builds Docker images for all changed components. Also builds the `linea-besu-package` if `has-changes-requiring-linea-besu-package-build` is true. At this stage, images are **not pushed** to the registry (`push_image` defaults to `false`).

---

### 8. `run-e2e-tests` (reuse-run-e2e-tests.yml)

Starts the full local Linea stack (L1 + L2 + coordinator + prover + postman + ...) using the images from step 7 and runs the protocol E2E test suite. Dumps logs on completion.

When `has-changes-requiring-e2e-testing == 'false'`, this job still runs but passes immediately - this keeps required status checks green on every PR.

---

### 9. Publish (branch-dependent)

| Condition | Job | What happens |
|-----------|-----|--------------|
| PR + same-repo + E2E passed | `publish-linea-besu-after-run-tests-success` | Publishes linea-besu artifacts |
| Push to `main` + testing passed + E2E passed | `publish-images-after-run-tests-success-on-main` | Re-runs `build-and-publish.yml` with `push_image: true`, tagging all images with `commit_tag` and `develop_tag` in the registry |

Images are **never pushed** from fork PRs or from PRs where tests did not pass.

---

### 10. `cleanup-deployments`

Removes the `docker-build-and-e2e` deployment record from GitHub's Deployments UI after the run completes (success or failure). Runs for all non-fork PRs.

---

### 11. `notify`

Sends a Slack alert to the engineering alerts channel if any job in the pipeline failed, but only on `main`. PRs do not generate Slack alerts.

---

## Key Patterns to Understand

### Path filtering drives everything

The filter step is the most important job in the pipeline. It determines what runs, what is skipped, and which images get built. Because each component's filter also watches the workflow files that test it (`.github/workflows/coordinator-*.yml`, etc.), changing a workflow file itself re-triggers the relevant test jobs.

### Docs-only changes are nearly free

A PR that only touches `*.md`, `docs/**`, or `contracts/token-generation-event/**` sets `has-changes-requiring-build=false`. This skips testing, docker-build, and E2E. Only CodeQL runs.

### Images are reused across runs

The `check-and-tag-images` step checks the registry before building. If a commit's image already exists (e.g., from a previous run), the build is skipped and the existing image is used for E2E. This makes re-runs cheap.

### E2E is gated twice

1. **Path gate** - only coordinator or jvm-libs production source changes trigger E2E.
2. **Manual approval gate** - on PRs, a maintainer must approve before docker-build and E2E proceed.

### Images are never pushed on PRs

Docker images are built during PR CI so they can be used for E2E, but they are only pushed to the registry when a merge to `main` passes both testing and E2E.

### Concurrency policy

- On PRs: runs are cancelled in-progress when a new commit is pushed (saves runner cost).
- On `main`: runs are never cancelled; all runs complete in order.

---

## Reusable Workflow Naming Conventions

| Prefix | Pattern | Purpose |
|--------|---------|---------|
| `reuse-` | `reuse-*.yml` | Called only from `main.yml` or other orchestrators; not triggered directly |
| `<component>-testing` | e.g. `coordinator-testing.yml` | Component-specific test suite, called by `testing.yml` |
| `<component>-*.yml` | various | Other per-component workflows (releases, etc.) |
| `build-and-publish.yml` | - | Docker build and optional push, parameterized by changed flags |
