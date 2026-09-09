# RISC-V local stack

## Prerequisites

Install the repository prerequisites from [get-started.md](get-started.md).
The stack uses the Besu commit pinned in `gradle/libs.versions.toml` and requires
Maru Amsterdam support (PR #3939 while it is unmerged). Build all three images
from this checkout so the Besu runtime and Linea plugins use the same commit:

```bash
PATH="$(go env GOPATH)/bin:$PATH" make docker-build-linea-besu-package DOCKER_IMAGE_TAG=pr3929
make docker-build-maru DOCKER_IMAGE_TAG=pr3929
make docker-build-coordinator DOCKER_IMAGE_TAG=pr3929
```

On an ARM host, pass `PLATFORMS=linux/arm64` to each build and export
`RISCV_PLATFORM=linux/arm64` when starting the stack.

`debug_executionWitness` requires Amsterdam block access lists and Bonsai state
storage. This stack enables both, sets the block gas limit to 30 million, and
creates its Amsterdam genesis files under `tmp/riscv/genesis`. The shared Osaka
genesis templates remain unchanged.

## Start

```bash
LINEA_BESU_PACKAGE_TAG=pr3929 MARU_TAG=pr3929 LINEA_COORDINATOR_TAG=pr3929 \
  make start-env-with-riscv
```

The RISC-V stack omits the legacy tracer-dependent `LineaTransactionSelectorPlugin`
and `LineaEstimateGasEndpointPlugin`. Besu handles transaction selection and provides
`eth_estimateGas`; `linea_estimateGas` and the legacy Linea selection limits and
profitability checks are not enabled.

The generated genesis uses no-op builder deposit/exit contracts for local testing.
It does not exercise Amsterdam builder requests.

This starts a development responder that returns structurally valid execution
proof responses without proving them. It can be replaced with an R5 prover once
a compatible image is available.

## Diagnostics

```bash
docker compose -p linea-riscv-dev -f docker/compose-riscv.yml ps
docker compose -p linea-riscv-dev -f docker/compose-riscv.yml logs coordinator riscv-proof-responder
find tmp/riscv/prover/riscv -type f -print
```

## E2E gaps observed on 2026-09-09

Tested with Besu `83d79591676a5a8086fbae6f3ed586a328e989d1`, PR #3939's
Maru implementation, and locally built Linea plugins. Compilation and Docker
startup, transaction inclusion and
`debug_executionWitness` succeed, and the coordinator creates execution requests
and consumes development proof responses.

Before enabling real E2E proving:

- Preserve Amsterdam data through the coordinator's block model and prover DTOs.
  Observed requests omit `slotNumber` and contain `blockAccessList: "0x"`, while
  the corresponding Besu block has a slot and non-empty block-access-list hash.
  Audit the block RLP mapper and remaining execution payload fields as part of this.
- Replace the development responder with a real prover and validate its input
  schema, verification keys and public inputs. The responder accepts incomplete
  payloads and returns placeholder proof/public-input data.
- Wire rollup proofs, aggregation, blob submission and L1 finalization. This stack
  disables submission/anchoring/forced transactions and deploys `LinethRollupV9Stub`,
  whose submission/finalization methods are no-ops. Its genesis hash/timestamp
  placeholders also need replacing before validating real proofs.

The coordinator JAR now declares runtime JAR filenames as build inputs. This
prevents an incremental Besu bump from leaving old filenames in its manifest and
failing at runtime with `NoClassDefFoundError: org/hyperledger/besu/crypto/SECP256K1`.

## Cleanup

```bash
make clean-riscv-environment
```
