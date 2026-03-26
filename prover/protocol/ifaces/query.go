package ifaces

import (
	"fmt"
	"strconv"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/google/uuid"
)

// QueryID denotes an unique identifier ID. It uniquely
// defines a query and can be used to provide context about the purpose of the
// query. The convention is to use screaming snake-case (e.g. ABDC_DGEG). The
// empty string should not be used as an ID.
type QueryID string

// QueryIDf formats a [QueryID] from a formatting string and arguments. It is
// a convenience shorthand for `QueryID(fmt.Sprintf(s, args...))`
func QueryIDf(s string, args ...interface{}) QueryID {
	return QueryID(fmt.Sprintf(s, args...))
}

// MarshalJSON implements [json.Marshaler] directly returning the name as a
// quoted string.
func (n *QueryID) MarshalJSON() ([]byte, error) {
	var (
		nString = string(*n)
		nQuoted = strconv.Quote(nString)
	)
	return []byte(nQuoted), nil
}

// UnmarshalJSON implements [json.Unmarshaler] directly assigning the receiver's
// value from the unquoted string value of the bytes.
func (n *QueryID) UnmarshalJSON(b []byte) error {
	var (
		nQuoted        = string(b)
		nUnquoted, err = strconv.Unquote(nQuoted)
	)

	if err != nil {
		return fmt.Errorf("could not unmarshal QueryID from unquoted string: %v : %w", nQuoted, err)
	}

	*n = QueryID(nUnquoted)
	return nil
}

// Query symbolically represents a logical predicate over the runtime of the
// protocol involving a column or a set of [Column], [coin.Info], [Accessor] or
// [QueryParams]. The package [github.com/consensys/linea-monorepo/prover/protocol/query] provides a handful of
// implementations of Query. A common example is the [github.com/consensys/linea-monorepo/protocol/query.GlobalConstraint] which
// requires that an arithmetic expression involving columns of the same size,
// and potentially coins or accessors vanishes over the domain of the involved
// column.
//
// A query can potentially require runtime parameters to be assigned by the
// prover. For instance, [github.com/consensys/linea-monorepo/protocol/query.UnivariateEval] requires that some alleged
// Lagrange interpolation of a [Column] is done correctly, but the declaration of
// the predicate does not specify the evaluation point nor the alleged value (
// because they are only defined at runtime and not during the definition of
// the protocol and thus, of the query).
//
// The interface requires implementing `Check` which manually verifies the
// predicate and `CheckGnark` which does the same thing but in a gnark circuit.
type Query interface {
	Check(run Runtime) error
	CheckGnark(api frontend.API, run GnarkRuntime)
	Name() QueryID
	// UUID returns a unique identifier for the query. It is stronger identifier
	// than the name of the query because two compiled IOPs with queries with
	// the same name won't have the same UUID. The UUID can then be used to
	// distinguish between the two.
	UUID() uuid.UUID
}

// QueryParams represents the runtime parameters of a query. As explained in
// [Query], certain type of queries can require the prover to provide runtime
// parameters in order to make the predicate verifiable. This is the case for
// [github.com/consensys/linea-monorepo/protocol/query.UnivariateEval] which requires the user to provide an evaluation
// point (X) and one or more alleged evaluation points (Ys) (depending on whether
// the query is applied over one or more columns for the same X).
//
// The interface requires only a method to explain how the query parameter
// should update the Fiat-Shamir state.
type QueryParams interface {
	// Update fiat-shamir with the query parameters
	UpdateFS(fiatshamir.FS)
}

// GnarkQueryParams mirrors exactly [QueryParams], but in a gnark circuit.
type GnarkQueryParams interface {
	// Update fiat-shamir with the query parameters in a circuit
	UpdateFS(fiatshamir.GnarkFS)
}
