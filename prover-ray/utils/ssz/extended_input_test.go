package ssz

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Golden vectors generated from the canonical reference
// rollup_spec/l2_execution_ssz.py::encode_l2_execution_input (schema 0x0002).
const (
	goldenEmpty    = "000211111111111111111111111111111111111111111111111111111111111111110700000000000000aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb08e70000000000005c000000"
	goldenOneNoFtx = "000211111111111111111111111111111111111111111111111111111111111111110700000000000000aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb08e70000000000005c00000004000000080000000c0000000001dead"
	goldenOneFtx   = "000211111111111111111111111111111111111111111111111111111111111111110700000000000000aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb08e70000000000005c00000004000000080000000c0000000001dead040000001000000000000000150000000097440f000000000002f86b"
	goldenTwo      = "000247474747474747474747474747474747474747474747474747474747474747470f00000000000000aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb08e70000000000005c0000000800000013000000080000000b0000000001aa080000000c0000000001bbcc08000000200000001100000000000000150000000098440f000000000002f86b1200000000000000150000000499440f0000000000"
)

func fill(n int, b byte) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func arr32(b byte) (out [32]byte) { copy(out[:], fill(32, b)); return }
func arr20(b byte) (out [20]byte) { copy(out[:], fill(20, b)); return }

func chainCfg() ChainConfig {
	return ChainConfig{L2MessageServiceAddress: arr20(0xaa), Coinbase: arr20(0xbb), ChainID: 59144}
}

func TestEncodeExtendedInput_Golden(t *testing.T) {
	cases := []struct {
		name  string
		in    ExtendedInput
		wantH string
	}{
		{
			name: "EmptyPayloads",
			in: ExtendedInput{
				ParentFtxRollingHash:         arr32(0x11),
				ParentLastProcessedFtxNumber: 7,
				ChainConfig:                  chainCfg(),
				Payloads:                     []PayloadInput{},
			},
			wantH: goldenEmpty,
		},
		{
			name: "OnePayloadNoForcedTx",
			in: ExtendedInput{
				ParentFtxRollingHash:         arr32(0x11),
				ParentLastProcessedFtxNumber: 7,
				ChainConfig:                  chainCfg(),
				Payloads: []PayloadInput{
					{StatelessInputSSZ: []byte{0x00, 0x01, 0xde, 0xad}},
				},
			},
			wantH: goldenOneNoFtx,
		},
		{
			name: "OnePayloadOneForcedTx",
			in: ExtendedInput{
				ParentFtxRollingHash:         arr32(0x11),
				ParentLastProcessedFtxNumber: 7,
				ChainConfig:                  chainCfg(),
				Payloads: []PayloadInput{
					{
						StatelessInputSSZ: []byte{0x00, 0x01, 0xde, 0xad},
						ForcedTransactions: []ForcedTransaction{
							{Number: 16, SignedTxRlp: []byte{0x02, 0xf8, 0x6b}, Acceptance: 0, Deadline: 1000599},
						},
					},
				},
			},
			wantH: goldenOneFtx,
		},
		{
			name: "TwoPayloads",
			in: ExtendedInput{
				ParentFtxRollingHash:         arr32(0x47),
				ParentLastProcessedFtxNumber: 15,
				ChainConfig:                  chainCfg(),
				Payloads: []PayloadInput{
					{StatelessInputSSZ: []byte{0x00, 0x01, 0xaa}},
					{
						StatelessInputSSZ: []byte{0x00, 0x01, 0xbb, 0xcc},
						ForcedTransactions: []ForcedTransaction{
							{Number: 17, SignedTxRlp: []byte{0x02, 0xf8, 0x6b}, Acceptance: 0, Deadline: 1000600},
							{Number: 18, SignedTxRlp: nil, Acceptance: 4, Deadline: 1000601},
						},
					},
				},
			},
			wantH: goldenTwo,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EncodeExtendedInput(tc.in)
			want, err := hex.DecodeString(tc.wantH)
			require.NoError(t, err)
			assert.Equal(t, want, got, "encoded bytes must match the Python reference golden")
		})
	}
}

// TestEncodeExtendedInput_SchemaFrame checks the two-byte big-endian 0x0002 frame.
func TestEncodeExtendedInput_SchemaFrame(t *testing.T) {
	got := EncodeExtendedInput(ExtendedInput{ChainConfig: chainCfg()})
	require.GreaterOrEqual(t, len(got), 2)
	assert.Equal(t, []byte{0x00, 0x02}, got[:2])
}
