package aggregation

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path"

	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"

	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"

	public_input "github.com/consensys/linea-monorepo/prover/public-input"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/linea-monorepo/prover/backend/blobdecompression"
	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/backend/execution/bridge"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/circuits/aggregation"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/hashtypes"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/sirupsen/logrus"
)

const (
	// Indicates the depth of a Merkle-tree for L2 messages, and implicitly how
	// many messages can be stored in a single root hash.
	l2MsgMerkleTreeDepth     = 5
	l2MsgMerkleTreeMaxLeaves = 1 << l2MsgMerkleTreeDepth
)

// Collect the fields, to make the aggregation proof
func collectFields(cfg *config.Config, req *Request) (*CollectedFields, error) {

	var (
		allL2MessageHashes []string
		l2MsgBlockOffsets  []bool
		cf                 = &CollectedFields{
			L2MsgTreeDepth:                          l2MsgMerkleTreeDepth,
			ParentAggregationLastBlockTimestamp:     uint(req.ParentAggregationLastBlockTimestamp),
			LastFinalizedL1RollingHash:              req.ParentAggregationLastL1RollingHash,
			LastFinalizedL1RollingHashMessageNumber: uint(req.ParentAggregationLastL1RollingHashMessageNumber),
		}
	)

	cf.ExecutionPI = make([]public_input.Execution, 0, len(req.ExecutionProofs))
	cf.InnerCircuitTypes = make([]pi_interconnection.InnerCircuitType, 0, len(req.ExecutionProofs)+len(req.DecompressionProofs))

	hshM := mimc.NewMiMC()

	for i, execReqFPath := range req.ExecutionProofs {

		var (
			po              execution.Response
			l2MessageHashes []string
			fpath           = path.Join(cfg.Execution.DirTo(), execReqFPath)
			f               = files.MustRead(fpath)
		)

		if err := json.NewDecoder(f).Decode(&po); err != nil {
			return nil, fmt.Errorf("fields collection, decoding %s, %w", execReqFPath, err)
		}

		if i == 0 {
			cf.LastFinalizedBlockNumber = uint(po.FirstBlockNumber) - 1
			cf.ParentStateRootHash = po.ParentStateRootHash
		}

		if po.ProverMode == config.ProverModeProofless {
			cf.IsProoflessJob = true
		}

		if i > 0 && po.HasParentStateRootHashMismatch {
			utils.Panic("conflated batch %v reports a parent state hash mismatch, but this is not the first batch of the sequence", i)
		}

		// This is purposefully overwritten at each iteration over i. We want to
		// keep the final value.
		cf.FinalBlockNumber = uint(po.FirstBlockNumber + len(po.BlocksData) - 1)
		rollingHashUpdateEvents := po.AllRollingHashEvent // check redundant data for discrepancy

		for _, blockdata := range po.BlocksData {

			for i := range blockdata.L2ToL1MsgHashes {
				l2MessageHashes = append(l2MessageHashes, blockdata.L2ToL1MsgHashes[i].Hex())
			}

			l2MsgBlockOffsets = append(l2MsgBlockOffsets, len(blockdata.L2ToL1MsgHashes) > 0)
			cf.HowManyL2Msgs += uint(len(blockdata.L2ToL1MsgHashes))

			// The goal is that we want to keep the final value
			lastRollingHashEvent := blockdata.LastRollingHashUpdatedEvent
			if lastRollingHashEvent != (bridge.RollingHashUpdated{}) {
				if len(rollingHashUpdateEvents) == 0 {
					return nil, fmt.Errorf("data discrepancy: only %d rolling hash update events available in the conflation object, more available in the block data", len(po.AllRollingHashEvent))
				}

				update := rollingHashUpdateEvents[0]
				rollingHashUpdateEvents = rollingHashUpdateEvents[1:]

				if lastRollingHashEvent.RollingHash != update.RollingHash {
					return nil, fmt.Errorf("data discrepancy: rolling hash update from conflation: %x. from block: %x", update.RollingHash, lastRollingHashEvent.RollingHash)
				}
				if lastRollingHashEvent.MessageNumber != update.MessageNumber {
					return nil, fmt.Errorf("data discrepancy: rolling hash message number update from conflation: %d. from block: %d", update.MessageNumber, lastRollingHashEvent.MessageNumber)
				}

				cf.L1RollingHash = lastRollingHashEvent.RollingHash.Hex()
				cf.L1RollingHashMessageNumber = uint(lastRollingHashEvent.MessageNumber)
			}

			cf.FinalTimestamp = uint(blockdata.TimeStamp)
		}
		if len(rollingHashUpdateEvents) != 0 {
			return nil, fmt.Errorf("data discrepancy: %d rolling hash updates in conflation object but only %d collected from blocks", len(po.AllRollingHashEvent), len(po.AllRollingHashEvent)-len(rollingHashUpdateEvents))
		}

		// Append the proof claim to the list of collected proofs
		if !cf.IsProoflessJob { // TODO @Tabaie @alexandre.belling proofless jobs will no longer be accepted post PI interconnection
			cf.InnerCircuitTypes = append(cf.InnerCircuitTypes, pi_interconnection.Execution)
			pClaim, err := parseProofClaim(po.Proof, po.PublicInput.Hex(), po.VerifyingKeyShaSum)
			if err != nil {
				return nil, fmt.Errorf("could not parse the proof claim for `%v` : %w", fpath, err)
			}
			cf.ProofClaims = append(cf.ProofClaims, *pClaim)

			pi := po.FuncInput()

			if pi.L2MessageServiceAddr != types.EthAddress(cfg.Layer2.MsgSvcContract) {
				return nil, fmt.Errorf("execution #%d: expected L2 msg service addr %x, encountered %x", i, cfg.Layer2.MsgSvcContract, pi.L2MessageServiceAddr)
			}
			if po.ChainID != cfg.Layer2.ChainID {
				return nil, fmt.Errorf("execution #%d: expected chain ID %x, encountered %x", i, cfg.Layer2.ChainID, po.ChainID)
			}

			// make sure public input and collected values match
			if pi := po.FuncInput().Sum(hshM); !bytes.Equal(pi, po.PublicInput[:]) {
				return nil, fmt.Errorf("execution #%d: public input mismatch: given %x, computed %x", i, po.PublicInput, pi)
			}

			cf.ExecutionPI = append(cf.ExecutionPI, *pi)
		}

		allL2MessageHashes = append(allL2MessageHashes, l2MessageHashes...)
	}

	cf.DecompressionPI = make([]blobsubmission.Response, 0, len(req.DecompressionProofs))

	for i, decompReqFPath := range req.DecompressionProofs {
		dp := &blobdecompression.Response{}
		fpath := path.Join(cfg.BlobDecompression.DirTo(), decompReqFPath)
		f := files.MustRead(fpath)

		if err := json.NewDecoder(f).Decode(dp); err != nil {
			return nil, fmt.Errorf("fields collection, decoding %s, %w", fpath, err)
		}

		if i == 0 {
			cf.DataParentHash = dp.DataParentHash
			cf.ParentAggregationFinalShnarf = dp.PrevShnarf
		}

		// These fields are overwritten after every iteration on purpose. We want
		// to keep the last value only.
		cf.FinalShnarf = dp.ExpectedShnarf
		cf.DataHashes = append(cf.DataHashes, dp.DataHash)

		// Append the proof claim to the list of collected proofs
		if !cf.IsProoflessJob {
			cf.InnerCircuitTypes = append(cf.InnerCircuitTypes, pi_interconnection.Decompression)
			pClaim, err := parseProofClaim(dp.DecompressionProof, dp.Debug.PublicInput, dp.VerifyingKeyShaSum)
			if err != nil {
				return nil, fmt.Errorf("could not parse the proof claim for `%v` : %w", fpath, err)
			}
			cf.ProofClaims = append(cf.ProofClaims, *pClaim)
			cf.DecompressionPI = append(cf.DecompressionPI, dp.Request)
		}
	}

	// If we did not collect the rolling hash, we instead pass the last
	// finalized one in the collected fields
	if len(cf.L1RollingHash) == 0 {
		cf.L1RollingHash = req.ParentAggregationLastL1RollingHash
		cf.L1RollingHashMessageNumber = uint(req.ParentAggregationLastL1RollingHashMessageNumber)
	}

	cf.L2MessagingBlocksOffsets = utils.HexEncodeToString(PackOffsets(l2MsgBlockOffsets))
	cf.L2MsgRootHashes = PackInMiniTrees(allL2MessageHashes)

	return cf, nil

}

