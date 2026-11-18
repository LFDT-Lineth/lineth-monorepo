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

Maru is built as part of the monorepo Gradle build. From the repository root:

```sh
# Create a distribution ready to run
./gradlew :maru:app:installDist
```

### Validating a full build

- Run unit and integration tests and checks: `./gradlew -p maru build` (several minutes).
- If Spotless reports drift: `./gradlew -p maru spotlessApply`, then `./gradlew -p maru spotlessCheck`.
- After changing `app/Dockerfile` or the Java baseline, confirm the image still builds (see **Build Docker Image Locally** below).

After building, you can run Maru using:

```sh
./maru/app/build/install/app/bin/app [options]
```

The distribution will be created in `maru/app/build/install/app/` with all necessary dependencies included.

## Validator signing

QBFT validator signing defaults to the local key at `persistence.private-key-path`:

```toml
[qbft]
fee-recipient = "0x0000000000000000000000000000000000000000"
signer-type = "local"
```

An application embedding Maru can inject a `CustomValidatorSignerFactory` that resolves a backend-neutral
`CloseableSigner` by logical name:

```toml
[qbft]
fee-recipient = "0x0000000000000000000000000000000000000000"
signer-type = "custom"
signer-name = "maru-validator"
```

With a custom signer, `persistence.private-key-path` remains the P2P and discovery identity only. The custom public key
determines the QBFT validator address, and startup fails if that address is absent from every configured validator set.
Maru never passes the P2P private key to the custom signer factory and never falls back to the local validator key.
The app owns the returned signer and closes it during shutdown or when startup fails.

## Switching a pre-merge network to PoS (TTD)

A network whose execution layer (EL) starts with its own proof-of-authority consensus can switch to Maru-driven PoS
once the EL reaches its terminal total difficulty (TTD). Configure it with a `difficultyAwareQbft` fork in the Maru
genesis file:

```json
"0": {
  "type": "difficultyAwareQbft",
  "blockTimeSeconds": 1,
  "terminalTotalDifficulty": 49575263,
  "postTtdConfig": {
    "validatorSet": ["0x9f31730181441beb67b10efaed5773767ea959bc"],
    "elFork": "Paris"
  }
}
```

EL clients no longer return `totalDifficulty` in `eth_getBlockBy*` responses (e.g. Besu >= 26.9.0, Geth >= 1.14.11),
so Maru determines the terminal block itself. It reads the difficulty of the genesis block (`g`, block 0) and of
block 1 (`d`), assumes every block after genesis has difficulty `d`, and switches to PoS once the EL head reaches the
first block `n` where `g + n * d >= terminalTotalDifficulty`.

### Caveats

- Only pre-merge chains where every block after genesis has the same difficulty are supported:
  - QBFT / IBFT 2.0: every block has difficulty 1.
  - Clique with a single signer that never changes: every block is in-turn with difficulty 2.
- Not supported: Clique with more than one signer (out-of-turn blocks have difficulty 1), consensus migrations that
  change the block difficulty, and proof-of-work chains. With such chains the computed terminal block can differ from
  the EL's, so the network either stalls until Maru switches or Maru switches before the EL accepts PoS blocks.
- The pre-merge chain is assumed not to reorg, which holds for QBFT / IBFT 2.0 (immediate finality) and in practice
  for single-signer Clique.
- `terminalTotalDifficulty` must match the TTD in the EL genesis file, as the EL stops producing pre-merge blocks at
  its own TTD.
- Maru logs an error if the EL head has a difficulty other than `d` (or 0 once post-merge), as this breaks the
  constant difficulty assumption, but still switches at the computed terminal block.
- If block 1 is already post-merge (difficulty 0) while the genesis difficulty is below `terminalTotalDifficulty`,
  Maru and EL TTDs do not match: Maru does not switch and logs an error on every poll.

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
