// The `ifaces` package provides the interface definition of the items
// constituting the Wizard. Namely, it provides all the interfaces satisfied
// by the Wizard building blocks. Importantly, it provides:
//
//   - [Item] the most basic interface, which nearly all items satisfy
//   - [Column] denotes a symbolic sequence of field elements with which
//     the protocol interacts. For instance, the witness of a commitment or
//     a secret polynomial to evaluate. The package [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column] and its
//     subpackages provide most of the implementations
//   - [Query] represents a predicate to be enforced by the protocol.
//     The package [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query] offers a wide variety of implementations
//   - [Runtime] is an interface implemented by [wizard.ProverRuntime] and
//     [wizard.VerifierRuntime]
package ifaces
