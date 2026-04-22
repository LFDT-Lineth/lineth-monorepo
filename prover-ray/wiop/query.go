package wiop

import "github.com/consensys/gnark/frontend"

// GnarkRuntime is the execution context passed to circuit-verification methods.
// It provides access to gnark-variable column assignments and coin values inside
// an arithmetic circuit.
type GnarkRuntime interface{}

// Query is the base interface for all verifier predicates in the protocol.
// A query declares a predicate over committed columns, coins, and cells that
// the verifier must check.
//
// Round returns the earliest round after which the query is ready for
// verification: every committed column and cell referenced by the query has
// been fixed and every coin has been drawn. The verifier may delay the check
// to any later round without soundness risk.
//
// IsReduced and MarkAsReduced support incremental compilation: a compiler pass
// that has consumed and rewritten a query marks it as reduced so subsequent
// passes can skip it.
type Query interface {
	// Round returns the earliest round at which this query can be verified.
	// For module-bound queries the round equals the column's round. For
	// cross-module or PCS-mediated queries it is the maximum round of all
	// referenced columns or the round of the response cells, respectively.
	Round() *Round
	// Check verifies the query predicate against a concrete runtime assignment.
	// Returns a non-nil error if the predicate does not hold.
	Check(Runtime) error
	// IsReduced reports whether a compiler pass has already consumed and
	// rewritten this query into a simpler form.
	IsReduced() bool
	// MarkAsReduced marks the query as consumed by a compiler pass. Subsequent
	// passes should skip reduced queries. Idempotent.
	MarkAsReduced()
	// Context returns the ContextFrame that uniquely identifies this query
	// within the protocol hierarchy.
	Context() *ContextFrame
}

// GnarkCheckableQuery is an optional interface implemented by queries that can
// be verified inside a gnark arithmetic circuit. Queries that cannot be
// expressed in-circuit (e.g. lookup tables) do not implement this interface
// and must be compiled away before gnark verification.
type GnarkCheckableQuery interface {
	Query
	// CheckGnark asserts the query predicate inside a gnark circuit. It
	// accesses gnark-variable assignments via run and enforces the predicate
	// through api.
	CheckGnark(api frontend.API, run GnarkRuntime)
}

// AssignableQuery is an optional interface implemented by queries that can
// automatically compute and store their own result cells. A caller should
// check IsAlreadyAssigned before calling SelfAssign to avoid overwriting a
// prover-supplied value.
type AssignableQuery interface {
	Query
	// IsAlreadyAssigned reports whether the query's result cells already hold
	// an assignment in the given runtime.
	IsAlreadyAssigned(Runtime) bool
	// SelfAssign evaluates the query predicate and writes the result into the
	// query's result cells in the given runtime.
	SelfAssign(Runtime)
}

// baseQuery is an internal helper struct that provides the boilerplate fields
// and method implementations shared by all [Query] implementations:
// IsReduced, MarkAsReduced, and Context.
//
// Embed this struct in concrete query types and supply a context and
// Annotations at construction time.
type baseQuery struct {
	// context is the ContextFrame that identifies this query in the hierarchy.
	context *ContextFrame
	// Annotations holds arbitrary metadata attached to this query.
	Annotations Annotations
	// isReduced tracks whether a compiler pass has consumed this query.
	isReduced bool
}

// IsReduced implements the IsReduced method of [Query]. Reports whether this
// query has been marked as consumed by a compiler pass.
func (b *baseQuery) IsReduced() bool { return b.isReduced }

// MarkAsReduced implements [Query]. Marks this query as consumed by a compiler
// pass. Idempotent.
func (b *baseQuery) MarkAsReduced() { b.isReduced = true }

// Context implements [Query]. Returns the ContextFrame that uniquely
// identifies this query within the protocol hierarchy.
func (b *baseQuery) Context() *ContextFrame { return b.context }

// roundOf extracts the owning [Round] from a [FieldPromise]. It handles *Cell
// and *CoinField, which are the two concrete scalar types that carry a round
// back-reference. Returns nil for any other implementation.
func roundOf(fp FieldPromise) *Round {
	switch f := fp.(type) {
	case *Cell:
		return f.Round()
	case *CoinField:
		return f.round
	default:
		return nil
	}
}

// maxRoundInExpr traverses the expression AST and returns the [Round] with
// the highest ID reachable from any leaf node, or nil if the expression
// contains no round-carrying leaf.
func maxRoundInExpr(expr Expression) *Round {
	switch e := expr.(type) {
	case *ColumnView:
		return e.Column.round
	case *ColumnPosition:
		return e.Column.round
	case *Cell:
		return e.round
	case *CoinField:
		return e.round
	case *ArithmeticOperation:
		var best *Round
		for _, op := range e.Operands {
			r := maxRoundInExpr(op)
			if r != nil && (best == nil || r.ID > best.ID) {
				best = r
			}
		}
		return best
	default:
		return nil
	}
}
