package v2

import (
	"errors"
	"math/big"
	"math/bits"

	gkrposeidon2 "github.com/consensys/gnark/std/hash/poseidon2/gkr-poseidon2"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	gkrposeidon2compressor "github.com/consensys/gnark/std/permutation/poseidon2/gkr-poseidon2"
	"github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/compress/lzss"
	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/circuits/internal/plonk"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
)

const (
	checkSumSize   = 32
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
func parseHeader(api frontend.API, expectedBytesPerBatch []frontend.Variable, blobBytes []frontend.Variable, blobLen frontend.Variable) (headerLen frontend.Variable, dictHash frontend.Variable, nbBatches frontend.Variable, err error) {
	if len(blobBytes) < 2+checkSumSize+blob.NbElemsEncodingBytes { // version + checksum + nbBatches
		return 0, 0, nil, errors.New("blob too short - no room for header")
	}

	maxNbBatches := len(expectedBytesPerBatch)

	api.AssertIsEqual(blobBytes[0], 255)
	api.AssertIsEqual(blobBytes[1], 254) // expect version 2
	blobBytes = blobBytes[2:]

	dictHash = compress.ReadNum(api, blobBytes[:checkSumSize], big.NewInt(256))
	blobBytes = blobBytes[checkSumSize:]

	// header structure: checksum, *nbBatches*, [batch lengths]
	nbBatches = compress.ReadNum(api, blobBytes[:blob.NbElemsEncodingBytes], big.NewInt(256))

	// no longer need the checksum or nbBatches encodings
	blobBytes = blobBytes[blob.NbElemsEncodingBytes:]

	// read MaxNbBatches 24-bit numbers
	blobWords := combine(api, blobBytes[:min(maxNbBatches*blob.ByteLenEncodingBytes, len(blobBytes))], blob.ByteLenEncodingBytes)

	// header length is the length of the checksum and nbBatches, plus the length of the batch lengths
	headerLen = api.Add(2+checkSumSize+blob.NbElemsEncodingBytes, api.Mul(blob.ByteLenEncodingBytes, nbBatches)) // length in words
	bytesPerBatch := internal.Truncate(api, blobWords[:maxNbBatches], nbBatches)                                 // zero out the "length" of the batches that don't exist

	// range checks for the batch lengths
	rc := rangecheck.New(api)
	const maxLMinus31 = blob.MaxUncompressedBytes - 31
	maxLMinus31Bits := bits.Len(uint(maxLMinus31))
	batchesRange := internal.NewRange(api, nbBatches, maxNbBatches)
	for i, inRange := range batchesRange.InRange {
		// check that the batch length is small, but no less than 31.
		rc.Check(api.MulAcc(api.Mul(-31, inRange), inRange, bytesPerBatch[i]), maxLMinus31Bits) // check for inRange * (bytesPerBatch[i] - 31). i.e. don't check past nbBatches
		internal.AssertEqualIf(api, inRange, expectedBytesPerBatch[i], bytesPerBatch[i])
	}

	api.AssertIsLessOrEqual(headerLen, blobLen)
	api.AssertIsLessOrEqual(blobLen, len(blobBytes)) // redundant (considering how it's used in the zkevm)
	api.AssertIsLessOrEqual(nbBatches, maxNbBatches)

	return
}

type batchPackingIteration struct {
	Current    frontend.Variable
	Next       frontend.Variable
	NoOp       frontend.Variable
	NextStarts frontend.Variable
	BatchI     frontend.Variable
}

type packedBatches struct {
	Iterations []batchPackingIteration
	Ends       []frontend.Variable
	Range      *internal.Range
}

func packBatches(api frontend.API, nbBatches frontend.Variable, blobPayload, batchLengths []frontend.Variable) (*packedBatches, error) {

	batchEnds := internal.PartialSums(api, batchLengths)

	if len(blobPayload) < 31 { // edge case
		api.AssertIsEqual(nbBatches, 0)
		for i := range batchEnds {
			api.AssertIsEqual(batchEnds[i], 0)
		}
		return nil, nil
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

	iterations := make([]batchPackingIteration, 1+len(blobPayload)/31) // one extra iteration of the main loop to simplify eof handling
	// every batch is only sealed when the next is about to begin. So we need to start creating the dummy batch when the loop ends.
	// to ensure that happens in the case of a full blob, we will need a dummy iteration as well

	// a practically infinite dummy batch at the end to prevent index overflows
	// and to make sure the dummy batch still doesn't "end" on the last iteration, we must give it and extra 31 bytes on top of that
	// nbHashes*31+1 is JUST beyond what the loop will reach so that the dummy batch is never sealed.
	dummyBatchEnd := len(iterations)*31 + 1

	batchesRange := internal.NewRange(api, nbBatches, len(batchEnds)) // this also range-checks nbBatches
	for i := range batchEnds {                                        // check that the size of every batch is at least 31
		// in particular this ensures that for ⌊ end[i] / 31 ⌋ != ⌊ end[i+1] / 31 ⌋ for all applicable i

		internal.AssertEqualIf(api, batchesRange.IsFirstBeyond[i], 31, // "Select" to avoid going out of range
			min0Max31(api.Select(batchesRange.InRange[i], batchLengths[i], 31)))

		batchEnds[i] = api.Select(batchesRange.IsFirstBeyond[i], dummyBatchEnd, batchEnds[i])
	}

	// side effect: batchEnds are range checked to be within a reasonable factor of the maximum blob payload length; useless because we will have to perform a stronger check in the end
	endQsV, endRsV, err := gnarkutil.DivManyBy31(api, batchEnds, bits.Len(uint(len(blobPayload))+31))
	if err != nil {
		return nil, err
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
	for range 31*len(iterations) + 2 { // TODO justify the +2
		inputAt31B.Insert(nr.Next())
	}

	iterations[0] = batchPackingIteration{
		Current: inputAt31B.Lookup(0)[0],
		Next:    inputAt31B.Lookup(batchLengths[0])[0],
		NoOp:    1, // No previous state to hash with.
	}

	batchI := frontend.Variable(0) // index of the current batch
	// each 31 byte block partially belongs to one or two batches (guaranteed by rejecting batches smaller than 31 bytes)
	startR := frontend.Variable(0) // the remainder by 31 of where the current batch starts

	// each iteration is able to process at most one new batch. This dictates that end[i] % 31 != end[i+1] % 31 for any applicable i
	for i := range iterations {

		endQ, endR := endQs.Lookup(batchI)[0], endRs.Lookup(batchI)[0]
		end := api.Add(api.Mul(31, endQ), endR)

		if i != 0 {

			currNbBytesRemaining := api.Sub(end, 31*i, startR) // 31i + startR is the location of the "head"
			opLen := min0Max31(currNbBytesRemaining)

			iterations[i].NoOp = api.IsZero(opLen) // or equivalently, isZero(currNbBytesRemaining)

			iterations[i].Current = inputAt(api.Add(31*i, startR), opLen)
			iterations[i].Next = inputAt31B.Lookup(end)[0]
		}

		iterations[i].NextStarts = api.IsZero(api.Sub(endQ, i))
		iterations[i].BatchI = batchI

		startR = api.Select(iterations[i].NextStarts, endR, startR) // if the next one starts, update the current start R
		batchI = api.Add(batchI, iterations[i].NextStarts)
	}

	api.AssertIsEqual(batchI, nbBatches) // check that we're actually done

	return &packedBatches{
		Iterations: iterations,
		Ends:       batchEnds,
		Range:      batchesRange,
	}, nil
}

func (b *packedBatches) iterate(api frontend.API, i int, state *checksumState) {

	// if we need to update the state, do it
	state.current = api.Select(
		b.Iterations[i].NoOp,
		state.current,
		state.updater(state.current, b.Iterations[i].Current),
	)

	// if we're done with the current checksum, check that the claimed one from the table is equal to it
	internal.AssertEqualIf(api,
		b.Iterations[i].NextStarts,
		state.current,
		state.finalValues.Lookup(b.Iterations[i].BatchI)[0],
	)

	// If the next batch starts, the checksum is the 31 byte "prefix" of the next batch.
	// THIS STEP REQUIRES THAT NO BATCH SHOULD BE SMALLER THAN 31 BYTES
	//
	// This is always the case in practice since a batch contains
	// at least one block which in turn contains a 32 byte root hash.
	state.current = api.Select(b.Iterations[i].NextStarts, b.Iterations[i].Next, state.current)
}

type checksumState struct {
	current     frontend.Variable
	finalValues logderivlookup.Table
	updater     func(state, current frontend.Variable) frontend.Variable
}

// CheckBatchesPartialSums checks the batch checksum H(batchLen, contentSum) where contentSum = Blocks[0] if len(Blocks) == 1 and H(Blocks...) otherwise. Blocks are consecutive 31-byte chunks of the data in the batch, with the last one right-padded with zeros if necessary.
// All batches must be at least 31 bytes long. The function performs this range check.
// It is also checked that the batches are all within the MAXIMUM range of the blob. CheckBatchesPartialSums does not have access to the actual blob size, so it remains the caller's responsibility to check that the batches are within the confines of the ACTUAL blob size.
// The expected checksums are not checked beyond nbBatches.
func CheckBatchesPartialSums(api frontend.API, nbBatches frontend.Variable, blobPayload []frontend.Variable, sums []execution.DataChecksumSnark) error {
	api.AssertIsLessOrEqual(nbBatches, len(sums)) // already range checked in parseHeader

	batchLengths := make([]frontend.Variable, len(sums))
	for i := range sums {
		batchLengths[i] = sums[i].Length
	}

	packedBatches, err := packBatches(api, nbBatches, blobPayload, batchLengths)
	if err != nil {
		return err
	}
	iterations := packedBatches.Iterations

	compressor, err := gkrposeidon2compressor.NewCompressor(api)
	if err != nil {
		return err
	}

	hashState := checksumState{
		current:     iterations[0].Current,
		finalValues: logderivlookup.New(api),
		updater:     compressor.Compress,
	}

	for i := range sums {
		hashState.finalValues.Insert(sums[i].PartialHash)
	}

	// extra dummy batch that is never sealed in case we hit maxNbBatches
	hashState.finalValues.Insert(0)

	for i := range iterations {
		packedBatches.iterate(api, i, &hashState)
	}

	return nil
}

func init() {
	lzss.RegisterHints()
	internal.RegisterHints()
	utils.RegisterHints()
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

func CheckDictChecksum(api frontend.API, checksum frontend.Variable, dict []frontend.Variable) error {
	dictCrumbs := internal.PackedBytesToCrumbs(api, dict, 8) // basically just turn bytes into bits
	dictCrumbs = append(dictCrumbs, 3, 3, 3, 3)              // add the 0xff end-of-stream marker
	dictPacked := internal.PackFull(api, dictCrumbs, 2)

	hsh, err := gkrposeidon2.New(api)
	if err != nil {
		return err
	}
	hsh.Write(dictPacked...)
	api.AssertIsEqual(hsh.Sum(), checksum)
	return nil
}
