package v1

import (
	"errors"

	"github.com/consensys/gnark/std/lookup/logderivlookup"

	"math/big"
	"math/bits"

	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"

	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/compress/lzss"
	snarkHash "github.com/consensys/gnark/std/hash"
	"github.com/consensys/gnark/std/rangecheck"
	public_input "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/public-input"
	"github.com/consensys/linea-monorepo/prover/circuits/internal/plonk"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
)

const (
	checkSumSize   = 32
	MaxNbBatches   = 100 // TODO ensure this is a reasonable maximum
	maxBlobNbBytes = 128 * 1024 * blob.PackingSizeU256 / 256
)

func combine(api frontend.API, bytes []frontend.Variable, perNewWord int) []frontend.Variable {
	res := make([]frontend.Variable, len(bytes)/perNewWord)
	for i := range res {
		res[i] = readNum(api, bytes[i*perNewWord:(i+1)*perNewWord])
	}
	return res
}

func readNum(api frontend.API, bytes []frontend.Variable) frontend.Variable {
	return compress.ReadNum(api, bytes, big.NewInt(256))
}

// parseHeader takes in a blob and returns the header length, the number of batches, and the length of each batch
// it assumes that the blob is already range-checked
// it ignores the dict checksum
// past nbBatches, the lengths are considered zero
// all lengths l are guaranteed to be within 0 ≤ l - 31 ≤ nextPowerOfTwo(maxPayloadBytes - 31)
func parseHeader(api frontend.API, blobBytes []frontend.Variable, blobLen frontend.Variable) (headerLen frontend.Variable, dictHash frontend.Variable, nbBatches frontend.Variable, bytesPerBatch []frontend.Variable, err error) {
	if len(blobBytes) < 2+checkSumSize+blob.NbElemsEncodingBytes { // version + checksum + nbBatches
		return 0, 0, 0, nil, errors.New("blob too short - no room for header")
	}

	api.AssertIsEqual(blobBytes[0], 255)
	api.AssertIsEqual(blobBytes[1], 255)
	blobBytes = blobBytes[2:]

	dictHash = compress.ReadNum(api, blobBytes[:checkSumSize], big.NewInt(256))
	blobBytes = blobBytes[checkSumSize:]

	// header structure: checksum, *nbBatches*, [batch lengths]
	nbBatches = compress.ReadNum(api, blobBytes[:blob.NbElemsEncodingBytes], big.NewInt(256))

	// no longer need the checksum or nbBatches encodings
	blobBytes = blobBytes[blob.NbElemsEncodingBytes:]

	// read MaxNbBatches 24-bit numbers
	blobWords := combine(api, blobBytes[:min(MaxNbBatches*blob.ByteLenEncodingBytes, len(blobBytes))], blob.ByteLenEncodingBytes)

	// header length is the length of the checksum and nbBatches, plus the length of the batch lengths
	headerLen = api.Add(2+checkSumSize+blob.NbElemsEncodingBytes, api.Mul(blob.ByteLenEncodingBytes, nbBatches)) // length in words
	bytesPerBatch = internal.Truncate(api, blobWords[:MaxNbBatches], nbBatches)                                  // zero out the "length" of the batches that don't exist

	// range checks for the batch lengths
	rc := rangecheck.New(api)
	const maxLMinus31 = blob.MaxUncompressedBytes - 31
	maxLMinus31Bits := bits.Len(uint(maxLMinus31))
	iterateInRange(api, nbBatches, MaxNbBatches, func(i int, inRange frontend.Variable) { // TODO-perf decide whether or not to merge this "loop" with the truncation above. PROBABLY NOT WORTH IT: currently this entire function is not even showing up in the profile graph
		rc.Check(api.MulAcc(api.Mul(-31, inRange), inRange, bytesPerBatch[i]), maxLMinus31Bits) // check for inRange * (bytesPerBatch[i] - 31). i.e. don't check past nbBatches
	})

	api.AssertIsLessOrEqual(headerLen, blobLen)
	api.AssertIsLessOrEqual(blobLen, len(blobBytes)) // redundant (considering how it's used in the zkevm)

	return
}

