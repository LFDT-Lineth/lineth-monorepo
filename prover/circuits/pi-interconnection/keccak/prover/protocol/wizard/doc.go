// Package wizard provides the main structure articulating the framework.
// Namely, it provides the following structures:
//
//   - [Builder] provides a user-friendly interface to generate
//     a custom polynomial IOP. Note that this structure will be deprecated and
//     the user should use instead the lower-level CompiledIOP to define its
//     own protocol. In particular, the [Builder] is a wrapper around
//     the [CompiledIOP] that has the capacity to auto-detect the rounds
//     at which the "items" (i.e. the columns, queries or coins) are declared.
//
//   - [CompiledIOP] stores a representation of the elaborated
//     protocol before, during and after its compilation.
//
//   - [ProverRuntime] is the entrypoint to interact with the
//     runtime of the prover of the protocol. It is used internally as a
//     placeholder to store the witness and is the entrypoint to specify
//     custom prover behaviors.
//
//   - [VerifierRuntime] is the entrypoint to interact with the
//     runtime of the verifier of the protocol. It is used internally to
//     capture all the computations and checks directly performed by the
//     verifier
//
//   - [GnarkVerifierRuntime] - similar to [VerifierRuntime] -
//     is the entrypoint to interact with the verifier of the protocol inside
//     a gnark circuit. It provides a way to do recursive composition of the
//     wizard into a gnark circuit.
//
// Here is a minimal example of the definition of a protocol to prove knowledge
// of the Fibonacci sequence.
//
// ```
// // This function is provided to the function [Compile] by the user to
// // specify what the protocol should be. The user has access to a pallet of
// // different queries
// func defineFibo(build *wizard.Builder) {
//
//		// Number of rows (e.g. the size of the fibonacci sequence to prove
//		// knowledge of).
//		n := 1 << 3
//
//		// This declares a column to commit to, allegedly containing the sequence.
//		p1 := build.RegisterCommit(P1, n)
//
//		// This declares a constraints that `p1` is valid fibonacci sequence:
//		// in other words it enforces that p1[i] = pi[i-1] + pi[i-2]
//		expr := ifaces.ColumnAsVariable(column.Shift(p1, -1)).
//			Add(ifaces.ColumnAsVariable(column.Shift(p1, -2))).
//			Sub(ifaces.ColumnAsVariable(p1))
//
//		_ = build.GlobalConstraint(GLOBAL1, expr)
//	}
//
//	// This function is passed to the wizard and assigns the witness (namely,
//	// the fibonacci sequence to the above-defined `p1`). All columns defined
//	// in the "define" function require an explicit assignment from the user. It
//	// is also the case for some types of query. This is not the case here, but
//	// for instance, if we had declared a polynomial evaluation query, we would
//	// have needed to also provide an evaluation point `x` and the corresponding
//	// evaluation claim `y``.
//	func proveFibo(run *wizard.ProverRuntime) {
//		x := smartvectors.ForTest(1, 1, 2, 3, 5, 8, 13, 21)
//		run.AssignColumn(P1, x)
//	}
//
//	func TestFibo(t *testing.T) {
//
//		// This instantiates the protocol, converting all the Wizard queries and
//		// columns into a concrete protocol.
//		compiled := wizard.Compile(
//			defineFibo,
//			compiler.Arcane(8, 8),
//			vortex.Compile(2),
//		)
//
//		// This generates a proof based on the witness assigned by `proverFibo`
//		proof := wizard.Prove(compiled, proveFibo)
//
//		// This runs the verifier and returns an error if the proof was incorrect
//		if err := wizard.Verify(compiled, proof); err != nil {
//			panic("invalid proof")
//		}
//	}
//
// ```
package wizard
