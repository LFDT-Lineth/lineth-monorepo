package jobadapter

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
)

func TestProofTypeForName(t *testing.T) {
	cases := map[string]backend.ProofType{
		"10-11-getZkL2ExecutionProofV1.json":       backend.ProofTypeL2Execution,
		"10-14-getZkRollupProofV1.json":            backend.ProofTypeRollup,
		"10-18-getZkRollupAggregationProofV1.json": backend.ProofTypeRollupAggregation,
		"bad.json": backend.ProofTypeL2Execution,
	}
	for name, want := range cases {
		assert.Equal(t, want, ProofTypeForName(name), name)
	}
}
