# Linea tracer (zkEVM)

This folder hosts a Linea tracing implementation for
[Besu](https://github.com/hyperledger/besu) based on an
implementation in Go.

Tracing refers to the process of extracting data from the execution of an EVM client in order to construct large matrices known as execution traces. Execution traces are subject to the constraint system specified in the [linea-specification](https://github.com/Consensys/linea-specification) repo and implemented in the [tracer-constraints](../tracer-constraints) folder.

It serves developers by making the Linea tech stack open source under
the [Apache 2.0 license](LICENSE).

## What is Linea?

[Linea](https://linea.build) is a developer-ready layer 2 network scaling Ethereum. It's secured with a zero-knowledge rollup, built on lattice-based cryptography, and powered by [Consensys](https://consensys.io).

## Get started

If you already have an understanding of the tech stack, use our [Get Started](docs/get-started.md) guide.

### Looking for plugins?

Discover [existing plugins](docs/plugins.md) and understand the [plugin release process](docs/plugin-release.md).

## Looking for the Linea code?

Linea's stack is made up of multiple components, these include:

- This folder, [tracer](../tracer): Linea-Besu plugin which produces the traces that the constraint system applies and that serve as inputs to the prover

> This repository contains the elements of the Linea stack responsible for this process.

- [linea-monorepo](https://github.com/Consensys/linea-monorepo): The main repository for the Linea stack & network in which this folder lives
- [besu](https://github.com/hyperledger/besu): Besu client
- [linea-sequencer](https://github.com/Consensys/linea-sequencer): A set of Linea-Besu plugins for the sequencer and RPC nodes
- [tracer-constraints](../tracer-constraints) folder: Implementation of the constraint system from the specification
- [linea-specification](https://github.com/Consensys/linea-specification): Specification of the constraint system defining Linea's zkEVM

Linea abstracts away the complexity of this technical architecture to allow developers to:

- [Bridge tokens](https://docs.linea.build/developers/guides/bridge)
- [Deploy a contract](https://docs.linea.build/developers/quickstart/deploy-smart-contract)
- [Run a node](https://docs.linea.build/developers/guides/run-a-node)

... and more.

## How to contribute

Contributions are welcome!

### Guidelines for Non-Code and other Trivial Contributions

Please keep in mind that we do not accept non-code contributions like fixing comments, typos or some other trivial fixes. Although we appreciate the extra help, managing lots of these small contributions is unfeasible, and puts extra pressure in our continuous delivery systems (running all tests, etc). Feel free to open an issue pointing to any of those errors, and we will batch them into a single change.

1. [Create an issue](https://github.com/Consensys/linea-tracer/issues).

> If the proposed update requires input, also tag us for discussion.

2. Submit the update as a pull request from your [fork of this repo](https://github.com/Consensys/linea-monorepo/fork), and tag us for review.

> Include the issue number in the pull request description and (optionally) in the branch name.

Consider starting with a ["good first issue"](https://github.com/ConsenSys/linea-tracer/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).

Before contributing, ensure you're familiar with:

- Our [Linea contribution guide](https://github.com/Consensys/linea-monorepo/blob/main/docs/contribute.md)
- Our [Linea code of conduct](https://github.com/Consensys/linea-monorepo/blob/main/docs/code-of-conduct.md)
- The [Besu contribution guide](https://wiki.hyperledger.org/display/BESU/Coding+Conventions), for Besu:Linea related contributions
- Our [Security policy](https://github.com/Consensys/linea-monorepo/blob/main/docs/security.md)

### Useful links

- [Linea docs](https://docs.linea.build)
- [Linea blog](https://linea.mirror.xyz)
- [Support](https://support.linea.build)
- [Discord](https://discord.gg/linea)
- [Twitter](https://twitter.com/LineaBuild)

### Update reference tests

To update the reference tests, follow the steps below:

- In [tracer/reference-tests/build.gradle](../tracer/reference-tests/build.gradle) : update the `src` from `downloadExecutionSpecFixtures` task with the `ethereum-spec-tests` repo's new fixtures version you would like to update to
- At the root, launch
  - ```shell
    ./gradlew tracer:reference-tests:downloadExecutionSpecFixtures
    ```
  - ```shell
    ./gradlew tracer:reference-tests:copyExecutionSpecFixtures
    ```
  - ```shell
    ./gradlew tracer:reference-tests:generateExecutionSpecBlockchainTests
    ```
- Commit the changes
- You can launch all tests to ensure everything is passing locally with the command line below or on the CI with this [GitHub Action](https://github.com/Consensys/linea-monorepo/actions/workflows/tracer-reference-blockchain-daily-tests.yml)
  - ```shell
    ./gradlew tracer:reference-tests:referenceExecutionSpecBlockchainTests
    ```
