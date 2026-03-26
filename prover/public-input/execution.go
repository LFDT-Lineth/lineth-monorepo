package public_input

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash"
	"math/big"
	"math/bits"
	"slices"

	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	poseidon2_bls12377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr/poseidon2"
	gchash "github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	ghash "github.com/consensys/gnark/std/hash"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/linea-monorepo/prover/crypto/encoding"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// ExecDataChecksum consists of information enabling the computation
// of a checksum for execution data.
type ExecDataChecksum struct {

	// Length of the execution data in bytes
	Length uint64

	// PartialHash consists of hₙ₋₁ where hₙ₋₁ = H(hₙ₋₂, cₙ₋₁), ..., h₁ = H(h₀, c₁), h₀ = c₀ and
	// the cᵢ are the data packed into 31-byte BLS12-377 scalars in Big-Endian fashion.
	// H here is the Poseidon2 compression function for BLS12-377 scalars.
	// The last element is padded with less significant zeros as needed.
	// This sum is not collision resistant. To become so it needs the data length
	// incorporated into it as well.
	PartialHash types.Bls12377Fr

	// Hash is a modified Poseidon2 hash of the execution data over
	// the BLS12-377 scalar field. It is computed as H(PartialHash, Length).
	Hash types.Bls12377Fr
}

// ExecDataMultiCommitment consists of the Poseidon2 hash of the execution data
// but also the Schwarz-Zipfel check data that we need to instantiate the
// wizard proof
type ExecDataMultiCommitment struct {
	Bls12377 ExecDataChecksum
	Koala    types.KoalaOctuplet
	X        fext.Element
	Y        fext.Element
	Data     []byte
}

type Execution struct {
	InitialBlockTimestamp        uint64
	FinalStateRootHash           [32]byte
	FinalBlockNumber             uint64
	FinalBlockTimestamp          uint64
	LastRollingHashUpdate        [32]byte
	LastRollingHashUpdateNumber  uint64
	InitialRollingHashUpdate     [32]byte
	FirstRollingHashUpdateNumber uint64
	DataChecksum                 ExecDataChecksum
	L2MessageHashes              [][32]byte
	InitialStateRootHash         [32]byte
	InitialBlockNumber           uint64

	// Dynamic chain configuration
	ChainID              uint64
	BaseFee              uint64
	CoinBase             types.EthAddress
	L2MessageServiceAddr types.EthAddress
}

func (pi *Execution) Sum() []byte {
	hsh := gchash.POSEIDON2_BLS12_377.New()

	for i := range pi.L2MessageHashes {
		if _, err := hsh.Write(pi.L2MessageHashes[i][:16]); err != nil {
			utils.Panic("could not hash the L2MessageHashes HI : %v", err)
		}

		if _, err := hsh.Write(pi.L2MessageHashes[i][16:]); err != nil {
			utils.Panic("could not hash the L2MessageHashes LO : %v", err)
		}
	}

	l2MessagesSum := hsh.Sum(nil)

	hsh.Reset()

	if _, err := hsh.Write(pi.DataChecksum.Hash[:]); err != nil {
		utils.Panic("could not hash the DataChecksum : %v", err)
	}

	if _, err := hsh.Write(l2MessagesSum); err != nil {
		utils.Panic("could not hash the L2MessageHashes : %v", err)
	}

	if _, err := hsh.Write(pi.FinalStateRootHash[:16]); err != nil {
		utils.Panic("could not hash the FinalStateRootHash HI : %v", err)
	}

	if _, err := hsh.Write(pi.FinalStateRootHash[16:]); err != nil {
		utils.Panic("could not hash the FinalStateRootHash LO : %v", err)
	}

	if _, err := writeNum(hsh, pi.FinalBlockNumber); err != nil {
		utils.Panic("could not hash the FinalBlockNumber : %v", err)
	}

	if _, err := writeNum(hsh, pi.FinalBlockTimestamp); err != nil {
		utils.Panic("could not hash the FinalBlockTimestamp : %v", err)
	}

	if _, err := hsh.Write(pi.LastRollingHashUpdate[:16]); err != nil {
		utils.Panic("could not hash the LastRollingHashUpdate HI : %v", err)
	}

	if _, err := hsh.Write(pi.LastRollingHashUpdate[16:]); err != nil {
		utils.Panic("could not hash the LastRollingHashUpdate LO : %v", err)
	}

	if _, err := writeNum(hsh, pi.LastRollingHashUpdateNumber); err != nil {
		utils.Panic("could not hash the LastRollingHashUpdateNumber : %v", err)
	}

	if _, err := hsh.Write(pi.InitialStateRootHash[:16]); err != nil {
		utils.Panic("could not hash the InitialStateRootHash HI: %v", err)
	}

	if _, err := hsh.Write(pi.InitialStateRootHash[16:]); err != nil {
		utils.Panic("could not hash the InitialStateRootHash LO: %v", err)
	}

	if _, err := writeNum(hsh, pi.InitialBlockNumber); err != nil {
		utils.Panic("could not hash the InitialBlockNumber : %v", err)
	}

	if _, err := writeNum(hsh, pi.InitialBlockTimestamp); err != nil {
		utils.Panic("could not hash the InitialBlockTimestamp : %v", err)
	}

	if _, err := hsh.Write(pi.InitialRollingHashUpdate[:16]); err != nil {
		utils.Panic("could not hash the InitialRollingHashUpdate HI : %v", err)
	}

	if _, err := hsh.Write(pi.InitialRollingHashUpdate[16:]); err != nil {
		utils.Panic("could not hash the InitialRollingHashUpdate LO : %v", err)
	}

	if _, err := writeNum(hsh, pi.FirstRollingHashUpdateNumber); err != nil {
		utils.Panic("could not hash the FirstRollingHashUpdateNumber : %v", err)
	}

	// dynamic chain configuration
	if _, err := writeNum(hsh, pi.ChainID); err != nil {
		utils.Panic("could not hash the ChainID : %v", err)
	}

	if _, err := writeNum(hsh, pi.BaseFee); err != nil {
		utils.Panic("could not hash the baseFee : %v", err)
	}

	if _, err := hsh.Write(pi.CoinBase[:]); err != nil {
		utils.Panic("could not hash the coinbase : %v", err)
	}

	if _, err := hsh.Write(pi.L2MessageServiceAddr[:]); err != nil {
		utils.Panic("could not hash the L2MessageServiceAddr : %v", err)
	}

	return hsh.Sum(nil)
}

