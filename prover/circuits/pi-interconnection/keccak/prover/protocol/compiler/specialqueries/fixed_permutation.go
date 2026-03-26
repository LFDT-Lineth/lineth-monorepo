package specialqueries

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// Reduce the fixed permutations into a permutation query
func CompileFixedPermutations(comp *wizard.CompiledIOP) {

	numRounds := comp.NumRounds()

	for i := 0; i < numRounds; i++ {
		queries := comp.QueriesNoParams.AllKeysAt(i)
		for _, qName := range queries {
			// Skip if it was already compiled to avoid recompiling a second
			// time the same query.
			if comp.QueriesNoParams.IsIgnored(qName) {
				continue
			}

			switch q_ := comp.QueriesNoParams.Data(qName).(type) {
			case query.FixedPermutation:
				reduceFixedPermutation(comp, q_)
			}
		}
	}
}

// Reduce a permutation query. Follows the grand product argument
// from PLONK paper: extended permutation part
func reduceFixedPermutation(comp *wizard.CompiledIOP, q query.FixedPermutation) {
	/*
		Sanity checks : Mark the query as compiled and make sure that
		it was not previously compiled.
	*/
	if comp.QueriesNoParams.MarkAsIgnored(q.ID) {
		panic("did not expect that a query no param could be ignored at this stage")
	}

	var (
		round  = comp.QueriesNoParams.Round(q.ID)
		sid    = make([]ifaces.Column, len(q.A))
		s      = make([]ifaces.Column, len(q.S))
		permID = q.Name() + "_FIXED_PERM"
		permA  = [][]ifaces.Column{}
		permB  = [][]ifaces.Column{}
		cnt    = 0
	)

	for i := range s {
		s[i] = comp.InsertPrecomputed(deriveNamePerm("S", q.ID, i), q.S[i])
		if column.StatusOf(q.B[i]).IsPublic() {
			comp.Columns.SetStatus(s[i].GetColID(), column.VerifyingKey)
		}
		permB = append(permB, []ifaces.Column{q.B[i], s[i]})
	}

	for i := range q.A {
		size := q.A[i].Size()
		sid[i] = dedicated.CounterPrecomputed(comp, cnt, cnt+size)
		if column.StatusOf(q.A[i]).IsPublic() {
			comp.Columns.SetStatus(sid[i].GetColID(), column.VerifyingKey)
		}
		permA = append(permA, []ifaces.Column{q.A[i], sid[i]})
		cnt += size
	}

	comp.InsertFragmentedPermutation(round, permID, permA, permB)
}

func deriveNamePerm(r string, queryName ifaces.QueryID, i int) ifaces.ColID {
	return ifaces.ColIDf("%v_%v_%v", queryName, r, i)
}
