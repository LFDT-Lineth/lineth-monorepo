package wizard

import "github.com/consensys/linea-monorepo/prover/protocol/ifaces"

// PublicInput represents a public input in a wizard protocol. Public inputs
// are materialized with a functional identifier and a local opening query.
// The identifier is what ultimately identifies the public input as the query
// may be mutated by compilation (if we use the FullRecursion compiler), therefore
// it would unsafe to use the ID of the query to identify the public input in
// the circuit.
type PublicInput struct {
	Name string
	Acc  ifaces.Accessor
}
