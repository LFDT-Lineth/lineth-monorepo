# Maru
A consensus layer client implementing QBFT protocol adhering to Eth 2.0 CL / EL separation and API

## Requirements

- Java 25+
- Make 3.81

## Quick Start

```sh
docker-run-stack
```

## Build from sources

Use **JDK 25** (same major version as CI). Check with `java -version`.

To build Maru from source code:

```sh
# Create a distribution ready to run
./gradlew :app:installDist
```

### Validating a full build

- Run unit and integration tests and checks: `./gradlew build` (several minutes).
- If Spotless reports drift: `./gradlew spotlessApply`, then `./gradlew spotlessCheck`.
- After changing `app/Dockerfile` or the Java baseline, confirm the image still builds (see **Build Docker Image Locally** below).

After building, you can run Maru using:

```sh
./app/build/install/app/bin/app [options]
```

The distribution will be created in `app/build/install/app/` with all necessary dependencies included.

### Build Docker Image Locally

```sh
docker-build-local-image
MARU_TAG=local make docker-run-stack
```

### Contribution
* Please stick to https://www.conventionalcommits.org/en/v1.0.0/#specification for the ease of changelog maintenance
when creating PRs

### Release process
Release process is not automated at this time, so this section aims to help to streamline it

⚠️To speed up the hotfixes, release process doesn't enforce tests. Always ensure the tests are passing, before
releasing a new version of Maru! ⚠️

* Each released version follows the template `<semver>-<product-label-ver>?` e.g. `v2.0.1-betav4` OR `v2.0.1`.
* Check the changelog for the changes. Cheatsheet:
  * ! / BREAKING CHANGE = major version update
  * feat = minor version update
  * fix = patch version update
* MAJOR.MINOR.PATCH should be incremented manually. Changelog and conventional commits are aimed to help to make it easy
* Tag the latest commit on main with the new version, like `git tag v2.0.1` or `v2.0.1-major-upgrade`
* Push tag to main with git push --tags
* This will trigger a release draft with the distributive artifact created, and it will push a new docker image to
Dockerhub. `-<date>-<commit-hash>` suffix is added to the actual docker image and archive distribution like
  `v2.0.1-betav4-20251027155452-cd25bfd`
* Changelog will be pulled automatically into the release description. Review it and publish the release
* Make a PR to clean up the changelog



