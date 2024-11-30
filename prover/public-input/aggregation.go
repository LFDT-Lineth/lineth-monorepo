package public_input

import (
	"golang.org/x/crypto/sha3"
	"hash"
	"slices"

	bn254fr "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// Aggregation collects all the field that are used to construct the public
// input of the finalization proof.
type Aggregation struct {
	FinalShnarf                             string
	ParentAggregationFinalShnarf            string
	ParentStateRootHash                     string
	ParentAggregationLastBlockTimestamp     uint
	FinalTimestamp                          uint
	LastFinalizedBlockNumber                uint
	FinalBlockNumber                        uint
	LastFinalizedL1RollingHash              string
	L1RollingHash                           string
	LastFinalizedL1RollingHashMessageNumber uint
	L1RollingHashMessageNumber              uint
	L2MsgRootHashes                         []string
	L2MsgMerkleTreeDepth                    int
	ChainID                                 uint64
	L2MessageServiceAddr                    types.EthAddress
}

func (p Aggregation) Sum(hsh hash.Hash) []byte {
	if hsh == nil {
		hsh = sha3.NewLegacyKeccak256()
	}

	writeHex := func(hex string) {
		b, err := utils.HexDecodeString(hex)
		if err != nil {
			panic(err)
		}
		hsh.Write(b)
	}

	writeInt := func(i int) {
		b := utils.FmtInt32Bytes(i)
		hsh.Write(b[:])
	}

	writeUint := func(i uint) {
		b := utils.FmtUint32Bytes(i)
		hsh.Write(b[:])
	}

	hsh.Reset()

	for _, hex := range p.L2MsgRootHashes {
		writeHex(hex)
	}

	l2Msgs := hsh.Sum(nil)

	hsh.Reset()
	writeHex(p.ParentAggregationFinalShnarf)
	writeHex(p.FinalShnarf)
	writeUint(p.ParentAggregationLastBlockTimestamp)
	writeUint(p.FinalTimestamp)
	writeUint(p.LastFinalizedBlockNumber)
	writeUint(p.FinalBlockNumber)
	writeHex(p.LastFinalizedL1RollingHash)
	writeHex(p.L1RollingHash)
	writeUint(p.LastFinalizedL1RollingHashMessageNumber)
	writeUint(p.L1RollingHashMessageNumber)
	writeInt(p.L2MsgMerkleTreeDepth)
	hsh.Write(l2Msgs)

	// represent canonically as a bn254 scalar
	var x bn254fr.Element
	x.SetBytes(hsh.Sum(nil))

	res := x.Bytes()

	return res[:]

}

// GetPublicInputHex computes the public input of the finalization proof
func (p Aggregation) GetPublicInputHex() string {
	return utils.HexEncodeToString(p.Sum(nil))
}

// AggregationFPI holds the same info as public_input.Aggregation, except in parsed form
type AggregationFPI struct {
	ParentShnarf                      [32]byte
	NbDecompression                   uint64
	InitialStateRootHash              [32]byte
	LastFinalizedBlockNumber          uint64
	LastFinalizedBlockTimestamp       uint64
	LastFinalizedRollingHash          [32]byte
	LastFinalizedRollingHashMsgNumber uint64
	ChainID                           uint64 // for now we're forcing all executions to have the same chain ID
	L2MessageServiceAddr              types.EthAddress
	NbL2Messages                      uint64 // TODO not used in hash. delete if not necessary
	L2MsgMerkleTreeRoots              [][32]byte
	FinalBlockNumber                  uint64
	FinalBlockTimestamp               uint64
	FinalRollingHash                  [32]byte
	FinalRollingHashNumber            uint64
	FinalShnarf                       [32]byte
	L2MsgMerkleTreeDepth              int
}

func (pi *AggregationFPI) ToSnarkType() AggregationFPISnark {
	s := AggregationFPISnark{
		AggregationFPIQSnark: AggregationFPIQSnark{
			LastFinalizedBlockNumber:       pi.LastFinalizedBlockNumber,
			LastFinalizedBlockTimestamp:    pi.LastFinalizedBlockTimestamp,
			LastFinalizedRollingHash:       [32]frontend.Variable{},
			LastFinalizedRollingHashNumber: pi.LastFinalizedRollingHashMsgNumber,
			InitialStateRootHash:           pi.InitialStateRootHash[:],

			NbDecompression:      pi.NbDecompression,
			ChainID:              pi.ChainID,
			L2MessageServiceAddr: pi.L2MessageServiceAddr[:],
		},
		L2MsgMerkleTreeRoots:   make([][32]frontend.Variable, len(pi.L2MsgMerkleTreeRoots)),
		FinalBlockNumber:       pi.FinalBlockNumber,
		FinalBlockTimestamp:    pi.FinalBlockTimestamp,
		L2MsgMerkleTreeDepth:   pi.L2MsgMerkleTreeDepth,
		FinalRollingHashNumber: pi.FinalRollingHashNumber,
	}

	utils.Copy(s.FinalRollingHash[:], pi.FinalRollingHash[:])
	utils.Copy(s.LastFinalizedRollingHash[:], pi.LastFinalizedRollingHash[:])
	utils.Copy(s.ParentShnarf[:], pi.ParentShnarf[:])
	utils.Copy(s.FinalShnarf[:], pi.FinalShnarf[:])

	for i := range s.L2MsgMerkleTreeRoots {
		utils.Copy(s.L2MsgMerkleTreeRoots[i][:], pi.L2MsgMerkleTreeRoots[i][:])
	}

	return s
}

type AggregationFPIQSnark struct {
	ParentShnarf                   [32]frontend.Variable
	NbDecompression                frontend.Variable
	InitialStateRootHash           frontend.Variable
	LastFinalizedBlockNumber       frontend.Variable
	LastFinalizedBlockTimestamp    frontend.Variable
	LastFinalizedRollingHash       [32]frontend.Variable
	LastFinalizedRollingHashNumber frontend.Variable
	ChainID                        frontend.Variable // WARNING: Currently not bound in Sum
	L2MessageServiceAddr           frontend.Variable // WARNING: Currently not bound in Sum
}

type AggregationFPISnark struct {
	AggregationFPIQSnark
	NbL2Messages           frontend.Variable // TODO not used in hash. delete if not necessary
	L2MsgMerkleTreeRoots   [][32]frontend.Variable
	NbL2MsgMerkleTreeRoots frontend.Variable
	// FinalStateRootHash     frontend.Variable redundant: incorporated into final shnarf
	FinalBlockNumber       frontend.Variable
	FinalBlockTimestamp    frontend.Variable
	FinalShnarf            [32]frontend.Variable
	FinalRollingHash       [32]frontend.Variable
	FinalRollingHashNumber frontend.Variable
	L2MsgMerkleTreeDepth   int
}

// NewAggregationFPI does NOT set all fields, only the ones covered in public_input.Aggregation
func NewAggregationFPI(fpi *Aggregation) (s *AggregationFPI, err error) {
	s = &AggregationFPI{
		LastFinalizedBlockNumber:          uint64(fpi.LastFinalizedBlockNumber),
		LastFinalizedBlockTimestamp:       uint64(fpi.ParentAggregationLastBlockTimestamp),
		LastFinalizedRollingHashMsgNumber: uint64(fpi.LastFinalizedL1RollingHashMessageNumber),
		L2MsgMerkleTreeRoots:              make([][32]byte, len(fpi.L2MsgRootHashes)),
		FinalBlockNumber:                  uint64(fpi.FinalBlockNumber),
		FinalBlockTimestamp:               uint64(fpi.FinalTimestamp),
		FinalRollingHashNumber:            uint64(fpi.L1RollingHashMessageNumber),
		L2MsgMerkleTreeDepth:              fpi.L2MsgMerkleTreeDepth,
		ChainID:                           fpi.ChainID,
		L2MessageServiceAddr:              fpi.L2MessageServiceAddr,
	}

	if err = copyFromHex(s.InitialStateRootHash[:], fpi.ParentStateRootHash); err != nil {
		return
	}
	if err = copyFromHex(s.FinalRollingHash[:], fpi.L1RollingHash); err != nil {
		return
	}
	if err = copyFromHex(s.LastFinalizedRollingHash[:], fpi.LastFinalizedL1RollingHash); err != nil {
		return
	}
	if err = copyFromHex(s.ParentShnarf[:], fpi.ParentAggregationFinalShnarf); err != nil {
		return
	}
	if err = copyFromHex(s.FinalShnarf[:], fpi.FinalShnarf); err != nil {
		return
	}

	for i := range s.L2MsgMerkleTreeRoots {
		if err = copyFromHex(s.L2MsgMerkleTreeRoots[i][:], fpi.L2MsgRootHashes[i]); err != nil {
			return
		}
	}
	return
}

func (pi *AggregationFPISnark) Sum(api frontend.API, hash keccak.BlockHasher) [32]frontend.Variable {
	// number of hashes: 12
	sum := hash.Sum(nil,
		pi.ParentShnarf,
		pi.FinalShnarf,
		utils.ToBytes(api, pi.LastFinalizedBlockTimestamp),
		utils.ToBytes(api, pi.FinalBlockTimestamp),
		utils.ToBytes(api, pi.LastFinalizedBlockNumber),
		utils.ToBytes(api, pi.FinalBlockNumber),
		pi.LastFinalizedRollingHash,
		pi.FinalRollingHash,
		utils.ToBytes(api, pi.LastFinalizedRollingHashNumber),
		utils.ToBytes(api, pi.FinalRollingHashNumber),
		utils.ToBytes(api, pi.L2MsgMerkleTreeDepth),
		hash.Sum(pi.NbL2MsgMerkleTreeRoots, pi.L2MsgMerkleTreeRoots...),
	)

	// turn the hash into a bn254 element
	var res [32]frontend.Variable
	copy(res[:], utils.ReduceBytes[emulated.BN254Fr](api, sum[:]))
	return res
}

func (pi *AggregationFPIQSnark) RangeCheck(api frontend.API) {
	rc := rangecheck.New(api)
	for _, v := range append(slices.Clone(pi.LastFinalizedRollingHash[:]), pi.ParentShnarf[:]...) {
		rc.Check(v, 8)
	}

	// range checking the initial "ordered" values makes sure that future comparisons are valid
	// each comparison in turn ensures that its final value is within a reasonable, less than 100 bit range
	rc.Check(pi.LastFinalizedBlockTimestamp, 64)
	rc.Check(pi.LastFinalizedBlockNumber, 64)
	rc.Check(pi.LastFinalizedRollingHashNumber, 64)
	// not checking L2MsgServiceAddr as its range is never assumed in the pi circuit
	// not checking NbDecompressions as the NewRange in the pi circuit range checks it; TODO do it here instead
}

func copyFromHex(dst []byte, src string) error {
	b, err := utils.HexDecodeString(src)
	if err != nil {
		return err
	}
	copy(dst[len(dst)-len(b):], b) // panics if src is too long
	return nil
}
