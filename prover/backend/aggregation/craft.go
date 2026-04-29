package aggregation

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path"

	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"

	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"
	"github.com/consensys/linea-monorepo/prover/backend/invalidity"

	public_input "github.com/consensys/linea-monorepo/prover/public-input"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	dataavailability "github.com/consensys/linea-monorepo/prover/backend/dataavailability"
	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/backend/execution/bridge"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/circuits/aggregation"
	"github.com/consensys/linea-monorepo/prover/config"
	hashtypes "github.com/consensys/linea-monorepo/prover/crypto/state-management/hashtypes_legacy"
	smt "github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_mimcbls12377"
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
			ParentStateRootHashContract:             req.ParentAggregationStateRootHashContract,
			LastFinalizedFtxRollingHash:             req.ParentAggregationLastFtxRollingHash,
			LastFinalizedFtxNumber:                  uint(req.ParentAggregationLastFtxNumber),
		}
	)

	cf.ExecutionPI = make([]public_input.Execution, 0, len(req.ExecutionProofs))
	cf.InvalidityPI = make([]public_input.Invalidity, 0, len(req.InvalidityProofs))
	cf.InnerCircuitTypes = make([]pi_interconnection.InnerCircuitType, 0, len(req.ExecutionProofs)+len(req.DecompressionProofs)+len(req.InvalidityProofs))

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
			// If the user does not pass the value (which will be the case 99%
			// of the time), we assume the parent state root hash is the same
			// as the first block's state root hash.
			if cf.ParentStateRootHashContract.IsZero() {
				cf.ParentStateRootHashContract = po.ParentStateRootHash.ToBytes32()
			} else if !isKoalaBearHash(cf.ParentStateRootHashContract) {
				logrus.WithFields(logrus.Fields{
					"parentStateRootHashContract": cf.ParentStateRootHashContract.Hex(),
					"firstExecutionParentHash":    po.ParentStateRootHash.Hex(),
				}).Warn("BLS-to-KoalaBear transition detected: parentStateRootHashContract is a BLS hash, circuit will fall back to first execution's initial state root hash")
			}
			cf.LastFinalizedFtxNumber = uint(req.ParentAggregationLastFtxNumber)
			cf.LastFinalizedFtxRollingHash = req.ParentAggregationLastFtxRollingHash
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
			cf.ProofClaimSources = append(cf.ProofClaimSources, execReqFPath)

			pi := po.FuncInput()

			if po.ChainID != cfg.Layer2.ChainID {
				return nil, fmt.Errorf("execution #%d: dynamic chain config mismatch: Chain ID from config (layer2.chain_id) is %d (0x%x), but execution proof has %d (0x%x)", i, cfg.Layer2.ChainID, cfg.Layer2.ChainID, po.ChainID, po.ChainID)
			}
			if po.BaseFee != cfg.Layer2.BaseFee {
				return nil, fmt.Errorf("execution #%d: dynamic chain config mismatch: Base Fee from config (layer2.base_fee) is %d (0x%x), but execution proof has %d (0x%x)", i, cfg.Layer2.BaseFee, cfg.Layer2.BaseFee, po.BaseFee, po.BaseFee)
			}

			if pi.CoinBase != types.EthAddress(cfg.Layer2.CoinBase) {
				return nil, fmt.Errorf("execution #%d: dynamic chain config mismatch: CoinBase from config (layer2.coin_base) is 0x%x, but execution proof has 0x%x", i, cfg.Layer2.CoinBase, pi.CoinBase)
			}

			if pi.L2MessageServiceAddr != types.EthAddress(cfg.Layer2.MsgSvcContract) {
				return nil, fmt.Errorf("execution #%d: dynamic chain config mismatch: L2 Message Service from config (layer2.message_service_contract) is 0x%x, but execution proof has 0x%x", i, cfg.Layer2.MsgSvcContract, pi.L2MessageServiceAddr)
			}

			// make sure public input and collected values match
			if pi := po.FuncInput().Sum(); !bytes.Equal(pi, po.PublicInput[:]) {
				return nil, fmt.Errorf("execution #%d: public input mismatch: given %x, computed %x", i, po.PublicInput, pi)
			}

			cf.ExecutionPI = append(cf.ExecutionPI, *pi)
		}

		allL2MessageHashes = append(allL2MessageHashes, l2MessageHashes...)
	}

	cf.DecompressionPI = make([]blobsubmission.Response, 0, len(req.DecompressionProofs))

	for i, decompReqFPath := range req.DecompressionProofs {
		dp := &dataavailability.Response{}
		fpath := path.Join(cfg.DataAvailability.DirTo(), decompReqFPath)
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
			cf.ProofClaimSources = append(cf.ProofClaimSources, decompReqFPath)
			cf.DecompressionPI = append(cf.DecompressionPI, dp.Request)
		}
	}

	// If we did not collect the rolling hash, we instead pass the last
	// finalized one in the collected fields
	if len(cf.L1RollingHash) == 0 {
		cf.L1RollingHash = req.ParentAggregationLastL1RollingHash
		cf.L1RollingHashMessageNumber = uint(req.ParentAggregationLastL1RollingHashMessageNumber)
	}

	// similarly for the stream hash
	if len(cf.FinalFtxRollingHash) == 0 {
		cf.FinalFtxRollingHash = req.ParentAggregationLastFtxRollingHash
		cf.FinalFtxNumber = uint(req.ParentAggregationLastFtxNumber)
	}

	cf.L2MessagingBlocksOffsets = utils.HexEncodeToString(PackOffsets(l2MsgBlockOffsets))
	cf.L2MsgRootHashes = PackInMiniTrees(allL2MessageHashes)

	if err := cf.collectInvalidityInfo(cfg, req); err != nil {
		return nil, fmt.Errorf("could not collect invalidity info: %w", err)
	}

	return cf, nil
}

