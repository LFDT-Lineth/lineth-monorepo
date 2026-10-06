package codegen

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
)

// GrandProductSystem is the compiled metadata for every wiop.GrandProduct
// query in a wiop.System, in the form the Zig grandproduct sub-verifier
// consumes.
//
// The Z-recurrence and the row-0 boundary are ordinary vanishing constraints
// (registered by the grandproduct compiler's buildZ), so they are discharged
// by the vanishing sub-verifier. What remains for this sub-verifier is the
// pair of boundary identities the compiler's verifier actions enforce:
//
//   - FinalProductCheck (always present, one per GrandProduct):
//     ∏_entries Z[n-1] == Result
//   - a query-specific check on Result itself, whichever of
//     grandproduct.CheckResultIsOne or messagebus.CheckHandleSumInShard the
//     compiler attached to this GrandProduct (permutation arguments always
//     attach the former; the message-bus pass attaches the latter, skippable
//     per handle via MessageBus.SkipInShardCheck) — both reduce to
//     Result == Expected for a compile-time constant Expected.
//
// All cell operands are (round, index) references that the Zig verifier reads
// from ctx.rounds at verify time, so the check is against the adversary's
// transcript rather than baked-in honest-prover values.
type GrandProductSystem struct {
	SourceName string
	Queries    []GrandProductQuery
}

// GrandProductQuery is one reduced wiop.GrandProduct: the transcript
// positions of the Z-column endpoints and the claimed Result, plus the
// optional Result == Expected boundary check.
type GrandProductQuery struct {
	SourceName string
	// ZFinalRefs are the (round, index) positions of Z[n-1] for each Z column
	// FinalProductCheck folds into the claimed product.
	ZFinalRefs []ScalarCellRef
	// ResultRef is the (round, index) position of the claimed grand product.
	ResultRef ScalarCellRef
	// HasExpected reports whether a CheckResultIsOne / CheckHandleSumInShard
	// verifier action also constrains ResultRef to Expected. False for a
	// GrandProduct whose in-shard check was skipped (messagebus's
	// SkipInShardCheck, left to a downstream cross-shard layer).
	HasExpected bool
	// Expected is the base-field constant ResultRef must equal, valid only
	// when HasExpected is true. field.ElemOne() for both CheckResultIsOne and
	// the default (unsharded) CheckHandleSumInShard.
	Expected uint64
}

// BuildGrandProductSystem extracts every wiop.GrandProduct registered on sys
// and records its Z-endpoint / Result cell-reference coordinates, together
// with whichever Result == Expected check (if any) the compiler attached.
// Queries are collected in round/registration order so the output is
// deterministic.
//
// Requires the grandproduct compiler's FinalProductCheck to have been
// registered for every GrandProduct (compileGrandProducts runs unconditionally
// for every GrandProduct, including ones a caller — e.g. messagebus —
// constructs directly), and global.Compile to have been called afterwards so
// no cell lands in the last wiop round (protocol.replay excludes it from
// ctx.rounds).
func BuildGrandProductSystem(sys *wiop.System) (GrandProductSystem, error) {
	out := GrandProductSystem{SourceName: sys.Context.Path()}
	lastSlot := len(sys.Rounds) - 1

	expectedByResultCell := map[*wiop.Cell]uint64{}
	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			switch a := action.(type) {
			case *grandproduct.CheckResultIsOne:
				expectedByResultCell[a.GrandProduct.Result] = 1
			case *messagebus.CheckHandleSumInShard:
				base := a.Expected.AsBase()
				expectedByResultCell[a.Cell] = base.Uint64()
			}
		}
	}

	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			fpc, ok := action.(*grandproduct.FinalProductCheck)
			if !ok {
				continue
			}

			resultSlot := fpc.GrandProduct.Result.Context.ID.Slot()
			if err := checkNotLastSlot("result", fpc.GrandProduct.Context().Path(), resultSlot, lastSlot); err != nil {
				return GrandProductSystem{}, err
			}

			query := GrandProductQuery{
				SourceName: fpc.GrandProduct.Context().Path(),
				ResultRef:  ScalarCellRef{Round: resultSlot, Index: fpc.GrandProduct.Result.Context.ID.Position()},
				ZFinalRefs: make([]ScalarCellRef, len(fpc.Entries)),
			}
			for i, e := range fpc.Entries {
				zSlot := e.ZFinal.Context.ID.Slot()
				if err := checkNotLastSlot("z_final", e.ZFinal.Context.Path(), zSlot, lastSlot); err != nil {
					return GrandProductSystem{}, err
				}
				query.ZFinalRefs[i] = ScalarCellRef{Round: zSlot, Index: e.ZFinal.Context.ID.Position()}
			}
			if expected, ok := expectedByResultCell[fpc.GrandProduct.Result]; ok {
				query.HasExpected = true
				query.Expected = expected
			}
			out.Queries = append(out.Queries, query)
		}
	}

	return out, nil
}
