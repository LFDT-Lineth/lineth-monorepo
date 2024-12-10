package public_input

import (
	"encoding/binary"
	"hash"

	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/field"

	"github.com/consensys/linea-monorepo/prover/utils/types"
)

type Execution struct {
	L2MessageServiceAddr         types.EthAddress
	ChainID                      uint64
	InitialBlockTimestamp        uint64
	FinalStateRootHash           [32]byte
	FinalBlockNumber             uint64
	FinalBlockTimestamp          uint64
	LastRollingHashUpdate        [32]byte
	LastRollingHashUpdateNumber  uint64
	InitialRollingHashUpdate     [32]byte
	FirstRollingHashUpdateNumber uint64
	DataChecksum                 [32]byte
	L2MessageHashes              [][32]byte
	InitialStateRootHash         [32]byte
	InitialBlockNumber           uint64
}

func (pi *Execution) Sum(hsh hash.Hash) []byte {
	if hsh == nil {
		hsh = mimc.NewMiMC()
	}

	hsh.Reset()
	for i := range pi.L2MessageHashes {
		hsh.Write(pi.L2MessageHashes[i][:16])
		hsh.Write(pi.L2MessageHashes[i][16:])
	}
	l2MessagesSum := hsh.Sum(nil)

	hsh.Reset()

	hsh.Write(pi.DataChecksum[:])
	hsh.Write(l2MessagesSum)
	hsh.Write(pi.FinalStateRootHash[:])

	writeNum(hsh, pi.FinalBlockNumber)
	writeNum(hsh, pi.FinalBlockTimestamp)
	hsh.Write(pi.LastRollingHashUpdate[:16])
	hsh.Write(pi.LastRollingHashUpdate[16:])
	writeNum(hsh, pi.LastRollingHashUpdateNumber)
	hsh.Write(pi.InitialStateRootHash[:])
	writeNum(hsh, pi.InitialBlockNumber)
	writeNum(hsh, pi.InitialBlockTimestamp)
	hsh.Write(pi.InitialRollingHashUpdate[:16])
	hsh.Write(pi.InitialRollingHashUpdate[16:])
	writeNum(hsh, pi.FirstRollingHashUpdateNumber)
	writeNum(hsh, pi.ChainID)
	hsh.Write(pi.L2MessageServiceAddr[:])

	return hsh.Sum(nil)

}

func (pi *Execution) SumAsField() field.Element {

	var (
		sumBytes = pi.Sum(nil)
		sum      = new(field.Element).SetBytes(sumBytes)
	)

	return *sum
}

func writeNum(hsh hash.Hash, n uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	hsh.Write(b[:])
}