// Prepare the response without running the actual proof
// TODO @gbotrel well, this is a bit of a lie, we do run the proof, don't we?
func CraftResponse(cfg *config.Config, cf *CollectedFields) (resp *Response, err error) {

	if err := validate(cf); err != nil {
		return resp, err
	}

	filteredAddrs := collectFilteredAddresses(cf.InvalidityPI)

	resp = &Response{
		DataHashes:     cf.DataHashes,
		DataParentHash: cf.DataParentHash,
		FinalStateRootHash: func() string {
			if len(cf.ExecutionPI) > 0 {
				return utils.HexEncodeToString(cf.ExecutionPI[len(cf.ExecutionPI)-1].FinalStateRootHash[:])
			}
			return ""
		}(),
		ParentStateRootHash:                     cf.ParentStateRootHashContract.Hex(),
		ParentAggregationLastBlockTimestamp:     cf.ParentAggregationLastBlockTimestamp,
		FinalTimestamp:                          cf.FinalTimestamp,
		LastFinalizedL1RollingHash:              cf.LastFinalizedL1RollingHash,
		L1RollingHash:                           cf.L1RollingHash,
		LastFinalizedL1RollingHashMessageNumber: cf.LastFinalizedL1RollingHashMessageNumber,
		L1RollingHashMessageNumber:              cf.L1RollingHashMessageNumber,
		L2MerkleRoots:                           cf.L2MsgRootHashes,
		L2MsgTreesDepth:                         cf.L2MsgTreeDepth,
		L2MessagingBlocksOffsets:                cf.L2MessagingBlocksOffsets,
		LastFinalizedBlockNumber:                cf.LastFinalizedBlockNumber,
		FinalBlockNumber:                        cf.FinalBlockNumber,
		ParentAggregationFinalShnarf:            cf.ParentAggregationFinalShnarf,
		FinalShnarf:                             cf.FinalShnarf,
		FinalFtxRollingHash:                     cf.FinalFtxRollingHash,
		FinalFtxNumber:                          cf.FinalFtxNumber,
		ParentAggregationFtxNumber:              cf.LastFinalizedFtxNumber,
		ParentAggregationFtxRollingHash:         cf.LastFinalizedFtxRollingHash,
		// chain configuration
		ChainID:              uint64(cfg.Layer2.ChainID),
		BaseFee:              uint64(cfg.Layer2.BaseFee),
		CoinBase:             types.EthAddress(cfg.Layer2.CoinBase),
		L2MessageServiceAddr: types.EthAddress(cfg.Layer2.MsgSvcContract),

		FilteredAddresses: filteredAddrs,
	}

	// @alex: proofless jobs are triggered once during the migration introducing
	// the compression and the aggregation.
	if cf.IsProoflessJob {
		return resp, nil
	}

	pubInputParts := public_input.Aggregation{
		FinalShnarf:                             cf.FinalShnarf,
		ParentAggregationFinalShnarf:            cf.ParentAggregationFinalShnarf,
		ParentStateRootHash:                     cf.ParentStateRootHashContract.Hex(),
		ParentAggregationLastBlockTimestamp:     cf.ParentAggregationLastBlockTimestamp,
		FinalTimestamp:                          cf.FinalTimestamp,
		LastFinalizedBlockNumber:                cf.LastFinalizedBlockNumber,
		FinalBlockNumber:                        cf.FinalBlockNumber,
		LastFinalizedL1RollingHash:              cf.LastFinalizedL1RollingHash,
		L1RollingHash:                           cf.L1RollingHash,
		LastFinalizedL1RollingHashMessageNumber: cf.LastFinalizedL1RollingHashMessageNumber,
		L1RollingHashMessageNumber:              resp.L1RollingHashMessageNumber,
		LastFinalizedFtxRollingHash:             cf.LastFinalizedFtxRollingHash,
		FinalFtxRollingHash:                     cf.FinalFtxRollingHash,
		LastFinalizedFtxNumber:                  cf.LastFinalizedFtxNumber,
		FinalFtxNumber:                          cf.FinalFtxNumber,
		L2MsgRootHashes:                         cf.L2MsgRootHashes,
		L2MsgMerkleTreeDepth:                    l2MsgMerkleTreeDepth,

		// dynamic chain configuration
		ChainID:              uint64(cfg.Layer2.ChainID),
		BaseFee:              uint64(cfg.Layer2.BaseFee),
		CoinBase:             types.EthAddress(cfg.Layer2.CoinBase),
		L2MessageServiceAddr: types.EthAddress(cfg.Layer2.MsgSvcContract),
		IsAllowedCircuitID:   uint64(cfg.Aggregation.IsAllowedCircuitID),

		FilteredAddresses: filteredAddrs,
	}

	resp.AggregatedProofPublicInput = pubInputParts.GetPublicInputHex()

	// This log is aimed at helping debugging in-depth when the proofs are
	// reverted because the public input mismatches. The content of this log
	// can be compared with data on tenderly.
	logrus.Infof("[Aggregation] public inputs components for range (%v-%v): %++v",
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
	utils.ValidateHexString(&err, cf.ParentStateRootHashContract.Hex(), "ParentStateRootHash : %w", 32)
	utils.ValidateHexString(&err, cf.L1RollingHash, "L1RollingHash : %w", 32)
	utils.ValidateHexString(&err, cf.LastFinalizedFtxRollingHash, "parentAggregationLastFtxRollingHash : %w", 32)
	utils.ValidateHexString(&err, cf.FinalFtxRollingHash, "FinalFtxRollingHash : %w", 32)
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

// isKoalaBearHash checks whether a FullBytes32 represents a valid koalabear
// octuplet (8 x 32-bit limbs, each < 0x7f000001). Returns false for BLS hashes.
func isKoalaBearHash(h types.FullBytes32) bool {
	const koalaMod = 0x7f000001
	for i := 0; i < 8; i++ {
		limb := binary.BigEndian.Uint32(h[4*i : 4*i+4])
		if limb >= koalaMod {
			return false
		}
	}
	return true
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

		digests := make([]types.Bls12377Fr, l2MsgMerkleTreeMaxLeaves)

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

func (cf *CollectedFields) collectInvalidityInfo(cfg *config.Config, req *Request) error {
	var (
		po              invalidity.Response
		prevPo          invalidity.Response
		parentFtxNumber = uint64(req.ParentAggregationLastFtxNumber)
	)
	logrus.Infof(" Collecting invalidity info and validating interconnection among invalidity proofs")

	for i, invalReqFPath := range req.InvalidityProofs {

		var (
			fpath = path.Join(cfg.Invalidity.DirTo(), invalReqFPath)
			f     = files.MustRead(fpath)
		)

		if err := json.NewDecoder(f).Decode(&po); err != nil {
			return fmt.Errorf("fields collection, decoding %s, %w", invalReqFPath, err)
		}
		if i == 0 {
			prevPo = po
		}
		cf.InnerCircuitTypes = append(cf.InnerCircuitTypes, pi_interconnection.Invalidity)

		pClaim, err := parseProofClaim(po.Proof, po.PublicInput.Hex(), po.VerifyingKeyShaSum)
		if err != nil {
			return fmt.Errorf("could not parse the proof claim %v for `%v` : %w", i, fpath, err)
		}
		cf.ProofClaims = append(cf.ProofClaims, *pClaim)

		pi := po.FuncInput()
		// Sum(nil) uses Poseidon2
		if recomputed := po.FuncInput().Sum(nil); !bytes.Equal(recomputed, po.PublicInput[:]) {
			return fmt.Errorf("invalidity #%d: public input mismatch: given %x, computed %x", i, po.PublicInput, recomputed)
		}

		if po.SimulatedExecutionBlockTimestamp != prevPo.SimulatedExecutionBlockTimestamp {
			return fmt.Errorf("in the same aggregation, the invalidity proofs have different simulated block timestamps: %d vs %d", prevPo.SimulatedExecutionBlockTimestamp, po.SimulatedExecutionBlockTimestamp)
		}
		if po.SimulatedExecutionBlockNumber != prevPo.SimulatedExecutionBlockNumber {
			return fmt.Errorf("in the same aggregation, the invalidity proofs have different simulated block numbers: %d vs %d", prevPo.SimulatedExecutionBlockNumber, po.SimulatedExecutionBlockNumber)
		}
		if po.ForcedTransactionNumber != parentFtxNumber+1 {
			return fmt.Errorf("forced transaction numbers should be consecutive: jumping from %d to %d instead of incrementing by 1", parentFtxNumber, po.ForcedTransactionNumber)
		}

		if got, want := po.ChainID, cfg.Layer2.ChainID; got != want {
			return fmt.Errorf("invalidity #%d fails CHECK_CHAIN_ID:\n\texpected %x, encountered %x", i, want, got)
		}
		if got, want := po.BaseFee, cfg.Layer2.BaseFee; got != want {
			return fmt.Errorf("invalidity #%d fails CHECK_BASE_FEE:\n\texpected %x, encountered %x", i, want, got)
		}
		if got, want := po.CoinBase, cfg.Layer2.CoinBase; got != types.EthAddress(want) {
			return fmt.Errorf("invalidity #%d fails CHECK_COIN_BASE:\n\texpected CoinBase %x, encountered %x", i, want, got)
		}
		if got, want := po.L2BridgeAddress, cfg.Layer2.MsgSvcContract; got != types.EthAddress(want) {
			return fmt.Errorf("invalidity #%d fails CHECK_SVC_ADDR:\n\texpected L2 service address %x, encountered %x", i, want, got)
		}

		prevPo = po
		parentFtxNumber = po.ForcedTransactionNumber

		cf.InvalidityPI = append(cf.InvalidityPI, *pi)

		cf.FinalFtxNumber = uint(po.ForcedTransactionNumber)
		cf.FinalFtxRollingHash = po.FtxRollingHash.Hex()
	}
	return nil
}

// collectFilteredAddresses extracts filtered addresses from invalidity public
// inputs. For each invalidity PI, if FromIsFiltered is set the FromAddress is
// collected, and if ToIsFiltered is set the ToAddress is collected.
func collectFilteredAddresses(invalidityPIs []public_input.Invalidity) []types.EthAddress {
	var addrs []types.EthAddress
	for i := range invalidityPIs {
		pi := &invalidityPIs[i]
		if pi.FromIsFiltered {
			addrs = append(addrs, pi.FromAddress)
		}
		if pi.ToIsFiltered {
			addrs = append(addrs, pi.ToAddress)
		}
	}
	if addrs == nil {
		return []types.EthAddress{}
	}
	return addrs
}
