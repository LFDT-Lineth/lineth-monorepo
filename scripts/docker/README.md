# Docker image builds

The Docker images published by CI are built by a single script,
[`build-image.sh`](./build-image.sh), which is called from two places:

| Caller | Entry point |
|--------|-------------|
| CI | [`.github/actions/docker-build-publish/action.yml`](../../.github/actions/docker-build-publish/action.yml), used by `.github/workflows/<image>-build-and-publish.yml` |
| Local | `make docker-build-<image>` ([`images.mk`](./images.mk), included from the root `Makefile`) |
| Package-local | `make -C maru docker-build-local-image` and `make -C linea-besu/package build-image`, which delegate here so their documented flags (`BESU_PACKAGE_TAG`, `PLATFORM`, …) keep working |

Both paths produce the same `docker buildx build` command line, so a local build
reproduces what the pipeline does instead of approximating it.

## Local usage

```bash
make docker-build-list                                # available targets
make docker-build-coordinator                         # -> consensys/linea-coordinator:local
make docker-build-coordinator DOCKER_IMAGE_TAG=mytag
make docker-build-prover DRY_RUN=true                 # print the buildx command, build nothing
make docker-build-maru SKIP_PREBUILD=true             # reuse the gradle dist from a previous run
make docker-build-all
```

Each `docker-build-<image>` target mirrors the corresponding workflow: same
pre-build step (`./gradlew …:installDist` where the workflow has one), same
Dockerfile, context, build args and named build contexts.

### Shared zkEVM / RISC-V local stack

From the repository root, with the required Java/Node/pnpm versions and dependencies installed:

```bash
make start-env-with-tracing-v2 # zkEVM
make start-env-with-riscv      # RISC-V from genesis, with dummy execution proofs
```

Both targets use the **same environment**: Compose project, L1 Besu/Teku, Postgres,
networks, service names and persistent state. By default either target **resets that
shared chain, database, deployment metadata and proof files**. The RISC-V target first
builds Besu, Maru and the coordinator for the Docker host, reusing build caches;
a failed build leaves the existing environment intact.

The RISC-V smoke scenario starts Amsterdam at genesis, deploys the V9 rollup stub,
and produces execution requests and dummy responses. Real proving and L1 proof
submission are disabled. Restarting its coordinator can replay blocks because
L1 finalization is disabled.

- L1 RPC: `localhost:8445`
- L2 RPC: `localhost:8545`
- Coordinator health: `localhost:9545/health`
- Execution requests and responses: `tmp/local/prover/riscv/execution/`

Restart the same scenario without resetting state or redeploying contracts:

```bash
make start-env-with-riscv CLEAN_PREVIOUS_ENV=false SKIP_CONTRACTS_DEPLOYMENT=true
# The same flags work with start-env-with-tracing-v2.

COMPOSE_PROFILES=l1,l2,riscv docker compose \
  -f docker/compose-tracing-v2.yml -f docker/compose-riscv.yml \
  -f docker/compose-riscv-from-genesis.yml logs -f coordinator
make clean-environment # clears shared state for either mode
```

`clean-riscv-environment` is an alias for `clean-environment`. L1 EL/CL, sequencer,
Maru and Postgres data survive container recreation in shared named volumes.
Genesis initialization reuses complete existing files and fails on incomplete
state or an attempt to substitute the other smoke scenario's genesis.

#### Preparing transition tests

`compose-riscv.yml` is the reusable extension of `compose-tracing-v2.yml`: it adds
RISC-V prover transport configuration while
retaining the zkEVM services, execution clients, genesis, contract address and L1
submission settings. The dummy responder is opt-in via the `riscv` profile.
`compose-riscv-from-genesis.yml` contains the smoke-only execution clients,
Amsterdam-at-genesis settings and disabled submissions; omit it for transition tests.

A transition scenario should start with zkEVM, then use the common layers and its
own final override through `make start-env COMPOSE_FILE="..."`, with
`CLEAN_PREVIOUS_ENV=false SKIP_CONTRACTS_DEPLOYMENT=true`. It must retain L1/L2 chain
history, deployed contracts and the coordinator database, schedule a consistent
fork/cutover across Besu, Maru and coordinator, migrate the zkEVM FOREST database
before using the RISC-V BONSAI client, and upgrade the existing rollup contract.
Select compatible coordinator images explicitly (for example, build locally and
use `LINEA_COORDINATOR_TAG=local-riscv` in both stages); the common overlay preserves
the base image selection. Upgrading an existing coordinator database also requires
compatible migration history; changing image tags alone does not guarantee this.
Switching to the from-genesis target is **not** a transition test. This PR supplies
the shared harness; actual fork activation, contract migration, real proving and
finalization require a dedicated transition scenario.

Check configuration, initialization and startup/restart behavior without launching
containers (Docker Compose is required):

