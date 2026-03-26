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
			},
			expectedBitmask: 60, // 0b111100 = 2^2 + 2^3 + 2^4 + 2^5
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
			},
			expectedBitmask: 63, // 0b111111 = 2^0 + 2^1 + 2^2 + 2^3 + 2^4 + 2^5
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
			},
			expectedBitmask: 47, // 0b101111 = 2^0 + 2^1 + 2^2 + 2^3 + 2^5
			expectError:     false,
		},
		{
			name: "integration-full configuration",
			allowedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
				"execution",
			},
			expectedBitmask: 7, // 0b000111 = 2^0 + 2^1 + 2^2
			expectError:     false,
		},
		{
			name: "integration-development configuration",
			allowedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
				"data-availability-v2",
			},
			expectedBitmask: 35, // 0b100011 = 2^0 + 2^1 + 2^5
			expectError:     false,
		},
		{
			name: "single circuit",
			allowedCircuits: []string{
				"execution",
			},
			expectedBitmask: 4, // 0b000100 = 2^2
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
			name: "all payload circuits (0-5)",
			allowedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
				"execution",
				"execution-large",
				"execution-limitless",
				"data-availability-v2",
			},
			expectedBitmask: 63, // 0b111111 = all bits 0-5 set
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
			name:      "zero bitmask disallows everything",
			bitmask:   0,
			circuitID: 2,
			expected:  false,
		},
		{
			name:      "all bits set allows everything",
			bitmask:   63, // 0b111111
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
			bitmask: 60,
			expectedCircuits: []string{
				"execution",
				"execution-large",
				"execution-limitless",
				"data-availability-v2",
			},
		},
		{
			name:    "sepolia configuration",
			bitmask: 63,
			expectedCircuits: []string{
				"execution-dummy",
				"data-availability-dummy",
				"execution",
				"execution-large",
				"execution-limitless",
				"data-availability-v2",
			},
		},
		{
			name:             "zero bitmask",
			bitmask:          0,
			expectedCircuits: []string{},
		},
		{
			name:    "only dummy circuits",
			bitmask: 3, // 0b000011 = bits 0,1
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
	// Verify the mapping contains expected entries for payload circuits
	assert.Equal(t, uint(0), GlobalCircuitIDMapping["execution-dummy"])
	assert.Equal(t, uint(1), GlobalCircuitIDMapping["data-availability-dummy"])
	assert.Equal(t, uint(2), GlobalCircuitIDMapping["execution"])
	assert.Equal(t, uint(3), GlobalCircuitIDMapping["execution-large"])
	assert.Equal(t, uint(4), GlobalCircuitIDMapping["execution-limitless"])
	assert.Equal(t, uint(5), GlobalCircuitIDMapping["data-availability-v2"])

	// Verify infrastructure circuits
	assert.Equal(t, uint(8), GlobalCircuitIDMapping["emulation"])
	assert.Equal(t, uint(9), GlobalCircuitIDMapping["aggregation"])
	assert.Equal(t, uint(10), GlobalCircuitIDMapping["public-input-interconnection"])
	assert.Equal(t, uint(11), GlobalCircuitIDMapping["emulation-dummy"])

	// Verify no duplicate IDs
	seen := make(map[uint]string)
	for name, id := range GlobalCircuitIDMapping {
		if existingName, exists := seen[id]; exists {
			t.Errorf("Duplicate circuit ID %d for circuits '%s' and '%s'", id, existingName, name)
		}
		seen[id] = name
	}

	// Verify we have exactly 10 circuits (IDs: 0,1,2,3,4,5,8,9,10,11)
	assert.Equal(t, 10, len(GlobalCircuitIDMapping))
}

func TestCircuitNameByID(t *testing.T) {
	tests := []struct {
		id       uint
		expected string
	}{
		{0, "execution-dummy"},
		{1, "data-availability-dummy"},
		{2, "execution"},
		{3, "execution-large"},
		{4, "execution-limitless"},
		{5, "data-availability-v2"},
		{8, "emulation"},
		{9, "aggregation"},
		{10, "public-input-interconnection"},
		{11, "emulation-dummy"},
		{99, "circuit-id-99"}, // unknown ID
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, CircuitNameByID(tt.id))
		})
	}
}

