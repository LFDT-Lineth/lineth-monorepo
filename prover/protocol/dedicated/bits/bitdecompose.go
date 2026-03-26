package bits

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// BitDecomposed represents the output of a bit decomposition of
// a slice of columns. The struct implements the [wizard.ProverAction] interface
// to self-assign itself.
type BitDecomposed struct {
	// Packed is the input of the bit-decomposition
	Packed []ifaces.Column
	// Bits lists the decomposed bits of the "packed" column in LSbit
	// order.
	Bits                []ifaces.Column
	IsPackedLimbNotZero []ifaces.Column
}

// BitDecompose generates a bit decomposition of a column and returns
// a struct that implements the [wizard.ProverAction] interface to
// self-assign itself.
func BitDecompose(comp *wizard.CompiledIOP, packed []ifaces.Column, numBits int) *BitDecomposed {

	var (
		round = packed[0].Round()
		bd    = &BitDecomposed{
			Packed: packed,
			Bits:   make([]ifaces.Column, numBits),
		}
	)

	bitExpr := []*symbolic.Expression{}

	for j := 0; j < numBits; j++ {
		bd.Bits[j] = comp.InsertCommit(round, ifaces.ColIDf("%v_BIT_%v", packed[0].GetColID(), j), packed[0].Size(), true)
		MustBeBoolean(comp, bd.Bits[j])
		bitExpr = append(bitExpr, symbolic.NewVariable(bd.Bits[j]))
	}

	// This constraint ensures that the recombined bits are equal to the
	// original column.
	for i := 0; i*16 < numBits; i++ {
		bd.IsPackedLimbNotZero = append(bd.IsPackedLimbNotZero, comp.InsertCommit(round, ifaces.ColIDf("IS_PACKED_NOT_ZERO_%v", i), packed[0].Size(), true))
	}

	for i := len(packed) - 1; i >= 0; i-- {
		ind := len(packed) - i - 1

		if ind*16 >= numBits {
			continue
		}

		s := bitExpr[ind*16:]
		if len(s) > 16 {
			s = s[:16]
		}

		comp.InsertGlobal(
			round,
			ifaces.QueryIDf("%v_BIT_RECOMBINATION", packed[i].GetColID()),
			symbolic.Mul(
				bd.IsPackedLimbNotZero[ind],
				symbolic.Sub(
					packed[i],
					symbolic.NewPolyEval(symbolic.NewConstant(2), s),
				),
			),
		)
	}

	return bd
}

// Run implements the [wizard.ProverAction] interface and assigns the bits
// columns
func (bd *BitDecomposed) Run(run *wizard.ProverRuntime) {
	bits := make([][]field.Element, len(bd.Bits))

	// Obtain packed elements from
	elements := make([][]field.Element, 0, len(bd.Packed))
	for i, packed := range bd.Packed {
		// ind mirrors the mapping in BitDecompose's constraint loop:
		// packed[len-1] (LSB limb) → IsPackedLimbNotZero[0], etc.
		ind := len(bd.Packed) - 1 - i

		v := packed.GetColAssignment(run)
		var packedElements []field.Element
		var packedElementsIsZero []field.Element

		for colElement := range v.IterateCompact() {
			packedElements = append(packedElements, colElement)

			isPackedLimbNotZero := field.One()
			if colElement.IsZero() {
				isPackedLimbNotZero = field.Zero()
			}

			packedElementsIsZero = append(packedElementsIsZero, isPackedLimbNotZero)
		}

		// Guard mirrors the `if ind*16 >= numBits { continue }` in BitDecompose:
		// packed limbs beyond numBits have no corresponding IsPackedLimbNotZero entry.
		if ind < len(bd.IsPackedLimbNotZero) {
			run.AssignColumn(bd.IsPackedLimbNotZero[ind].GetColID(), smartvectors.RightZeroPadded(packedElementsIsZero, bd.Packed[0].Size()))
		}

		elements = append(elements, packedElements)
	}

	for i := range elements[0] {
		var el []field.Element
		for j := range elements {
			el = append(el, elements[j][i])
		}

		x := combineElements(el)

		if !x.IsUint64() {
			panic("can handle 64 bits at most")
		}

		xNum := x.Uint64()
		for j := range len(bd.Bits) {
			if xNum>>j&1 == 1 {
				bits[j] = append(bits[j], field.One())
			} else {
				bits[j] = append(bits[j], field.Zero())
			}
		}
	}

	for i, bitCol := range bd.Bits {
		run.AssignColumn(bitCol.GetColID(), smartvectors.FromCompactWithShape(bd.Packed[0].GetColAssignment(run), bits[i]))
	}
}

// MustBeBoolean adds a constraint ensuring that the input is a boolean
// column. The constraint is named after the column.
func MustBeBoolean(comp *wizard.CompiledIOP, col ifaces.Column) {
	// This adds the constraint x^2 = x
	comp.InsertGlobal(
		col.Round(),
		ifaces.QueryID(col.GetColID())+"_IS_BOOLEAN",
		symbolic.Sub(col, symbolic.Mul(col, col)))
}

// combineElements combines an array of limb elements into a single element.
// It extracts a specific suffix of bytes from each field.Element
// in the input slice and concatenates them into a single byte slice.
// It then uses this concatenated byte slice to initialize and return a new
// field.Element.
func combineElements(elements []field.Element) field.Element {
	var bytes []byte
	for _, element := range elements {
		elementBytes := element.Bytes()
		bytes = append(bytes, elementBytes[len(elementBytes)-common.LimbBytes:]...)
	}

	var res field.Element
	res.SetBytes(bytes)

	return res
}
