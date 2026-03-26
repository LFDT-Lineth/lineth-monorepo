package selector

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

type SubsampleProverAction struct {
	Large        []ifaces.Column
	Small        []ifaces.Column
	AccLarge     ifaces.Column
	AccSmall     ifaces.Column
	AccLargeLast ifaces.QueryID
	AccSmallLast ifaces.QueryID
	Gamma        coin.Info
	Alpha        coin.Info
	LenSmall     int
	Period       int
	Offset       int
	NeedGamma    bool
}

func (a *SubsampleProverAction) Run(run *wizard.ProverRuntime) {
	r := a.Large[0].GetColAssignment(run)
	if a.NeedGamma {
		largeWit := make([]smartvectors.SmartVector, len(a.Large))
		largeWit[0] = r
		for i := 1; i < len(a.Large); i++ {
			largeWit[i] = a.Large[i].GetColAssignment(run)
		}
		gamma := run.GetRandomCoinField(a.Gamma.Name)
		r = smartvectors.PolyEval(largeWit, gamma)
	}

	prev := field.Zero()
	accLargeWit := make([]field.Element, a.Period*a.LenSmall)
	alpha_ := run.GetRandomCoinField(a.Alpha.Name)

	for hashID := 0; hashID < a.LenSmall; hashID++ {
		for i := 0; i < a.Period; i++ {
			pos := hashID*a.Period + i
			if i != a.Offset {
				accLargeWit[pos] = prev
				continue
			}
			currentNewState := r.Get(pos)
			accLargeWit[pos].Mul(&alpha_, &prev)
			accLargeWit[pos].Add(&accLargeWit[pos], &currentNewState)
			prev = accLargeWit[pos]
		}
	}

	run.AssignColumn(a.AccLarge.GetColID(), smartvectors.NewRegular(accLargeWit))
	run.AssignLocalPoint(a.AccLargeLast, prev)

	rPrime := a.Small[0].GetColAssignment(run)
	if a.NeedGamma {
		smallWit := make([]smartvectors.SmartVector, len(a.Small))
		smallWit[0] = rPrime
		for i := 1; i < len(a.Small); i++ {
			smallWit[i] = a.Small[i].GetColAssignment(run)
		}
		gamma := run.GetRandomCoinField(a.Gamma.Name)
		rPrime = smartvectors.PolyEval(smallWit, gamma)
	}

	accSmallWit := make([]field.Element, a.LenSmall)
	prev = field.Zero()

	for hashID := 0; hashID < a.LenSmall; hashID++ {
		currExpectedHash := rPrime.Get(hashID)
		accSmallWit[hashID].Mul(&alpha_, &prev)
		accSmallWit[hashID].Add(&accSmallWit[hashID], &currExpectedHash)
		prev = accSmallWit[hashID]
	}

	run.AssignColumn(a.AccSmall.GetColID(), smartvectors.NewRegular(accSmallWit))
	run.AssignLocalPoint(a.AccSmallLast, prev)
}

type SubsampleVerifierAction struct {
	AccLargeLast ifaces.QueryID
	AccSmallLast ifaces.QueryID
}

func (a *SubsampleVerifierAction) Run(run wizard.Runtime) error {
	resAccLast := run.GetLocalPointEvalParams(a.AccLargeLast)
	expectedResAccLast := run.GetLocalPointEvalParams(a.AccSmallLast)
	if resAccLast.Y != expectedResAccLast.Y {
		return fmt.Errorf("linear hashing failed : the ResAcc and ExpectedResAcc do not match on their last inputs %v, %v", resAccLast.Y.String(), expectedResAccLast.Y.String())
	}
	return nil
}

func (a *SubsampleVerifierAction) RunGnark(frontend frontend.API, run wizard.GnarkRuntime) {
	resAccLast := run.GetLocalPointEvalParams(a.AccLargeLast)
	expectedResAccLast := run.GetLocalPointEvalParams(a.AccSmallLast)
	frontend.AssertIsEqual(resAccLast.Y, expectedResAccLast.Y)
}

