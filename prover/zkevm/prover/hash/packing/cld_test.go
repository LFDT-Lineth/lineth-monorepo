package packing

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// It generates Define and Assign function of Packing module, for testing
func makeTestCaseCLDModule(uc generic.HashingUsecase) (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	var (
		// max number of blocks that can be extracted from limbs
		// if the number of blocks passes the max, newPack() would panic.
		maxNumBlock = 108
		// if the blockSize is not consistent with PackingParam, newPack() would panic.
		blockSize = uc.BlockSizeBytes()
		// for testing; used to populate the importation columns
		// since we have at least one block per hash, the umber of hashes should be less than maxNumBlocks
		numHash = 73
		// max number of limbs
		size = utils.NextPowerOfTwo(maxNumBlock * blockSize)
	)

	imported := Importation{}
	decomposed := decomposition{}

	define = func(build *wizard.Builder) {
		comp := build.CompiledIOP
		imported = createImportationColumns(comp, size)

		inp := decompositionInputs{
			Param:    uc,
			Name:     "Decomposition",
			Lookup:   NewLookupTables(comp),
			Imported: imported,
		}
		decomposed = newDecomposition(comp, inp)
	}
	prover = func(run *wizard.ProverRuntime) {
		// assign the importation columns
		assignImportationColumns(run, &imported, numHash, blockSize, size)

		decomposed.Assign(run)
	}
	return define, prover
}

func TestCLDModule(t *testing.T) {
	for _, uc := range testCases {
		t.Run(uc.Name, func(t *testing.T) {
			define, prover := makeTestCaseCLDModule(uc.UseCase)
			comp := wizard.Compile(define, dummy.Compile)
			proof := wizard.Prove(comp, prover)
			assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
		})
	}
}

func TestDecomposeNByte(t *testing.T) {
	tests := []struct {
		name     string
		nbytes   []field.Element
		expected [][]int
	}{
		{
			name:     "single byte with value 0",
			nbytes:   []field.Element{field.NewElement(0)},
			expected: [][]int{{0}, {0}, {0}, {0}, {0}, {0}, {0}, {0}},
		},
		{
			name:     "single byte with value 1",
			nbytes:   []field.Element{field.NewElement(1)},
			expected: [][]int{{1}, {0}, {0}, {0}, {0}, {0}, {0}, {0}},
		},
		{
			name:     "single byte with value 2",
			nbytes:   []field.Element{field.NewElement(2)},
			expected: [][]int{{2}, {0}, {0}, {0}, {0}, {0}, {0}, {0}},
		},
		{
			name:     "single byte with value 3 (exceeds MAXNBYTE)",
			nbytes:   []field.Element{field.NewElement(3)},
			expected: [][]int{{2}, {1}, {0}, {0}, {0}, {0}, {0}, {0}},
		},
		{
			name:     "single byte with value 5 (multiple limbs)",
			nbytes:   []field.Element{field.NewElement(5)},
			expected: [][]int{{2}, {2}, {1}, {0}, {0}, {0}, {0}, {0}},
		},
		{
			name:   "multiple bytes with different values",
			nbytes: []field.Element{field.NewElement(2), field.NewElement(4), field.NewElement(7)},
			expected: [][]int{
				{2, 2, 2},
				{0, 2, 2},
				{0, 0, 2},
				{0, 0, 1},
				{0, 0, 0},
				{0, 0, 0},
				{0, 0, 0},
				{0, 0, 0},
			},
		},
		{
			name:   "value at maximum (16)",
			nbytes: []field.Element{field.NewElement(16)},
			expected: [][]int{
				{2}, {2}, {2}, {2}, {2}, {2}, {2}, {2},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := decomposeNByte(tc.nbytes)
			assert.Equal(t, tc.expected, result, "decomposition result doesn't match expected output")
		})
	}
}

func alignedFromBytes(a ...byte) field.Element {
	zeros := make([]byte, MAXNBYTE-len(a))
	bytes := append(a, zeros...)

	var value field.Element
	value.SetBytes(bytes)
	return value
}