func (pi *Execution) SumAsField() fr377.Element {
	sumBytes := pi.Sum()
	sum := new(fr377.Element).SetBytes(sumBytes)
	return *sum
}

func writeNum(hsh hash.Hash, n uint64) (_ int, err error) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	if _, err := hsh.Write(b[:]); err != nil {
		return 0, fmt.Errorf("could not write number : %v", err)
	}
	return 8, nil
}

// compressionChain range-extends a compressor into a non-MerkleDamgard semi-hasher.
// It does not gracefully recover from errors.
type compressionChain struct {
	state      []byte
	buffer     bytes.Buffer
	compressor gchash.Compressor
}

func (c *compressionChain) Write(in []byte) (n int, err error) {
	for len(in)+c.buffer.Len() >= c.compressor.BlockSize() {
		m := c.compressor.BlockSize() - c.buffer.Len()
		c.buffer.Write(in[:m])
		in = in[m:]

		if len(c.state) == 0 {
			c.state = bytes.Clone(c.buffer.Bytes())
		} else if c.state, err = c.compressor.Compress(c.state, c.buffer.Bytes()); err != nil {
			return
		}

		c.buffer.Reset()

		n += m
	}

	c.buffer.Write(in)
	n += len(in)
	return

}

// NewExecDataChecksum computes the checksum of execution data as a
// Poseidon hash. The result is used in the compression proof and execution
// proof public inputs.
func NewExecDataChecksum(data []byte) (sums ExecDataChecksum, err error) {
	sums.Length = uint64(len(data))

	compressionChain := compressionChain{compressor: poseidon2_bls12377.NewDefaultPermutation()}

	if err = gnarkutil.PackLoose(&compressionChain, data, fr377.Bytes, 1); err != nil {
		return
	}
	copy(sums.PartialHash[:], compressionChain.state)
	var length [fr377.Bytes]byte
	binary.BigEndian.PutUint64(length[len(length)-8:], sums.Length)
	if _, err = compressionChain.Write(length[:]); err != nil {
		return
	}
	copy(sums.Hash[:], compressionChain.state)
	return
}

