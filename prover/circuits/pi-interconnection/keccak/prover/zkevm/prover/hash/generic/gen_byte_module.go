package generic

import (
	"bytes"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// GenericByteModule encodes the limbs with a left alignment approach
const GEN_LEFT_ALIGNMENT = 16

// Generic byte module as specified by the arithmetization. It contains two set
// of columns, Data and Info. This module can be used for various concrete
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
	HashNum ifaces.Column // identifier for the hash
	Index   ifaces.Column // identifier for the current limb
	Limb    ifaces.Column // the content of the limb to hash
	NBytes  ifaces.Column // indicates the size of the current limb
	ToHash  ifaces.Column
}

// GenInfoModule collects the columns summarizing information about the result of the hash
type GenInfoModule struct {
	HashNum  ifaces.Column // Identifier for the hash. Allows joining with the data module
	HashLo   ifaces.Column // The Low part of the hash result
	HashHi   ifaces.Column // The Hi part of the hash result
	IsHashLo ifaces.Column // indicating the location of HashHi
	IsHashHi ifaces.Column // indicting the location od HashLo
}

// ScanStream scans the receiver GenDataModule's assignment and returns the list
// of the byte stream encoded in the assignment.
func (gdm *GenDataModule) ScanStreams(run *wizard.ProverRuntime) [][]byte {

	var (
		numRow      = gdm.Limb.Size()
		index       = gdm.Index.GetColAssignment(run).IntoRegVecSaveAlloc()
		limbs       = gdm.Limb.GetColAssignment(run).IntoRegVecSaveAlloc()
		toHash      = gdm.ToHash.GetColAssignment(run).IntoRegVecSaveAlloc()
		hashNum     = gdm.HashNum.GetColAssignment(run).IntoRegVecSaveAlloc()
		nByte       = gdm.NBytes.GetColAssignment(run).IntoRegVecSaveAlloc()
		streams     = [][]byte(nil)
		buffer      = &bytes.Buffer{}
		currHashNum field.Element
	)

	for row := 0; row < numRow; row++ {

		if toHash[row].IsZero() {
			continue
		}

		if index[row].IsZero() {
			if !currHashNum.IsZero() {
				streams = append(streams, buffer.Bytes())
				buffer = &bytes.Buffer{}
			}
			currHashNum = hashNum[row]
		}

		var (
			currLimbLA  = limbs[row].Bytes() // LA = left-aligned on the 16-th byte
			currNbBytes = nByte[row].Uint64()
			currLimb    = currLimbLA[16 : 16+currNbBytes]
		)

		buffer.Write(currLimb)
	}

	streams = append(streams, buffer.Bytes())
	return streams
}
