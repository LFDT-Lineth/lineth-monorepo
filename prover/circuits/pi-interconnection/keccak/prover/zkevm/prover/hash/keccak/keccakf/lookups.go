package keccakf

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// LookupTables instantiates all the tables used by the keccakf wizard.
// It is used to map between all the different bits representations
// that we use.
type lookUpTables struct {

	// Collects all the 4-bits (clean) tuples (a0, a1, a2, a3) in baseA
	// and in baseB. i.e all numbers of the form (0/1 + 0/1*BaseX + 0/1*BaseX^2
	// ...)
	BaseAClean ifaces.Column
	BaseBClean ifaces.Column

	// Collects all the 4-bits (dirty) tuples from baseA and baseB.
	// Meaning that BaseXDirty contains the (a0 + a1*BaseX + a2*BaseX^2
	// + a3*BaseX^3+...). Here, dirty means each ai belongs to [0,X-1].
	BaseADirty ifaces.Column
	BaseBDirty ifaces.Column

	// Column containing the round constants.
	RC *dedicated.RepeatedPattern

	// DontUsePrevAIota indicates whether we should be looking at the previous
	// of aIota to set a. It consists of a 1, followed by 23 0s, repeated
	// numKeccakF times and then zero padded.
	DontUsePrevAIota *dedicated.HeartBeatColumn
}

// Instantiate and populates the lookup tables for KeccakF.
func newLookUpTables(comp *wizard.CompiledIOP, maxNumKeccakf int) lookUpTables {

	l := lookUpTables{}

	baseADirty, baseBClean := valBaseXToBaseY(BaseA, BaseB, 0)
	baseBDirty, baseAClean := valBaseXToBaseY(BaseB, BaseA, 1)

	// tables for bit representation conversions
	l.BaseAClean = comp.InsertPrecomputed(deriveName("BASE1_CLEAN"), baseAClean)
	l.BaseBClean = comp.InsertPrecomputed(deriveName("BASE2_CLEAN"), baseBClean)
	l.BaseADirty = comp.InsertPrecomputed(deriveName("BASE1_DIRTY"), baseADirty)
	l.BaseBDirty = comp.InsertPrecomputed(deriveName("BASE2_DIRTY"), baseBDirty)

	// This constraint is a small hack fixing an issue unrelated to keccak
	// itself. The above-declared precomputed columns are grouped in a module
	// named "STATIC" and it regroups common an small lookup tables and none of
	// theses constraints is otherwise by a global or local constraint. This
	// creates an edge-case where the corresponding GL module has nothing to
	// prove and this bugs the compilation. To remediate, we add this dummy
	// constraint that the first value of the column is equal to what it should
	// be equal. This is always true and avoids the issue.
	//
	// Note: @alex: I had tried using a global constraint that A = A, but this
	// gets simplified as 0 = 0 and panic instantly because the wizard won't
	// detect any column in the expression.
	comp.InsertLocal(0, "FILLING_FOR_STATIC_MODULE", symbolic.Sub(l.BaseAClean, baseAClean.Get(0)))

	// tables for the RC columns
	l.RC = dedicated.NewRepeatedPattern(
		comp, 0, valRCBase2Pattern(),
		verifiercol.NewConstantCol(
			field.One(),
			numRows(maxNumKeccakf),
			"keccak-rc-pattern",
		),
		"KECCAK_RC_PATTERN",
	)

	// tables to indicate when to use the output of the previous round as
	// input for the next round.
	l.DontUsePrevAIota = dedicated.CreateHeartBeat(comp, 0, keccak.NumRound, 0, verifiercol.NewConstantCol(field.One(), numRows(maxNumKeccakf), "keccak-dont-use-prev-a-iota-heart-beat"))

	return l
}

// Returns the values of the static vectors of l.Base2Clean and l.BaseXDirty.
// CleanBit indicates the position of the bit in `base x dirty` to map to in
// base Y clean. For base A -> B, this will be the zeroes bit and for base B -> A,
// this will be the second bit.
func valBaseXToBaseY(
	baseX, baseY int,
	cleanBit int,
) (baseXDirty, baseYClean smartvectors.SmartVector) {

	// Initializes the returned vectors
	realSize := IntExp(uint64(baseX), numChunkBaseX)
	bxDirty := make([]field.Element, realSize)
	byClean := make([]field.Element, realSize)
	colSize := utils.NextPowerOfTwo(realSize)

	// Runtime assertion to protect the structure of the tables
	if numChunkBaseX != 4 {
		utils.Panic(
			"The tables structure assumes `numChunkBaseX` == 4, but nBS is %v."+
				"change the  table", numChunkBaseX)
	}

	for l3 := 0; l3 < baseX; l3++ {
		d3 := l3 * utils.ToInt(IntExp(utils.ToUint64(baseX), 3))
		c3 := ((l3 >> cleanBit) & 1) * utils.ToInt(IntExp(uint64(baseY), 3))
		for l2 := 0; l2 < baseX; l2++ {
			d2 := l2 * utils.ToInt(IntExp(uint64(baseX), 2))
			c2 := ((l2 >> cleanBit) & 1) * utils.ToInt(IntExp(uint64(baseY), 2))
			for l1 := 0; l1 < baseX; l1++ {
				d1 := l1 * baseX
				c1 := ((l1 >> cleanBit) & 1) * baseY
				for l0 := 0; l0 < baseX; l0++ {
					d0 := l0
					c0 := (l0 >> cleanBit) & 1
					// Coincidentally, dirty1 ranges from 0 to realSize in
					// increasing order.
					dirtyx := d3 + d2 + d1 + d0
					cleany := c3 + c2 + c1 + c0
					bxDirty[dirtyx] = field.NewElement(uint64(dirtyx))
					byClean[dirtyx] = field.NewElement(uint64(cleany))
				}
			}
		}
	}

	// Since, Wizard requires powers-of-two vector length we zero-pad them. Note that
	// (0, 0) does constitute a valid entry in the mapping already.
	return smartvectors.RightZeroPadded(bxDirty, utils.ToInt(colSize)),
		smartvectors.RightZeroPadded(byClean, utils.ToInt(colSize))
}

// valRCBase2Pattern returns the list of the round constant of keccakf in base
// [baseBF].
func valRCBase2Pattern() []field.Element {

	var (
		res    = make([]field.Element, len(keccak.RC))
		baseBF = field.NewElement(uint64(BaseB))
	)

	for i := range res {
		res[i] = U64ToBaseX(keccak.RC[i], &baseBF)
	}

	return res
}
