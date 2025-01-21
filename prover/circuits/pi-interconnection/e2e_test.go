//go:build !fuzzlight

package pi_interconnection_test

import (
	"encoding/base64"
	"fmt"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/backend/aggregation"
	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	circuittesting "github.com/consensys/linea-monorepo/prover/circuits/internal/test_utils"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	pitesting "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/test_utils"
	"github.com/consensys/linea-monorepo/prover/config"
	blobtesting "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1/test_utils"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
)

// TODO test with random values instead of small ones

// some of the execution data are faked
func TestSingleBlockBlob(t *testing.T) {
	testPI(t, pitesting.AssignSingleBlockBlob(t), withSlack(0, 2))
}

func TestSingleBlockBlobE2E(t *testing.T) {
	req := pitesting.AssignSingleBlockBlob(t)
	cfg := config.PublicInput{
		MaxNbDecompression: len(req.Decompressions),
		MaxNbExecution:     len(req.Executions),
		ExecutionMaxNbMsg:  1,
		L2MsgMerkleDepth:   5,
		L2MsgMaxNbMerkle:   1,
	}
	compiled, err := pi_interconnection.Compile(cfg, dummy.Compile)
	assert.NoError(t, err)

	dictStore, err := dictionary.SingletonStore(blobtesting.GetDict(t), 1)
	assert.NoError(t, err)

	a, err := compiled.Assign(req, dictStore)
	assert.NoError(t, err)

	cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, compiled.Circuit, frontend.WithCapacity(3_000_000))
	assert.NoError(t, err)

	w, err := frontend.NewWitness(&a, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)

	assert.NoError(t, cs.IsSolved(w))

}

// some of the execution data are faked
func TestTinyTwoBatchBlob(t *testing.T) {

	blob := blobtesting.TinyTwoBatchBlob(t)

	const lastFinStateRootHash = 34
	stateRootHashes := [3][32]byte{
		internal.Uint64To32Bytes(lastFinStateRootHash),
		internal.Uint64To32Bytes(23),
		internal.Uint64To32Bytes(45),
	}

	execReq := []public_input.Execution{{
		InitialBlockTimestamp:        6,
		FinalStateRootHash:           stateRootHashes[1],
		FinalBlockNumber:             5,
		FinalBlockTimestamp:          6,
		LastRollingHashUpdate:        internal.Uint64To32Bytes(7),
		LastRollingHashUpdateNumber:  8,
		FirstRollingHashUpdateNumber: 8,
		L2MessageHashes:              [][32]byte{internal.Uint64To32Bytes(3)},
		InitialStateRootHash:         stateRootHashes[0],
		InitialBlockNumber:           5,
	}, {
		L2MessageHashes:              [][32]byte{internal.Uint64To32Bytes(9)},
		InitialBlockTimestamp:        7,
		FinalStateRootHash:           stateRootHashes[2],
		FinalBlockNumber:             11,
		FinalBlockTimestamp:          12,
		LastRollingHashUpdate:        internal.Uint64To32Bytes(13),
		LastRollingHashUpdateNumber:  14,
		FirstRollingHashUpdateNumber: 9,
		InitialStateRootHash:         stateRootHashes[1],
		InitialBlockNumber:           6,
	}}

	blobReq := blobsubmission.Request{
		Eip4844Enabled:      true,
		CompressedData:      base64.StdEncoding.EncodeToString(blob),
		ParentStateRootHash: utils.FmtIntHex32Bytes(lastFinStateRootHash),
		FinalStateRootHash:  utils.HexEncodeToString(execReq[1].FinalStateRootHash[:]),
		PrevShnarf:          utils.FmtIntHex32Bytes(2),
	}

	blobResp, err := blobsubmission.CraftResponse(&blobReq)
	assert.NoError(t, err)

	merkleRoots := aggregation.PackInMiniTrees(circuittesting.BlocksToHex(execReq[0].L2MessageHashes, execReq[1].L2MessageHashes))

	req := pi_interconnection.Request{
		Decompressions: []blobsubmission.Response{*blobResp},
		Executions:     execReq,
		Aggregation: public_input.Aggregation{
			FinalShnarf:                             blobResp.ExpectedShnarf,
			ParentAggregationFinalShnarf:            blobReq.PrevShnarf,
			ParentStateRootHash:                     blobReq.ParentStateRootHash,
			ParentAggregationLastBlockTimestamp:     5,
			FinalTimestamp:                          uint(execReq[1].FinalBlockTimestamp),
			LastFinalizedBlockNumber:                4,
			FinalBlockNumber:                        uint(execReq[1].FinalBlockNumber),
			LastFinalizedL1RollingHash:              utils.FmtIntHex32Bytes(13),
			L1RollingHash:                           utils.HexEncodeToString(execReq[1].LastRollingHashUpdate[:]),
			LastFinalizedL1RollingHashMessageNumber: 7,
			L1RollingHashMessageNumber:              uint(execReq[1].LastRollingHashUpdateNumber),
			L2MsgRootHashes:                         merkleRoots,
			L2MsgMerkleTreeDepth:                    5,
		},
	}

	testPI(t, req, withSlack(0, 2))
}