// Prepare the response without running the actual proof
// TODO @gbotrel well, this is a bit of a lie, we do run the proof, don't we?
func CraftResponse(cfg *config.Config, cf *CollectedFields) (resp *Response, err error) {

	if err := validate(cf); err != nil {
		return resp, err
	}

	resp = &Response{
		DataHashes:                          cf.DataHashes,
		DataParentHash:                      cf.DataParentHash,
		ParentStateRootHash:                 cf.ParentStateRootHash,
		ParentAggregationLastBlockTimestamp: cf.ParentAggregationLastBlockTimestamp,
		FinalTimestamp:                      cf.FinalTimestamp,
		L1RollingHash:                       cf.L1RollingHash,
		L1RollingHashMessageNumber:          cf.L1RollingHashMessageNumber,
		L2MerkleRoots:                       cf.L2MsgRootHashes,
		L2MsgTreesDepth:                     cf.L2MsgTreeDepth,
		L2MessagingBlocksOffsets:            cf.L2MessagingBlocksOffsets,
		LastFinalizedBlockNumber:            cf.LastFinalizedBlockNumber,
		FinalBlockNumber:                    cf.FinalBlockNumber,
		ParentAggregationFinalShnarf:        cf.ParentAggregationFinalShnarf,
		FinalShnarf:                         cf.FinalShnarf,
	}

	// @alex: proofless jobs are triggered once during the migration introducing
	// the compression and the aggregation.
	if cf.IsProoflessJob {
		return resp, nil
	}

	pubInputParts := public_input.Aggregation{
		FinalShnarf:                             cf.FinalShnarf,
		ParentAggregationFinalShnarf:            cf.ParentAggregationFinalShnarf,
		ParentStateRootHash:                     cf.ParentStateRootHash,
		ParentAggregationLastBlockTimestamp:     cf.ParentAggregationLastBlockTimestamp,
		FinalTimestamp:                          cf.FinalTimestamp,
		LastFinalizedBlockNumber:                cf.LastFinalizedBlockNumber,
		FinalBlockNumber:                        cf.FinalBlockNumber,
		LastFinalizedL1RollingHash:              cf.LastFinalizedL1RollingHash,
		L1RollingHash:                           cf.L1RollingHash,
		LastFinalizedL1RollingHashMessageNumber: cf.LastFinalizedL1RollingHashMessageNumber,
		L1RollingHashMessageNumber:              resp.L1RollingHashMessageNumber,
		L2MsgRootHashes:                         cf.L2MsgRootHashes,
		L2MsgMerkleTreeDepth:                    l2MsgMerkleTreeDepth,
	}

	resp.AggregatedProofPublicInput = pubInputParts.GetPublicInputHex()

	// This log is aimed at helping debugging in-depth when the proofs are
	// reverted because the public input mismatches. The content of this log
	// can be compared with data on tenderly.
	logrus.Infof("public inputs components for range (%v-%v): %++v",
		pubInputParts.LastFinalizedBlockNumber+1,
		pubInputParts.FinalBlockNumber,
		pubInputParts,
	)

	resp.AggregatedVerifierIndex = cfg.Aggregation.VerifierID
	resp.AggregatedProverVersion = cfg.Version

	resp.AggregatedProof, err = makeProof(cfg, cf, resp.AggregatedProofPublicInput)
	if err != nil {
		return nil, fmt.Errorf("failed to prove the aggregation: %w", err)
	}

	return resp, nil
}

