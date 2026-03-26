package packing

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	iszero "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/packing/dedicated"
)

// It stores the inputs for [newDecomposition] function.
type decompositionInputs struct {
	// parameters for decomposition,
	// the are used to determine the number of slices and their lengths.
	Param       generic.HashingUsecase
	CleaningCtx cleaningCtx
	Name        string
}

// Decomposition struct stores all the intermediate columns required to constraint correct decomposition.
// Decomposition module is responsible for decomposing the cleanLimbs into
// slices of different sizes.
// The max bytes pushed into a slice is
//
//	min (laneSizeBytes, remaining bytes from the limb)
type decomposition struct {
	Inputs *decompositionInputs
	// slices of cleanLimbs
	DecomposedLimbs []ifaces.Column
	//  the length associated with decomposedLimbs
	DecomposedLen []ifaces.Column
	// DecomposedLenPowers = 2^(8*decomposedLen)
	DecomposedLenPowers []ifaces.Column
	// prover action for lengthConsistency;
	// it checks that decomposedLimb is of length decomposedLen.
	PA wizard.ProverAction
	// it indicates the active part of the decomposition module
	IsActive ifaces.Column
	// The Filter is obtained from decomposedLen.
	// Filter = 1 iff decomposedLen != 0.
	Filter []ifaces.Column
	//  the result and  the ProverAction for IsZero().
	ResIsZero []ifaces.Column
	PaIsZero  []wizard.ProverAction
	// Size of the module
	Size int
	// number of slices from the decomposition;
	// it equals the number of  columns in decomposedLimbs.
	NbSlices int
	// max length in the decomposition
	MaxLen int
}

/*
newDecomposition defines the columns and constraints asserting to the following facts:

 1. decomposedLimbs is the decomposition of cleanLimbs

 2. decomposedLimbs is of length decomposedLen
*/
func newDecomposition(comp *wizard.CompiledIOP, inp decompositionInputs) decomposition {

	var (
		size  = inp.CleaningCtx.CleanLimb.Size()
		nbCld = maxLanesFromLimbs(inp.Param.LaneSizeBytes())
	)

	decomposed := decomposition{
		Inputs:   &inp,
		Size:     size,
		NbSlices: nbCld,
		MaxLen:   inp.Param.LaneSizeBytes(),
		// the next assignment guarantees that isActive is from the Activation form.
		IsActive: inp.CleaningCtx.Inputs.Imported.IsActive,
	}

	// Declare the columns
	decomposed.insertCommit(comp)

	for j := 0; j < decomposed.NbSlices; j++ {
		// since they are later used for building the  decomposed.filter.
		commonconstraints.MustZeroWhenInactive(comp, decomposed.IsActive, decomposed.DecomposedLen[j])
		// this guarantees that filter and decompodedLimbs full fill the same constrains.
	}

	// Declare the constraints
	decomposed.csFilter(comp)
	decomposed.csDecomposLen(comp, inp.CleaningCtx.Inputs.Imported)
	decomposed.csDecomposition(comp, inp.CleaningCtx.CleanLimb)

	// check the length consistency between decomposedLimbs and decomposedLen
	lcInputs := dedicated.LcInputs{
		Table:    decomposed.DecomposedLimbs,
		TableLen: decomposed.DecomposedLen,
		MaxLen:   inp.Param.LaneSizeBytes(),
		Name:     inp.Name,
	}
	decomposed.PA = dedicated.LengthConsistency(comp, lcInputs)

	return decomposed
}

// declare the native columns
func (decomposed *decomposition) insertCommit(comp *wizard.CompiledIOP) {

	createCol := common.CreateColFn(comp, DECOMPOSITION+"_"+decomposed.Inputs.Name, decomposed.Size, pragmas.RightPadded)
	for x := 0; x < decomposed.NbSlices; x++ {
		decomposed.DecomposedLimbs = append(decomposed.DecomposedLimbs, createCol("Decomposed_Limbs_%v", x))
		decomposed.DecomposedLen = append(decomposed.DecomposedLen, createCol("Decomposed_Len_%v", x))
		decomposed.DecomposedLenPowers = append(decomposed.DecomposedLenPowers, createCol("Decomposed_Len_Powers_%v", x))
	}

	decomposed.PaIsZero = make([]wizard.ProverAction, decomposed.NbSlices)
	decomposed.ResIsZero = make([]ifaces.Column, decomposed.NbSlices)
	decomposed.Filter = make([]ifaces.Column, decomposed.NbSlices)
	for j := 0; j < decomposed.NbSlices; j++ {
		decomposed.Filter[j] = createCol("Filter_%v", j)
	}

}

