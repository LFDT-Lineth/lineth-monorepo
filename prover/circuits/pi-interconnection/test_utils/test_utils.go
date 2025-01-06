package test_utils

import (
	"encoding/base64"

	"github.com/consensys/linea-monorepo/prover/backend/aggregation"
	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/circuits/internal/test_utils"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	blobtesting "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1/test_utils"

	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func AssignSingleBlockBlob(t require.TestingT) pi_interconnection.Request {
	blob := blobtesting.SingleBlockBlob(t)

	finalStateRootHash := internal.Uint64To32Bytes(2)

	blobReq := blobsubmission.Request{
		Eip4844Enabled:      true,
		CompressedData:      base64.StdEncoding.EncodeToString(blob),
		ParentStateRootHash: utils.FmtIntHex32Bytes(1),
		FinalStateRootHash:  utils.HexEncodeToString(finalStateRootHash[:]),
		PrevShnarf:          utils.FmtIntHex32Bytes(3),
	}

	blobResp, err := blobsubmission.CraftResponse(&blobReq)
	assert.NoError(t, err)

	execReq := public_input.Execution{
		L2MessageHashes:              [][32]byte{internal.Uint64To32Bytes(4)},
		InitialBlockTimestamp:        7,
		FinalStateRootHash:           finalStateRootHash,
		FinalBlockNumber:             9,
		FinalBlockTimestamp:          10,
		LastRollingHashUpdate:        internal.Uint64To32Bytes(11),
		LastRollingHashUpdateNumber:  9,
		FirstRollingHashUpdateNumber: 9,
		InitialBlockNumber:           6,
		InitialStateRootHash:         internal.Uint64To32Bytes(1),
	}

	merkleRoots := aggregation.PackInMiniTrees(test_utils.BlocksToHex(execReq.L2MessageHashes))

	return pi_interconnection.Request{
		DictPath:       "../../lib/compressor/compressor_dict.bin",
		Decompressions: []blobsubmission.Response{*blobResp},
		Executions:     []public_input.Execution{execReq},
		Aggregation: public_input.Aggregation{
			FinalShnarf:                             blobResp.ExpectedShnarf,
			ParentAggregationFinalShnarf:            blobReq.PrevShnarf,
			ParentStateRootHash:                     blobReq.ParentStateRootHash,
			ParentAggregationLastBlockTimestamp:     6,
			FinalTimestamp:                          uint(execReq.FinalBlockTimestamp),
			LastFinalizedBlockNumber:                5,
			FinalBlockNumber:                        uint(execReq.FinalBlockNumber),
			LastFinalizedL1RollingHash:              utils.FmtIntHex32Bytes(7),
			L1RollingHash:                           utils.HexEncodeToString(execReq.LastRollingHashUpdate[:]),
			LastFinalizedL1RollingHashMessageNumber: 8,
			L1RollingHashMessageNumber:              uint(execReq.LastRollingHashUpdateNumber),
			L2MsgRootHashes:                         merkleRoots,
			L2MsgMerkleTreeDepth:                    5,
		},
	}
}
