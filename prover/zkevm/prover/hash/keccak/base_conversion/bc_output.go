package base_conversion

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf"
)

// HashBaseConversionInput stores the input for [newHashBaseConversion]
type HashBaseConversionInput struct {
	// limbs of hash from keccakf,
	// they are in baseB-LE.
	LimbsHiB      []ifaces.Column
	LimbsLoB      []ifaces.Column
	IsActive      ifaces.Column
	MaxNumKeccakF int
	Lookup        lookUpTables
}

type HashBaseConversion struct {
	Inputs *HashBaseConversionInput
	// hash limbs in uint-BE
	LimbsHi, LimbsLo []ifaces.Column
	// it indicates the active part of HashHi/HashLo
	IsActive ifaces.Column
	// the hash result in BE
	HashLo, HashHi ifaces.Column
	Size           int
}

// NewHashBaseConversion declare the intermediate columns,
// and the constraints for changing the hash result from BaseB-LE to uint-BE.
func NewHashBaseConversion(comp *wizard.CompiledIOP, inp HashBaseConversionInput) *HashBaseConversion {

	h := &HashBaseConversion{
		Inputs:   &inp,
		Size:     utils.NextPowerOfTwo(inp.MaxNumKeccakF),
		IsActive: inp.IsActive,
	}
	// declare the columns
	h.DeclareColumns(comp)

	// constraints over decomposition hash to limbs
	h.csDecompose(comp)
	return h
}

// it declares the native columns
func (h *HashBaseConversion) DeclareColumns(comp *wizard.CompiledIOP) {

	createCol := common.CreateColFn(comp, HASH_OUTPUT, h.Size, pragmas.RightPadded)
	h.LimbsHi = make([]ifaces.Column, numLimbsOutput)
	h.LimbsLo = make([]ifaces.Column, numLimbsOutput)

	h.HashLo = createCol("Hash_Lo")
	h.HashHi = createCol("Hash_Hi")

	for i := 0; i < numLimbsOutput; i++ {
		h.LimbsHi[i] = createCol("Limbs_Hi_%v", i)
		h.LimbsLo[i] = createCol("Limbs_Lo_%v", i)
	}
}

// constraints over decomposition of HashHi and HashLo to limbs.
func (h *HashBaseConversion) csDecompose(comp *wizard.CompiledIOP) {

	var (
		sliceLoB = make([]ifaces.Column, numLimbsOutput)
		sliceHiB = make([]ifaces.Column, numLimbsOutput)
		lookup   = h.Inputs.Lookup
	)
	// go from LE to BE order
	copy(sliceLoB, h.Inputs.LimbsLoB[:])
	copy(sliceHiB, h.Inputs.LimbsHiB[:])
	SlicesLeToBe(sliceLoB)
	SlicesLeToBe(sliceHiB)

	// base conversion; from BaseB to uint
	for j := range sliceLoB {
		comp.InsertInclusion(0, ifaces.QueryIDf("BaseConversion_HashOutput_LO_%v", j),
			[]ifaces.Column{lookup.ColBaseBDirty, lookup.ColUint4},
			[]ifaces.Column{sliceLoB[j], h.LimbsLo[j]})

		comp.InsertInclusion(0, ifaces.QueryIDf("BaseConversion_HashOutput_HI_%v", j),
			[]ifaces.Column{lookup.ColBaseBDirty, lookup.ColUint4},
			[]ifaces.Column{sliceHiB[j], h.LimbsHi[j]})

	}

	// recomposition of limbsHi into HashHi
	res := sym.NewConstant(0)
	for k := len(h.LimbsHi) - 1; k >= 0; k-- {
		res = sym.Add(
			sym.Mul(POWER4, res),
			h.LimbsHi[k])
	}

	comp.InsertGlobal(0, ifaces.QueryIDf("RECOMPSE_TO_HASH_HI"),
		sym.Sub(res, h.HashHi),
	)

	// recomposition of limbsLo into HashLo
	res = sym.NewConstant(0)
	for k := len(h.LimbsHi) - 1; k >= 0; k-- {
		res = sym.Add(
			sym.Mul(POWER4, res),
			h.LimbsLo[k])
	}

	comp.InsertGlobal(0, ifaces.QueryIDf("RECOMPSE_TO_HASH_LO"),
		sym.Sub(res, h.HashLo),
	)

}

// It assigns the columns specific to the module.
func (h *HashBaseConversion) Run(
	run *wizard.ProverRuntime,
) {

	var (
		limbsHiB = make([][]field.Element, numLimbsOutput)
		limbsLoB = make([][]field.Element, numLimbsOutput)
		limbsHi  = make([]*common.VectorBuilder, numLimbsOutput)
		limbsLo  = make([]*common.VectorBuilder, numLimbsOutput)
		size     = h.Size
		v        = make([][]field.Element, numLimbsOutput)
		w        = make([][]field.Element, numLimbsOutput)
		hashHi   = common.NewVectorBuilder(h.HashHi)
		hashLo   = common.NewVectorBuilder(h.HashLo)
	)

	for i := range h.Inputs.LimbsHiB {
		limbsHiB[i] = h.Inputs.LimbsHiB[i].GetColAssignment(run).IntoRegVecSaveAlloc()
		limbsLoB[i] = h.Inputs.LimbsLoB[i].GetColAssignment(run).IntoRegVecSaveAlloc()
		limbsHi[i] = common.NewVectorBuilder(h.LimbsHi[i])
		limbsLo[i] = common.NewVectorBuilder(h.LimbsLo[i])
	}

	copy(v, limbsLoB[:])
	copy(w, limbsHiB[:])

	SlicesLeToBe(v)
	SlicesLeToBe(w)

	for i := range h.Inputs.LimbsHiB {
		for row := 0; row < size; row++ {
			limbsLo[i].PushInt(utils.ToInt(BaseBToUint4(v[i][row], keccakf.BaseB)))
			limbsHi[i].PushInt(utils.ToInt(BaseBToUint4(w[i][row], keccakf.BaseB)))
		}

		limbsHi[i].PadAndAssign(run, field.Zero())
		limbsLo[i].PadAndAssign(run, field.Zero())
	}

	// recomposing limbsHi into hashHi,
	// since this is output of the module, it should be big-Endian
	res := vector.Repeat(field.Zero(), size)
	t := make([]field.Element, size)
	for j := len(limbsHi) - 1; j >= 0; j-- {
		vector.ScalarMul(t, res, field.NewElement(POWER4))
		vector.Add(res, t, limbsHi[j].Slice())
	}
	hashHi.PushSliceF(res)
	hashHi.PadAndAssign(run, field.Zero())

	res = vector.Repeat(field.Zero(), size)
	for j := len(limbsHi) - 1; j >= 0; j-- {
		vector.ScalarMul(t, res, field.NewElement(POWER4))
		vector.Add(res, t, limbsLo[j].Slice())
	}
	hashLo.PushSliceF(res)
	hashLo.PadAndAssign(run, field.Zero())
}

// It converts a slices of uint4 from  Bing-endian to little-endian.
// Two adjacent slices are in the same byte, thus their order should be preserved.
func SlicesLeToBe[S ~[]E, E any](s S) {
	i := 0
	a := make([]E, len(s))
	copy(a, s)
	for i < len(s)/2 {
		s[i] = a[len(s)-1-i-1]
		s[i+1] = a[len(s)-1-i]

		s[len(s)-1-i-1] = a[i]
		s[len(s)-1-i] = a[i+1]

		i = i + 2
	}

}
