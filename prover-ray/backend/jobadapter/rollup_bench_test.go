package jobadapter

import (
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/ssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRollupBench_Input checks that the rollup guest input built from a request
// equals the guest input the request was written from (scripts/bench). It runs
// only when ROLLUP_BENCH_REQUEST and ROLLUP_BENCH_INPUT are set.
func TestRollupBench_Input(t *testing.T) {
	requestPath, inputPath := os.Getenv("ROLLUP_BENCH_REQUEST"), os.Getenv("ROLLUP_BENCH_INPUT")
	if requestPath == "" || inputPath == "" {
		t.Skip("set ROLLUP_BENCH_REQUEST and ROLLUP_BENCH_INPUT")
	}
	request, err := os.ReadFile(requestPath)
	require.NoError(t, err)
	want, err := os.ReadFile(inputPath)
	require.NoError(t, err)

	req, err := DecodeRollupRequest(request)
	require.NoError(t, err)
	got, err := ssz.EncodeRollupInput(buildRollupInput(req))
	require.NoError(t, err)
	assert.Equal(t, want, got, "built input != ROLLUP_BENCH_INPUT")
}