// /  Constraints over the form of decomposedLen and decomposedLenPowers;
//   - decomposedLen over a row adds up to NBytes
//   - decomposedLenPowers = 2^(8*decomposedLen)
func (decomposed *decomposition) csDecomposLen(
	comp *wizard.CompiledIOP,
	imported Importation,
) {

	lu := decomposed.Inputs.CleaningCtx.Inputs.Lookup
	// The rows of decomposedLen adds up to NByte; \sum_i decomposedLen[i]=NByte
	s := sym.NewConstant(0)
	for j := range decomposed.DecomposedLimbs {
		s = sym.Add(s, decomposed.DecomposedLen[j])

		// Equivalence of "decomposedLenPowers" with "2^(decomposedLen * 8)"
		comp.InsertInclusion(0,
			ifaces.QueryIDf("%v_Decomposed_Len_Powers_%v", decomposed.Inputs.Name, j), []ifaces.Column{lu.ColNumber, lu.ColPowers},
			[]ifaces.Column{decomposed.DecomposedLen[j], decomposed.DecomposedLenPowers[j]})
	}
	// \sum_i decomposedLen[i]=NByte
	comp.InsertGlobal(0, ifaces.QueryIDf("%v_DecomposedLen_IsNByte", decomposed.Inputs.Name), sym.Sub(s, imported.NByte))

}

// decomposedLimbs is the the decomposition of cleanLimbs
func (decomposed *decomposition) csDecomposition(
	comp *wizard.CompiledIOP, cleanLimbs ifaces.Column) {

	// recomposition of decomposedLimbs into cleanLimbs.
	cleanLimb := ifaces.ColumnAsVariable(decomposed.DecomposedLimbs[0])
	for k := 1; k < decomposed.NbSlices; k++ {
		cleanLimb = sym.Add(sym.Mul(cleanLimb, decomposed.DecomposedLenPowers[k]), decomposed.DecomposedLimbs[k])
	}

	comp.InsertGlobal(0, ifaces.QueryIDf("Decompose_CleanLimbs_%v", decomposed.Inputs.Name), sym.Sub(cleanLimb, cleanLimbs))
}

// /  Constraints over the form of filter and decomposedLen;
//   - filter = 1 iff decomposedLen != 0
func (decomposed decomposition) csFilter(comp *wizard.CompiledIOP) {
	// filtre = 1 iff decomposedLen !=0
	for j := 0; j < decomposed.NbSlices; j++ {
		// s.resIsZero = 1 iff decomposedLen = 0
		decomposed.ResIsZero[j], decomposed.PaIsZero[j] = iszero.IsZero(comp, decomposed.DecomposedLen[j]).GetColumnAndProverAction()
		// s.filter = (1 - s.resIsZero), this enforces filters to be binary.
		comp.InsertGlobal(0, ifaces.QueryIDf("%v_%v_%v", decomposed.Inputs.Name, "IS_NON_ZERO", j),
			sym.Sub(decomposed.Filter[j],
				sym.Sub(1, decomposed.ResIsZero[j])),
		)
	}

	// filter[0] = 1 over is Active.
	// this ensures that the first slice of the limb falls in the first column.
	comp.InsertGlobal(0, ifaces.QueryIDf("%v_FIRST_SLICE_IN_FIRST_COLUMN", decomposed.Inputs.Name),
		sym.Sub(
			decomposed.Filter[0], decomposed.IsActive),
	)

}

// assign the columns specific to the module.
func (decomposed *decomposition) Assign(run *wizard.ProverRuntime) {
	decomposed.assignMainColumns(run)

	// assign s.filter
	for j := 0; j < decomposed.NbSlices; j++ {
		decomposed.PaIsZero[j].Run(run)

		var (
			filter        = decomposed.Filter[j]
			compactFilter = make([]field.Element, 0)
			a             = decomposed.ResIsZero[j].GetColAssignment(run)
			one           = field.One()
		)

		for ai := range a.IterateCompact() {
			var f field.Element
			f.Sub(&one, &ai)
			compactFilter = append(compactFilter, f)
		}

		run.AssignColumn(filter.GetColID(), smartvectors.FromCompactWithShape(a, compactFilter))
	}

	// assign Iszero()
	decomposed.PA.Run(run)
}

// get number of slices for the decomposition
func maxLanesFromLimbs(laneBytes int) int {
	if laneBytes > MAXNBYTE {
		return 2
	} else {
		return (MAXNBYTE / laneBytes) + 1
	}
}

// it builds the inputs for [newDecomposition]
func getDecompositionInputs(cleaning cleaningCtx, pckParam PackingInput) decompositionInputs {
	decInp := decompositionInputs{
		CleaningCtx: cleaning,
		Param:       pckParam.PackingParam,
		Name:        pckParam.Name,
	}
	return decInp
}

