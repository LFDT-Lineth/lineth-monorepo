# Besu Plugins related to tracer and sequencer functionality

This repository hosts the implementation of the sequencer, the component of the Linea stack responsible for ordering transactions and building blocks, as well as executing them. It provides a set of [Besu](https://github.com/hyperledger/besu):Linea plugins. 

It serves developers by making the Linea tech stack open source under 
the [Apache 2.0 license](LICENSE).

## What is Linea?

[Linea](https://linea.build) is a developer-ready layer 2 network scaling Ethereum. It's secured with a zero-knowledge rollup, built on lattice-based cryptography, and powered by [Consensys](https://consensys.io).

## Get started

If you already have an understanding of the tech stack, use our [Quickstart](docs/quickstart.md) guide.

### Looking for Plugins?

Discover [existing plugins](docs/plugins.md) and understand the [plugin release process](docs/plugin-release.md). 

## Looking for the Linea code?

Linea's stack is made up of multiple repositories, these include:
- This repo, [linea-monorepo](https://github.com/Consensys/linea-monorepo): main repository for the Linea stack & network
- [besu](https://github.com/hyperledger/besu): Besu client
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

### Guidelines for Non-Code and other Trivial Contributions
Please keep in mind that we do not accept non-code contributions like fixing comments, typos or some other trivial fixes. Although we appreciate the extra help, managing lots of these small contributions is unfeasible, and puts extra pressure in our continuous delivery systems (running all tests, etc). Feel free to open an issue pointing any of those errors, and we will batch them into a single change.

1. [Create an issue](https://github.com/Consensys/linea-monorepo/issues).
> If the proposed update requires input, also tag us for discussion.
2. Submit the update as a pull request from your [fork of this repo](https://github.com/Consensys/linea-monorepo/fork), and tag us for review. 
> Include the issue number in the pull request description and (optionally) in the branch name.

Consider starting with a ["good first issue"](https://github.com/ConsenSys/linea-monorepo/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).

Before contributing, ensure you're familiar with:

- Our [Linea contribution guide](https://github.com/Consensys/linea-monorepo/blob/main/docs/contribute.md)
- Our [Linea code of conduct](https://github.com/Consensys/linea-monorepo/blob/main/docs/code-of-conduct.md)
- The [Besu contribution guide](https://github.com/hyperledger/besu/blob/main/CONTRIBUTING.md), for Besu:Linea related contributions
- Our [Security policy](https://github.com/Consensys/linea-monorepo/blob/main/docs/security.md)


### Useful links

- [Linea docs](https://docs.linea.build)
- [Linea blog](https://linea.mirror.xyz)
- [Support](https://support.linea.build)
- [Discord](https://discord.gg/linea)
- [X](https://x.com/LineaBuild)
