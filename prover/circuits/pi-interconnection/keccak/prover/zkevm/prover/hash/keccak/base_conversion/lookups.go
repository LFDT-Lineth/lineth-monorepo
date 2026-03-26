package base_conversion

import (
	"encoding/binary"
	"math"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/keccak/keccakf"
)

type lookUpTables struct {

	// columns for base conversion
	ColUint16 ifaces.Column
	ColBaseA  ifaces.Column
	ColBaseB  ifaces.Column

	// columns for base conversion from baseBDirty to 4bit integers
	ColUint4      ifaces.Column
	ColBaseBDirty ifaces.Column
}

// It commits to the lookUp tables used by dataTransfer module.
func NewLookupTables(comp *wizard.CompiledIOP) lookUpTables {
	res := lookUpTables{}

	// table for base conversion (used for converting blocks to what keccakf expect)
	colUint16, colBaseA, colBaseB := baseConversionKeccakBaseX()
	res.ColUint16 = comp.InsertPrecomputed(ifaces.ColIDf("LOOKUP_Uint16"), colUint16)
	res.ColBaseA = comp.InsertPrecomputed(ifaces.ColIDf("LOOKUP_BaseA"), colBaseA)
	res.ColBaseB = comp.InsertPrecomputed(ifaces.ColIDf("LOOKUP_BaseB"), colBaseB)

	// table for base conversion (from BaseBDirty to uint4)
	colUint4, colBaseBDirty := baseConversionKeccakBaseBDirtyToUint4()
	res.ColUint4 = comp.InsertPrecomputed(ifaces.ColIDf("LOOKUP_Uint4"), colUint4)
	res.ColBaseBDirty = comp.InsertPrecomputed(ifaces.ColIDf("LOOKUP_BaseBDirty"), colBaseBDirty)
	return res

}

// convert slices of 16bits to keccak.BaseX (from uint16-BE to baseA_LE/baseB_LE)
func baseConversionKeccakBaseX() (uint16Col, baseACol, baseBCol smartvectors.SmartVector) {
	var u, v, w []field.Element
	for i := 0; i <= math.MaxUint16; i++ {
		u = append(u, field.NewElement(uint64(i)))

		bs := make([]byte, 2)
		// from uint16-BE to baseA_LE/baseB_LE
		binary.LittleEndian.PutUint16(bs, uint16(i)) // #nosec G115 -- Bounded by loop condition
		v = append(v, bytesToBaseX(bs, &keccakf.BaseAFr))
		w = append(w, bytesToBaseX(bs, &keccakf.BaseBFr))
	}
	return smartvectors.NewRegular(u), smartvectors.NewRegular(v), smartvectors.NewRegular(w)
}

func baseConversionKeccakBaseBDirtyToUint4() (
	uint4Col, baseBDirtyCol smartvectors.SmartVector) {
	var u, v []field.Element
	for j := 0; j < keccakf.BaseBPow4; j++ {
		x := field.NewElement(uint64(j))
		uint4 := BaseBToUint4(x, keccakf.BaseB)
		u = append(u, x)
		v = append(v, field.NewElement(uint4))
	}
	n := utils.NextPowerOfTwo(keccakf.BaseBPow4)
	for i := keccakf.BaseBPow4; i < n; i++ {
		u = append(u, u[len(u)-1])
		v = append(v, v[len(v)-1])
	}
	return smartvectors.NewRegular(v), smartvectors.NewRegular(u)
}

func BaseBToUint4(x field.Element, base int) (res uint64) {
	res = 0
	decomposedF := keccakf.DecomposeFr(x, base, 4)

	bitPos := 1
	for i, limb := range decomposedF {
		bit := (limb.Uint64() >> bitPos) & 1
		res |= bit << i
	}

	return res
}
