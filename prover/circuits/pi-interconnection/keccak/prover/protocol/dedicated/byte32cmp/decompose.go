package byte32cmp

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizardutils"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// LimbColumns is a collection of column, all allegedly with short values
// forming the limbs of an arbitrary large number. The Limbs can represent
// the number in big-endian or little-endian order. Note that the endianness is
// meant for the limbs and not for the the bytes here.
type LimbColumns struct {
	// Limbs stores the list of the columns, each one storing an individual
	// limb.
	Limbs []ifaces.Column
	// LimbBitSize is the number of bits stored in each limbs
	LimbBitSize int
	// IsBigEndian returns the endianness of the decomposition.
	IsBigEndian bool
}

// Shift applies the column.Shift operator over the [LimbsColumns] and returns a
// separate object.
func (lc LimbColumns) Shift(offset int) LimbColumns {
	res := LimbColumns{
		Limbs:       make([]ifaces.Column, len(lc.Limbs)),
		LimbBitSize: lc.LimbBitSize,
		IsBigEndian: lc.IsBigEndian,
	}

	for i := range res.Limbs {
		res.Limbs[i] = column.Shift(lc.Limbs[i], offset)
	}

	return res
}

// FuseLimbs constructs a wider LimbColumns object by concatenating the limbs
// of the operands. The function sanity-checks that the operands are compatible.
func FuseLimbs(col ...LimbColumns) LimbColumns {

	var (
		limbs       = []ifaces.Column{}
		numRows     = ifaces.AssertSameLength(col[0].Limbs...)
		limbBitSize = col[0].LimbBitSize
		isBigEndian = col[0].IsBigEndian
	)

	for i := range col {
		if ifaces.AssertSameLength(col[i].Limbs...) != numRows {
			utils.Panic("tried to fuse limbs with inconsistent number of rows")
		}

		if col[i].LimbBitSize != limbBitSize {
			utils.Panic("tried to fuse limbs with inconsistent limb size")
		}

		if col[i].IsBigEndian != isBigEndian {
			utils.Panic("tried to fuse limbs with inconsistent endianness")
		}

		limbs = append(limbs, col[i].Limbs...)
	}

	return LimbColumns{
		Limbs:       limbs,
		LimbBitSize: limbBitSize,
		IsBigEndian: isBigEndian,
	}
}

// Decompose returns [LimbColumn] representing the value stored in col. `col`
// may be either an [ifaces.Column] or a [symbolic.Expression]. The returned
// limb columns is in little-endian order.
//
// The returned limbs are pre-constrained. The function also returns a
// [wizard.ProverAction] to be run during the runtime to assign the generated
// column.
// flag is an option indicating wether the limb is empty and can be ignored;
// zero limbs in the msb part are considered as empty.
// flag can be used to prove the length of elements in 'col' are correct.
func Decompose(comp *wizard.CompiledIOP, col any, numLimbs int, bitPerLimbs int, flag ...ifaces.Column) (LimbColumns, wizard.ProverAction) {

	var (
		ctxName = func(subName string) string {
			return fmt.Sprintf("LIMB_DECOMPOSITION_%v_%v", len(comp.ListCommitments()), subName)
		}
		limbs             = make([]ifaces.Column, numLimbs)
		expr, round, size = wizardutils.AsExpr(col)
		boarded           = expr.Board()
	)

	for i := range limbs {
		// Declare the limbs for the number
		limbs[i] = comp.InsertCommit(
			round,
			ifaces.ColID(ctxName("LIMB_"+strconv.Itoa(i))),
			size,
		)
		// Enforces the range over the limbs
		comp.InsertRange(
			round,
			ifaces.QueryID(ctxName("LIMB_"+strconv.Itoa(i))),
			limbs[i],
			1<<bitPerLimbs,
		)
	}

	// Build the linear combination with powers of 2^bitPerLimbs. The limbs are
	// in "little-endian" order. Namely, the first limb encodes the least
	// significant bits first.
	pow2 := sym.NewConstant(1 << bitPerLimbs)
	acc := sym.NewVariable(limbs[numLimbs-1])

	// if the flag is zero, the limb should not have any effect.
	if len(flag) == numLimbs {
		acc = sym.Mul(acc, flag[numLimbs-1])
		for i := numLimbs - 2; i >= 0; i-- {

			acc = sym.Add(sym.Mul(limbs[i], flag[i]), sym.Mul(pow2, acc))
		}
	} else {
		for i := numLimbs - 2; i >= 0; i-- {
			acc = sym.Add(limbs[i], sym.Mul(pow2, acc))
		}
	}
	// Declare the global constraint
	comp.InsertGlobal(
		round,
		ifaces.QueryIDf("LIMB_DECOMPOSE_GLOBAL_RECONSTRUCTION_%v", len(comp.ListCommitments())),
		sym.Sub(expr, acc),
	)

	res := LimbColumns{
		Limbs:       limbs,
		LimbBitSize: bitPerLimbs,
		IsBigEndian: false,
	}

	return res, &DecompositionCtx{
		Original:   boarded,
		Decomposed: res,
	}

}

// DecompositionCtx implements the [wizard.ProverAction] interface and is
// responsible for assigning the limbs of the column. It should be called
// before trying to use the values of the limbs and after the original column
// has been assigned.
type DecompositionCtx struct {
	Original   sym.ExpressionBoard
	Decomposed LimbColumns
}

// Run implements the [wizard.ProverAction] interface
func (d *DecompositionCtx) Run(run *wizard.ProverRuntime) {

	var (
		numLimbs     = len(d.Decomposed.Limbs)
		bitPerLimbs  = d.Decomposed.LimbBitSize
		totalNumBits = numLimbs * bitPerLimbs
		original     = column.EvalExprColumn(run, d.Original)
		limbsWitness = make([][]field.Element, numLimbs)
		size         = original.Len()
	)

	for i := range limbsWitness {
		// The division by 16 is because 99% of the time, we won't need that
		// data.
		limbsWitness[i] = make([]field.Element, 0, size/16)
	}

	// As eval expr column is defective in giving out optimized smart-vectors,
	// we try to reduce the size of the smart-vector. This empirically
	// improves the performances of the protocol.
	original, _ = smartvectors.TryReduceSizeRight(original)

	for x := range original.IterateCompact() {

		var tmp big.Int
		x.BigInt(&tmp)

		if tmp.BitLen() > totalNumBits {
			utils.Panic(
				"BigRange: cannot prove that the bitLen is smaller than %v : the provided witness has %v bits (%v)",
				totalNumBits, tmp.BitLen(), x.String(),
			)
		}

		for i := 0; i < numLimbs; i++ {
			l := uint64(0)
			for k := i * bitPerLimbs; k < (i+1)*bitPerLimbs; k++ {
				extractedBit := tmp.Bit(k)
				l |= uint64(extractedBit) << (k % bitPerLimbs)
			}
			limbsWitness[i] = append(limbsWitness[i], field.NewElement(l))
		}
	}

	// Then assigns the limbs
	for i := range limbsWitness {
		run.AssignColumn(d.Decomposed.Limbs[i].GetColID(), smartvectors.FromCompactWithShape(original, limbsWitness[i]))
	}
}