// Example test showing how to use these functions for config validation
func TestExampleUsage(t *testing.T) {
	// Example: User wants to configure mainnet
	mainnetAllowedInputs := []string{
		"execution",
		"execution-large",
		"execution-limitless",
		"data-availability-v2",
	}

	// Compute the bitmask
	bitmask, err := ComputeIsAllowedCircuitID(mainnetAllowedInputs)
	require.NoError(t, err)
	assert.Equal(t, uint64(60), bitmask)

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
// Each field corresponds to a circuit ID (0-5 for payload circuits).
// Set to true to allow the circuit, false to disallow.
type CircuitConfig struct {
	ExecutionDummy        bool // ID 0
	DataAvailabilityDummy bool // ID 1
	Execution             bool // ID 2
	ExecutionLarge        bool // ID 3
	ExecutionLimitless    bool // ID 4
	DataAvailabilityV2    bool // ID 5
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
	return bitmask
}

// String returns a visual representation of the circuit configuration.
func (c CircuitConfig) String() string {
	return fmt.Sprintf(`Circuit Configuration:
  ID 0 - execution-dummy:         %v
  ID 1 - data-availability-dummy: %v
  ID 2 - execution:               %v
  ID 3 - execution-large:         %v
  ID 4 - execution-limitless:     %v
  ID 5 - data-availability-v2:    %v
  Bitmask: %d (binary: 0b%06b)`,
		c.ExecutionDummy,
		c.DataAvailabilityDummy,
		c.Execution,
		c.ExecutionLarge,
		c.ExecutionLimitless,
		c.DataAvailabilityV2,
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
				ExecutionDummy:        false,
				DataAvailabilityDummy: false,
				Execution:             true,
				ExecutionLarge:        true,
				ExecutionLimitless:    true,
				DataAvailabilityV2:    true,
			},
			expectedBitmask: 60, // 0b111100
		},
		{
			name: "Sepolia/Testnet (includes dummy circuits)",
			config: CircuitConfig{
				ExecutionDummy:        true,
				DataAvailabilityDummy: true,
				Execution:             true,
				ExecutionLarge:        true,
				ExecutionLimitless:    true,
				DataAvailabilityV2:    true,
			},
			expectedBitmask: 63, // 0b111111
		},
		{
			name: "Devnet (limitless only, no execution/execution-large)",
			config: CircuitConfig{
				ExecutionDummy:        true,
				DataAvailabilityDummy: true,
				Execution:             false,
				ExecutionLarge:        false,
				ExecutionLimitless:    true,
				DataAvailabilityV2:    true,
			},
			expectedBitmask: 51, // 0b110011
		},
		{
			name: "Integration-full (minimal)",
			config: CircuitConfig{
				ExecutionDummy:        true,
				DataAvailabilityDummy: true,
				Execution:             true,
				ExecutionLarge:        false,
				ExecutionLimitless:    false,
				DataAvailabilityV2:    false,
			},
			expectedBitmask: 7, // 0b000111
		},
		{
			name: "Integration-development (dummy + data-availability)",
			config: CircuitConfig{
				ExecutionDummy:        true,
				DataAvailabilityDummy: true,
				Execution:             false,
				ExecutionLarge:        false,
				ExecutionLimitless:    false,
				DataAvailabilityV2:    true,
			},
			expectedBitmask: 35, // 0b100011
		},
		{
			name: "All payload circuits enabled",
			config: CircuitConfig{
				ExecutionDummy:        true,
				DataAvailabilityDummy: true,
				Execution:             true,
				ExecutionLarge:        true,
				ExecutionLimitless:    true,
				DataAvailabilityV2:    true,
			},
			expectedBitmask: 63, // 0b111111
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bitmask := tt.config.ToBitmask()

			// Print visual representation
			t.Log("\n" + tt.config.String())

			// Verify bitmask matches expected
			assert.Equal(t, tt.expectedBitmask, bitmask,
				"Expected bitmask %d (0b%06b), got %d (0b%06b)",
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

			computedBitmask, err := ComputeIsAllowedCircuitID(allowedCircuits)
			require.NoError(t, err)
			assert.Equal(t, bitmask, computedBitmask,
				"ToBitmask() and ComputeIsAllowedCircuitID() should produce the same result")
		})
	}
}

// TestCalculateCustomBitmask is a helper test you can modify to calculate
// a bitmask for a custom configuration. Modify the config below and run
// with `go test -v -run TestCalculateCustomBitmask ./circuits/` to see the result.
func TestCalculateCustomBitmask(t *testing.T) {
	// ==========================================
	// MODIFY THIS CONFIGURATION AS NEEDED
	// ==========================================
	config := CircuitConfig{
		ExecutionDummy:        false, // ID 0: Set true to allow execution-dummy
		DataAvailabilityDummy: false, // ID 1: Set true to allow data-availability-dummy
		Execution:             true,  // ID 2: Set true to allow execution
		ExecutionLarge:        true,  // ID 3: Set true to allow execution-large
		ExecutionLimitless:    true,  // ID 4: Set true to allow execution-limitless
		DataAvailabilityV2:    true,  // ID 5: Set true to allow data-availability-v2
	}
	// ==========================================

	t.Log("\n" + config.String())
	t.Logf("\n>>> Use this value in your config: is_allowed_circuit_id = %d\n", config.ToBitmask())
}
