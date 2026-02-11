# Linea zkEVM

<a href="https://twitter.com/LineaBuild">
  <img src="https://img.shields.io/twitter/follow/LineaBuild?style=for-the-badge" alt="Twitter Follow" height="20">
</a>
<a href="https://discord.com/invite/consensys">
  <img src="https://img.shields.io/badge/Discord-%235865F2.svg?style=for-the-badge&logo=discord&logoColor=white" alt="Discord" height="20">
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

Linea is compatible with the execution clients [Besu](https://github.com/hyperledger/besu/) or [Geth](https://github.com/ethereum/go-ethereum). To run a full node, an execution client is paired with the consensus client [Maru](https://github.com/Consensys/maru).

## Get started

If you already have an understanding of the tech stack, use our [Get Started](docs/get-started.md) guide.

For developers looking to build services locally (such as, the coordinator), see our detailed [Local Development Guide](docs/local-development-guide.md).

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

Consider starting with a ["good first issue"](https://github.com/ConsenSys/linea-monorepo/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).

Before contributing, ensure you're familiar with:

- Our [contribution guide](docs/contribute.md)
- Our [code of conduct](docs/code-of-conduct.md)
- The [Besu contribution guide](https://wiki.hyperledger.org/display/BESU/Coding+Conventions), for Besu:Linea related contributions
- Our [security policy](docs/security.md)

### Useful links

- [Linea docs](https://docs.linea.build) managed from a [public repo](https://github.com/Consensys/doc.linea)
- [Linea blog](https://linea.mirror.xyz)
- [Support](https://support.linea.build)
- [Discord](https://discord.gg/linea)
- [Twitter](https://twitter.com/LineaBuild)
