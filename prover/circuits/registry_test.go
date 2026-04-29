package circuits

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeIsAllowedCircuitID(t *testing.T) {
	tests := []struct {
		name            string
		allowedCircuits []string
		expectedBitmask uint64
		expectError     bool
		errorContains   string
	}{
		{
			name: "mainnet configuration",
			allowedCircuits: []string{
				"execution",
				"execution-large",
				"execution-limitless",
				"data-availability-v2",
				"invalidity-nonce-balance",
				"invalidity-precompile-logs",
				"invalidity-filtered-address",
				"invalidity-precompile-logs-limitless",
				"invalidity-precompile-logs-large",
			},
			expectedBitmask: 15932, // bits 2,3,4,5,9,10,11,12,13
			expectError:     false,
		},
		{
			name: "sepolia/testnet configuration",
			allowedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
				"execution",
				"execution-large",
				"execution-limitless",
				"data-availability-v2",
				"invalidity-nonce-balance-dummy",
				"invalidity-precompile-logs-dummy",
				"invalidity-filtered-address-dummy",
				"invalidity-nonce-balance",
				"invalidity-precompile-logs",
				"invalidity-filtered-address",
				"invalidity-precompile-logs-limitless",
				"invalidity-precompile-logs-large",
			},
			expectedBitmask: 16383, // bits 0-13 all set
			expectError:     false,
		},
		{
			name: "devnet configuration",
			allowedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
				"execution",
				"execution-large",
				"data-availability-v2",
				"invalidity-nonce-balance-dummy",
				"invalidity-precompile-logs-dummy",
				"invalidity-filtered-address-dummy",
				"invalidity-nonce-balance",
				"invalidity-precompile-logs",
				"invalidity-filtered-address",
				"invalidity-precompile-logs-large",
			},
			expectedBitmask: 12271, // bits 0,1,2,3,5,6,7,8,9,10,11,13
			expectError:     false,
		},
		{
			name: "integration-full configuration",
			allowedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
				"execution",
				"invalidity-nonce-balance-dummy",
				"invalidity-precompile-logs-dummy",
				"invalidity-filtered-address-dummy",
				"invalidity-nonce-balance",
				"invalidity-precompile-logs",
				"invalidity-filtered-address",
			},
			expectedBitmask: 4039, // bits 0,1,2,6,7,8,9,10,11
			expectError:     false,
		},
		{
			name: "integration-development configuration",
			allowedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
				"data-availability-v2",
				"invalidity-nonce-balance-dummy",
				"invalidity-precompile-logs-dummy",
				"invalidity-filtered-address-dummy",
			},
			expectedBitmask: 483, // bits 0,1,5,6,7,8
			expectError:     false,
		},
		{
			name: "single circuit",
			allowedCircuits: []string{
				"execution",
			},
			expectedBitmask: 4, // 0b00000100 = 2^2
			expectError:     false,
		},
		{
			name:            "empty list",
			allowedCircuits: []string{},
			expectedBitmask: 0,
			expectError:     false,
		},
		{
			name: "unknown circuit name",
			allowedCircuits: []string{
				"execution",
				"nonexistent-circuit",
			},
			expectedBitmask: 0,
			expectError:     true,
			errorContains:   "unknown circuit name",
		},
		{
			name: "infrastructure circuit - aggregation",
			allowedCircuits: []string{
				"execution",
				"aggregation",
			},
			expectedBitmask: 0,
			expectError:     true,
			errorContains:   "infrastructure circuit",
		},
		{
			name: "infrastructure circuit - public-input-interconnection",
			allowedCircuits: []string{
				"execution",
				"public-input-interconnection",
			},
			expectedBitmask: 0,
			expectError:     true,
			errorContains:   "infrastructure circuit",
		},
		{
			name: "infrastructure circuit - emulation",
			allowedCircuits: []string{
				"execution",
				"emulation",
			},
			expectedBitmask: 0,
			expectError:     true,
			errorContains:   "infrastructure circuit",
		},
		{
			name: "infrastructure circuit - emulation-dummy",
			allowedCircuits: []string{
				"execution",
				"emulation-dummy",
			},
			expectedBitmask: 0,
			expectError:     true,
			errorContains:   "infrastructure circuit",
		},
		{
			name: "all payload circuits (0-13)",
			allowedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
				"execution",
				"execution-large",
				"execution-limitless",
				"data-availability-v2",
				"invalidity-nonce-balance-dummy",
				"invalidity-precompile-logs-dummy",
				"invalidity-filtered-address-dummy",
				"invalidity-nonce-balance",
				"invalidity-precompile-logs",
				"invalidity-filtered-address",
				"invalidity-precompile-logs-limitless",
				"invalidity-precompile-logs-large",
			},
			expectedBitmask: 16383, // 0b11111111111111 = all bits 0-13 set
			expectError:     false,
		},
		{
			name: "invalidity production circuits only",
			allowedCircuits: []string{
				"invalidity-nonce-balance",
				"invalidity-precompile-logs",
				"invalidity-filtered-address",
			},
			expectedBitmask: 3584, // 2^9 + 2^10 + 2^11 = 512+1024+2048
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bitmask, err := ComputeIsAllowedCircuitID(tt.allowedCircuits)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Equal(t, uint64(0), bitmask)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedBitmask, bitmask,
					"Expected bitmask %d (0b%b), got %d (0b%b)",
					tt.expectedBitmask, tt.expectedBitmask, bitmask, bitmask)
			}
		})
	}
}

