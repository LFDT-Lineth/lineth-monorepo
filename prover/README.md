# lineth-monorepo/prover

This directory contains the implementation of the Lineth prover. As part of it,
it contains an implementation of the Vortex polynomial commitment, of the
Arcane compiler, the instantiation of the zkEVM using the arithmetization and
the server implementation.

# Building and running

The prover has the following build dependencies
* `rust@1.74.0` and `cargo`
* `go@1.25.7`
* `make`

The repository counts 2 main binaries:

- `bin/prover` : `bin/prover setup` generate the assets (setup / preprocessing) `bin/prover prove` run process a request, create a proof and outputs a response.
- `bin/controller` : a file-system based server to run the Lineth prover

### Building and running the setup generator

The setup-generation (`make setup`) is used to generate the setup for all the types of provers. Execution, Decompression and Aggregation.
By default, if the `--force` flag is not provided, the tool will compile the circuit and check if the destination dir already contains a setup that matches, skipping the CPU intensive phase of the actual plonk Setup if needed.

The setup also persists the Lagrange form of the SRS into the `kzgsrs` directory
(as `kzg_srs_lagrange_<size>_<curve>_derived.memdump`) whenever it is missing, so
prover starts load it in seconds instead of re-deriving it for hours. It only ever
adds these `derived`-tagged files — ceremony files are never modified, and nothing
writes into the directory at prove time. Set `persist_derived_srs = false` in the
config to keep the SRS directory strictly read-only; `--force` additionally
re-validates an existing derived dump in full and repairs it if it does not load.

**Run**

```sh
make setup
```

# Integration tests

```
./integration/run.sh dev-mode
./integration/run.sh full-mode
```