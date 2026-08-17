package bits

import (
	"testing"

	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/stretchr/testify/require"
)

// limbsOf splits a value into big-endian limbs, as [combineElements] reassembles them.
func limbsOf(v uint64) []uint64 {
	limbs := make([]uint64, common.NbElemForHasingU64)
	mask := uint64(1)<<(8*common.LimbBytes) - 1
	for i := common.NbElemForHasingU64 - 1; i >= 0; i-- {
		limbs[i] = v & mask
		v >>= 8 * common.LimbBytes
	}
	return limbs
}

// TestBitDecomposeHeterogeneousLimbs covers all-zero high-order limbs, which
// compact to a Constant while the low limbs stay full. Decomposing on the first
// limb's shape would freeze the bits to row 0 and break recombination.
func TestBitDecomposeHeterogeneousLimbs(t *testing.T) {

	const (
		size    = 8
		numBits = 32
	)

	// Positions from the failing trace; 70000 also exercises the second limb.
	positions := []uint64{260, 29, 70000, 3, 45640, 95, 19228, 7}
	require.Len(t, positions, size)

	limbCols := make([][]field.Element, common.NbElemForHasingU64)
	for i := range limbCols {
		limbCols[i] = make([]field.Element, size)
	}
	for row, p := range positions {
		ls := limbsOf(p)
		for i := range ls {
			limbCols[i][row] = field.NewElement(ls[i])
		}
	}

	var bd *BitDecomposed

	define := func(b *wizard.Builder) {
		packed := make([]ifaces.Column, common.NbElemForHasingU64)
		for i := range packed {
			packed[i] = b.RegisterCommit(ifaces.ColIDf("POS_%v", i), size)
		}
		bd = BitDecompose(b.CompiledIOP, packed, numBits)
	}

	prove := func(run *wizard.ProverRuntime) {
		for i := range limbCols {
			run.AssignColumn(ifaces.ColIDf("POS_%v", i), sv.NewRegular(limbCols[i]))
		}

		bd.Run(run)

		// Bits must track each row's position, not a frozen value.
		for row, p := range positions {
			var got uint64
			for j := range bd.Bits {
				bit := bd.Bits[j].GetColAssignment(run).Get(row)
				if bit.IsOne() {
					got |= uint64(1) << j
				} else {
					require.Truef(t, bit.IsZero(), "row %v bit %v is non-boolean", row, j)
				}
			}
			require.Equalf(t, p, got, "row %v: recombined bits %v != position %v", row, got, p)
		}
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	require.NoError(t, wizard.Verify(comp, proof), "recombination constraints must hold")
}

// TestBitDecomposeRejectsMismatchedBits is the soundness counterpart to the
// happy-path test: it asserts that the bit-recombination constraint binds the
// committed packed limbs to the decomposed bits unconditionally.
//
// Previously the recombination was multiplied by a committed, otherwise
// unconstrained IsPackedLimbNotZero gate; a prover could zero that gate to make
// the identity vacuous and assign bits unrelated to the packed value. For the
// Merkle gadget that decouples the committed accumulator position from the path
// actually walked (false (non-)membership / state-root forgery). Here the
// prover commits one position (committedPos) but assigns boolean bits that
// decompose a different value (pathPos); verification must reject.
func TestBitDecomposeRejectsMismatchedBits(t *testing.T) {

	const (
		size    = 8
		numBits = 32
	)

	const (
		committedPos = uint64(5) // value pinned in the packed limbs
		pathPos      = uint64(2) // value the malicious bits decompose to
	)
	require.NotEqual(t, committedPos, pathPos)

	limbCols := make([][]field.Element, common.NbElemForHasingU64)
	for i := range limbCols {
		limbCols[i] = make([]field.Element, size)
	}
	for row := 0; row < size; row++ {
		ls := limbsOf(committedPos)
		for i := range ls {
			limbCols[i][row] = field.NewElement(ls[i])
		}
	}

	var bd *BitDecomposed

	define := func(b *wizard.Builder) {
		packed := make([]ifaces.Column, common.NbElemForHasingU64)
		for i := range packed {
			packed[i] = b.RegisterCommit(ifaces.ColIDf("POS_%v", i), size)
		}
		bd = BitDecompose(b.CompiledIOP, packed, numBits)
	}

	prove := func(run *wizard.ProverRuntime) {
		for i := range limbCols {
			run.AssignColumn(ifaces.ColIDf("POS_%v", i), sv.NewRegular(limbCols[i]))
		}

		// Adversarial: assign boolean bits that decompose pathPos rather than
		// the committed committedPos. This is exactly what the removed gate used
		// to permit.
		for j := range bd.Bits {
			col := make([]field.Element, size)
			for row := 0; row < size; row++ {
				if pathPos>>j&1 == 1 {
					col[row] = field.One()
				} else {
					col[row] = field.Zero()
				}
			}
			run.AssignColumn(bd.Bits[j].GetColID(), sv.NewRegular(col))
		}
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	require.Error(t, wizard.Verify(comp, proof),
		"recombination must reject bits that do not match the committed packed position")
}
