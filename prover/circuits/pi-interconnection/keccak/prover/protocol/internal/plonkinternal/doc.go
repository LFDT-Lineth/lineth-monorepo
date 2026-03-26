/*
`plonk` provides a dedicated Wizard utility to embed a gnark Plonk circuit in a
Wizard's compiled IOP. This utility can be used by passing the [plonk.PlonkCheck]
function.

For instance, say we have a gnark circuit that can verify digital signatures
and which takes a set of message hashes and public keys as public inputs. The
user can provide a column allegedly containing all the public input and a
gnark circuit performing the above-mentioned signature verification.

The user can call the [plonk.PlonkCheck] function by passing the column and the
circuit alongside a [wizard.CompiledIOP] object. The utility will build all the
necessary columns and declare all the necessary constraints to emulate the
circuit's satisfiability within the currently compiled IOP.

This comes in handy in situation where we wish to prove complex relations that
are difficult to express directly in the form of a Wizard-IOP but easier to
express in a language that is more expressive. In the case, of Linea's zkEVM,
this is used for the ECDSA verification and the precompiles.

The package optionally offers optimization when,
  - declaring multiple instances of the same circuit
  - deferring the range-checks outside of the circuit so that they can be
    implemented directly using [bigrange.BigRange] which has less overheads
    than in-circuit range-checks.
*/
package plonkinternal