// validate the content of the collected fields.
func validate(cf *CollectedFields) (err error) {

	utils.ValidateHexString(&err, cf.FinalShnarf, "FinalizedShnarf : %w", 32)
	utils.ValidateHexString(&err, cf.ParentStateRootHash, "ParentStateRootHash : %w", 32)
	utils.ValidateHexString(&err, cf.L1RollingHash, "L1RollingHash : %w", 32)
	utils.ValidateHexString(&err, cf.L2MessagingBlocksOffsets, "L2MessagingBlocksOffsets : %w", -1)
	utils.ValidateTimestamps(&err, cf.ParentAggregationLastBlockTimestamp, cf.FinalTimestamp)
	utils.ValidateHexString(&err, cf.DataParentHash, "DataParentHash : %w", 32)

	for i := range cf.L2MsgRootHashes {
		wrapper := fmt.Sprintf("L2MsgRootHashes[%d] : ", i) + "%w"
		utils.ValidateHexString(&err, cf.L2MsgRootHashes[i], wrapper, 32)
	}

	for i := range cf.DataHashes {
		wrapper := fmt.Sprintf("DataHashes[%d] : ", i) + "%w"
		utils.ValidateHexString(&err, cf.DataHashes[i], wrapper, 32)
	}

	return err
}

// Pack an array of boolean into an offset list. The offset list encodes the
// position of all the boolean whose value is true. Each position is encoded
// as a big-endian uint16.
func PackOffsets(unpacked []bool) []byte {
	resWrite := &bytes.Buffer{}
	tmp := [2]byte{}

	for i, b := range unpacked {
		if b {
			// @alex: issue #2261 requires the prover to start counting from 1
			// and not from zero for the offsets.
			binary.BigEndian.PutUint16(tmp[:], utils.ToUint16(i+1)) // #nosec G115 -- Check above precludes overflowing
			resWrite.Write(tmp[:])
		}
	}

	return resWrite.Bytes()
}

