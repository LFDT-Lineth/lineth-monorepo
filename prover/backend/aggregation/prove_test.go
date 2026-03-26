package aggregation

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/aggregation"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeProofClaim creates a ProofClaimAssignment with the given circuitID
// and a deterministic VK based on the index.
func makeProofClaim(circuitID int) aggregation.ProofClaimAssignment {
	var vk types.FullBytes32
	vk[0] = byte(circuitID)
	return aggregation.ProofClaimAssignment{
		CircuitID:          circuitID,
		VerifyingKeyShasum: vk,
	}
}

func TestValidateCircuitIDsAllowed(t *testing.T) {
	tests := []struct {
		name       string
		bitmask    uint64
		circuitIDs []int
		sources    []string
		expectErr  bool
		errContain string
	}{
		{
			name:       "all allowed - mainnet bitmask with execution circuits",
			bitmask:    60, // 0b111100 = execution, execution-large, execution-limitless, data-availability-v2
			circuitIDs: []int{2, 3, 4, 5},
			sources:    []string{"file1.json", "file2.json", "file3.json", "file4.json"},
			expectErr:  false,
		},
		{
			name:       "all allowed - sepolia with dummies",
			bitmask:    63, // 0b111111 = all payload circuits
			circuitIDs: []int{0, 1, 2, 4},
			sources:    []string{"dummy-exec.json", "dummy-da.json", "exec.json", "limitless.json"},
			expectErr:  false,
		},
		{
			name:       "disallowed - execution-limitless on execution-only bitmask",
			bitmask:    4, // 0b000100 = only execution (ID 2)
			circuitIDs: []int{4, 4, 4},
			sources:    []string{"file1.json", "file2.json", "file3.json"},
			expectErr:  true,
			errContain: "execution-limitless",
		},
		{
			name:       "disallowed - dummy on mainnet",
			bitmask:    60, // 0b111100 = no dummies
			circuitIDs: []int{0, 2, 3},
			sources:    []string{"dummy.json", "exec.json", "large.json"},
			expectErr:  true,
			errContain: "execution-dummy",
		},
		{
			name:       "mixed allowed and disallowed",
			bitmask:    51, // 0b110011 = dummy-exec, dummy-da, limitless, da-v2
			circuitIDs: []int{0, 1, 2, 4, 5},
			sources:    []string{"de.json", "dd.json", "exec.json", "limitless.json", "da.json"},
			expectErr:  true,
			errContain: "execution",
		},
		{
			name:       "empty proof claims",
			bitmask:    4,
			circuitIDs: []int{},
			sources:    []string{},
			expectErr:  false,
		},
		{
			name:       "single allowed proof",
			bitmask:    16, // 0b010000 = only execution-limitless (ID 4)
			circuitIDs: []int{4},
			sources:    []string{"limitless.json"},
			expectErr:  false,
		},
		{
			name:       "single disallowed proof",
			bitmask:    16, // 0b010000 = only execution-limitless (ID 4)
			circuitIDs: []int{2},
			sources:    []string{"exec.json"},
			expectErr:  true,
			errContain: "execution",
		},
		{
			name:       "zero bitmask disallows all",
			bitmask:    0,
			circuitIDs: []int{2},
			sources:    []string{"exec.json"},
			expectErr:  true,
			errContain: "disallowed",
		},
	}

	// Create a dummy VK list (not used for the allowance check, just needed by signature)
	dummyVKList := []string{
		"0x0000000000000000000000000000000000000000000000000000000000000000",
		"0x0000000000000000000000000000000000000000000000000000000000000001",
		"0x0000000000000000000000000000000000000000000000000000000000000002",
		"0x0000000000000000000000000000000000000000000000000000000000000003",
		"0x0000000000000000000000000000000000000000000000000000000000000004",
		"0x0000000000000000000000000000000000000000000000000000000000000005",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := make([]aggregation.ProofClaimAssignment, len(tt.circuitIDs))
			for i, cid := range tt.circuitIDs {
				claims[i] = makeProofClaim(cid)
			}

			err := validateCircuitIDsAllowed(tt.bitmask, claims, tt.sources, dummyVKList)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateCircuitIDsAllowed_ErrorMessage(t *testing.T) {
	// Test that the error message contains actionable information
	claims := []aggregation.ProofClaimAssignment{
		makeProofClaim(4), // execution-limitless
		makeProofClaim(4),
		makeProofClaim(5), // data-availability-v2 (allowed)
	}
	sources := []string{"block-1.json", "block-2.json", "da.json"}
	dummyVKList := make([]string, 6)

	err := validateCircuitIDsAllowed(4, claims, sources, dummyVKList) // bitmask=4 = only execution (ID 2)
	require.Error(t, err)

	errMsg := err.Error()
	// Should mention how many are disallowed
	assert.Contains(t, errMsg, "3/3")
	// Should mention the disallowed circuit name
	assert.Contains(t, errMsg, "execution-limitless")
	// Should mention the bitmask value
	assert.Contains(t, errMsg, "is_allowed_circuit_id=4")
}
