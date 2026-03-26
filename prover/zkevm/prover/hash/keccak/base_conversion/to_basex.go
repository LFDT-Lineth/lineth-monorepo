package baseconversion

/* iokeccakf package prepares the input and output for keccakf.
The inputs to keccakf are blocks in BaseA or BaseB (little-endian).
The output from keccakf is the hash result in baseB (little-endian).
 Thus, the implementation applies a base conversion over the blocks;
going from uint to BaseA/BaseB. Also, a base conversion over the hash result,
going from BaseB to uint. */

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

const (
	MAXNBYTE                  = 2 // maximum number of bytes due to the field size.
	KECCAKF_INPUT_PREPARATION = "KECCAKF_INPUT_PREPARATION"
)

// lookup holds the lookup tables for base conversion.
type lookup struct {
	ColMAXNBYTE ifaces.Column     // holds the uint values from 0 to 2^(8*MAXNBYTE)-1
	ColBasex    [][]ifaces.Column // holds the baseA representation of uint values
}

// @ azam decide if to add the constraint on isBaseX.
// ToBaseXInputs stores the data [NewToBaseX] requireds as inputs.
type ToBaseXInputs struct {
	// for keccakf each lane is 64 bits (8 bytes),
	// but due to the MAXNBYTE constraint, a lane is stored over multiple rows.
	Lane ifaces.Column
	// it is 1 when the lane is active
	IsLaneActive ifaces.Column
	// bases for conversion
	BaseX []int
	// number of bits per baseX (baseA or baseB)
	// the constraint is that BaseX^nbBitPerBaseX < field.Modulus and nbBitPerBaseX is a power of 2.
	NbBitsPerBaseX int
	// the flag indicating where to convert the lane and to with base.
	IsBaseX []ifaces.Column
}

// The submodule ToBaseX implements the base conversion over
// the inputs to the keccakf (i.e., blocks/lanes), in order to export them to the keccakf.
// The lanes from the first block of hash should be in baseA and others are in baseB.
type ToBaseX struct {
	Inputs *ToBaseXInputs
	LaneX  []ifaces.Column // lanes from first block in baseA, others in baseB
	Lookup lookup          // lookup table for base conversion
	Size   int             // number of rows
}

// NewToBaseX declare the intermediate columns,
// and the constraints for changing the blocks in base uint64 into baseA/baseB.
// Note : it does not check the validity of the inputs. Particularly it does not check that
// IsBaseX are MustBeMutuallyExclusiveBinaryFlags w.r.t the IsLaneActive column.
func NewToBaseX(
	comp *wizard.CompiledIOP,
	inp ToBaseXInputs,
) *ToBaseX {

	var (
		nbSlicesBaseX = MAXNBYTE * 8 / inp.NbBitsPerBaseX

		b = &ToBaseX{
			Inputs: &inp,
			Size:   inp.Lane.Size(),
			Lookup: lookup{
				ColBasex: make([][]ifaces.Column, len(inp.BaseX)),
			},
			LaneX: make([]ifaces.Column, nbSlicesBaseX),
		}
		// declare the columns
		createCol = common.CreateColFn(comp, KECCAKF_INPUT_PREPARATION, b.Size, pragmas.RightPadded)
	)
	for j := 0; j < nbSlicesBaseX; j++ {
		b.LaneX[j] = createCol("LaneX_%v", j)
	}

	for j := 0; j < len(b.Inputs.IsBaseX); j++ {
		b.Lookup.ColBasex[j] = make([]ifaces.Column, nbSlicesBaseX)
	}

	// table for base conversion (used for converting blocks to what keccakf expect)
	colMAXNBYTE, colBasex := createLookupTablesBaseX(b.Inputs.BaseX, nbSlicesBaseX)

	b.Lookup.ColMAXNBYTE = comp.InsertPrecomputed(ifaces.ColIDf("LOOKUP_MAXNBYTE"), colMAXNBYTE)
	for k := range b.Inputs.BaseX {
		for i := 0; i < nbSlicesBaseX; i++ {
			b.Lookup.ColBasex[k][i] = comp.InsertPrecomputed(ifaces.ColIDf("LOOKUP_BaseA_%v_%v", k, i), colBasex[k][i])
		}
	}

	// if isBaseX = 1  ---> convert to basex
	// otherwise convert to keccak.BaseB
	for k := range b.Inputs.IsBaseX {
		comp.InsertInclusionConditionalOnIncluded(0,
			ifaces.QueryIDf("BaseConversion_Into_BaseX_%v", k),
			append(b.Lookup.ColBasex[k], b.Lookup.ColMAXNBYTE),
			append(b.LaneX, b.Inputs.Lane),
			b.Inputs.IsBaseX[k],
		)
	}

	return b
}

// assign column isFromFirstBlock and laneX
func (b *ToBaseX) Run(run *wizard.ProverRuntime) {
	var (
		isBaseX = make([][]field.Element, len(b.Inputs.BaseX))
	)

	for i := range b.Inputs.BaseX {
		// get isBaseX column
		isBaseX[i] = b.Inputs.IsBaseX[i].GetColAssignment(run).IntoRegVecSaveAlloc()
	}

	// assign the laneX
	var (
		lane  = b.Inputs.Lane.GetColAssignment(run).IntoRegVecSaveAlloc()
		laneX = make([]*common.VectorBuilder, len(b.LaneX))
		base  int
	)

	// initialize laneX builders
	for j := range laneX {
		laneX[j] = common.NewVectorBuilder(b.LaneX[j])
	}

	// populate laneX
	for j := range lane {
		// get the base
		for i := range b.Inputs.BaseX {
			if isBaseX[i][j].IsOne() {
				base = b.Inputs.BaseX[i]
				continue
			}
		}
		// convert lane to baseX
		a := decomposeAndConvertToBaseX(int(lane[j].Uint64()), base, len(b.LaneX), b.Inputs.NbBitsPerBaseX)

		for k := range laneX {
			laneX[k].PushField(a[k])
		}

	}

	// assign the laneX
	for j := range laneX {
		laneX[j].PadAndAssign(run, field.Zero())
	}
}

