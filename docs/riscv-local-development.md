# RISC-V local stack

## Prerequisites

Install the repository prerequisites from [get-started.md](get-started.md) and
build the coordinator image:

```bash
make docker-build-coordinator
```

The sequencer image must return execution witnesses for the local Lineth
network, which currently uses Osaka. The repository's default image predates
`debug_executionWitness`, while the initial upstream implementation requires
Amsterdam block access lists. Compatible Linea Besu integration is tracked by
[issue #3084](https://github.com/LFDT-Lineth/lineth-monorepo/issues/3084).

## Start

```bash
LINEA_BESU_PACKAGE_TAG="<tag-with-debug_executionWitness>" make start-env-with-riscv
```

This starts a development responder that returns structurally valid execution
proof responses without proving them. It can be replaced with an R5 prover once
a compatible image is available.

## Diagnostics

```bash
docker compose -p linea-riscv-dev -f docker/compose-riscv.yml ps
docker compose -p linea-riscv-dev -f docker/compose-riscv.yml logs coordinator riscv-proof-responder
find tmp/riscv/prover/riscv -type f -print
```

## Cleanup

```bash
make clean-riscv-environment
```