// CheckBatchesSums checks batch checksums consisting of H(batchLen, contentSum) where contentSum = Blocks[0] if len(Blocks) == 1 and H(Blocks...) otherwise. Blocks are consecutive 31-byte chunks of the data in the batch, with the last one right-padded with zeros if necessary.
// All batches must be at least 31 bytes long. The function performs this range check.
// It is also checked that the batches are all within the MAXIMUM range of the blob. CheckBatchesSums does not have access to the actual blob size, so it remains the caller's responsibility to check that the batches are within the confines of the ACTUAL blob size.
// The expected checksums are not checked beyond nbBatches
func CheckBatchesSums(api frontend.API, hasher snarkHash.FieldHasher, nbBatches frontend.Variable, blobPayload []frontend.Variable, batchLengths []frontend.Variable, expectedChecksums []frontend.Variable) error {

	batchEnds := internal.PartialSums(api, batchLengths)

	if len(batchEnds) > len(expectedChecksums) {
		return errors.New("more batches than checksums")
	}

	if len(blobPayload) < 31 { // edge case
		api.AssertIsEqual(nbBatches, 0)
		for i := range batchEnds {
			api.AssertIsEqual(batchEnds[i], 0)
		}
		return nil
	}

	api.AssertIsDifferent(nbBatches, 0)

	cappedRamp := logderivlookup.New(api)
	const cappedRampNegMin = 31
	for i := -cappedRampNegMin; i < len(blobPayload)+93; i++ {
		cappedRamp.Insert(min(31, max(i, 0)))
	}
	// min0Max31(x) = min(31, max(x, 0)), i.e. it returns 0 if x < 0, 31 if x > 31, and x otherwise
	min0Max31 := func(x frontend.Variable) frontend.Variable {
		return cappedRamp.Lookup(api.Add(x, cappedRampNegMin))[0]
	}

	nbHashes := 1 + len(blobPayload)/31 // one extra iteration of the main loop to simplify eof handling
	// every batch is only sealed when the next is about to begin. So we need to start creating the dummy batch when the loop ends.
	// to ensure that happens in the case of a full blob, we will need a dummy iteration as well

	// a practically infinite dummy batch at the end to prevent index overflows
	// and to make sure the dummy batch still doesn't "end" on the last iteration, we must give it and extra 31 bytes on top of that
	// nbHashes*31+1 is JUST beyond what the loop will reach so that the dummy batch is never sealed.
	dummyBatchEnd := nbHashes*31 + 1

	batchesRange := internal.NewRange(api, nbBatches, len(batchEnds)) // this also range-checks nbBatches
	for i := range batchEnds {                                        // check that the size of every batch is at least 31
		// in particular this ensures that for ⌊ end[i] / 31 ⌋ != ⌊ end[i+1] / 31 ⌋ for all applicable i

		internal.AssertEqualIf(api, batchesRange.IsFirstBeyond[i], 31, // "Select" to avoid going out of range
			min0Max31(api.Select(batchesRange.InRange[i], batchLengths[i], 31)))

		batchEnds[i] = api.Select(batchesRange.IsFirstBeyond[i], dummyBatchEnd, batchEnds[i])
	}

	// side effect: batchEnds are range checked to be within a reasonable factor of the maximum blob payload length; useless because we will have to perform a stronger check in the end
	endQsV, endRsV, err := divBy31(api, batchEnds, bits.Len(uint(len(blobPayload))+31))
	if err != nil {
		return err
	}
	endQs, endRs := internal.SliceToTable(api, endQsV), internal.SliceToTable(api, endRsV)

	// another practically infinite dummy batch in case nbBatches == len(batchEnds)
	endQs.Insert(dummyBatchEnd / 31)
	endRs.Insert(dummyBatchEnd % 31)
	// we need an extra dummy input element past the end of the dummy batch, because the loop is always considering
	// sealing the dummy batch and starting yet another one after it, though it never actually happens.
	// still, the circuit computes the 31-byte prefix of the next batch.
	inputExt := make([]frontend.Variable, dummyBatchEnd+31)
	for n := copy(inputExt, blobPayload); n < len(inputExt); n++ {
		inputExt[n] = 0
	}
	inputT := internal.SliceToTable(api, inputExt)
	// inputAt returns a packed, zero-padded substring of length min(l,31) starting at i
	inputAt := func(i, l frontend.Variable) frontend.Variable {
		out := make([]frontend.Variable, 31)
		r := internal.NewRange(api, l, len(out))
		for j := range out {
			out[j] = api.Mul(r.InRange[j], inputT.Lookup(api.Add(i, j))[0]) // Perf note this enables substrings of length 0 which we never use
		}
		return compress.ReadNum(api, out, big.NewInt(256))
	}

	// let the payload be p₀ p₁ ... pₙ₋₁
	// then inputAt_31B[i] = (pᵢ pᵢ₊₁ ... pᵢ₊₃₀)₃₁      (with zero padding, if necessary)
	// i.e. a full word to incorporate into the checksum, starting at the i-th byte
	inputAt31B := logderivlookup.New(api)
	nr := compress.NewNumReader(api, inputExt, 31*8, 8)
	for i := 0; i < 31*nbHashes+2; i++ { // TODO justify the +2
		inputAt31B.Insert(nr.Next())
	}

	_hsh := func(a, b frontend.Variable) frontend.Variable {
		hasher.Reset()
		hasher.Write(a, b)
		res := hasher.Sum()
		return res
	}

	var (
		partialSumsT *logderivlookup.Table
		partialSums  []frontend.Variable
	)
	// create a table of claimed sums and prove their correctness as we go through the payload
	{
		hintIn := make([]frontend.Variable, 1, 1+len(batchEnds)+len(blobPayload))
		hintIn[0] = nbBatches
		hintIn = append(hintIn, batchEnds[:]...)
		hintIn = append(hintIn, blobPayload...)
		if partialSums, err = api.Compiler().NewHint(partialChecksumBatchesPackedHint, len(batchEnds), hintIn...); err != nil {
			return err
		}
	}
	partialSumsT = internal.SliceToTable(api, partialSums)
	partialSumsT.Insert(0) // dummy in case of maximum nbBatches

	batchSum := inputAt(0, 31)     // normally this should be taken care of by the api.Select(currAlreadyOver,... line. But the very first batch doesn't get this treatment because we know it starts at 0
	batchI := frontend.Variable(0) // index of the current batch
	// each 31 byte block partially belongs to one or two batches (guaranteed by rejecting batches smaller than 31 bytes)
	startR := frontend.Variable(0) // the remainder by 31 of where the current batch starts

	// each iteration is able to process at most one new batch. This dictates that end[i] % 31 != end[i+1] % 31 for any applicable i
	for i := 0; i < nbHashes; i++ {

		endQ, endR := endQs.Lookup(batchI)[0], endRs.Lookup(batchI)[0]
		end := api.Add(api.Mul(31, endQ), endR)

		currNbBytesRemaining := api.Sub(end, 31*i, startR) // 31i + startR is the location of the "head"
		hashLen := min0Max31(currNbBytesRemaining)
		startNext := api.IsZero(api.Sub(endQ, i))
		noHash := api.IsZero(hashLen) // or equivalently, isZero(currNbBytesRemaining)

		if i != 0 {
			batchSum = api.Select(
				noHash,
				batchSum,
				_hsh(batchSum, inputAt(api.Add(31*i, startR), hashLen)),
			)

			internal.AssertEqualIf(api, startNext, batchSum, partialSumsT.Lookup(batchI)[0]) // if we're done with the current checksum, check that the claimed one from the table is equal to it
			// THIS STEP REQUIRES THAT NO BATCH SHOULD BE SMALLER THAN 31 BYTES
			//
			// @alex: this is always the case in practice since a batch contains
			// at least one block and one block stores a root hash which is is
			// already more than 31 bytes.
			batchSum = api.Select(startNext, inputAt31B.Lookup(end)[0], batchSum) // if the next one starts, the sum is the 31 byte "prefix" of the next batch; assumes any batch is at least 31 bytes long

		}

		startR = api.Select(startNext, endR, startR) // if the next one starts, update the current start R
		batchI = api.Add(batchI, startNext)
	}

	api.AssertIsEqual(batchI, nbBatches) // check that we're actually done

	// hash along the lengths and compare with expected values
	for i := range batchEnds {
		hasher.Reset()
		hasher.Write(batchLengths[i], partialSums[i])
		batchesRange.AssertEqualI(i, expectedChecksums[i], hasher.Sum())
	}

	return nil
}

