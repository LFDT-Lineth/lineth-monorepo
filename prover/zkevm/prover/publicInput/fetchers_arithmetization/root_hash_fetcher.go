package fetchers_arithmetization

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

// RootHashFetcher is a struct used to fetch the first/final root hashes from the state summary module
type RootHashFetcher struct {
	First, Last ifaces.Column
}

// NewRootHashFetcher returns a new RootHashFetcher with initialized columns that are not constrained.
func NewRootHashFetcher(comp *wizard.CompiledIOP, name string, sizeSS int) *RootHashFetcher {
	return &RootHashFetcher{
		First: util.CreateCol(name, "FIRST", sizeSS, comp),
		Last:  util.CreateCol(name, "LAST", sizeSS, comp),
	}
}

// DefineRootHashFetcher specifies the constraints of the RootHashFetcher with respect to the StateSummary
func DefineRootHashFetcher(comp *wizard.CompiledIOP, fetcher *RootHashFetcher, name string, ss statesummary.Module) {

	commonconstraints.MustBeConstant(comp, fetcher.First)
	commonconstraints.MustBeConstant(comp, fetcher.Last)

	// if the first state summary segment starts with storage operations, fetcher.First
	// must equal the first value in the state summary's worldstatehash
	// otherwise, we take it from the first value of the accumulator
	comp.InsertLocal(
		0,
		ifaces.QueryIDf("%s_%s", name, "FIRST_LOCAL"),
		sym.Sub(
			fetcher.First,
			util.Ternary(ss.IsStorage, ss.WorldStateRoot, ss.AccumulatorStatement.StateDiff.InitialRoot),
		),
	)

	// ss.IsActive is already constrained in the state summary as a typical IsActive pattern,
	// with 1s followed by 0s, no need to constrain it again
	// two cases: Case 1: ss.IsActive is not completely full, then fetcher.Last is equal to
	// the accumulator's final root at the last cell where isActive is 1
	// (ss.IsActive[i]*(1-ss.IsActive[i+1]))*(fetcher.Last-ss.FinalRoot[i])
	// Case 2: ss.IsActive is completely full, in which case we ask that
	// ss.IsActive[size]*(fetcher.Last-ss.FinalRoot[size]) = 0
	// i.e. at the last row, counter is equal to ctMax
	util.CheckLastELemConsistency(comp, ss.IsActive, ss.AccumulatorStatement.StateDiff.FinalRoot, fetcher.Last, name)

}

// AssignRootHashFetcher assigns the data in the RootHashFetcher using the data fetched from the StateSummary
func AssignRootHashFetcher(run *wizard.ProverRuntime, fetcher *RootHashFetcher, ss statesummary.Module) {
	// if the first state summary segment starts with storage operations, fetch the value in worldstatehash
	// otherwise, we take it from the first value of the accumulator
	var first field.Element
	initialStorage := ss.IsStorage.GetColAssignmentAt(run, 0)
	if initialStorage.IsOne() {
		worldStateHash := ss.WorldStateRoot.GetColAssignmentAt(run, 0)
		first.Set(&worldStateHash)
	} else {
		firstAcc := ss.AccumulatorStatement.StateDiff.InitialRoot.GetColAssignmentAt(run, 0)
		first.Set(&firstAcc)
	}

	// get the value in the last row of FinalRoot before it goes inactive
	var last field.Element
	size := ss.IsActive.Size()
	for i := 0; i < size; i++ {
		isActive := ss.IsActive.GetColAssignmentAt(run, i)
		if isActive.IsOne() {
			finalRoot := ss.AccumulatorStatement.StateDiff.FinalRoot.GetColAssignmentAt(run, i)
			last.Set(&finalRoot)
		} else {
			// reached the end
			break
		}
	}

	// assign the fetcher columns
	run.AssignColumn(fetcher.First.GetColID(), smartvectors.NewConstant(first, size))
	run.AssignColumn(fetcher.Last.GetColID(), smartvectors.NewConstant(last, size))
}