// it assigns the main columns (not generated via an inner ProverAction)
func (decomposed *decomposition) assignMainColumns(run *wizard.ProverRuntime) {
	var (
		imported   = decomposed.Inputs.CleaningCtx.Inputs.Imported
		cleanLimbs = decomposed.Inputs.CleaningCtx.CleanLimb.GetColAssignment(run)
		nByte      = imported.NByte.GetColAssignment(run)

		// Assign the columns decomposedLimbs and decomposedLen
		decomposedLen   [][]field.Element
		decomposedLimbs = make([][]field.Element, decomposed.NbSlices)

		// These are needed for sanity-checking the implementation which
		// crucially relies on the fact that the input vectors are post-padded.
		cleanLimbsStartRange, _ = smartvectors.CoWindowRange(cleanLimbs)
		nByteStartRange, _      = smartvectors.CoWindowRange(nByte)
	)

	if cleanLimbsStartRange > 0 || nByteStartRange > 0 {
		utils.Panic("the implementation relies on the fact that the input vectors are post-padded, but their range start after 0, range-start:[%v %v]", cleanLimbsStartRange, nByteStartRange)
	}

	// assign row-by-row
	decomposedLen = cutUpToMax(nByte, decomposed.NbSlices, decomposed.MaxLen)

	for j := range decomposedLimbs {
		decomposedLimbs[j] = make([]field.Element, len(decomposedLen[0]))
	}

	for i := 0; i < len(decomposedLen[0]); i++ {

		cleanLimb := cleanLimbs.Get(i)
		nByte := nByte.Get(i)

		// i-th row of DecomposedLen
		var lenRow []int
		for j := 0; j < decomposed.NbSlices; j++ {
			lenRow = append(lenRow, utils.ToInt(decomposedLen[j][i].Uint64()))
		}

		// populate DecomposedLimb
		decomposedLimb := decomposeByLength(cleanLimb, field.ToInt(&nByte), lenRow)

		for j := 0; j < decomposed.NbSlices; j++ {
			decomposedLimbs[j][i] = decomposedLimb[j]
		}
	}

	for j := 0; j < decomposed.NbSlices; j++ {
		run.AssignColumn(decomposed.DecomposedLimbs[j].GetColID(), smartvectors.RightZeroPadded(decomposedLimbs[j], decomposed.Size))
		run.AssignColumn(decomposed.DecomposedLen[j].GetColID(), smartvectors.RightZeroPadded(decomposedLen[j], decomposed.Size))
	}

	// powersOf256 stores the successive powers of 256. This is used to compute
	// the decomposedLenPowers.
	powersOf256 := make([]field.Element, decomposed.MaxLen+1)
	for i := range powersOf256 {
		powersOf256[i].Exp(field.NewElement(256), big.NewInt(int64(i)))
	}

	// assign decomposedLenPowers
	for j := range decomposedLen {
		decomposedLenPowers := make([]field.Element, len(decomposedLen[j]))
		for i := range decomposedLen[0] {
			decomLen := field.ToInt(&decomposedLen[j][i])
			decomposedLenPowers[i] = powersOf256[decomLen]
		}
		run.AssignColumn(decomposed.DecomposedLenPowers[j].GetColID(), smartvectors.RightPadded(decomposedLenPowers, field.One(), decomposed.Size))
	}
}

// It receives an (set of) integer number N and outputs (n_1,...,n_nbChunk) such that
// N = \sum_{i < nbChunk} n_i
// n_i = min ( max, N - \sum_{j<i} n_j)
func cutUpToMax(nByte smartvectors.SmartVector, nbChunk, max int) [][]field.Element {

	var (
		missing = uint64(max)
		b       = make([][]field.Element, nbChunk)
	)

	for curr := range nByte.IterateSkipPadding() {

		// a is filled with nbChunk elements. Theses elements are the decomposition
		// of 'curr' into of integers less or equal to "max". For instance, curr = 10
		// and max=3 would give a and nChunk=5 would give a = [3, 3, 3, 1, 0].
		//
		// missing is a sort of carry that propagate the remainder of the decomposition
		// to the next line. With our example, the missing value would be 2 at the end
		// of the decomposition. And the next line would start with a 2 and then keep
		// going with 3s.
		var a = make([]uint64, 0, nbChunk)
		curr := curr.Uint64()
		for curr != 0 {
			if curr >= missing {
				a = append(a, missing)
				curr = curr - missing
				missing = uint64(max)
			} else {
				a = append(a, curr)
				missing = missing - curr
				curr = 0
			}
		}

		for len(a) < nbChunk {
			a = append(a, 0)
		}

		for j := 0; j < nbChunk; j++ {
			b[j] = append(b[j], field.NewElement(a[j]))
		}
	}

	return b
}

// It receives the length of the slices and decompose the element to the slices with the given lengths.
func decomposeByLength(cleanLimb field.Element, nBytes int, givenLen []int) (slices []field.Element) {

	//sanity check
	s := 0
	for i := range givenLen {
		s = s + givenLen[i]
	}
	if s != nBytes {
		utils.Panic("input can not be decomposed to the given lengths s=%v nBytes=%v", s, nBytes)
	}

	b := cleanLimb.Bytes()
	bytes := b[32-nBytes:]
	slices = make([]field.Element, len(givenLen))
	for i := range givenLen {
		if givenLen[i] == 0 {
			slices[i] = field.Zero()
		} else {
			b := bytes[:givenLen[i]]
			slices[i].SetBytes(b)
			bytes = bytes[givenLen[i]:]
		}
	}

	return slices

}
