package zkcdriver_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNativeMulmodZeroModulus checks that a mulmod call with a zero modulus
// cannot be proven, and in particular that nativeMulmodPaddingFixup, which
// turns all-zero padding rows into valid ones, does not make it provable.
//
// testdata/mulmod_zero_modulus.zkc has a native mulmod that the zkc tracer
// accepts on a zero modulus, so these calls reach the prover:
//   - (0, 0, 0) is indistinguishable from a padding row in the native module,
//     and is rewritten to modulus 1, so the caller's lookup finds no row;
//   - (3, 5, 0) is not rewritten, and the NonNative query rejects its row.
//
// The honest call (3, 5, 7) is the control: the fixture itself proves.
func TestNativeMulmodZeroModulus(t *testing.T) {
	binf, err := compileBinaryConstraints("testdata/mulmod_zero_modulus.zkc")
	require.NoError(t, err)

	for _, tc := range []struct {
		name          string
		lhs, rhs, mod uint64
		provable      bool
	}{
		{"honest", 3, 5, 7, true},
		{"zero modulus, zero operands", 0, 0, 0, false},
		{"zero modulus, nonzero operands", 3, 5, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(
				`{"in_lhs": "0x%016x", "in_rhs": "0x%016x", "in_mod": "0x%016x"}`,
				tc.lhs, tc.rhs, tc.mod,
			)
			// zkc itself accepts all three traces: only the prover can reject.
			inputs, _, err := parseTestCase(zkcTestCase{InputStr: input}, binf, true)
			require.NoError(t, err)

			err = runProveVerify(inputs, binf, proverCompilePipeline)
			if tc.provable {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