func partialChecksumBatchesPackedHint(_ *big.Int, ins []*big.Int, outs []*big.Int) error {
	return gnarkutil.PartialChecksumBatchesPackedHint(MaxNbBatches)(nil, ins, outs)
}

func registerHints() {
	lzss.RegisterHints()
	solver.RegisterHint(partialChecksumBatchesPackedHint, divBy31Hint)
	internal.RegisterHints()
}

// side effect: ensures 0 ≤ v[i] < 2ᵇⁱᵗˢ⁺² for all i
func divBy31(api frontend.API, v []frontend.Variable, bits int) (q, r []frontend.Variable, err error) {
	qNbBits := bits - 4

	if hintOut, err := api.Compiler().NewHint(divBy31Hint, 2*len(v), v...); err != nil {
		return nil, nil, err
	} else {
		q, r = hintOut[:len(v)], hintOut[len(v):]
	}

	rChecker := rangecheck.New(api)

	for i := range v { // TODO See if lookups or api.AssertIsLte would be more efficient
		rChecker.Check(r[i], 5)
		api.AssertIsDifferent(r[i], 31)
		rChecker.Check(q[i], qNbBits)
		api.AssertIsEqual(v[i], api.Add(api.Mul(q[i], 31), r[i])) // 31 × q < 2ᵇⁱᵗˢ⁻⁴ 2⁵ ⇒ v < 2ᵇⁱᵗˢ⁺¹ + 31 < 2ᵇⁱᵗˢ⁺²
	}
	return q, r, nil
}

