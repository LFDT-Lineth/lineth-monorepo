package execution

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	gkrposeidon2 "github.com/consensys/gnark/std/hash/poseidon2/gkr-poseidon2"
	poseidon2permutation "github.com/consensys/gnark/std/permutation/poseidon2/gkr-poseidon2"
	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// FunctionalPublicInputQSnark the information on this execution that cannot be
// extracted from other input in the same aggregation batch
type FunctionalPublicInputQSnark struct {
	DataChecksum                 DataChecksumSnark
	L2MessageHashes              L2MessageHashes
	InitialBlockTimestamp        frontend.Variable
	FinalStateRootHash           [2]frontend.Variable
	FinalBlockNumber             frontend.Variable
	FinalBlockTimestamp          frontend.Variable
	InitialRollingHashUpdate     [32]frontend.Variable
	FirstRollingHashUpdateNumber frontend.Variable
	FinalRollingHashUpdate       [32]frontend.Variable
	LastRollingHashUpdateNumber  frontend.Variable
}

// L2MessageHashes is a wrapper for [Var32Slice] it is used to instantiate the
// sequence of L2MessageHash that we extract from the arithmetization. The
// reason we need a wrapper here is that we hash the L2MessageHashes in a
// specific way.
type L2MessageHashes internal.Var32Slice

func (s *L2MessageHashes) Assign(values [][32]byte) error {
	if len(values) > len(s.Values) {
		return fmt.Errorf("%d values cannot fit in %d-long slice", len(values), len(s.Values))
	}
	for i := range values {
		utils.Copy(s.Values[i][:], values[i][:])
	}
	var zeros [32]byte
	for i := len(values); i < len(s.Values); i++ {
		utils.Copy(s.Values[i][:], zeros[:])
	}
	s.Length = len(values)
	return nil
}

func (s *L2MessageHashes) RangeCheck(api frontend.API) {
	api.AssertIsLessOrEqual(s.Length, uint64(len(s.Values)))
}

// CheckSumPoseidon2 returns the hash of the [L2MessageHashes]. The encoding is done as
// follows:
//
//   - each L2 hash is decomposed in a hi and lo part: each over 16 bytes
//   - they are sequentially hashed in the following order: (hi_0, lo_0, hi_1, lo_1 ...)
//
// The function also performs a consistency check to ensure that the length of
// the slice if consistent with the number of non-zero elements. And the function
// also ensures that the non-zero elements are all packed at the beginning of the
// struct. The function returns zero if the slice encodes zero message hashes
// (this is what happens if no L2 message events are emitted during the present
// execution frame).
func (s *L2MessageHashes) CheckSumPoseidon2(api frontend.API) frontend.Variable {

	var (
		// sumIsUsed is used to count the number of non-zero hashes that we
		// found in s. It is to be tested against s.Length.
		sumIsUsed = frontend.Variable(0)
		res       = frontend.Variable(0)
	)

	compressor, err := poseidon2permutation.NewCompressor(api)
	if err != nil {
		panic(err)
	}

	for i := range s.Values {
		var (
			hi     = internal.Pack(api, s.Values[i][:16], 128, 8)[0]
			lo     = internal.Pack(api, s.Values[i][16:], 128, 8)[0]
			isUsed = api.Sub(
				1,
				api.Mul(
					api.IsZero(hi),
					api.IsZero(lo),
				),
			)
		)

		tmpRes := compressor.Compress(res, hi)
		tmpRes = compressor.Compress(tmpRes, lo)

		res = api.Select(isUsed, tmpRes, res)
		sumIsUsed = api.Add(sumIsUsed, isUsed)
	}

	api.AssertIsEqual(sumIsUsed, s.Length)
	return res
}

type FunctionalPublicInputSnark struct {
	FunctionalPublicInputQSnark
	InitialStateRootHash [2]frontend.Variable
	InitialBlockNumber   frontend.Variable
	ChainID              frontend.Variable
	BaseFee              frontend.Variable
	CoinBase             frontend.Variable
	L2MessageServiceAddr frontend.Variable
}

