# Linea zkEVM

<a href="https://x.com/LineaBuild">
  <img src="https://img.shields.io/twitter/follow/LineaBuild?style=for-the-badge" alt="X (formerly Twitter) Follow" height="20">
</a>
<a href="https://github.com/Consensys/linea-monorepo/blob/main/LICENSE-APACHE">
  <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="Apache 2.0 License" height="20">
</a>
<a href="https://github.com/Consensys/linea-monorepo/blob/main/LICENSE-MIT">
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="MIT License" height="20">
</a>
<a href="https://codecov.io/gh/Consensys/linea-monorepo">
  <img src="https://codecov.io/gh/Consensys/linea-monorepo/graph/badge.svg?token=2TM55P0CGJ" alt="Codecov" height="20">
</a>

This is the principal Linea repository. It mainly includes the smart contracts covering Linea's core functions, the prover in charge of generating ZK proofs, the coordinator responsible for multiple orchestrations, and the Postman to execute bridge messages.

It serves developers by making the Linea tech stack open source under the [Apache 2.0](LICENSE-APACHE) and the [MIT](LICENSE-MIT) licenses.

## What is Linea?

[Linea](https://linea.build) is a developer-ready layer 2 network scaling Ethereum. It's secured with a zero-knowledge rollup, built on lattice-based cryptography, and powered by [Consensys](https://consensys.io).

Linea is compatible with the execution clients [Besu](https://github.com/besu-eth/besu/) or [Geth](https://github.com/ethereum/go-ethereum). To run a full node, an execution client is paired with the consensus client [Maru](https://github.com/Consensys/maru).

## Get started

If you already have an understanding of the tech stack, use our [Get Started](docs/get-started.md) guide.

For developers looking to build services locally (such as, the coordinator), see our detailed [Local Development Guide](docs/local-development-guide.md).

## Agent Documentation

For AI coding agents and developer tools:

- Canonical instructions: [AGENTS.md](AGENTS.md)
- Claude Code entry point: [CLAUDE.md](CLAUDE.md)
- GitHub Copilot entry point: [.github/copilot-instructions.md](.github/copilot-instructions.md)
- Cursor documentation index: [.cursor/rules/documentation.mdc](.cursor/rules/documentation.mdc)
- Cursor review/rule set: [.cursor/BUGBOT.md](.cursor/BUGBOT.md)

## Release workflows

Releases are driven by GitHub Actions workflows under `.github/workflows`. There are two flavors: **per-component releases** and **milestone releases**.

### Release tag and version

Release tag of each component is in the format of `releases/[component]/v[semver]` and the semver version is computed from the relevant Git history commit messages [using Conventional Commits format](#commit-message-format) by using [git-cliff](https://github.com/orhun/git-cliff)

### Per-component release

Each component has its own release workflow. Run the one that matches the component you want to ship:

| Component        | Workflow                                         | Release tag pattern              |
| ---------------- | ------------------------------------------------ | -------------------------------- |
| linea-besu       | [.github/workflows/linea-besu-release.yml](https://github.com/Consensys/linea-monorepo/actions/workflows/linea-besu-release.yml)       | `releases/linea-besu-package/v[semver]` |
| coordinator      | [.github/workflows/coordinator-release.yml](https://github.com/Consensys/linea-monorepo/actions/workflows/coordinator-release.yml)      | `releases/coordinator/v[semver]`        |
| maru             | [.github/workflows/maru-release-manual.yml](https://github.com/Consensys/linea-monorepo/actions/workflows/maru-release-manual.yml)      | `releases/maru/v[semver]`        |
| postman          | [.github/workflows/postman-release.yml](https://github.com/Consensys/linea-monorepo/actions/workflows/postman-release.yml)          | `releases/postman/v[semver]`            |
| prover           | [.github/workflows/prover-release.yml](https://github.com/Consensys/linea-monorepo/actions/workflows/prover-release.yml)           | `releases/prover/v[semver]`             |
| tx-exclusion-api | [.github/workflows/tx-exclusion-api-release.yml](https://github.com/Consensys/linea-monorepo/actions/workflows/tx-exclusion-api-release.yml) | `releases/tx-exclusion-api/v[semver]`   |

Notes:

- **Branches.** A per-component release can be cut from either `main` or a feature branch (e.g. for a hot-fix release).
- **Feature-branch restriction.** When the workflow is run from a feature branch, `release_tag_suffix` is **required** (e.g. producing `releases/coordinator/v1.2.3-hotfix`). Without a suffix the new tag could collide with tags produced from other branches.
- **Docker image suffix.** `image_tag_suffix` is **optional**.
- **GitHub Release page.** Each successful run publishes a GitHub Release containing the updated component `CHANGELOG.md` and the docker image pull instructions.

### Milestone release

Milestone releases bundle every component into a single Linea release.

- **Workflow:** [.github/workflows/linea-milestone-release-with-dry-run.yml](https://github.com/Consensys/linea-monorepo/actions/workflows/linea-milestone-release-with-dry-run.yml)
- **Release tag pattern:** `releases/linea/v[semver]`
- **Branch:** can only be run from `main`.

#### Unified-cut behavior

For each component, the milestone workflow decides between two paths based on whether the component's release version has bumped at the milestone commit:

- **Bumped → release the component.** A new per-component release is cut as part of the milestone (new tag, docker image, GitHub Release page).
- **Not bumped → do nothing.** No new component release is cut. The existing docker image associated with the component's latest release tag will be shown on the milestone release page.

The milestone GitHub Release page aggregates the `CHANGELOG` entries from every component (newly released or carried over) and lists their docker image pull instructions.

#### Dry-run on a temporary branch before releasing on main

The milestone workflow defaults to running a **dry run on a temporary branch forked from the latest `main`** before touching `main` itself. This catches issues (e.g. failing e2e, docker push permission gaps, changelog generation errors) without leaving any artifacts on `main`. The job graph is:

1. **`create-temp-branch-and-dispatch-release`** — forks a new branch `ci/milestone-dry-run-<timestamp>` from `origin/main` and dispatches the `linea-milestone-release.yml` workflow against it. All artifacts (i.e. tags and GitHub releases) produced in the dispatched run will be thrown away later in later workflow.
2. **`manual-run-release-on-main`** — guarded by the `run-release-on-main` GitHub Environment, this job blocks on **manual approval** in the GitHub UI. Reviewers should:
   - Open the dispatched milestone run (URL is printed in the kickoff job's summary) and confirm every job (build, e2e, per-component gh-release, milestone gh-release) succeeded against the temp branch.
   - Spot-check the draft GitHub Releases that the dry run produced (they are clearly suffixed with `-dry-run-<timestamp>`).
   - Approve the environment gate to proceed with the real release on `main`, or reject to abort.
3. **`dry-run-release-cleanup`** — runs after the manual gate is approved. It waits for the dispatched dry-run to reach a terminal state and then removes every artifact the dry run created:
   - **GitHub releases + git tags** — enumerates tags reachable from the temp branch but not from `origin/main` and removed all of them with their associated releases.
   - **Temp branch** — `git push origin --delete ci/milestone-dry-run-<timestamp>`.
   - **Please note that no images will be pushed to Docker Hub during dry run release.**
4. **`milestone-release-on-main`** — only runs after the manual gate is approved. Calls `linea-milestone-release.yml` against `main`.

## Looking for the Linea code?

Linea's stack is made up of multiple repositories, these include:

- This repo, [linea-monorepo](https://github.com/Consensys/linea-monorepo): The main repository for the Linea stack & network
> Also maintains a set of Linea-Besu plugins for the sequencer and RPC nodes.
- [linea-besu-upstream](https://github.com/Consensys/linea-besu-upstream/): Besu build configured for Linea
- [linea-tracer](https://github.com/Consensys/linea-tracer): Linea-Besu plugin which produces the traces that the constraint system applies and that serve as inputs to the prover
- [linea-constraints](https://github.com/Consensys/linea-constraints): Implementation of the constraint system from the specification
- [linea-specification](https://github.com/Consensys/linea-specification): Specification of the constraint system defining Linea's zkEVM

Linea abstracts away the complexity of this technical architecture to allow developers to:

- [Bridge tokens](https://docs.linea.build/developers/guides/bridge)
- [Deploy a contract](https://docs.linea.build/developers/quickstart/deploy-smart-contract)
- [Run a node](https://docs.linea.build/developers/guides/run-a-node)

... and more.

## How to contribute

Contributions are welcome!

### Guidelines for non-code and other trivial contributions

Please keep in mind that we do not accept non-code contributions like fixing comments, typos or some other trivial fixes. Although we appreciate the extra help, managing lots of these small contributions is unfeasible, and puts extra pressure in our continuous delivery systems (running all tests, etc). Feel free to open an issue pointing to any of those errors, and we will batch them into a single change.

1. [Create an issue](https://github.com/Consensys/linea-monorepo/issues)
> If the proposed update is non-trivial, also tag us for discussion.
2. Submit the update as a pull request from your [fork of this repo](https://github.com/Consensys/linea-monorepo/fork), and tag us for review.
> Include the issue number in the pull request description and (optionally) in the branch name.

Consider starting with a ["good first issue"](https://github.com/Consensys/linea-monorepo/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).

### Commit message format

All commits must follow the [Conventional Commits](https://www.conventionalcommits.org) format, enforced locally by a Husky `commit-msg` hook:

```
<type>(<scope>): <short description>

[optional body]

[optional footer: Closes #<issue>, BREAKING CHANGE: ...]
```

**Allowed types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`, `revert`, `build`

**Required scope** (one or multiple of):

| Scope | Area |
|---|---|
| `coordinator` | Coordinator service |
| `maru` | Maru consensus client |
| `prover` | Prover |
| `prover-ray` | Prover Ray (RISC-V) |
| `postman` | Message bridging and executor |
| `tx-exclusion-api` | Transaction exclusion API |
| `linea-besu` | Linea-Besu package & plugins |
| `tracer` | Tracer |
| `sequencer` | Sequencer |
| `state-recovery` | State recovery |
| `contracts` | Smart contracts |
| `sdk-core` / `sdk-ethers` / `sdk-viem` | SDKs |
| `jvm-libs` | JVM shared libraries |
| `blob-libs` | Blob libraries |
| `e2e` | End-to-end tests |
| `ci` | CI/CD workflows |
| `docker` | Docker / compose |
| `deps` | Dependency updates |
| `misc` | For anything that does not have impact on deliverable, e.g docs, configs, AI agents, etc |

**Examples:**
```
feat(coordinator): add retry logic for L1 message sending

Retries up to 3 times on transient network errors before failing.

Closes #456
```

```
chore(coordinator,sequencer,tracer,tx-exclusion-api): update to java 25
```

To write a single-line breaking change commit from the terminal:
```bash
git commit -m 'feat(coordinator)!: breaking changes'
```

To write a multi-line commit from the terminal:
```bash
git commit -m $'feat(coordinator): add retry logic\n\nRetries up to 3 times on transient network errors.\n\nCloses issue# 123'
```

Before contributing, ensure you're familiar with:

- Our [contribution guide](docs/contribute.md)
- Our [code of conduct](docs/code-of-conduct.md)
- The [Besu contribution guide](https://wiki.hyperledger.org/display/BESU/Coding+Conventions), for Besu:Linea related contributions
- Our [security policy](docs/security.md)

### PR title

**PR titles must follow the same [Conventional Commits](https://www.conventionalcommits.org) format as commit messages** (see [Commit message format](#commit-message-format) above for the allowed `<type>(<scope>): <short description>` shape, types, and scopes).

This matters because PRs are **squash-merged** into `main`: GitHub uses the PR title as the single resulting commit message on `main`. Our release tooling — [git-cliff](https://github.com/orhun/git-cliff), which drives automated version bumps and `CHANGELOG.md` generation in the [release workflows](#release-workflows) — parses those commit messages to decide the next semver bump and to categorize entries in the changelog. A non-conforming PR title turns into a non-conforming commit on `main`, which means:

- **No automatic version bump** for the affected component (git-cliff will skip it).
- **Missing or miscategorized changelog entry** on the next release.

Examples of good PR titles:

```
feat(coordinator): add retry logic for L1 message sending
fix(prover): correct integer overflow in trace builder
chore(linea-besu,sequencer): bump dependency versions
feat(coordinator)!: rename public API method (BREAKING CHANGE)
```

Please note that the linea-monorepo GitHub [CI](https://github.com/Consensys/linea-monorepo/actions/workflows/main.yml) will lint the PR title when new commits pushed to the PR branch, and the whole CI will fail if the PR title doesn't conform.

### Useful links

- [Linea docs](https://docs.linea.build) managed from a [public repo](https://github.com/Consensys/doc.linea)
- [Linea blog](https://linea.mirror.xyz)
- [Support](https://support.linea.build)
- [X](https://x.com/LineaBuild)

