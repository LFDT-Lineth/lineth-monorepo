package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProofTypeForName(t *testing.T) {
	cases := map[string]backend.ProofType{
		"10-11-getZkL2ExecutionProofV1.json":       backend.ProofTypeL2Execution,
		"10-14-getZkRollupProofV1.json":            backend.ProofTypeRollup,
		"10-18-getZkRollupAggregationProofV1.json": backend.ProofTypeRollupAggregation,
		"bad.json": backend.ProofTypeL2Execution,
	}
	for name, want := range cases {
		assert.Equal(t, want, proofTypeForName(name), name)
	}
}

func TestAdapter_RollupRequest_RoutedAndShaped(t *testing.T) {
	mock := &mockProver{}
	a, root := newAdapter(t, mock)
	name := "10-14-getZkRollupProofV1.json"
	placeRequest(t, root, name, "10-14-getZkRollupProofV1.request.json")

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	require.Len(t, mock.jobs, 1)
	assert.Equal(t, backend.ProofTypeRollup, mock.jobs[0].Type)
	assert.Equal(t, uint64(10), mock.jobs[0].StartBlock)
	assert.Equal(t, uint64(14), mock.jobs[0].EndBlock)

	resp := readExecutionResponse(t, root, name)
	assertShapeMatchesFixture(t, "10-14-getZkRollupProofV1.response.json", resp)
	assert.Equal(t, "0x31139b3eaece046f5675fe237c36246e7bb2a5acc4cf4b358aef65c6d3771f4d", resp["programVk"])
}

func TestAdapter_AggregationRequest_RoutedAndShaped(t *testing.T) {
	mock := &mockProver{}
	a, root := newAdapter(t, mock)
	name := "10-18-getZkRollupAggregationProofV1.json"
	placeRequest(t, root, name, "10-18-getZkRollupAggregationProofV1.request.json")

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	require.Len(t, mock.jobs, 1)
	assert.Equal(t, backend.ProofTypeRollupAggregation, mock.jobs[0].Type)
	assert.Equal(t, uint64(10), mock.jobs[0].StartBlock)
	assert.Equal(t, uint64(18), mock.jobs[0].EndBlock)

	resp := readExecutionResponse(t, root, name)
	assertShapeMatchesFixture(t, "10-18-getZkRollupAggregationProofV1.response.json", resp)
	assert.NotContains(t, resp, "programVk")
}

func assertShapeMatchesFixture(t *testing.T, fixture string, got map[string]any) {
	t.Helper()
	data, err := os.ReadFile("../testdata/" + fixture)
	require.NoError(t, err)
	var want map[string]any
	require.NoError(t, json.Unmarshal(data, &want))
	assertJSONShape(t, want, got, "response")
}
