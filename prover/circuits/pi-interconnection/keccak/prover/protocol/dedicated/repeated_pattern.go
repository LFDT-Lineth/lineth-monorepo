package dedicated

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	commonconstraints "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common/common_constraints"
)

// NewRepeatedPattern creates a new [RepeatedPattern] column. Any can be either a column or
// a sym expression.
func NewRepeatedPattern(comp *wizard.CompiledIOP, round int, pattern []field.Element, isActive ifaces.Column, name string) *RepeatedPattern {

	if len(name) == 0 {
		panic("name cannot be empty")
	}

	name = "REPEATED_PATTERN_" + name

	var (
		size              = isActive.Size()
		period            = len(pattern)
		patternSizePadded = utils.NextPowerOfTwo(period)
		patternPos        = make([]field.Element, period)
	)

	for i := range period {
		patternPos[i] = field.NewElement(uint64(i))
	}

	res := &RepeatedPattern{
		Natural: comp.InsertCommit(round, ifaces.ColID(name)+"_NATURAL", size).(column.Natural),
		Pattern: pattern,
		PatternPrecomp: comp.InsertPrecomputed(
			ifaces.ColID(name)+"_PATTERN",
			smartvectors.RightZeroPadded(pattern, patternSizePadded),
		),
		PatternPosPrecomp: comp.InsertPrecomputed(
			ifaces.ColID(name)+"_PATTERNPOS",
			smartvectors.RightPadded(patternPos, field.NewFromString("-1"), patternSizePadded),
		),
		Counter: *NewCyclicCounter(comp, round, period, isActive),
	}

	pragmas.AddModuleRef(res.PatternPrecomp, name)

	commonconstraints.MustZeroWhenInactive(comp, isActive, res.Natural)

	comp.InsertInclusionConditionalOnIncluded(
		round,
		ifaces.QueryID(name)+"_LOOKUP",
		[]ifaces.Column{
			res.PatternPosPrecomp,
			res.PatternPrecomp,
		},
		[]ifaces.Column{
			res.Counter.Natural,
			res.Natural,
		},
		isActive,
	)

	return res
}

func (rp RepeatedPattern) Assign(run *wizard.ProverRuntime) {

	var (
		isActive      []field.Element
		size          = rp.Counter.ColumnSize
		isFullyActive = rp.Counter.FullyActive
		res           = make([]field.Element, 0, size)
		period        = len(rp.Pattern)
	)

	if !isFullyActive {
		board := rp.Counter.IsActive.Board()
		isActive = column.EvalExprColumn(run, board).IntoRegVecSaveAlloc()
	}

	for i := 0; i < size; i += period {

		if !isFullyActive && isActive[i].IsZero() {
			break
		}

		for j := range rp.Pattern {

			if i+j >= size {
				break
			}

			if !isFullyActive && isActive[i+j].IsZero() {
				break
			}

			res = append(res, rp.Pattern[j])
		}
	}

	run.AssignColumn(rp.Natural.ID, smartvectors.RightZeroPadded(res, size))
	rp.Counter.Assign(run)
}