// Tests that a small table is obtained from subsampling a larger column with a given offset
func CheckSubsample(comp *wizard.CompiledIOP, name string, large, small []ifaces.Column, offset int) {

	// Check that there are as many columns in smalls and in large
	if len(small) != len(large) {
		utils.Panic("small and large should have the same number of columns (%v and %v)", len(small), len(large))
	}

	// Check that there is a non-zero number of columns (testing
	// only in small is enough since we already test that large == small)
	if len(small) == 0 {
		utils.Panic("small is empty")
	}

	numCol := len(small)
	round := 0

	// sanity-check : all the large have the same size and the same for the smalls
	lenSmall := small[0].Size()
	lenLarge := large[0].Size()
	period := lenLarge / lenSmall

	for i := 0; i < numCol; i++ {
		round = utils.Max(round, small[i].Round(), large[i].Round())

		if small[i].Size() != lenSmall {
			utils.Panic("small[%v].Size()!= lenSmall (%v!= %v)", i, small[i].Size(), lenSmall)
		}

		if large[i].Size() != lenLarge {
			utils.Panic("large[%v].Size()!= lenLarge (%v!= %v)", i, large[i].Size(), lenLarge)
		}
	}

	// If the number of columns exceeds one, then we need to
	// add a random coin.
	needGamma := numCol > 1
	var gamma coin.Info

	if needGamma {
		gamma = comp.InsertCoin(round+1, coin.Namef("%v_GAMMA", name), coin.Field)
	}

	alpha := comp.InsertCoin(round+1, coin.Namef("%v_ALPHA", name), coin.Field)

	// Registers the two accumulators
	accSmall := comp.InsertCommit(
		round+1,
		ifaces.ColIDf("%v_ACCUMULATOR_SMALL", name),
		lenSmall,
	)

	accLarge := comp.InsertCommit(
		round+1,
		ifaces.ColIDf("%v_ACCUMULATOR_LARGE", name),
		lenLarge,
	)

	// Also declares the queries on ResAcc and ExpectedResAcc

	r := ifaces.ColumnAsVariable(large[0])
	rPrime := ifaces.ColumnAsVariable(small[0])

	if needGamma {
		// Assign r and rPrime to linear operations
		largeVar := []*symbolic.Expression{}
		smallVar := []*symbolic.Expression{}

		for i := range large {
			largeVar = append(largeVar, ifaces.ColumnAsVariable(large[i]))
			smallVar = append(smallVar, ifaces.ColumnAsVariable(small[i]))
		}

		r = symbolic.NewPolyEval(gamma.AsVariable(), largeVar)
		rPrime = symbolic.NewPolyEval(gamma.AsVariable(), smallVar)
	}

	//
	// Accumulation of the hashes
	//
	// ResAcc[i] = ResAcc[i-1] + Zn,T,−1[(α−1)ResAcc[i-1] + NewState[i]]
	comp.InsertGlobal(
		round+1,
		ifaces.QueryIDf("RESULT_ACCUMULATION_%v", name),
		alpha.AsVariable().
			Sub(symbolic.NewConstant(1)).
			Mul(ifaces.ColumnAsVariable(column.Shift(accLarge, -1))).
			Add(r).
			Mul(variables.NewPeriodicSample(period, offset)).
			Add(ifaces.ColumnAsVariable(column.Shift(accLarge, -1))).
			Sub(ifaces.ColumnAsVariable(accLarge)),
	)

	//
	// Accumulation of the expected results
	//
	// ExpectedResAcc[i] = αExpectedResAcc[i-1] + ExpectedHash[i]
	comp.InsertGlobal(
		round+1,
		ifaces.QueryIDf("SUMSAMPLED_ACCUMULATION_%v", name),
		alpha.AsVariable().
			Mul(ifaces.ColumnAsVariable(column.Shift(accSmall, -1))).
			Add(rPrime).
			Sub(ifaces.ColumnAsVariable(accSmall)),
	)

	//
	// At zero, the ResAcc[i] is equal to 0 if the offset is not zero
	// or equal to r if the offset is 0.
	//
	locExpr := ifaces.ColumnAsVariable(accLarge)
	if offset == 0 {
		locExpr = locExpr.Sub(r)
	}
	comp.InsertLocal(
		round+1,
		ifaces.QueryIDf("RES_ACC_ZERO_%v", name),
		locExpr,
	)

	//
	// At zero, expectedResAcc[i] is equal to expectedHash
	//
	comp.InsertLocal(
		round+1,
		ifaces.QueryIDf("EXPECTED_RES_ACC_ZERO_%v", name),
		ifaces.ColumnAsVariable(accSmall).Sub(rPrime),
	)

	// And we also want to extract the last values of ResAcc and ExpectedResAcc
	accLargeLast := comp.InsertLocalOpening(
		round+1,
		ifaces.QueryIDf("ACC_LARGE_LAST_%v", name),
		column.Shift(accLarge, -1),
	)

	accSmallLast := comp.InsertLocalOpening(
		round+1,
		ifaces.QueryIDf("ACC_SMALL_LAST_%v", name),
		column.Shift(accSmall, -1),
	)

	// And assign them
	comp.RegisterProverAction(round+1, &SubsampleProverAction{
		Large:        large,
		Small:        small,
		AccLarge:     accLarge,
		AccSmall:     accSmall,
		AccLargeLast: accLargeLast.ID,
		AccSmallLast: accSmallLast.ID,
		Gamma:        gamma,
		Alpha:        alpha,
		LenSmall:     lenSmall,
		Period:       period,
		Offset:       offset,
		NeedGamma:    needGamma,
	})

	comp.RegisterVerifierAction(round+1, &SubsampleVerifierAction{
		AccLargeLast: accLargeLast.ID,
		AccSmallLast: accSmallLast.ID,
	})
}
