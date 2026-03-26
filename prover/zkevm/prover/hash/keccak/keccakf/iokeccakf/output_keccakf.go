package iokeccakf

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	kcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/common"
)

const HashSizeBytes = 32 // keccak256

var (
	NbChunksHash = HashSizeBytes // @TODO : change to chunks of two bytes.
)

type KeccakFOutputs struct {
	HashBytes []ifaces.Column // hash result represented in chunks of  bytes
	Hash      []ifaces.Column // hash result represented in chunks of 2 bytes. This is the final output.
	IsHash    ifaces.Column   // indicates whether this row contains a hash output
	HashNum   ifaces.Column   // number of hashes outputted up to this row
	// prover action to assign the output columns Hash from HashBytes.
	Pa wizard.ProverAction
}

// it first applies to-basex to get laneX, then a projection query to map lanex to blocks
func NewOutputKeccakF(comp *wizard.CompiledIOP,
	stateCurr [5][5][8]ifaces.Column,
	isBase2 ifaces.Column,
) *KeccakFOutputs {

	var (
		output = &KeccakFOutputs{
			HashBytes: make([]ifaces.Column, NbChunksHash),
			HashNum:   comp.InsertCommit(0, ifaces.ColIDf("KECCAKF_HASH_NUM"), isBase2.Size(), true),
		}
	)

	// extract the hash result from the state
	j := 0
	for x := 0; x < 4; x++ {
		for z := 0; z < kcommon.NumSlices; z++ {
			output.HashBytes[j] = stateCurr[x][0][z]
			j++
		}
	}

	output.IsHash = isBase2

	comp.InsertGlobal(0,
		ifaces.QueryIDf("KECCAKF_HASHNUM"),
		sym.Sub(output.HashNum,
			sym.Add(
				column.Shift(output.HashNum, -1),
				output.IsHash,
			),
		),
	)

	comp.InsertLocal(0,
		ifaces.QueryIDf("KECCAKF_HASHNUM_FIRST_ROW"),
		sym.Sub(output.HashNum, output.IsHash),
	)

	// combine every two bytes into one to form the final hash output
	twoByTwo := common.NewTwoByTwoCombination(comp, output.HashBytes)
	output.Hash = twoByTwo.CombinationCols
	output.Pa = twoByTwo

	return output

}

// assignOutput assigns the values to the columns of output step.
func (o *KeccakFOutputs) Run(run *wizard.ProverRuntime) {
	// assign hash num column
	var (
		hashNum = *common.NewVectorBuilder(o.HashNum)
		isHash  = run.GetColumn(o.IsHash.GetColID()).IntoRegVecSaveAlloc()
	)
	for i := 0; i < o.IsHash.Size(); i++ {
		if isHash[i].IsOne() {
			hashNum.PushInc()
		} else if i != 0 {
			hashNum.PushIncBy(0)
		} else {
			hashNum.PushZero()
		}
	}
	hashNum.PadAndAssign(run)
	// assign the  2-byte hash output columns from the 1-byte  hash output columns.
	o.Pa.Run(run)

}

// helper function to extract the hash Digests, ignoring the rows where isHash = 0
func (o *KeccakFOutputs) GetDigests(run *wizard.ProverRuntime) (hashes []keccak.Digest) {

	var (
		hashCol = make([][]field.Element, len(o.HashBytes))
		size    = o.HashBytes[0].Size()
		isHash  = o.IsHash.GetColAssignment(run).IntoRegVecSaveAlloc()
	)

	for i := range o.HashBytes {
		hashCol[i] = make([]field.Element, size)
		hashCol[i] = o.HashBytes[i].GetColAssignment(run).IntoRegVecSaveAlloc()
	}

	// we just look at row%23 = 0 since the hash results are possibaly stored there.
	k := 0
	for row := 0; row < size; row = k*kcommon.NumRounds - 1 {

		if isHash[row].IsOne() {
			var a keccak.Digest
			for j := range hashCol {
				a[j] = hashCol[j][row].Bytes()[field.Bytes-1] // we take the last byte since each column stores one byte
			}
			hashes = append(hashes, a)
		}
		k++
	}

	return hashes
}