```bash
node --test scripts/docker/shared-stack.test.mjs
```

### linea-besu-package

`make docker-build-linea-besu-package` is the slowest target by a wide margin: its
pre-build chain compiles Besu from source and builds the tracer and sequencer
plugins, so expect `docker-build-all` to take a long time because of it. It runs
the same chain as CI's
`.github/actions/linea-besu-package/build-plugins-and-assemble`
(`build-besu` → `build-tracer-and-sequencer` → `clean` → `assemble` in
`linea-besu/package/Makefile`) and produces `consensys/linea-besu-package:local`,
the tag `docker/compose-*.yml` expects via `LINEA_BESU_PACKAGE_TAG`.

Once assembled, iterate on the image alone with `SKIP_PREBUILD=true`. Two
deliberate deviations from CI: the build context stays at `linea-besu/package/tmp`
instead of CI's `linea-besu/package/linea-besu/.` (identical image — the Dockerfile
only does `COPY besu /opt/besu/` — but all generated files stay inside `tmp/`,
which `make -C linea-besu/package clean` removes), and the `-with-fleet` variant is
not reproduced because it needs a token for the private `Consensys/besu-fleet-plugin`
repository.

### Variables

| Variable | Default | Meaning |
|----------|---------|---------|
| `DOCKER_IMAGE_TAG` | `local` | Tag applied to the built image |
| `DOCKER_BUILDER` | `linea-local` | Buildx builder to use; created on demand with the `docker-container` driver, matching CI. Set to empty to use your current builder |
| `PLATFORMS` | *(empty)* | Comma-separated platforms; empty means `linux/amd64`, as in the CI test build |
| `REGISTRY_CACHE` | `false` | `true` imports `<image>:buildcache-amd64` from Docker Hub like CI does. Off by default so local builds need no network or credentials |
| `DRY_RUN` | `false` | `true` prints the commands instead of running them, and skips the pre-build step. A missing Dockerfile or build context is a warning rather than an error, so you can inspect the command before anything is assembled |
| `SKIP_PREBUILD` | `false` | `true` skips the gradle/dist step |
| `NODE_VERSION` | from `.nvmrc` | Passed to the Node-based images, like `.github/actions/get-node-version` does |
| `LINEA_BESU_VCS_REF` | `git rev-parse HEAD` | `VCS_REF` build arg for `linea-besu-package` (CI passes `github.sha`) |
| `LINEA_BESU_BUILD_DATE` | today, UTC | `BUILD_DATE` build arg for `linea-besu-package` |

### Simulating a multi-arch publish build

```bash
make docker-build-coordinator PLATFORMS=linux/amd64,linux/arm64
```

A multi-platform build cannot be loaded into the local image store, so the result
stays in the build cache — enough to verify that the image builds for `arm64`.
The script warns about this. QEMU must be available (Docker Desktop ships it;
on Linux run `docker run --privileged --rm tonistiigi/binfmt --install arm64`).

## Behaviour reproduced from CI

* **Tags** — `--tags` takes a comma-separated list. Entries are trimmed,
  de-duplicated (first occurrence wins) and an empty result is a hard error. The
  first entry is the *primary* tag: it is the only one applied to a local
  (`--load`) build and the one `--save-to` exports.
* **Push vs local** — without `--push` the build targets `linux/amd64`, tags the
  primary tag only and `--load`s it. With `--push` it targets
  `linux/amd64,linux/arm64` (unless `--platforms` says otherwise) and pushes
  every tag.
* **Cache** — registry cache at `<image>:buildcache-amd64` / `-arm64`; imported
  on both paths and exported (`mode=max`) only when pushing. Disabled for runs
  without registry credentials: fork pull requests and dependabot.
* **Metadata** — `VERSION` (primary tag), `VCS_REF` (`GITHUB_SHA`, else git
  `HEAD`) and `BUILD_DATE` (RFC 3339, UTC) are injected as build args on every
  build. Each first-party Dockerfile declares those three `ARG`s and turns them
  into `org.label-schema.*` labels, so images are traceable to a commit. A
  caller-supplied `--build-arg` of the same name still wins.
* **Artifacts** — `--save-to FILE` runs `docker save … | gzip` on the primary
  image, which CI uploads as a workflow artifact.

The GitHub-specific parts stay in the composite action: QEMU/buildx setup,
appending `develop_tag` on `main`, credential-less run detection and artifact
upload.

## Adding an image

1. Add the workflow under `.github/workflows/`, calling
   `./.github/actions/docker-build-publish`.
2. Add a matching `docker-build-<name>` target in [`images.mk`](./images.mk)
   with the same Dockerfile, context, build args and build contexts, and append
   it to `DOCKER_IMAGE_TARGETS`.

The workflow and the make target hold the per-image recipe independently — when
you change one, change the other.
