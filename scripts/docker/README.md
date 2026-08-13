# Docker image builds

The Docker images published by CI are built by a single script,
[`build-image.sh`](./build-image.sh), which is called from two places:

| Caller | Entry point |
|--------|-------------|
| CI | [`.github/actions/docker-build-publish/action.yml`](../../.github/actions/docker-build-publish/action.yml), used by `.github/workflows/<image>-build-and-publish.yml` |
| Local | `make docker-build-<image>` ([`images.mk`](./images.mk), included from the root `Makefile`) |

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

### Variables

| Variable | Default | Meaning |
|----------|---------|---------|
| `DOCKER_IMAGE_TAG` | `local` | Tag applied to the built image |
| `DOCKER_BUILDER` | `linea-local` | Buildx builder to use; created on demand with the `docker-container` driver, matching CI. Set to empty to use your current builder |
| `PLATFORMS` | *(empty)* | Comma-separated platforms; empty means `linux/amd64`, as in the CI test build |
| `REGISTRY_CACHE` | `false` | `true` imports `<image>:buildcache-amd64` from Docker Hub like CI does. Off by default so local builds need no network or credentials |
| `DRY_RUN` | `false` | `true` prints the commands instead of running them |
| `SKIP_PREBUILD` | `false` | `true` skips the gradle/dist step |
| `NODE_VERSION` | from `.nvmrc` | Passed to the Node-based images, like `.github/actions/get-node-version` does |

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
  on both paths and exported (`mode=max`) only when pushing. Disabled for fork
  pull requests, which have no registry credentials.
* **Artifacts** — `--save-to FILE` runs `docker save … | gzip` on the primary
  image, which CI uploads as a workflow artifact.

The GitHub-specific parts stay in the composite action: QEMU/buildx setup,
appending `develop_tag` on `main`, fork detection and artifact upload.

## Adding an image

1. Add the workflow under `.github/workflows/`, calling
   `./.github/actions/docker-build-publish`.
2. Add a matching `docker-build-<name>` target in [`images.mk`](./images.mk)
   with the same Dockerfile, context, build args and build contexts, and append
   it to `DOCKER_IMAGE_TARGETS`.

The workflow and the make target hold the per-image recipe independently — when
you change one, change the other.
