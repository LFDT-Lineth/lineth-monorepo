package generic

import (
	"bytes"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/expr_handle"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

const (
	// TotalLimbSize is the total size of a limb in bytes.
	TotalLimbSize = 16
)

// GenericByteModule encodes the limbs with a left alignment approach as
// specified by the arithmetization. It contains two set of columns,
// Data and Info. This module can be used for various concrete
// arithmetization modules that call the hash module.
type GenericByteModule struct {
	// Data module summarizing the information about the data to hash.
	Data GenDataModule
	// Info module contains the result of the hash.
	Info GenInfoModule
}

// GenDataModule collects the columns summarizing the information about the
// data to hash.
type GenDataModule struct {
	HashNum ifaces.Column   // identifier for the hash
	Index   ifaces.Column   // identifier for the current limb
	Limbs   limbs.Uint128Be // the content of the limbs to hash
	NBytes  ifaces.Column   // indicates the total number of bytes to use from limbs cols
	ToHash  ifaces.Column
}

// GenInfoModule collects the columns summarizing information about the result of the hash
type GenInfoModule struct {
	HashNum  ifaces.Column   // Identifier for the hash. Allows joining with the data module
	HashHi   limbs.Uint128Be // The hash result
	HashLo   limbs.Uint128Be // The hash result
	IsHashHi ifaces.Column   // indicating the location of Hash
	IsHashLo ifaces.Column   // indicating the location of Hash
}

// ScanStreams scans the receiver GenDataModule's assignment and returns the list
// of the byte stream encoded in the assignment.
func (gdm *GenDataModule) ScanStreams(run *wizard.ProverRuntime) [][]byte {

	var (
		numRow      = gdm.Limbs.NumRow()
		index       = expr_handle.GetExprHandleAssignment(run, gdm.Index).IntoRegVecSaveAlloc()
		toHash      = gdm.ToHash.GetColAssignment(run).IntoRegVecSaveAlloc()
		hashNum     = gdm.HashNum.GetColAssignment(run).IntoRegVecSaveAlloc()
		nByte       = gdm.NBytes.GetColAssignment(run).IntoRegVecSaveAlloc()
		limbs       = gdm.Limbs.GetAssignmentAsByte16Exact(run)
		streams     = [][]byte(nil)
		buffer      = &bytes.Buffer{}
		currHashNum field.Element
	)

	for row := 0; row < numRow; row++ {

		if toHash[row].IsZero() {
			continue
		}

		// Index = 0; indicates the start of a new hash. We flush the current
		// buffer and return the current hash.
		if index[row].IsZero() {
			if !currHashNum.IsZero() {
				streams = append(streams, buffer.Bytes())
				buffer = &bytes.Buffer{}
			}
			currHashNum = hashNum[row]
		}

		currNbBytes := nByte[row].Uint64()
		buffer.Write(limbs[row][:currNbBytes])
	}

	streams = append(streams, buffer.Bytes())
	return streams
}