// outs: [quotients], [remainders]
func divBy31Hint(_ *big.Int, ins []*big.Int, outs []*big.Int) error {
	if len(outs) != 2*len(ins) {
		return errors.New("expected output layout: [quotients][remainders]")
	}

	q := outs[:len(ins)]
	r := outs[len(ins):]
	for i := range ins {
		v := ins[i].Uint64()
		q[i].SetUint64(v / 31)
		r[i].SetUint64(v % 31)
	}

	return nil
}

// iterateInRange runs f(i, inRange) for 0 ≤ i < staticRange where inRange is 1 if i < dynamicRange and 0 otherwise
func iterateInRange(api frontend.API, dynamicRange frontend.Variable, staticRange int, f func(i int, inRange frontend.Variable)) {
	inRange := frontend.Variable(1)
	for i := 0; i < staticRange; i++ {
		inRange = api.Sub(inRange, api.IsZero(api.Sub(i, dynamicRange)))
		f(i, inRange)
	}
}

// crumbStreamToByteStream converts a slice of bits into a slice of bytes, taking the last non-zero byte as signifying the end of the data
func crumbStreamToByteStream(api frontend.API, crumbs []frontend.Variable) (bytes []frontend.Variable, nbBytes frontend.Variable) {
	bytes = internal.Pack(api, crumbs, 8, 2) // sanity check

	found := frontend.Variable(0)
	nbBytes = frontend.Variable(0)
	for i := len(bytes) - 1; i >= 0; i-- {

		z := api.IsZero(bytes[i])

		lastNonZero := plonk.EvaluateExpression(api, z, found, -1, -1, 1, 1)   // nz - found
		nbBytes = api.Add(nbBytes, api.Mul(lastNonZero, frontend.Variable(i))) // the last nonzero byte itself is useless

		//api.AssertIsEqual(api.Mul(api.Sub(bytesPerElem-i%bytesPerElem, unpacked[i]), lastNonZero), 0) // sanity check, technically unnecessary TODO @Tabaie make sure it's one constraint only or better yet, remove

		found = plonk.EvaluateExpression(api, z, found, -1, 0, 1, 1) // found ? 1 : nz = nz + found (1 - nz) = 1 - z + found z
	}

	return
}

