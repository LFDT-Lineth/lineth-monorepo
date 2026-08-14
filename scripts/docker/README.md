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
