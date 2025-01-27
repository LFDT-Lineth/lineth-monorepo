# Linea constraints 

This repository hosts the implementation of the constraint system specified in the [linea-specification repo](https://github.com/Consensys/linea-specification). 

Linea’s constraint system applies to so-called traces which are large matrices of fixed width (i.e. fixed number of columns or ‘registries’) and variable depth (correlating with the complexity of the EVM execution). The production of such traces is the job of the [linea-tracer repo](https://github.com/Consensys/linea-tracer).

Constraints and traces are two of the inputs to the prover.

It serves developers by making the Linea tech stack open source under 
the [Apache 2.0 license](LICENSE).

## What is Linea?

[Linea](https://linea.build) is a developer-ready layer 2 network scaling Ethereum. It's secured with a zero-knowledge rollup, built on lattice-based cryptography, and powered by [Consensys](https://consensys.io).

<!-- ## Get started

tbd -->

## Looking for the Linea code?

Linea's stack is made up of multiple repositories, these include:

- This repo, [linea-constraints](https://github.com/Consensys/linea-constraints): Implementation of the constraint system from the specification
- [linea-monorepo](https://github.com/Consensys/linea-monorepo): The main repository for the Linea stack & network 
- [linea-besu](https://github.com/Consensys/linea-besu): Fork of Besu to implement the Linea-Besu client
- [linea-sequencer](https://github.com/Consensys/linea-sequencer): A set of Linea-Besu plugins for the sequencer and RPC nodes
- [linea-tracer](https://github.com/Consensys/linea-tracer): Linea-Besu plugin which produces the traces that the constraint system applies and that serve as inputs to the prover
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

1. [Create an issue](https://github.com/Consensys/linea-constraints/issues).
> If the proposed update requires input, also tag us for discussion.
2. Submit the update as a pull request from your [fork of this repo](https://github.com/Consensys/linea-constraints/fork), and tag us for review. 
> Include the issue number in the pull request description and (optionally) in the branch name.

Consider starting with a ["good first issue"](https://github.com/ConsenSys/linea-constraints/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).

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
- [X](https://x.com/LineaBuild)
