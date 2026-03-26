package fetchers_arithmetization

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

// RootHashFetcher is a struct used to fetch the first/final root hashes from the state summary module
type RootHashFetcher struct {
	// First and Last are the columns that store the first and last root hashes.
	// They are divided into 16 16-bit limb columns. 256 bits in total.
	First, Last [common.NbLimbU128]ifaces.Column
}

// NewRootHashFetcher returns a new RootHashFetcher with initialized columns that are not constrained.
func NewRootHashFetcher(comp *wizard.CompiledIOP, name string, sizeSS int) *RootHashFetcher {
	var res RootHashFetcher

	for i := range res.First {
		res.First[i] = util.CreateCol(name, fmt.Sprintf("FIRST_%d", i), sizeSS, comp)
		res.Last[i] = util.CreateCol(name, fmt.Sprintf("LAST_%d", i), sizeSS, comp)
	}

	return &res
}

// DefineRootHashFetcher specifies the constraints of the RootHashFetcher with respect to the StateSummary
func DefineRootHashFetcher(comp *wizard.CompiledIOP, fetcher *RootHashFetcher, name string, ss statesummary.Module) {
	for i := range fetcher.First {
		commonconstraints.MustBeConstant(comp, fetcher.First[i])
		commonconstraints.MustBeConstant(comp, fetcher.Last[i])

		// if the first state summary segment starts with storage operations, fetcher.First
		// must equal the first value in the state summary's worldstatehash
		// otherwise, we take it from the first value of the accumulator
		comp.InsertLocal(
			0,
			ifaces.QueryIDf("%s_FIRST_LOCAL_%d", name, i),
			sym.Sub(
				fetcher.First[i],
				util.Ternary(ss.IsStorage, ss.WorldStateRoot[i], ss.AccumulatorStatement.StateDiff.InitialRoot[i]),
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
		util.CheckLastELemConsistency(comp, ss.IsActive, ss.AccumulatorStatement.StateDiff.FinalRoot[i], fetcher.Last[i], name)
	}
}

// AssignRootHashFetcher assigns the data in the RootHashFetcher using the data fetched from the StateSummary
func AssignRootHashFetcher(run *wizard.ProverRuntime, fetcher *RootHashFetcher, ss statesummary.Module) {
	// if the first state summary segment starts with storage operations, fetch the value in worldstatehash
	// otherwise, we take it from the first value of the accumulator
	var first, last [common.NbLimbU128]field.Element

	firstSrcCols := ss.AccumulatorStatement.StateDiff.InitialRoot
	initialStorage := ss.IsStorage.GetColAssignmentAt(run, 0)
	if initialStorage.IsOne() {
		firstSrcCols = ss.WorldStateRoot
	}

	for i := range first {
		first[i] = firstSrcCols[i].GetColAssignmentAt(run, 0)
	}

	// get the value in the last row of FinalRoot before it goes inactive
	size := ss.IsActive.Size()
	for i := 0; i < size; i++ {
		isActive := ss.IsActive.GetColAssignmentAt(run, i)
		if isActive.IsOne() && i != size-1 {
			continue
		}

		if i == 0 {
			break
		}

		for j := range last {
			last[j] = ss.AccumulatorStatement.StateDiff.FinalRoot[j].GetColAssignmentAt(run, i-1)
		}

		break
	}

	// assign the fetcher columns
	for i := range fetcher.First {
		run.AssignColumn(fetcher.First[i].GetColID(), smartvectors.NewConstant(first[i], size))
		run.AssignColumn(fetcher.Last[i].GetColID(), smartvectors.NewConstant(last[i], size))
	}
}