// x has nbBits bits, convert it to baseX representation
func uintToBaseX(x uint, base field.Element, nbBits int) field.Element {

	res := field.Zero()
	one := field.One()
	resIsZero := true

	for k := nbBits - 1; k >= 0; k-- {
		// The test allows skipping useless field muls or testing
		// the entire field element.
		if !resIsZero {
			res.Mul(&res, &base)
		}

		// Skips the field addition if the bit is zero
		bit := (x >> k) & 1
		if bit > 0 {
			res.Add(&res, &one)
			resIsZero = false
		}
	}

	return res
}

// / createLookupTablesBaseX constructs lookup tables for representing every possible
// MAXNBYTE-byte integer (from 0 to 2^(8*MAXNBYTE) - 1) in `nbSlices` slices,
// where each slice corresponds to a fixed-width segment of `bitsPerSlice` bits.
//
// For each integer j in [0, 2^(8*MAXNBYTE) - 1], the function decomposes j into
// `nbSlices` chunks of size `bitsPerSlice`, msb-order, then
// encodes each chunk both in base `baseA` and in base `baseB` using `uintToBaseX`.
//
// Parameters:
//   - baseA, baseB: field elements representing the numeric bases for two different encodings.
//   - nbSlices:     number of slices into which each integer is decomposed.
//
// Returns three columns of lookup tables:
//   - uint16Col: SmartVector of all possible field elements j ∈ [0, 2^(8*MAXNBYTE) - 1].
//   - baseACol:  baseACol[i][j] holds the i-th slice of j encoded in baseA.
//   - baseBCol:  baseBCol[i][j] holds the i-th slice of j encoded in baseB.
//
// Example (conceptual):
//
//	Suppose MAXNBYTE = 2 and nbSlices = 2.
//
//	bitsPerSlice = (8 * 2) / 2 = 8
//
//	For integer j = 0x1234 (4660 decimal):
//	    slices = [0x34, 0x12]
//	    baseACol[1][4660] = uintToBaseX(0x34, baseA, 8)
//	    baseACol[0][4660] = uintToBaseX(0x12, baseA, 8)
//
//	Thus each row j in the tables represents one full decomposition.
//
// Note:
//   - The decomposition is msb.
//   - The total number of table entries grows as 2^(8*MAXNBYTE), which can be extremely large.
//     This function is only feasible for small MAXNBYTE (e.g., 1 or 2).
func createLookupTablesBaseX(basex []int, nbSlices int) (uint16Col smartvectors.SmartVector, basexCol [][]smartvectors.SmartVector) {
	var (
		bitsPerSlice = MAXNBYTE * 8 / nbSlices
		u            []field.Element
		v            = make([][][]field.Element, len(basex))
		uintMAXNBYTE = 1<<(8*MAXNBYTE) - 1
		basexFr      = make([]field.Element, len(basex))
	)

	basexCol = make([][]smartvectors.SmartVector, len(basex))

	for i := range basex {
		basexFr[i] = field.NewElement(uint64(basex[i]))
		v[i] = make([][]field.Element, nbSlices)
	}

	if MAXNBYTE > 2 {
		// too big to create a lookup table, in this case lane column needs to be decomposed into smaller chunks.
		utils.Panic("MAXNBYTE = %v create an unsupported size of lookuptable", MAXNBYTE)
	}

	// preallocate slices
	for j := range basex {
		for i := 0; i < nbSlices; i++ {
			v[j][i] = make([]field.Element, 0, uintMAXNBYTE+1)
		}
	}

	for j := 0; j <= uintMAXNBYTE; j++ {
		// decompose j into chunks of size bitsPerSlice

		u = append(u, field.NewElement(uint64(j)))
		for k := range basex {
			a := decomposeAndConvertToBaseX(j, basex[k], nbSlices, bitsPerSlice)
			for i := 0; i < nbSlices; i++ {
				v[k][i] = append(v[k][i], a[i]) // append the i-th slice of j in base basex[k]

			}
		}
	}
	for k := range basex {
		for j := 0; j < nbSlices; j++ {
			basexCol[k] = append(basexCol[k], smartvectors.NewRegular(v[k][j]))
		}
	}

	return smartvectors.NewRegular(u), basexCol
}

// decomposeAndConvertToBaseX decomposes the non-negative integer n into nbSlices chunks of bitsPerSlice bits each,
// and converts each chunk into its representation in base `basex`.
// The decomposition is done in big-endian order, meaning the most significant chunk is processed first.
func decomposeAndConvertToBaseX(n int, basex, nbSlices int, bitsPerSlice int) []field.Element {
	var (
		basexFr = field.NewElement(uint64(basex))
		v       = make([]field.Element, nbSlices)
	)

	mask := (1 << bitsPerSlice) - 1
	result := make([]uint, nbSlices)

	// total bit length represented
	totalBits := nbSlices * bitsPerSlice

	// shift n so that the MSB chunk is aligned first
	for i := 0; i < nbSlices; i++ {
		shift := totalBits - bitsPerSlice*(i+1)
		result[i] = uint((n >> shift) & mask) // extract MSB chunk
	}

	for i := 0; i < nbSlices; i++ {
		v[i] = uintToBaseX(result[i], basexFr, bitsPerSlice)
	}

	return v
}