// RangeCheck checks that values are within range.
func (spiq *FunctionalPublicInputQSnark) RangeCheck(api frontend.API) {
	// the length of the l2msg slice is range checked in Concat; no need to do it here; TODO do it here instead
	rc := rangecheck.New(api)
	for _, v := range spiq.L2MessageHashes.Values {
		for i := range v {
			rc.Check(v[i], 8)
		}
	}
	for i := range spiq.FinalRollingHashUpdate {
		rc.Check(spiq.FinalRollingHashUpdate[i], 8)
	}
	rc.Check(spiq.FinalBlockNumber, 64)
	rc.Check(spiq.FinalBlockTimestamp, 64)
	rc.Check(spiq.InitialBlockTimestamp, 64)
	rc.Check(spiq.FirstRollingHashUpdateNumber, 64)
	rc.Check(spiq.LastRollingHashUpdateNumber, 64)

	spiq.L2MessageHashes.RangeCheck(api)
}

func (spi *FunctionalPublicInputSnark) Sum(api frontend.API) frontend.Variable {

	var (
		finalRollingHash   = internal.CombineBytesIntoElements(api, spi.FinalRollingHashUpdate)
		initialRollingHash = internal.CombineBytesIntoElements(api, spi.InitialRollingHashUpdate)
		l2MessagesSum      = spi.L2MessageHashes.CheckSumPoseidon2(api)
	)

	hsh, err := gkrposeidon2.New(api)
	if err != nil {
		panic(err)
	}

	hsh.Write(
		spi.DataChecksum.Hash,
		l2MessagesSum,
		spi.FinalStateRootHash[0],
		spi.FinalStateRootHash[1],
		spi.FinalBlockNumber,
		spi.FinalBlockTimestamp,
		finalRollingHash[0],
		finalRollingHash[1],
		spi.LastRollingHashUpdateNumber,
		spi.InitialStateRootHash[0],
		spi.InitialStateRootHash[1],
		spi.InitialBlockNumber,
		spi.InitialBlockTimestamp,
		initialRollingHash[0],
		initialRollingHash[1],
		spi.FirstRollingHashUpdateNumber,
		spi.ChainID,
		spi.BaseFee,
		spi.CoinBase,
		spi.L2MessageServiceAddr,
	)

	return hsh.Sum()
}

func (spi *FunctionalPublicInputSnark) Assign(pi *public_input.Execution) error {

	spi.InitialStateRootHash[0] = pi.InitialStateRootHash[:16]
	spi.InitialStateRootHash[1] = pi.InitialStateRootHash[16:]
	spi.InitialBlockNumber = pi.InitialBlockNumber
	spi.ChainID = pi.ChainID
	spi.BaseFee = pi.BaseFee
	spi.CoinBase = pi.CoinBase[:]
	spi.L2MessageServiceAddr = pi.L2MessageServiceAddr[:]
	return spi.FunctionalPublicInputQSnark.Assign(pi)
}

func (spiq *FunctionalPublicInputQSnark) Assign(pi *public_input.Execution) error {

	spiq.DataChecksum.Assign(&pi.DataChecksum)
	spiq.InitialBlockTimestamp = pi.InitialBlockTimestamp
	spiq.FinalStateRootHash[0] = pi.FinalStateRootHash[:16]
	spiq.FinalStateRootHash[1] = pi.FinalStateRootHash[16:]
	spiq.FinalBlockNumber = pi.FinalBlockNumber
	spiq.FinalBlockTimestamp = pi.FinalBlockTimestamp
	spiq.FirstRollingHashUpdateNumber = pi.FirstRollingHashUpdateNumber
	spiq.LastRollingHashUpdateNumber = pi.LastRollingHashUpdateNumber

	utils.Copy(spiq.FinalRollingHashUpdate[:], pi.LastRollingHashUpdate[:])
	utils.Copy(spiq.InitialRollingHashUpdate[:], pi.InitialRollingHashUpdate[:])

	return spiq.L2MessageHashes.Assign(pi.L2MessageHashes)
}

// DataChecksumSnark represents the SNARK-friendly version of public_input.ExecDataChecksum
// meant to be used in a BLS12-377 circuit.
type DataChecksumSnark struct {
	Length      frontend.Variable
	PartialHash frontend.Variable
	Hash        frontend.Variable
}

func (ds *DataChecksumSnark) Assign(pi *public_input.ExecDataChecksum) {
	ds.Hash = pi.Hash[:]
	ds.PartialHash = pi.PartialHash[:]
	ds.Length = pi.Length
}

// Check consistency between the data, assuming the correctness of
// Length, PartialHash, PartialEvaluation, and KoalaBearHash.
func (ds *DataChecksumSnark) Check(api frontend.API) error {
	compressor, err := poseidon2permutation.NewCompressor(api)
	if err != nil {
		return err
	}

	api.AssertIsEqual(compressor.Compress(ds.PartialHash, ds.Length), ds.Hash)

	return nil
}
