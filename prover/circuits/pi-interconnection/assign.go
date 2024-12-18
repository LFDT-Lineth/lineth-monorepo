package pi_interconnection

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"hash"

	"github.com/consensys/linea-monorepo/prover/crypto/mimc"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"
	decompression "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v1"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

type Request struct {
	Decompressions []blobsubmission.Response
	Executions     []public_input.Execution
	Aggregation    public_input.Aggregation
	// Path to the compression dictionary. Used to extract the execution data
	// for each execution.
	DictPath string
}

func (c *Compiled) Assign(r Request) (a Circuit, err error) {
	internal.RegisterHints()
	keccak.RegisterHints()
	utils.RegisterHints()

	// TODO there is data duplication in the request. Check consistency

	// infer config
	cfg, err := c.getConfig()
	if err != nil {
		return
	}
	a = allocateCircuit(cfg)

	if len(r.Decompressions) > cfg.MaxNbDecompression {
		err = fmt.Errorf("failing CHECK_DECOMP_LIMIT:\n\t%d decompression proofs exceeds maximum of %d", len(r.Decompressions), cfg.MaxNbDecompression)
		return
	}
	if len(r.Executions) > cfg.MaxNbExecution {
		err = fmt.Errorf("failing CHECK_EXEC_LIMIT:\n\t%d execution proofs exceeds maximum of %d", len(r.Executions), cfg.MaxNbExecution)
		return
	}
	if nbC := len(r.Decompressions) + len(r.Executions); nbC > cfg.MaxNbCircuits && cfg.MaxNbCircuits > 0 {
		err = fmt.Errorf("failing CHECK_CIRCUIT_LIMIT:\n\t%d circuits exceeds maximum of %d", nbC, cfg.MaxNbCircuits)
		return
	}

	// @alex: We should pass that as a parameter. And also (@arya) pass a list
	// of dictionnary because this function.
	dict, err := blob.GetDict(r.DictPath)
	if err != nil {
		return Circuit{}, fmt.Errorf("could not find the dictionnary: path=%v err=%v", r.DictPath, err)
	}

	// For Shnarfs and Merkle Roots
	hshK := c.Keccak.GetHasher()

	prevShnarf, err := utils.HexDecodeString(r.Aggregation.ParentAggregationFinalShnarf)
	if err != nil {
		return
	}
	utils.Copy(a.ParentShnarf[:], prevShnarf)

	hshM := mimc.NewMiMC()
	// execDataChecksums is a list that we progressively fill to store the mimc
	// hash of the executionData for every execution (conflation) batch. The
	// is filled as we process the decompression proofs which store a list of
	// the corresponding execution data hashes. These are then checked against
	// the execution proof public inputs.
	execDataChecksums := make([][]byte, 0, len(r.Executions))
	shnarfs := make([][]byte, cfg.MaxNbDecompression)
	// Decompression FPI
	for i, p := range r.Decompressions {
		var blobData [1024 * 128]byte
		if b, err := base64.StdEncoding.DecodeString(p.CompressedData); err != nil {
			return a, err
		} else {
			copy(blobData[:], b)
		}

		var (
			x [32]byte
			y fr.Element
		)
		var b []byte
		if b, err = utils.HexDecodeString(p.ExpectedX); err != nil { // TODO this is reduced. find how to get the unreduced value
			return
		} else {
			copy(x[:], b)
		}
		if _, err = y.SetString(p.ExpectedY); err != nil {
			return
		}
		if shnarfs[i], err = utils.HexDecodeString(p.ExpectedShnarf); err != nil {
			return
		}

		// TODO this recomputes much of the data in p; check consistency
		var (
			fpi  decompression.FunctionalPublicInput
			sfpi decompression.FunctionalPublicInputSnark
		)
		if fpi, err = decompression.AssignFPI(blobData[:], dict, p.Eip4844Enabled, x, y); err != nil {
			return
		}
		execDataChecksums = append(execDataChecksums, fpi.BatchSums...) // len(execDataChecksums) = index of the first execution associated with the next blob
		if sfpi, err = fpi.ToSnarkType(); err != nil {
			return
		}
		a.DecompressionFPIQ[i] = sfpi.FunctionalPublicInputQSnark
		if a.DecompressionPublicInput[i], err = fpi.Sum(decompression.WithHash(hshM)); err != nil {
			return
		}

		// recompute shnarf
		shnarf := blobsubmission.Shnarf{
			OldShnarf:        prevShnarf,
			SnarkHash:        fpi.SnarkHash,
			NewStateRootHash: r.Executions[len(execDataChecksums)-1].FinalStateRootHash[:],
			X:                fpi.X[:],
			Y:                y,
			Hash:             &hshK,
		}

		if prevShnarf = shnarf.Compute(); !bytes.Equal(prevShnarf, shnarfs[i]) {
			err = fmt.Errorf("decompression %d fails CHECK_SHNARF:\n\texpected: %x, computed: %x, ", i, shnarfs[i], prevShnarf)
			return
		}
	}
	if len(execDataChecksums) != len(r.Executions) {
		err = fmt.Errorf("failing CHECK_NB_EXEC:\n\t%d execution circuits but %d batches in decompression circuits", len(r.Executions), len(execDataChecksums))
		return
	}
	var zero [32]byte
	for i := len(r.Decompressions); i < len(a.DecompressionFPIQ); i++ {
		shnarf := blobsubmission.Shnarf{
			OldShnarf: prevShnarf,
			SnarkHash: zero[:],
			X:         zero[:],
			Hash:      &hshK,
		}
		if len(r.Executions) == 0 { // edge case for integration testing
			if shnarf.NewStateRootHash, err = utils.HexDecodeString(r.Aggregation.ParentStateRootHash); err != nil {
				return
			}
		} else {
			shnarf.NewStateRootHash = r.Executions[len(execDataChecksums)-1].FinalStateRootHash[:]
		}
		prevShnarf = shnarf.Compute()
		shnarfs[i] = prevShnarf

		fpi := decompression.FunctionalPublicInput{
			SnarkHash: zero[:],
		}

		if fpis, err := fpi.ToSnarkType(); err != nil {
			return a, nil
		} else {
			a.DecompressionFPIQ[i] = fpis.FunctionalPublicInputQSnark
		}
		utils.Copy(a.DecompressionFPIQ[i].X[:], zero[:])

		a.DecompressionPublicInput[i] = 0
	}

	// Aggregation FPI
	aggregationFPI, err := public_input.NewAggregationFPI(&r.Aggregation)
	if err != nil {
		return
	}

	// TODO @Tabaie combine the following two checks.
	if len(r.Decompressions) != 0 && !bytes.Equal(shnarfs[len(r.Decompressions)-1], aggregationFPI.FinalShnarf[:]) { // first condition is an edge case for tests
		err = fmt.Errorf("aggregation fails CHECK_FINAL_SHNARF:\n\tcomputed %x, given %x", shnarfs[len(r.Decompressions)-1], aggregationFPI.FinalShnarf)
		return
	}

	if len(r.Decompressions) == 0 || len(r.Executions) == 0 {
		err = fmt.Errorf("aggregation fails NO EXECUTION OR NO COMPRESSION:\n\tnbDecompression %d, nbExecution %d", len(r.Decompressions), len(r.Executions))
		return
	}

	aggregationFPI.NbDecompression = uint64(len(r.Decompressions))
	a.AggregationFPIQSnark = aggregationFPI.ToSnarkType().AggregationFPIQSnark

	merkleNbLeaves := 1 << cfg.L2MsgMerkleDepth
	maxNbL2MessageHashes := cfg.L2MsgMaxNbMerkle * merkleNbLeaves
	l2MessageHashes := make([][32]byte, 0, maxNbL2MessageHashes)

	lastRollingHash, lastRollingHashNumber := aggregationFPI.LastFinalizedRollingHash, aggregationFPI.LastFinalizedRollingHashMsgNumber
	lastFinBlockNum, lastFinBlockTs := aggregationFPI.LastFinalizedBlockNumber, aggregationFPI.LastFinalizedBlockTimestamp
	lastFinalizedStateRootHash := aggregationFPI.InitialStateRootHash

	for i := range a.ExecutionFPIQ {

		// padding
		executionFPI := public_input.Execution{
			InitialBlockTimestamp: lastFinBlockTs + 1,
			InitialBlockNumber:    lastFinBlockNum + 1,
			InitialStateRootHash:  lastFinalizedStateRootHash,
			FinalStateRootHash:    lastFinalizedStateRootHash,
			L2MessageServiceAddr:  r.Aggregation.L2MessageServiceAddr,
			ChainID:               r.Aggregation.ChainID,
		}
		executionFPI.FinalBlockNumber = executionFPI.InitialBlockNumber
		executionFPI.FinalBlockTimestamp = executionFPI.InitialBlockTimestamp
		a.ExecutionPublicInput[i] = 0 // the aggregation circuit dictates that padded executions must have public input 0

		if i < len(r.Executions) {
			executionFPI = r.Executions[i]
			copy(executionFPI.DataChecksum[:], execDataChecksums[i])
			// compute the public input
			a.ExecutionPublicInput[i] = executionFPI.Sum(hshM)
		}

		if l := len(executionFPI.L2MessageHashes); l > cfg.ExecutionMaxNbMsg {
			err = fmt.Errorf("execution #%d fails CHECK_MSG_LIMIT:\n\thas %d messages. only %d allowed by config", i, l, cfg.ExecutionMaxNbMsg)
			return
		}
		l2MessageHashes = append(l2MessageHashes, executionFPI.L2MessageHashes...)

		// consistency checks
		if initial := executionFPI.InitialStateRootHash; initial != lastFinalizedStateRootHash {
			err = fmt.Errorf("execution #%d fails CHECK_STATE_CONSEC:\n\tinitial state root hash does not match the last finalized\n\t%x≠%x", i, initial, lastFinalizedStateRootHash)
			return
		}
		if initial := executionFPI.InitialBlockNumber; initial != lastFinBlockNum+1 {
			err = fmt.Errorf("execution #%d fails CHECK_NUM_CONSEC:\n\tinitial block number %d is not right after to the last finalized %d", i, initial, lastFinBlockNum)
			return
		}

		// This is asserted against a constant in the circuit. Thus we have
		// different circuit for differents values of the msgSvcAddress and
		// chainID.
		if got, want := &executionFPI.L2MessageServiceAddr, &r.Aggregation.L2MessageServiceAddr; *got != *want {
			err = fmt.Errorf("execution #%d fails CHECK_SVC_ADDR:\n\texpected L2 service address %x, encountered %x", i, *want, *got)
			return
		}
		if got, want := executionFPI.ChainID, r.Aggregation.ChainID; got != want {
			err = fmt.Errorf("execution #%d fails CHECK_CHAIN_ID:\n\texpected %x, encountered %x", i, want, got)
			return
		}
		if initial := executionFPI.InitialBlockTimestamp; initial <= lastFinBlockTs {
			err = fmt.Errorf("execution #%d fails CHECK_TIME_INCREASE:\n\tinitial block timestamp is not after the final block timestamp from previous execution %d≤%d", i, initial, lastFinBlockTs)
			return
		}

		// @alex: This check is duplicating a check already done on the execution
		// proof.
		if first, last := executionFPI.InitialBlockNumber, executionFPI.FinalBlockNumber; first > last {
			err = fmt.Errorf("execution #%d fails CHECK_NUM_NODECREASE:\n\tinitial block number is greater than the final block number %d>%d", i, first, last)
			return
		}

		// @alex: This check is a duplicate of an execution proof check.
		if first, last := executionFPI.InitialBlockTimestamp, executionFPI.FinalBlockTimestamp; first > last {
			err = fmt.Errorf("execution #%d fails CHECK_TIME_NODECREASE:\n\tinitial block timestamp is greater than the final block timestamp %d>%d", i, first, last)
			return
		}

		// if there is a first, there shall be a last, no lesser than the first
		if executionFPI.LastRollingHashUpdateNumber < executionFPI.FirstRollingHashUpdateNumber {
			err = fmt.Errorf("execution #%d fails CHECK_RHASH_NODECREASE:\n\tfinal rolling hash message number %d is less than the initial %d", i, executionFPI.LastRollingHashUpdateNumber, executionFPI.FirstRollingHashUpdateNumber)
			return
		}

		// @alex: This check is a duplicate of an execution proof check.
		if (executionFPI.FirstRollingHashUpdateNumber == 0) != (executionFPI.LastRollingHashUpdateNumber == 0) {
			err = fmt.Errorf("execution #%d fails CHECK_RHASH_FIRSTLAST:\n\tif there is a rolling hash update there must be both a first and a last.\n\tfirst update msg num = %d, last update msg num = %d", i, executionFPI.FirstRollingHashUpdateNumber, executionFPI.LastRollingHashUpdateNumber)
			return
		}
		// TODO @Tabaie check that if the initial and final rolling hash msg nums were equal then so should the hashes, or decide not to

		// consistency check and record keeping
		if executionFPI.FirstRollingHashUpdateNumber != 0 { // there is an update
			// @alex: Not sure this check is a duplicate because we already check
			// that the state root hash is well-propagated and this should be
			// enough that the rolling hash update events are emitted in sequence.
			if executionFPI.FirstRollingHashUpdateNumber <= lastRollingHashNumber {
				err = fmt.Errorf("execution #%d fails CHECK_RHASH_CONSEC:\n\tinitial rolling hash message number %d is not right after the last finalized one %d", i, executionFPI.FirstRollingHashUpdateNumber, lastRollingHashNumber)
				return
			}
			lastRollingHashNumber = executionFPI.LastRollingHashUpdateNumber
			lastRollingHash = executionFPI.LastRollingHashUpdate
		}

		lastFinBlockNum, lastFinBlockTs = executionFPI.FinalBlockNumber, executionFPI.FinalBlockTimestamp
		lastFinalizedStateRootHash = executionFPI.FinalStateRootHash

		// convert to snark type
		if err = a.ExecutionFPIQ[i].Assign(&executionFPI); err != nil {
			err = fmt.Errorf("execution #%d: %w", i, err)
			return
		}
	}
	// consistency checks
	lastExec := &r.Executions[len(r.Executions)-1]

	if lastExec.FinalBlockTimestamp != aggregationFPI.FinalBlockTimestamp {
		err = fmt.Errorf("aggregation fails CHECK_FINAL_TIME:\n\tfinal block timestamps do not match: execution=%d, aggregation=%d", lastExec.FinalBlockTimestamp, aggregationFPI.FinalBlockTimestamp)
		return
	}
	if lastExec.FinalBlockNumber != aggregationFPI.FinalBlockNumber {
		err = fmt.Errorf("aggregation fails CHECK_FINAL_NUM:\n\tfinal block numbers do not match: execution=%d, aggregation=%d", lastExec.FinalBlockNumber, aggregationFPI.FinalBlockNumber)
		return
	}

	if lastRollingHash != aggregationFPI.FinalRollingHash {
		err = fmt.Errorf("aggregation fails CHECK_FINAL_RHASH:\n\tfinal rolling hashes do not match: execution=%x, aggregation=%x", lastRollingHash, aggregationFPI.FinalRollingHash)
		return
	}

	if lastRollingHashNumber != aggregationFPI.FinalRollingHashNumber {
		err = fmt.Errorf("aggregation fails CHECK_FINAL_RHASH_NUM:\n\tfinal rolling hash numbers do not match: execution=%v, aggregation=%v", lastRollingHashNumber, aggregationFPI.FinalRollingHashNumber)
		return
	}

	if len(l2MessageHashes) > maxNbL2MessageHashes {
		err = fmt.Errorf("failing CHECK_MSG_TOTAL_LIMIT:\n\ttotal of %d L2 messages, more than the %d allowed by config", len(l2MessageHashes), maxNbL2MessageHashes)
		return
	}

	if minNbRoots := (len(l2MessageHashes) + merkleNbLeaves - 1) / merkleNbLeaves; len(r.Aggregation.L2MsgRootHashes) < minNbRoots {
		err = fmt.Errorf("failing CHECK_MERKLE_CAP0:\n\tthe %d merkle roots provided are too few to accommodate all %d execution messages. A minimum of %d is needed", len(r.Aggregation.L2MsgRootHashes), len(l2MessageHashes), minNbRoots)
		return
	}

	for i := range r.Aggregation.L2MsgRootHashes {
		var expectedRoot []byte
		if expectedRoot, err = utils.HexDecodeString(r.Aggregation.L2MsgRootHashes[i]); err != nil {
			return
		}
		computedRoot := MerkleRoot(&hshK, merkleNbLeaves, l2MessageHashes[i*merkleNbLeaves:min((i+1)*merkleNbLeaves, len(l2MessageHashes))])
		if !bytes.Equal(expectedRoot[:], computedRoot[:]) {
			err = fmt.Errorf("failing CHECK_MERKLE:\n\tcomputed merkle root %x, expected %x", computedRoot, expectedRoot)
			return
		}
	}

	// padding merkle root hashes
	emptyTree := make([][]byte, cfg.L2MsgMerkleDepth+1)
	emptyTree[0] = make([]byte, 64)
	hsh := sha3.NewLegacyKeccak256()
	for i := 1; i < len(emptyTree); i++ {
		hsh.Reset()
		hsh.Write(emptyTree[i-1])
		emptyTree[i] = hsh.Sum(nil)
		emptyTree[i] = append(emptyTree[i], emptyTree[i]...)
	}

	// pad the merkle roots
	if len(r.Aggregation.L2MsgRootHashes) > cfg.L2MsgMaxNbMerkle {
		err = fmt.Errorf("failing CHECK_MERKLE_CAP1:\n\tgiven %d merkle roots, more than the %d allowed by config", len(r.Aggregation.L2MsgRootHashes), cfg.L2MsgMaxNbMerkle)
		return
	}

	for i := len(r.Aggregation.L2MsgRootHashes); i < cfg.L2MsgMaxNbMerkle; i++ {
		for depth := cfg.L2MsgMerkleDepth; depth > 0; depth-- {
			for j := 0; j < 1<<(depth-1); j++ {
				hshK.Skip(emptyTree[cfg.L2MsgMerkleDepth-depth])
			}
		}
	}

	aggregationPI := r.Aggregation.Sum(&hshK)

	a.AggregationPublicInput[0] = aggregationPI[:16]
	a.AggregationPublicInput[1] = aggregationPI[16:]

	logrus.Infof("generating wizard proof for %d hashes from %d permutations", hshK.NbHashes(), hshK.MaxNbKeccakF())
	a.Keccak, err = hshK.Assign()

	// These values are currently hard-coded in the circuit
	// This assignment is then redundant, but it helps with debugging in the test engine
	// TODO @Tabaie when we remove the hard-coding, this will still run correctly
	// but would be doubly redundant. We can remove it then.
	a.ChainID = r.Aggregation.ChainID
	a.L2MessageServiceAddr = r.Aggregation.L2MessageServiceAddr

	return
}

// MerkleRoot computes the merkle root of data using the given hasher.
// TODO modify aggregation.PackInMiniTrees to optionally take a hasher instead of reimplementing
func MerkleRoot(hsh hash.Hash, treeNbLeaves int, data [][32]byte) [32]byte {
	if len(data) == 0 || len(data) > treeNbLeaves {
		panic("unacceptable tree size")
	}

	// duplicate; pad if necessary
	b := make([][32]byte, treeNbLeaves)
	copy(b, data)

	for len(b) != 1 {
		n := len(b) / 2
		for i := 0; i < n; i++ {
			hsh.Reset()
			hsh.Write(b[2*i][:])
			hsh.Write(b[2*i+1][:])
			copy(b[i][:], hsh.Sum(nil))
		}
		b = b[:n]
	}

	return b[0]
}
