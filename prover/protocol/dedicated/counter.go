package dedicated

import (
	"fmt"
	"math"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
)

// NewCyclicCounter creates a structured [CyclicCounter]. When passing period
// = math.MaxInt it turns into a non-periodic counter.
func NewCyclicCounter(comp *wizard.CompiledIOP, round, period int, isActiveAny any) *CyclicCounter {

	var (
		isActive, fullyActive, size = cleanIsActive(isActiveAny)
		numCols                     = comp.Columns.NumEntriesTotal()
		// The appending of the round to the name is necessary to disambiguate
		// the name of the column when serializing. This is particularly useful
		// for self-recursion columns.
		name = fmt.Sprintf("CYCLIC_COUNTER_%v_%v_%v", numCols, period, round)
	)

	var (
		rc = &CyclicCounter{
			IsActive:    isActive,
			Period:      period,
			FullyActive: fullyActive,
			Natural:     comp.InsertCommit(round, ifaces.ColID(name+"_COUNTER"), size, true).(column.Natural),
			ColumnSize:  size,
		}
	)

	if !fullyActive {
		commonconstraints.MustZeroWhenInactive(comp, isActive, rc.Natural)
	}

	comp.InsertLocal(
		round,
		ifaces.QueryID(name+"_COUNTER_STARTS_AT_ZERO"),
		sym.NewVariable(rc.Natural),
	)

	comp.InsertGlobal(
		round,
		ifaces.QueryID(name+"COUNTER_INCREASES"),
		sym.Mul(
			column.ShiftExpr(isActive, 1),
			sym.Sub(rc.Natural, period-1),
			sym.Sub(
				column.Shift(rc.Natural, 1),
				rc.Natural,
				1,
			),
		),
	)

	// If the size if really big,
	if size == math.MaxInt {
		return rc
	}

	var (
		cptIsEndOfPeriod *IsZeroCtx
	)

	if !fullyActive {
		cptIsEndOfPeriod = IsZeroMask(comp, sym.Sub(rc.Natural, period-1), isActive)
	} else {
		cptIsEndOfPeriod = IsZero(comp, sym.Sub(rc.Natural, period-1))
	}

	comp.InsertGlobal(
		round,
		ifaces.QueryID(name+"_COUNTER_RESET"),
		sym.Mul(
			column.Shift(rc.Natural, 1),
			cptIsEndOfPeriod.IsZero,
		),
	)

	rc.PAs = append(rc.PAs, cptIsEndOfPeriod)
	rc.Reset = cptIsEndOfPeriod.IsZero

	return rc
}

// Assign runs the prover steps and assign the CounterColumn
func (rc CyclicCounter) Assign(run *wizard.ProverRuntime) {

	var (
		size     = rc.Natural.Size()
		res      = make([]field.Element, size)
		isActive []field.Element
	)

	if !rc.FullyActive {
		board := rc.IsActive.Board()
		isActive = column.EvalExprColumn(run, board).IntoRegVecSaveAlloc()
	}

	for i := 0; i < size; i++ {

		if !rc.FullyActive && isActive[i].IsZero() {
			res = res[:i:i]
			break
		}

		n := i
		// In theory, protecting the PositiveMod with this check should not be
		// necessary but we consider it safer to not risk having overflow issue
		// in case rc.Period = [math.MaxInt]
		if rc.Period < size {
			n = utils.PositiveMod(i, rc.Period)
		}
		res[i].SetUint64(uint64(n))
	}

	run.AssignColumn(rc.Natural.ID, smartvectors.RightZeroPadded(res, size))

	for i := range rc.PAs {
		rc.PAs[i].Run(run)
	}
}
