package jobadapter

import (
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/ssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAggregationBench_Input checks that the aggregation guest input built
// from a request equals the guest input the request was written from
// (scripts/bench). It runs only when AGGREGATION_BENCH_REQUEST and
// AGGREGATION_BENCH_INPUT are set.
func TestAggregationBench_Input(t *testing.T) {
	requestPath, inputPath := os.Getenv("AGGREGATION_BENCH_REQUEST"), os.Getenv("AGGREGATION_BENCH_INPUT")
	if requestPath == "" || inputPath == "" {
		t.Skip("set AGGREGATION_BENCH_REQUEST and AGGREGATION_BENCH_INPUT")
	}
	request, err := os.ReadFile(requestPath)
	require.NoError(t, err)
	want, err := os.ReadFile(inputPath)
	require.NoError(t, err)

	req, err := DecodeAggregationRequest(request)
	require.NoError(t, err)
	in, err := buildAggregationInput(req)
	require.NoError(t, err)
	got, err := ssz.EncodeAggregationInput(in)
	require.NoError(t, err)
	assert.Equal(t, want, got, "built input != AGGREGATION_BENCH_INPUT")
}