func TestIsCircuitAllowed(t *testing.T) {
	tests := []struct {
		name      string
		bitmask   uint64
		circuitID uint
		expected  bool
	}{
		{
			name:      "mainnet allows execution",
			bitmask:   60, // mainnet
			circuitID: 2,  // execution
			expected:  true,
		},
		{
			name:      "mainnet disallows execution-dummy",
			bitmask:   60,
			circuitID: 0, // execution-dummy
			expected:  false,
		},
		{
			name:      "sepolia allows execution-dummy",
			bitmask:   63, // sepolia
			circuitID: 0,  // execution-dummy
			expected:  true,
		},
		{
			name:      "sepolia allows execution-large",
			bitmask:   63,
			circuitID: 3, // execution-large
			expected:  true,
		},
		{
			name:      "sepolia disallows emulation-dummy",
			bitmask:   63,
			circuitID: 17, // emulation-dummy
			expected:  false,
		},
		{
			name:      "zero bitmask disallows everything",
			bitmask:   0,
			circuitID: 2,
			expected:  false,
		},
		{
			name:      "all bits set allows everything",
			bitmask:   63, // 0b00111111
			circuitID: 5,  // data-availability-v2
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCircuitAllowed(tt.bitmask, tt.circuitID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAllowedCircuitNames(t *testing.T) {
	tests := []struct {
		name             string
		bitmask          uint64
		expectedCircuits []string
	}{
		{
			name:    "mainnet configuration",
			bitmask: 15932,
			expectedCircuits: []string{
				"execution",
				"execution-large",
				"execution-limitless",
				"data-availability-v2",
				"invalidity-nonce-balance",
				"invalidity-precompile-logs",
				"invalidity-filtered-address",
				"invalidity-precompile-logs-limitless",
				"invalidity-precompile-logs-large",
			},
		},
		{
			name:    "sepolia configuration",
			bitmask: 16383,
			expectedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
				"execution",
				"execution-large",
				"execution-limitless",
				"data-availability-v2",
				"invalidity-nonce-balance-dummy",
				"invalidity-precompile-logs-dummy",
				"invalidity-filtered-address-dummy",
				"invalidity-nonce-balance",
				"invalidity-precompile-logs",
				"invalidity-filtered-address",
				"invalidity-precompile-logs-limitless",
				"invalidity-precompile-logs-large",
			},
		},
		{
			name:             "zero bitmask",
			bitmask:          0,
			expectedCircuits: []string{},
		},
		{
			name:    "only dummy circuits",
			bitmask: 3, // 0b00000011 = bits 0,1
			expectedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAllowedCircuitNames(tt.bitmask)

			// Sort both slices for comparison (map iteration order is random)
			assert.ElementsMatch(t, tt.expectedCircuits, result,
				"Expected circuits %v, got %v for bitmask %d (0b%b)",
				tt.expectedCircuits, result, tt.bitmask, tt.bitmask)
		})
	}
}

func TestRoundTripComputeAndCheck(t *testing.T) {
	// Test that computing a bitmask and then checking circuits gives expected results
	allowedCircuits := []string{
		"execution",
		"execution-large",
		"data-availability-v2",
	}

	bitmask, err := ComputeIsAllowedCircuitID(allowedCircuits)
	require.NoError(t, err)

	// These should be allowed
	assert.True(t, IsCircuitAllowed(bitmask, 2), "execution should be allowed")
	assert.True(t, IsCircuitAllowed(bitmask, 3), "execution-large should be allowed")
	assert.True(t, IsCircuitAllowed(bitmask, 5), "data-availability-v2 should be allowed")

	// These should NOT be allowed
	assert.False(t, IsCircuitAllowed(bitmask, 0), "execution-dummy should not be allowed")
	assert.False(t, IsCircuitAllowed(bitmask, 4), "execution-limitless should not be allowed")
	assert.False(t, IsCircuitAllowed(bitmask, 1), "data-availability-dummy should not be allowed")

	// Get allowed names and verify
	names := GetAllowedCircuitNames(bitmask)
	assert.ElementsMatch(t, allowedCircuits, names)
}

func TestGlobalCircuitIDMapping(t *testing.T) {
	// Verify the mapping contains expected entries
	assert.Equal(t, uint(0), GlobalCircuitIDMapping["execution-dummy"])
	assert.Equal(t, uint(1), GlobalCircuitIDMapping["data-availability-dummy"])
	assert.Equal(t, uint(2), GlobalCircuitIDMapping["execution"])
	assert.Equal(t, uint(3), GlobalCircuitIDMapping["execution-large"])
	assert.Equal(t, uint(4), GlobalCircuitIDMapping["execution-limitless"])
	assert.Equal(t, uint(5), GlobalCircuitIDMapping["data-availability-v2"])
	assert.Equal(t, uint(6), GlobalCircuitIDMapping["invalidity-nonce-balance-dummy"])
	assert.Equal(t, uint(7), GlobalCircuitIDMapping["invalidity-precompile-logs-dummy"])
	assert.Equal(t, uint(8), GlobalCircuitIDMapping["invalidity-filtered-address-dummy"])
	assert.Equal(t, uint(9), GlobalCircuitIDMapping["invalidity-nonce-balance"])
	assert.Equal(t, uint(10), GlobalCircuitIDMapping["invalidity-precompile-logs"])
	assert.Equal(t, uint(11), GlobalCircuitIDMapping["invalidity-filtered-address"])
	assert.Equal(t, uint(12), GlobalCircuitIDMapping["invalidity-precompile-logs-limitless"])
	assert.Equal(t, uint(13), GlobalCircuitIDMapping["invalidity-precompile-logs-large"])
	assert.Equal(t, uint(14), GlobalCircuitIDMapping["emulation"])
	assert.Equal(t, uint(15), GlobalCircuitIDMapping["aggregation"])
	assert.Equal(t, uint(16), GlobalCircuitIDMapping["public-input-interconnection"])
	assert.Equal(t, uint(17), GlobalCircuitIDMapping["emulation-dummy"])

	// Verify no duplicate IDs
	seen := make(map[uint]string)
	for name, id := range GlobalCircuitIDMapping {
		if existingName, exists := seen[id]; exists {
			t.Errorf("Duplicate circuit ID %d for circuits '%s' and '%s'", id, existingName, name)
		}
		seen[id] = name
	}

	// Verify we have exactly 18 circuits
	assert.Equal(t, 18, len(GlobalCircuitIDMapping))
}

// Example test showing how to use these functions for config validation
func TestExampleUsage(t *testing.T) {
	// Example: User wants to configure mainnet
	mainnetAllowedInputs := []string{
		"execution",
		"execution-large",
		"execution-limitless",
		"data-availability-v2",
		"invalidity-nonce-balance",
		"invalidity-precompile-logs",
		"invalidity-filtered-address",
		"invalidity-precompile-logs-limitless",
		"invalidity-precompile-logs-large",
	}

	// Compute the bitmask
	bitmask, err := ComputeIsAllowedCircuitID(mainnetAllowedInputs)
	require.NoError(t, err)
	assert.Equal(t, uint64(15932), bitmask)

	// Verify it matches what's in the config
	t.Logf("Mainnet is_allowed_circuit_id = %d (binary: 0b%b)", bitmask, bitmask)

	// Check specific circuits
	assert.True(t, IsCircuitAllowed(bitmask, 2), "execution should be allowed in mainnet")
	assert.False(t, IsCircuitAllowed(bitmask, 0), "execution-dummy should not be allowed in mainnet")

	// Get human-readable list
	allowed := GetAllowedCircuitNames(bitmask)
	t.Logf("Mainnet allows: %v", allowed)
}

// CircuitConfig represents a visual configuration of allowed circuits.
// Each field corresponds to a circuit ID (0-14 for payload circuits).
// Set to true to allow the circuit, false to disallow.
type CircuitConfig struct {
	ExecutionDummy                    bool // ID 0
	DataAvailabilityDummy             bool // ID 1
	Execution                         bool // ID 2
	ExecutionLarge                    bool // ID 3
	ExecutionLimitless                bool // ID 4
	DataAvailabilityV2                bool // ID 5
	InvalidityNonceBalanceDummy       bool // ID 6
	InvalidityPrecompileLogsDummy     bool // ID 7
	InvalidityFilteredAddrDummy       bool // ID 8
	InvalidityNonceBalance            bool // ID 9
	InvalidityPrecompileLogs          bool // ID 10
	InvalidityFilteredAddr            bool // ID 11
	InvalidityPrecompileLogsLimitless bool // ID 12
	InvalidityPrecompileLogsLarge     bool // ID 13
}

// ToBitmask converts a CircuitConfig to a bitmask value.
func (c CircuitConfig) ToBitmask() uint64 {
	var bitmask uint64
	if c.ExecutionDummy {
		bitmask |= 1 << 0
	}
	if c.DataAvailabilityDummy {
		bitmask |= 1 << 1
	}
	if c.Execution {
		bitmask |= 1 << 2
	}
	if c.ExecutionLarge {
		bitmask |= 1 << 3
	}
	if c.ExecutionLimitless {
		bitmask |= 1 << 4
	}
	if c.DataAvailabilityV2 {
		bitmask |= 1 << 5
	}
	if c.InvalidityNonceBalanceDummy {
		bitmask |= 1 << 6
	}
	if c.InvalidityPrecompileLogsDummy {
		bitmask |= 1 << 7
	}
	if c.InvalidityFilteredAddrDummy {
		bitmask |= 1 << 8
	}
	if c.InvalidityNonceBalance {
		bitmask |= 1 << 9
	}
	if c.InvalidityPrecompileLogs {
		bitmask |= 1 << 10
	}
	if c.InvalidityFilteredAddr {
		bitmask |= 1 << 11
	}
	if c.InvalidityPrecompileLogsLimitless {
		bitmask |= 1 << 12
	}
	if c.InvalidityPrecompileLogsLarge {
		bitmask |= 1 << 13
	}
	return bitmask
}

// String returns a visual representation of the circuit configuration.
func (c CircuitConfig) String() string {
	return fmt.Sprintf(`Circuit Configuration:
  ID 0  - execution-dummy:                          %v
  ID 1  - data-availability-dummy:                  %v
  ID 2  - execution:                                %v
  ID 3  - execution-large:                          %v
  ID 4  - execution-limitless:                      %v
  ID 5  - data-availability-v2:                     %v
  ID 6  - invalidity-nonce-balance-dummy:           %v
  ID 7  - invalidity-precompile-logs-dummy:         %v
  ID 8  - invalidity-filtered-address-dummy:        %v
  ID 9  - invalidity-nonce-balance:                 %v
  ID 10 - invalidity-precompile-logs:               %v
  ID 11 - invalidity-filtered-address:              %v
  ID 12 - invalidity-precompile-logs-limitless:     %v
  ID 13 - invalidity-precompile-logs-large:         %v
  
  Bitmask: %d (binary: 0b%014b)`,
		c.ExecutionDummy,
		c.DataAvailabilityDummy,
		c.Execution,
		c.ExecutionLarge,
		c.ExecutionLimitless,
		c.DataAvailabilityV2,
		c.InvalidityNonceBalanceDummy,
		c.InvalidityPrecompileLogsDummy,
		c.InvalidityFilteredAddrDummy,
		c.InvalidityNonceBalance,
		c.InvalidityPrecompileLogs,
		c.InvalidityFilteredAddr,
		c.InvalidityPrecompileLogsLimitless,
		c.InvalidityPrecompileLogsLarge,
		c.ToBitmask(),
		c.ToBitmask())
}

// TestVisualCircuitConfiguration provides a visual way to configure and calculate
// bitmask values for different environments. This test helps understand which
// circuits are enabled and makes it easy to calculate the is_allowed_circuit_id value.
//
// To use this test for calculating a new bitmask:
// 1. Copy one of the configurations below
// 2. Set each circuit to true/false based on your requirements
// 3. Run the test with -v flag to see the calculated bitmask
// 4. Use the printed decimal value as is_allowed_circuit_id in your config
func TestVisualCircuitConfiguration(t *testing.T) {
	tests := []struct {
		name            string
		config          CircuitConfig
		expectedBitmask uint64
	}{
		{
			name: "Mainnet (production only)",
			config: CircuitConfig{
				Execution:                         true,
				ExecutionLarge:                    true,
				ExecutionLimitless:                true,
				DataAvailabilityV2:                true,
				InvalidityNonceBalance:            true,
				InvalidityPrecompileLogs:          true,
				InvalidityFilteredAddr:            true,
				InvalidityPrecompileLogsLimitless: true,
				InvalidityPrecompileLogsLarge:     true,
			},
			expectedBitmask: 15932,
		},
		{
			name: "Sepolia/Testnet (includes dummy circuits)",
			config: CircuitConfig{
				ExecutionDummy:                    true,
				DataAvailabilityDummy:             true,
				Execution:                         true,
				ExecutionLarge:                    true,
				ExecutionLimitless:                true,
				DataAvailabilityV2:                true,
				InvalidityNonceBalanceDummy:       true,
				InvalidityPrecompileLogsDummy:     true,
				InvalidityFilteredAddrDummy:       true,
				InvalidityNonceBalance:            true,
				InvalidityPrecompileLogs:          true,
				InvalidityFilteredAddr:            true,
				InvalidityPrecompileLogsLimitless: true,
				InvalidityPrecompileLogsLarge:     true,
			},
			expectedBitmask: 16383,
		},
		{
			name: "Devnet (no execution-limitless)",
			config: CircuitConfig{
				ExecutionDummy:                true,
				DataAvailabilityDummy:         true,
				Execution:                     true,
				ExecutionLarge:                true,
				DataAvailabilityV2:            true,
				InvalidityNonceBalanceDummy:   true,
				InvalidityPrecompileLogsDummy: true,
				InvalidityFilteredAddrDummy:   true,
				InvalidityNonceBalance:        true,
				InvalidityPrecompileLogs:      true,
				InvalidityFilteredAddr:        true,
				InvalidityPrecompileLogsLarge: true,
			},
			expectedBitmask: 12271,
		},
		{
			name: "Integration-full (minimal)",
			config: CircuitConfig{
				ExecutionDummy:                true,
				DataAvailabilityDummy:         true,
				Execution:                     true,
				InvalidityNonceBalanceDummy:   true,
				InvalidityPrecompileLogsDummy: true,
				InvalidityFilteredAddrDummy:   true,
				InvalidityNonceBalance:        true,
				InvalidityPrecompileLogs:      true,
				InvalidityFilteredAddr:        true,
			},
			expectedBitmask: 4039,
		},
		{
			name: "Integration-development (dummy + data-availability)",
			config: CircuitConfig{
				ExecutionDummy:                true,
				DataAvailabilityDummy:         true,
				DataAvailabilityV2:            true,
				InvalidityNonceBalanceDummy:   true,
				InvalidityPrecompileLogsDummy: true,
				InvalidityFilteredAddrDummy:   true,
			},
			expectedBitmask: 483,
		},
		{
			name: "All payload circuits enabled",
			config: CircuitConfig{
				ExecutionDummy:                    true,
				DataAvailabilityDummy:             true,
				Execution:                         true,
				ExecutionLarge:                    true,
				ExecutionLimitless:                true,
				DataAvailabilityV2:                true,
				InvalidityNonceBalanceDummy:       true,
				InvalidityPrecompileLogsDummy:     true,
				InvalidityFilteredAddrDummy:       true,
				InvalidityNonceBalance:            true,
				InvalidityPrecompileLogs:          true,
				InvalidityFilteredAddr:            true,
				InvalidityPrecompileLogsLimitless: true,
				InvalidityPrecompileLogsLarge:     true,
			},
			expectedBitmask: 16383, // 0b11111111111111
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bitmask := tt.config.ToBitmask()

			// Print visual representation
			t.Log("\n" + tt.config.String())

			// Verify bitmask matches expected
			assert.Equal(t, tt.expectedBitmask, bitmask,
				"Expected bitmask %d (0b%07b), got %d (0b%07b)",
				tt.expectedBitmask, tt.expectedBitmask, bitmask, bitmask)

			// Also verify using the official ComputeIsAllowedCircuitID function
			var allowedCircuits []string
			if tt.config.ExecutionDummy {
				allowedCircuits = append(allowedCircuits, "execution-dummy")
			}
			if tt.config.DataAvailabilityDummy {
				allowedCircuits = append(allowedCircuits, "data-availability-dummy")
			}
			if tt.config.Execution {
				allowedCircuits = append(allowedCircuits, "execution")
			}
			if tt.config.ExecutionLarge {
				allowedCircuits = append(allowedCircuits, "execution-large")
			}
			if tt.config.ExecutionLimitless {
				allowedCircuits = append(allowedCircuits, "execution-limitless")
			}
			if tt.config.DataAvailabilityV2 {
				allowedCircuits = append(allowedCircuits, "data-availability-v2")
			}
			if tt.config.InvalidityNonceBalanceDummy {
				allowedCircuits = append(allowedCircuits, "invalidity-nonce-balance-dummy")
			}
			if tt.config.InvalidityPrecompileLogsDummy {
				allowedCircuits = append(allowedCircuits, "invalidity-precompile-logs-dummy")
			}
			if tt.config.InvalidityFilteredAddrDummy {
				allowedCircuits = append(allowedCircuits, "invalidity-filtered-address-dummy")
			}
			if tt.config.InvalidityNonceBalance {
				allowedCircuits = append(allowedCircuits, "invalidity-nonce-balance")
			}
			if tt.config.InvalidityPrecompileLogs {
				allowedCircuits = append(allowedCircuits, "invalidity-precompile-logs")
			}
			if tt.config.InvalidityFilteredAddr {
				allowedCircuits = append(allowedCircuits, "invalidity-filtered-address")
			}
			if tt.config.InvalidityPrecompileLogsLimitless {
				allowedCircuits = append(allowedCircuits, "invalidity-precompile-logs-limitless")
			}
			if tt.config.InvalidityPrecompileLogsLarge {
				allowedCircuits = append(allowedCircuits, "invalidity-precompile-logs-large")
			}

			computedBitmask, err := ComputeIsAllowedCircuitID(allowedCircuits)
			require.NoError(t, err)
			assert.Equal(t, bitmask, computedBitmask,
				"ToBitmask() and ComputeIsAllowedCircuitID() should produce the same result")
		})
	}
}

// TestCalculateCustomBitmask is a helper test you can modify to calculate
// a bitmask for a custom configuration. Modify the config below and run
// with `go test -v -run TestCalculateCustomBitmask` to see the result.
func TestCalculateCustomBitmask(t *testing.T) {
	// ==========================================
	// MODIFY THIS CONFIGURATION AS NEEDED
	// ==========================================
	config := CircuitConfig{
		ExecutionDummy:                    false, // ID 0: Set true to allow execution-dummy
		DataAvailabilityDummy:             false, // ID 1: Set true to allow data-availability-dummy
		Execution:                         true,  // ID 2: Set true to allow execution
		ExecutionLarge:                    true,  // ID 3: Set true to allow execution-large
		ExecutionLimitless:                true,  // ID 4: Set true to allow execution-limitless
		DataAvailabilityV2:                true,  // ID 5: Set true to allow data-availability-v2
		InvalidityNonceBalanceDummy:       false, // ID 6: Set true to allow invalidity-nonce-balance-dummy
		InvalidityPrecompileLogsDummy:     false, // ID 7: Set true to allow invalidity-precompile-logs-dummy
		InvalidityFilteredAddrDummy:       false, // ID 8: Set true to allow invalidity-filtered-address-dummy
		InvalidityNonceBalance:            true,  // ID 9: Set true to allow invalidity-nonce-balance
		InvalidityPrecompileLogs:          true,  // ID 10: Set true to allow invalidity-precompile-logs
		InvalidityFilteredAddr:            true,  // ID 11: Set true to allow invalidity-filtered-address
		InvalidityPrecompileLogsLimitless: true,  // ID 12: Set true to allow invalidity-precompile-logs-limitless
		InvalidityPrecompileLogsLarge:     true,  // ID 13: Set true to allow invalidity-precompile-logs-large
	}
	// ==========================================

	t.Log("\n" + config.String())
	t.Logf("\n>>> Use this value in your config: is_allowed_circuit_id = %d\n", config.ToBitmask())
}