func TestTwoTwoBatchBlobs(t *testing.T) {
	blobs := blobtesting.ConsecutiveBlobs(t, 2, 2)

	execReq := []public_input.Execution{{
		L2MessageHashes:              [][32]byte{internal.Uint64To32Bytes(3)},
		InitialBlockTimestamp:        6,
		FinalStateRootHash:           internal.Uint64To32Bytes(4),
		FinalBlockNumber:             5,
		FinalBlockTimestamp:          6,
		LastRollingHashUpdate:        internal.Uint64To32Bytes(7),
		LastRollingHashUpdateNumber:  8,
		InitialStateRootHash:         internal.Uint64To32Bytes(1),
		InitialBlockNumber:           5,
		FirstRollingHashUpdateNumber: 8,
	}, {
		L2MessageHashes:              [][32]byte{internal.Uint64To32Bytes(9)},
		InitialBlockTimestamp:        7,
		InitialStateRootHash:         internal.Uint64To32Bytes(4),
		InitialBlockNumber:           6,
		FirstRollingHashUpdateNumber: 9,
		FinalStateRootHash:           internal.Uint64To32Bytes(10),
		FinalBlockNumber:             11,
		FinalBlockTimestamp:          12,
		LastRollingHashUpdate:        internal.Uint64To32Bytes(13),
		LastRollingHashUpdateNumber:  14,
	}, {
		L2MessageHashes:              [][32]byte{internal.Uint64To32Bytes(15)},
		InitialBlockTimestamp:        13,
		InitialBlockNumber:           12,
		InitialStateRootHash:         internal.Uint64To32Bytes(10),
		FirstRollingHashUpdateNumber: 15,
		FinalStateRootHash:           internal.Uint64To32Bytes(16),
		FinalBlockNumber:             17,
		FinalBlockTimestamp:          18,
		LastRollingHashUpdate:        internal.Uint64To32Bytes(19),
		LastRollingHashUpdateNumber:  20,
	}, {
		InitialBlockNumber:           18,
		InitialStateRootHash:         internal.Uint64To32Bytes(16),
		L2MessageHashes:              [][32]byte{internal.Uint64To32Bytes(21)},
		InitialBlockTimestamp:        19,
		FirstRollingHashUpdateNumber: 21,
		FinalStateRootHash:           internal.Uint64To32Bytes(22),
		FinalBlockNumber:             23,
		FinalBlockTimestamp:          24,
		LastRollingHashUpdate:        internal.Uint64To32Bytes(25),
		LastRollingHashUpdateNumber:  26,
	}}

	blobReq0 := blobsubmission.Request{
		Eip4844Enabled:      true,
		CompressedData:      base64.StdEncoding.EncodeToString(blobs[0]),
		ParentStateRootHash: utils.FmtIntHex32Bytes(1),
		FinalStateRootHash:  utils.HexEncodeToString(execReq[1].FinalStateRootHash[:]),
		PrevShnarf:          utils.FmtIntHex32Bytes(2),
	}

	blobResp0, err := blobsubmission.CraftResponse(&blobReq0)
	require.NoError(t, err)

	blobReq1 := blobsubmission.Request{
		Eip4844Enabled:      true,
		CompressedData:      base64.StdEncoding.EncodeToString(blobs[1]),
		ParentStateRootHash: blobReq0.FinalStateRootHash,
		FinalStateRootHash:  utils.HexEncodeToString(execReq[3].FinalStateRootHash[:]),
		PrevShnarf:          blobResp0.ExpectedShnarf,
	}

	blobResp1, err := blobsubmission.CraftResponse(&blobReq1)
	require.NoError(t, err)

	merkleRoots := aggregation.PackInMiniTrees(circuittesting.BlocksToHex(execReq[0].L2MessageHashes, execReq[1].L2MessageHashes, execReq[2].L2MessageHashes, execReq[3].L2MessageHashes))

	req := pi_interconnection.Request{
		Decompressions: []blobsubmission.Response{*blobResp0, *blobResp1},
		Executions:     execReq,
		Aggregation: public_input.Aggregation{
			FinalShnarf:                             blobResp1.ExpectedShnarf,
			ParentAggregationFinalShnarf:            blobReq0.PrevShnarf,
			ParentStateRootHash:                     blobReq0.ParentStateRootHash,
			ParentAggregationLastBlockTimestamp:     5,
			FinalTimestamp:                          uint(execReq[3].FinalBlockTimestamp),
			LastFinalizedBlockNumber:                4,
			FinalBlockNumber:                        uint(execReq[3].FinalBlockNumber),
			LastFinalizedL1RollingHash:              utils.FmtIntHex32Bytes(7),
			L1RollingHash:                           utils.HexEncodeToString(execReq[3].LastRollingHashUpdate[:]),
			LastFinalizedL1RollingHashMessageNumber: 7,
			L1RollingHashMessageNumber:              uint(execReq[3].LastRollingHashUpdateNumber),
			L2MsgRootHashes:                         merkleRoots,
			L2MsgMerkleTreeDepth:                    5,
		},
	}

	testPI(t, req, withSlack(0, 2))
}