// checksumExecDataSnark computes the checksum of execution data as a BLS12-377 element.
// Caller must ensure that all data values past nbBytes are zero, or the result will be incorrect.
// Each element of data is meant to contain wordNbBits many bits. This claim is not guaranteed to be checked by this function.
func checksumExecDataSnark(api frontend.API, data []frontend.Variable, wordNbBits int, nbBytes frontend.Variable, compressor ghash.Compressor) (frontend.Variable, error) {

	if len(data) == 0 {
		return 0, nil
	}

	// turn the data into bytes
	_bytes, err := gnarkutil.ToBytes(api, data, wordNbBits)
	if err != nil {
		return nil, err
	}

	// turn the bytes into blocks
	radix := big.NewInt(256)
	blocks := make([]frontend.Variable, (len(_bytes)+31-1)/31)
	for i := range blocks {
		blocks[i] = compress.ReadNum(api, _bytes[i*31:min(i*31+31, len(_bytes))], radix)
	}

	// pad the last block with zeros
	for range len(blocks)*31 - len(_bytes) {
		blocks[len(blocks)-1] = api.Mul(blocks[len(blocks)-1], radix)
	}

	// chain of hashes
	partials := logderivlookup.New(api)
	state := blocks[0]
	for i := 1; i < len(blocks); i++ {
		partials.Insert(state)
		state = compressor.Compress(state, blocks[i])
	}
	partials.Insert(state)

	// find the partial checksum to use
	// number of used blocks is ⌈nbBytes / 31⌉
	blockI, _, err := gnarkutil.DivBy31(api, api.Add(nbBytes, 30), 1+bits.Len(uint(len(_bytes))))
	if err != nil {
		return nil, err
	}
	blockI = api.Sub(blockI, 1)
	partial := partials.Lookup(blockI)[0]

	// Return the partial hash (before incorporating the length). The full hash
	// Compress(partial, nbBytes) is verified separately by DataChecksumSnark.Check.
	return partial, nil
}

// ComputeExecutionDataLinkingCommitment computes the linking commitment for
// the execution proof.
func ComputeExecutionDataMultiCommitment(execData []byte) ExecDataMultiCommitment {
	var err error
	res := ExecDataMultiCommitment{}
	if res.Bls12377, err = NewExecDataChecksum(execData); err != nil {
		utils.Panic("could not compute bls12377 checksum : %v", err)
	}
	res.Koala = newExecDataChecksumKoala(execData)
	res.X = computeSchwarzZipfelEvaluationPoint(res.Bls12377, res.Koala)
	res.Y = evaluateExecDataForSchwarzZipfel(execData, res.X)
	res.Data = execData
	return res
}

// CheckExecDataMultiCommitmentOpeningGnark checks the linking commitment for the
// execution proof.
func CheckExecDataMultiCommitmentOpeningGnark(api frontend.API,
	execData [1 << 17]frontend.Variable, execDataNBytes frontend.Variable,
	hashKoala [8]frontend.Variable, compressor ghash.Compressor) (recoveredX,
	recoveredY [4]frontend.Variable, hashBLS frontend.Variable,
) {

	// hash the execution data: each element of execData is an individual byte
	hashBLS, err := checksumExecDataSnark(api, execData[:], 8, execDataNBytes, compressor)
	if err != nil {
		utils.Panic("could not compute bls12377 checksum : %v", err)
	}

	// computes the X evaluation point using the purported hashKoala and the
	// freshly computed hashBLS
	recoveredXFext := computeSchwarzZipfelEvaluationPointGnark(api, hashBLS, hashKoala, compressor)
	recoveredYFext := evaluateExecDataForSchwarzZipfelGnark(api, execData, recoveredXFext)

	// Reduce the emulated Koalabear values to canonical form before
	// converting to native variables. Without this, the emulated arithmetic
	// may leave values unreduced (larger than the Koalabear modulus), making
	// them incomparable with canonical values extracted from the wizard proof.
	koalaAPI := koalagnark.NewAPI(api)
	recoveredXFext = koalaAPI.ModReduceExt(recoveredXFext)
	recoveredYFext = koalaAPI.ModReduceExt(recoveredYFext)

	recoveredX = [4]frontend.Variable{
		recoveredXFext.B0.A0.Native(),
		recoveredXFext.B0.A1.Native(),
		recoveredXFext.B1.A0.Native(),
		recoveredXFext.B1.A1.Native(),
	}

	recoveredY = [4]frontend.Variable{
		recoveredYFext.B0.A0.Native(),
		recoveredYFext.B0.A1.Native(),
		recoveredYFext.B1.A0.Native(),
		recoveredYFext.B1.A1.Native(),
	}

	return recoveredX, recoveredY, hashBLS
}

// computeSchwarzZipfeEvaluation hashes the BLS and Koala checksums.
func computeSchwarzZipfelEvaluationPoint(hashBLS ExecDataChecksum, hashKoala types.KoalaOctuplet) (x fext.Element) {

	hasher := poseidon2_bls12377.NewMerkleDamgardHasher()

	// Writing the BLS counter-part of the checksum
	hasher.Write(hashBLS.PartialHash[:])

	// The koalabear counterpart of the checksum, is formatted as 2 BLS12-377
	// field element
	hashKoalaBytes := hashKoala.ToBytes()
	hasher.Write(hashKoalaBytes[:16])
	hasher.Write(hashKoalaBytes[16:])

	// The mapping from a BLS12-377 to a koalabear field extension element is
	// done by
	digest := types.Bls12377Fr(hasher.Sum(nil))
	digestBls12377Fr := digest.MustGetFrElement()
	octuplet := encoding.EncodeFrElementToOctuplet(digestBls12377Fr)

	x.B0.A0 = octuplet[0]
	x.B0.A1 = octuplet[1]
	x.B1.A0 = octuplet[2]
	x.B1.A1 = octuplet[3]
	return x
}