// Hash the L2 messages into Merkle trees or arity 2 and depth
// `l2MsgMerkleTreeDepth`. The leaves are zero-padded on the right.
func PackInMiniTrees(l2MsgHashes []string) []string {

	paddedLen := utils.NextMultipleOf(len(l2MsgHashes), l2MsgMerkleTreeMaxLeaves)
	paddedL2MsgHashes := make([]string, paddedLen)
	copy(paddedL2MsgHashes, l2MsgHashes)

	res := []string{}

	for i := 0; i < paddedLen; i += l2MsgMerkleTreeMaxLeaves {

		digests := make([]types.Bytes32, l2MsgMerkleTreeMaxLeaves)

		// Convert the leaves into digests that can be processed by the smt
		// package.
		for j := range digests {
			leaf := paddedL2MsgHashes[i+j]
			decoded, err := utils.HexDecodeString(leaf)
			copy(digests[j][:], decoded)

			if err != nil {
				panic(err)
			}
		}

		tree := smt.BuildComplete(digests, hashtypes.Keccak)
		res = append(res, tree.Root.Hex())
	}

	return res
}

func parseProofClaim(
	proofHexString string,
	publicInputHexString string,
	verifyinKeyShasum string,
) (*aggregation.ProofClaimAssignment, error) {

	proofByte, err := utils.HexDecodeString(proofHexString)
	if err != nil {
		return nil, fmt.Errorf("could not parse proof as an hex string: the proof `%v`: %w", proofHexString, err)
	}

	if verifyinKeyShasum == "" {
		return nil, fmt.Errorf("the verifying key shasum is empty")
	}

	// This can potentially panic if the checksum is not a valid one.
	res := &aggregation.ProofClaimAssignment{
		VerifyingKeyShasum: types.FullBytes32FromHex(verifyinKeyShasum),
	}

	// This should be fine because, this field is set by the execution/compression
	// prover.
	if _, err := res.PublicInput.SetString(publicInputHexString); err != nil {
		return nil, fmt.Errorf("the public input could not be parsed from bytes: %x because: %w", publicInputHexString, err)
	}

	// @alex: the proof need to be (pre)-allocated before being read
	res.Proof = plonk.NewProof(ecc.BLS12_377)
	if _, err := res.Proof.ReadFrom(bytes.NewBuffer(proofByte)); err != nil {
		return nil, fmt.Errorf("could not parse the proof from bytes: %v", err)
	}

	return res, nil
}