func Test_DecomposeLimbsAndCarry(t *testing.T) {
	tests := []struct {
		name          string
		limbs         [][]field.Element
		decomposedLen [][]field.Element
		nbytes        [][]int
		expectedLimbs [][]field.Element
		expectedCarry [][]field.Element
	}{
		{
			name: "no decomposition needed",
			limbs: [][]field.Element{
				{alignedFromBytes(0x12, 0x34)},
				{alignedFromBytes(0x56, 0x78)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
			},
			decomposedLen: [][]field.Element{
				{field.NewElement(2)},
				{field.NewElement(2)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
			},
			nbytes: [][]int{
				{2},
				{2},
				{0},
				{0},
				{0},
				{0},
				{0},
				{0},
			},
			expectedLimbs: [][]field.Element{
				{field.NewElement(0x1234)},
				{field.NewElement(0x5678)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
			},
			expectedCarry: [][]field.Element{
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
			},
		},
		{
			name: "decomposition with carry",
			limbs: [][]field.Element{
				{alignedFromBytes(0x12, 0x34)},
				{alignedFromBytes(0x56, 0x78)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
			},
			decomposedLen: [][]field.Element{
				{field.NewElement(1)},
				{field.NewElement(2)},
				{field.NewElement(1)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
			},
			nbytes: [][]int{
				{2},
				{2},
				{0},
				{0},
				{0},
				{0},
				{0},
				{0},
			},
			expectedLimbs: [][]field.Element{
				{field.NewElement(0x12)},
				{field.NewElement(0x3456)},
				{field.NewElement(0x78)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
			},
			expectedCarry: [][]field.Element{
				{field.NewElement(0x34)},
				{field.NewElement(0x78)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
			},
		},
		{
			name: "decomposition with carry small",
			limbs: [][]field.Element{
				{alignedFromBytes(0x12, 0x34)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
				{alignedFromBytes(0)},
			},
			decomposedLen: [][]field.Element{
				{field.NewElement(1)},
				{field.NewElement(1)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
			},
			nbytes: [][]int{
				{2},
				{0},
				{0},
				{0},
				{0},
				{0},
				{0},
				{0},
			},
			expectedLimbs: [][]field.Element{
				{field.NewElement(0x12)},
				{field.NewElement(0x34)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
			},
			expectedCarry: [][]field.Element{
				{field.NewElement(0x34)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
				{field.NewElement(0)},
			},
		},
		{
			name: "decomposition with carry all",
			limbs: [][]field.Element{
				{alignedFromBytes(0x12, 0x34)},
				{alignedFromBytes(0x56, 0x78)},
				{alignedFromBytes(0x9a, 0xbc)},
				{alignedFromBytes(0xde, 0xf0)},
				{alignedFromBytes(0x12, 0x34)},
				{alignedFromBytes(0x56, 0x78)},
				{alignedFromBytes(0x9a, 0xbc)},
				{alignedFromBytes(0xde, 0xf0)},
			},
			decomposedLen: [][]field.Element{
				{field.NewElement(1)},
				{field.NewElement(2)},
				{field.NewElement(2)},
				{field.NewElement(2)},
				{field.NewElement(2)},
				{field.NewElement(2)},
				{field.NewElement(2)},
				{field.NewElement(2)},
				{field.NewElement(1)},
			},
			nbytes: [][]int{
				{2},
				{2},
				{2},
				{2},
				{2},
				{2},
				{2},
				{2},
			},
			expectedLimbs: [][]field.Element{
				{field.NewElement(0x12)},
				{field.NewElement(0x3456)},
				{field.NewElement(0x789a)},
				{field.NewElement(0xbcde)},
				{field.NewElement(0xf012)},
				{field.NewElement(0x3456)},
				{field.NewElement(0x789a)},
				{field.NewElement(0xbcde)},
				{field.NewElement(0xf0)},
			},
			expectedCarry: [][]field.Element{
				{field.NewElement(0x34)},
				{field.NewElement(0x78)},
				{field.NewElement(0xbc)},
				{field.NewElement(0xf0)},
				{field.NewElement(0x34)},
				{field.NewElement(0x78)},
				{field.NewElement(0xbc)},
				{field.NewElement(0xf0)},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resultLimbs, resultCarry := decomposeLimbsAndCarry(tc.limbs, tc.decomposedLen, tc.nbytes)

			require.Equal(t, len(tc.expectedLimbs), len(resultLimbs), "number of limb columns should match")
			require.Equal(t, len(tc.expectedCarry), len(resultCarry), "number of carry columns should match")

			for i := range tc.expectedLimbs {
				require.Equal(t, len(tc.expectedLimbs[i]), len(resultLimbs[i]), "limb column %d should have correct length", i)
				for j := range tc.expectedLimbs[i] {
					require.True(t, tc.expectedLimbs[i][j].Equal(&resultLimbs[i][j]),
						"limb[%d][%d] should match: expected %v, got %v", i, j, tc.expectedLimbs[i][j].Bytes(), resultLimbs[i][j].Bytes())
				}
			}

			for i := range tc.expectedCarry {
				require.Equal(t, len(tc.expectedCarry[i]), len(resultCarry[i]), "carry column %d should have correct length", i)
				for j := range tc.expectedCarry[i] {
					require.True(t, tc.expectedCarry[i][j].Equal(&resultCarry[i][j]),
						"carry[%d][%d] should match: expected %v, got %v", i, j, tc.expectedCarry[i][j].Bytes(), resultCarry[i][j].Bytes())
				}
			}
		})
	}
}