// computeSchwarzZipfelEvaluationPointGnark is as [computeSchwarzZipfeEvaluation] but
// in a gnark circuit.
func computeSchwarzZipfelEvaluationPointGnark(api frontend.API,
	hashBLS frontend.Variable, hashKoala [8]frontend.Variable,
	compressor ghash.Compressor,
) koalagnark.Ext {

	packKoalaQuads := func(quad []frontend.Variable) frontend.Variable {
		_ = [4]frontend.Variable(quad)
		return api.Add(
			api.Mul(quad[0], bigPowOfTwo(3*32)),
			api.Mul(quad[1], bigPowOfTwo(2*32)),
			api.Mul(quad[2], bigPowOfTwo(1*32)),
			quad[3],
		)
	}

	state := frontend.Variable(0)
	state = compressor.Compress(state, hashBLS)
	state = compressor.Compress(state, packKoalaQuads(hashKoala[0:4]))
	state = compressor.Compress(state, packKoalaQuads(hashKoala[4:8]))

	octuplet := encoding.EncodeFVTo8WVs(api, state)
	return koalagnark.Ext{
		B0: koalagnark.E2{A0: octuplet[0], A1: octuplet[1]},
		B1: koalagnark.E2{A0: octuplet[2], A1: octuplet[3]},
	}
}

// evaluateExecDataForSchwarzZipfel computes the evaluation of the
// SchwarzZipfel function.
func evaluateExecDataForSchwarzZipfel(execData []byte, x fext.Element) (y fext.Element) {

	var (
		nbKoala = utils.DivCeil(len(execData), 2)
		res     = fext.Element{}
	)

	for i := nbKoala - 1; i >= 0; i-- {

		var (
			pi  field.Element
			buf [4]byte
		)

		if 2*i < len(execData) {
			buf[2] = execData[2*i]
		}
		if 2*i+1 < len(execData) {
			buf[3] = execData[2*i+1]
		}

		if err := pi.SetBytesCanonical(buf[:]); err != nil {
			utils.Panic("could not set bytes : %v", err)
		}

		res.Mul(&res, &x)
		fext.AddByBase(&res, &res, &pi)
	}

	return res
}

// evaluateExecDataForSchwarzZipfel computes the evaluation of the
// SchwarzZipfel function.
func evaluateExecDataForSchwarzZipfelGnark(api frontend.API, execData [1 << 17]frontend.Variable, x koalagnark.Ext) (y koalagnark.Ext) {

	var (
		koalaAPI = koalagnark.NewAPI(api)
		res      = koalagnark.NewExt(fext.Zero())
		numWords = len(execData) / 2
	)

	for i := numWords - 1; i >= 0; i-- {

		packedNative := api.Add(api.Mul(execData[2*i], bigPowOfTwo(8)), execData[2*i+1])
		packed := koalaAPI.FromFrontendVar(packedNative)

		if i < numWords-1 {
			res = koalaAPI.MulExt(res, x)
			res = koalaAPI.AddByBaseExt(res, packed)
		} else {
			res.B0.A0 = packed
		}
	}

	return res
}

// newExecDataChecksumKoala computes the checksum of execution data as a
// Poseidon-koala hash. The result is used in the compression proof and
// execution proof public inputs. Each 16-bit word is mapped to a field element
// and we post-pad with zeroes to reach the block size.
func newExecDataChecksumKoala(data []byte) (sum types.KoalaOctuplet) {

	data = slices.Clone(data)
	const blockByteSize = 16
	hasherState := field.Octuplet{}

	for len(data) > 0 {
		var (
			blockBytes [blockByteSize]byte
			blockKoala field.Octuplet
		)

		copy(blockBytes[:], data)

		for w := 0; w < 8; w++ {
			blockKoala[w].SetBytes(blockBytes[2*w : 2*w+2])
		}

		hasherState = poseidon2_koalabear.Compress(hasherState, blockKoala)

		if len(data) <= blockByteSize {
			break
		}

		data = data[blockByteSize:]
	}

	return hasherState
}

// big2PowN returns 2^n in the form of a big integer
func bigPowOfTwo(n int) *big.Int {
	res := big.NewInt(1)
	res.Lsh(res, uint(n))
	return res
}