// ProcessBlob takes in a blob, an evaluation challenge, and a decompression dictionary. It returns a hash of the blob data along with its "evaluation" at the challenge point and a hash of all the batches in the blob payload
// TODO too many arguments; confusing. Replace with a request struct?
func ProcessBlob(api frontend.API, hsh snarkHash.FieldHasher, maxUncompressedBlobSize int, blobBytes []frontend.Variable, evaluationChallenge [32]frontend.Variable, eip4844Enabled frontend.Variable, expectedBatchSums internal.VarSlice, dict []frontend.Variable) (blobSum frontend.Variable, evaluation [2]frontend.Variable, err error) {

	blobCrumbs := internal.PackedBytesToCrumbs(api, blobBytes, blob.PackingSizeU256)

	blobPacked377 := internal.PackFull(api, blobCrumbs, 2) // repack into bls12-377 elements to compute a checksum
	hsh.Reset()
	hsh.Write(blobPacked377...)
	blobSum = hsh.Sum()

	// EIP-4844 stuff
	if evaluation, err = public_input.VerifyBlobConsistency(api, blobCrumbs, evaluationChallenge, eip4844Enabled); err != nil {
		return
	}

	// repack into bytes TODO possible optimization: pass bits directly to decompressor
	// unpack into bytes
	blobUnpackedBytes, blobUnpackedNbBytes := crumbStreamToByteStream(api, blobCrumbs)

	// get header length, number of batches, and length of each batch
	headerLen, dictChecksum, nbBatches, bytesPerBatch, err := parseHeader(api, blobUnpackedBytes[:maxBlobNbBytes], blobUnpackedNbBytes)
	if err != nil {
		return
	}
	api.AssertIsEqual(nbBatches, expectedBatchSums.Length)

	// check if the decompression dictionary checksum matches
	if err = CheckDictChecksum(api, dictChecksum, dict); err != nil {
		return
	}

	// decompress the batches
	payload := make([]frontend.Variable, maxUncompressedBlobSize)
	payloadLen, err := lzss.Decompress(
		api,
		compress.ShiftLeft(api, blobUnpackedBytes[:maxBlobNbBytes], headerLen), // TODO Signal to the decompressor that the input is zero padded; to reduce constraint numbers
		api.Sub(blobUnpackedNbBytes, headerLen),
		payload,
		dict,
	)
	if err != nil {
		return
	}
	api.AssertIsDifferent(payloadLen, -1) // decompression should not fail

	// compute checksum for each batch
	if err = CheckBatchesSums(api, hsh, nbBatches, payload, bytesPerBatch, expectedBatchSums.Values); err != nil {
		return
	}

	return
}

func CheckDictChecksum(api frontend.API, checksum frontend.Variable, dict []frontend.Variable) error {
	dictCrumbs := internal.PackedBytesToCrumbs(api, dict, 8) // basically just turn bytes into bits
	dictCrumbs = append(dictCrumbs, 3, 3, 3, 3)              // add the 0xff end-of-stream marker
	dictPacked := internal.PackFull(api, dictCrumbs, 2)
	return compress.AssertChecksumEquals(api, dictPacked, checksum)
}
