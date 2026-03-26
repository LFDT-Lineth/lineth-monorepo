package dedicated

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// HeartBeatColumn is an implementation of the [ifaces.Column] interface that
// represents a structured column. The structured column pulsates a "1" every
// "period" rows with the provided offset.
type HeartBeatColumn struct {
	Natural column.Natural
	Counter CyclicCounter
	Offset  int
	PAs     []wizard.ProverAction
}

// CyclicCounter is a column counting and periodically resetting. The
// column type is implemented as a composite of [column.Natural]. The
// column can auto-assign if [GetColAssignment] is called before the
// column is explictly assigned.
//
// The counter is furthermore controlled by an 'isActive' column: the values
// of the column are forced to zero if isActive=0.
type CyclicCounter struct {
	Natural     column.Natural
	Reset       ifaces.Column
	Period      int
	IsActive    *sym.Expression
	FullyActive bool
	ColumnSize  int
	PAs         []wizard.ProverAction
}

// RepeatedPattern is a column populated with an ever-repeated pattern.
// The pattern may have a non-zero power of two size. The column is
// subjected to an "is-active" column.
type RepeatedPattern struct {
	Natural           column.Natural
	Pattern           []field.Element
	Counter           CyclicCounter
	PatternPrecomp    ifaces.Column
	PatternPosPrecomp ifaces.Column
}

// CreateHeartBeat creates a self-constrained column that repeats of "1",
// followed by "period" zero. The period does not have to be a power of
// two. CreateHeartBeat expands over [column.Natural] and lazily
// self-assign itself when its assignment is required. The function is
// masked by an [IsActive] column which control it is zero-padded.
//
// The function also defines and assign underlying columns
func CreateHeartBeat(comp *wizard.CompiledIOP, round, period, offset int, isActive any) *HeartBeatColumn {

	hb := &HeartBeatColumn{
		Offset:  offset,
		Counter: *NewCyclicCounter(comp, round, period, isActive),
	}

	if offset == -1 || offset == period-1 {
		hb.Natural = hb.Counter.Reset.(column.Natural)
		return hb
	}

	var (
		_, isFullyActive, _ = cleanIsActive(isActive)
		cptIsZero           *IsZeroCtx
	)

	if !isFullyActive {
		cptIsZero = IsZeroMask(comp, sym.Sub(hb.Counter.Natural, offset), isActive)
	} else {
		cptIsZero = IsZero(comp, sym.Sub(hb.Counter.Natural, offset))
	}

	hb.PAs = append(hb.PAs, cptIsZero)
	hb.Natural = cptIsZero.IsZero.(column.Natural)

	return hb
}

func (hb HeartBeatColumn) Assign(run *wizard.ProverRuntime) {
	hb.Counter.Assign(run)
	for _, pa := range hb.PAs {
		pa.Run(run)
	}
}

// cleanIsActive analyzes isActive and returns it in the form of
// an expression. The function also returns a flag [isFullActive]
// indicating whether the isActive argument resolves into a
// constant equal to 1. The function returns also the resolved
// size corresponding to isActive.
func cleanIsActive(isActiveAny any) (isActive *sym.Expression, fullyActive bool, size int) {

	switch act := isActiveAny.(type) {
	case *sym.Expression:
		isActive = act
		board := act.Board()
		size = column.ExprIsOnSameLengthHandles(&board)
	case verifiercol.ConstCol:
		isActive = sym.NewConstant(act.F)
		fullyActive = act.F.IsOne()
		size = act.Size()
	case ifaces.Column:
		isActive = sym.NewVariable(act)
		size = act.Size()
	default:
		utils.Panic("unexpected type for isActive: %v\n", isActiveAny)
	}

	return isActive, fullyActive, size
}