type testPIConfig struct {
	slack []int
}

type testPIOption func(*testPIConfig)

func withSlack(slack ...int) testPIOption {
	return func(cfg *testPIConfig) {
		cfg.slack = append(cfg.slack, slack...)
	}
}

func testPI(t *testing.T, req pi_interconnection.Request, options ...testPIOption) {
	var cfg testPIConfig
	for _, o := range options {
		o(&cfg)
	}
	slices.Sort(cfg.slack)
	cfg.slack = slices.Compact(cfg.slack)
	if len(cfg.slack) == 0 {
		cfg.slack = []int{0}
	}
	slackIterationNum := len(cfg.slack) * len(cfg.slack)
	slackIterationNum *= slackIterationNum

	dictStore, err := dictionary.SingletonStore(blobtesting.GetDict(t), 1)
	assert.NoError(t, err)

	var slack [4]int

	for i := 0; i < slackIterationNum; i++ {

		decomposeLittleEndian(t, slack[:], i, len(cfg.slack))
		for j := range slack {
			slack[j] = cfg.slack[slack[j]]
		}

		cfg := config.PublicInput{
			MaxNbDecompression: len(req.Decompressions) + slack[0],
			MaxNbExecution:     len(req.Executions) + slack[1],
			ExecutionMaxNbMsg:  1 + slack[2],
			L2MsgMerkleDepth:   5,
			L2MsgMaxNbMerkle:   1 + slack[3],
			MockKeccakWizard:   true,
		}

		t.Run(fmt.Sprintf("slack profile %v", slack), func(t *testing.T) {
			compiled, err := pi_interconnection.Compile(cfg, dummy.Compile)
			assert.NoError(t, err)

			a, err := compiled.Assign(req, dictStore)
			assert.NoError(t, err)

			assert.NoError(t, test.IsSolved(compiled.Circuit, &a, ecc.BLS12_377.ScalarField()))
		})
	}
}

func decomposeLittleEndian(t *testing.T, digits []int, n, base int) {
	for i := range digits {
		digits[i] = n % base
		n /= base
	}
	assert.Zero(t, n)
}
