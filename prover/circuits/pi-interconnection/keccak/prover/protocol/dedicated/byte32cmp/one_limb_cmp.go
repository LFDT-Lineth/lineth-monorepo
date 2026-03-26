package byte32cmp

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// OneLimbCmpCtx is a wizard subroutine responsible for computing the comparison
// of two columns storing each a small value.
type OneLimbCmpCtx struct {

	// A and B are the two columns being compared. The two columns should be
	// preemptively constrained to be somewhat small enough so that they are
	// lookup-able. There is A hard-limit at 64 bits.
	A, B ifaces.Column

	// NumBits is the maximum number of bits the absolute value of the
	// difference between A and B can use. There is a hard limit at 64. It
	// should be positive also.
	NumBits int

	// Greater, Lower and Equal are binary and mutually exclusive columns
	// indicating respectively if:
	//
	// 		- Greater[i]	= 1 => 	A[i] > B[i]
	// 		- Equal[i] 		= 1 => 	A[i] == B[i]
	// 		- Lower[i] 		= 1 => 	A[i] < B[i]
	IsGreater, IsLower, IsEqual ifaces.Column

	// RangeChecked is an internal column which is assigned to:
	//
	//  	- Greater[i]	= 1 => 	RangeChecked[i] = A[i] - B[i]
	// 		- Equal[i] 		= 1 => 	RangeChecked[i] = 0
	// 		- Lower[i] 		= 1 => 	RangeChecked[i] = B[i] - A[i]
	RangeChecked ifaces.Column

	// internalProverActions is a list of prover action coming from stuffs
	// defered from other dedicated wizards. This includes the computation of
	// isEqual which uses the IsZero dedicated wizard.
	InternalProverAction []wizard.ProverAction
}

// CmpSmallCols computes the comparison of two columns which are allegedly short
// the function does not include range-checks over a and b and it is the
// responsibility of the caller to ensure that they are small.
//
// The function returns g, e, l. There binary and mutually-exclusive indicator
// columns indicating the rows is which (g: a is greater than b, e: a and b are
// equals, l: a is smaller than b).
func CmpSmallCols(comp *wizard.CompiledIOP, a, b ifaces.Column, numBits int) (g, e, l ifaces.Column, pa wizard.ProverAction) {

	if a.Size() != b.Size() {
		utils.Panic("a and b have inconsistent sizes: %v != %v", a.Size(), b.Size())
	}

	if numBits > 64 {
		utils.Panic("the number of bits is too large (%v > 64)", numBits)
	}

	var (
		size       = a.Size()
		round      = max(a.Round(), b.Round())
		identifier = len(comp.Columns.AllKeys())
		ctxName    = func(subName string) string {
			return fmt.Sprintf("CMP_ONE_LIMB_CTX_%v_%v", identifier, subName)
		}

		// Note: that isEqual is already constrained to be correctly formed.
		isEqualCtx   = dedicated.IsZero(comp, sym.Sub(a, b)).WithPaddingVal(field.One())
		isEqual      = isEqualCtx.IsZero
		rangeChecked = comp.InsertCommit(round, ifaces.ColID(ctxName("RANGE_CHECKED")), size)
		isGreater    = comp.InsertCommit(round, ifaces.ColID(ctxName("IS_GREATER")), size)
		isLower      = comp.InsertCommit(round, ifaces.ColID(ctxName("IS_LOWER")), size)
	)

	res := &OneLimbCmpCtx{
		A: a, B: b, NumBits: numBits,
		IsGreater: isGreater, IsEqual: isEqual, IsLower: isLower,
		InternalProverAction: []wizard.ProverAction{isEqualCtx},
		RangeChecked:         rangeChecked,
	}

	comp.InsertRange(
		round,
		ifaces.QueryID(ctxName("RANGE_CHECK")),
		rangeChecked,
		1<<numBits,
	)

	comp.InsertGlobal(
		round,
		ifaces.QueryID(ctxName("IS_GREATER_IS_BINARY")),
		sym.Mul(isGreater, sym.Sub(isGreater, 1)),
	)

	comp.InsertGlobal(
		round,
		ifaces.QueryID(ctxName("IS_LOWER_IS_BINARY")),
		sym.Mul(isLower, sym.Sub(isLower, 1)),
	)

	comp.InsertGlobal(
		round,
		ifaces.QueryID(ctxName("FLAGS_ARE_MUTUALLY_EXCLUSIVE")),
		sym.Add(isLower, isEqual, isGreater, -1),
	)

	comp.InsertGlobal(
		round,
		ifaces.QueryID(ctxName("RANGE_CHECKED_CORRECTLY_ASSIGNED")),
		sym.Sub(
			rangeChecked,
			sym.Mul(isLower, sym.Sub(b, a)),
			sym.Mul(isGreater, sym.Sub(a, b)),
		),
	)

	return isGreater, isEqual, isLower, res
}

// Run implements the [wizard.ProverAction] interface.
func (ol *OneLimbCmpCtx) Run(run *wizard.ProverRuntime) {

	// This assigns isEqual. Therefore, only isGreater, isLower and rangeChecked
	// need to be assigned.
	ol.InternalProverAction[0].Run(run)

	var (
		a          = ol.A.GetColAssignment(run)
		b          = ol.B.GetColAssignment(run)
		length     = a.Len()
		g          = make([]field.Element, length)
		l          = make([]field.Element, length)
		rc         = make([]field.Element, length)
		minOffsetA = min(column.StackOffsets(ol.A))
		minOffsetB = min(column.StackOffsets(ol.B))
		maxOffsetA = max(column.StackOffsets(ol.A))
		maxOffsetB = max(column.StackOffsets(ol.B))
	)

	for i := 0; i < a.Len(); i++ {

		var (
			aiF, biF = a.Get(i), b.Get(i)
			ai, bi   = aiF.Uint64(), biF.Uint64()
		)

		// If we are in an area where A - B is outside the range of the
		// constraints due to the column offset, then we can put any value in
		// theory but with the limitless prover we have to account for the fact
		// that the columns are going to be extended and it is safer to use the
		// same value as initial/final constrained value so that the column
		// extension stays valid.
		if minOffsetA < 0 && i < -minOffsetA {
			aiF = a.Get(-minOffsetA)
			ai = aiF.Uint64()
		}

		if minOffsetB < 0 && i < -minOffsetB {
			biF = b.Get(-minOffsetB)
			bi = biF.Uint64()
		}

		if maxOffsetA > 0 && i >= a.Len()-maxOffsetA {
			aiF = a.Get(a.Len() - maxOffsetA - 1)
			ai = aiF.Uint64()
		}

		if maxOffsetB > 0 && i >= b.Len()-maxOffsetB {
			biF = b.Get(b.Len() - maxOffsetB - 1)
			bi = biF.Uint64()
		}

		if ai > bi {
			g[i].SetOne()
			rc[i].Sub(&aiF, &biF)
		}

		if ai < bi {
			l[i].SetOne()
			rc[i].Sub(&biF, &aiF)
		}
	}

	run.AssignColumn(ol.IsGreater.GetColID(), smartvectors.NewRegular(g))
	run.AssignColumn(ol.IsLower.GetColID(), smartvectors.NewRegular(l))
	run.AssignColumn(ol.RangeChecked.GetColID(), smartvectors.NewRegular(rc))
}
