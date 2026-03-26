package blobsubmission

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"

	"golang.org/x/crypto/sha3"

	bls12Fr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

func TestEvaluationChallenge(t *testing.T) {
	tests := []struct {
		desc       string
		snarkHash  []byte
		keccakHash []byte
		want       []byte
	}{
		{
			desc:       "SameLength",
			snarkHash:  []byte("5fe845dc61556e7cc1d07bbab18119e0d31d0ea1b63950f255d19b62f5f8f442"),
			keccakHash: []byte("9afc1620c5ce28b91558c3c6bf51c1cb782fd0dc95187baf29e9dc5588fbb71b"),
			want: wantTestConcatHash(
				t,
				[]byte("5fe845dc61556e7cc1d07bbab18119e0d31d0ea1b63950f255d19b62f5f8f442"),
				[]byte("9afc1620c5ce28b91558c3c6bf51c1cb782fd0dc95187baf29e9dc5588fbb71b"),
			),
		},
		{
			desc:       "EmptyHashes",
			snarkHash:  []byte{},
			keccakHash: []byte{},
			want:       wantTestConcatHash(t, []byte{}, []byte{}),
		},
		{
			desc:       "LongHashes",
			snarkHash:  []byte("3a9aebdbae1f1a639651fc8ced048e87"),
			keccakHash: []byte("7cf8547387e4274fb9177c829b83932f"),
			want: wantTestConcatHash(
				t,
				[]byte("3a9aebdbae1f1a639651fc8ced048e87"),
				[]byte("7cf8547387e4274fb9177c829b83932f"),
			),
		},
		{
			desc:       "Different lengths",
			snarkHash:  []byte("92a1b99e5f6e615a5b23"),
			keccakHash: []byte("120105bc5d14b8e8be5dcb1d1a7f54"),
			want: wantTestConcatHash(
				t,
				[]byte("92a1b99e5f6e615a5b23"),
				[]byte("120105bc5d14b8e8be5dcb1d1a7f54"),
			),
		},
	}

	for i, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := evaluationChallenge(tt.snarkHash, tt.keccakHash)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("test case %d:\nwant: %x\ngot:%x", i, tt.want, got)
			}
		})
	}
}

func hexToBytes(t *testing.T, s string) []byte {
	b, err := hex.DecodeString(s)
	assert.NoError(t, err)
	return b
}

func hexToElem(t *testing.T, s string) bls12Fr.Element {
	var res bls12Fr.Element
	assert.NoError(t, res.SetBytesCanonical(hexToBytes(t, s)))
	return res
}

func TestNewSchnarf(t *testing.T) {
	tests := []struct {
		desc        string
		parts       Shnarf
		wantSchnarf []byte
	}{
		{
			desc: "SameLength",
			parts: Shnarf{
				OldShnarf:        hexToBytes(t, "9cc2e1b9eb24657287f4218215b0e06f74a101b2b9d5a31843cd4389f372244c"),
				SnarkHash:        hexToBytes(t, "9b1d412c846ef9a34f0852a84426232abf2d077fd89a51b975fec6ddc7574338"),
				NewStateRootHash: hexToBytes(t, "5fb64ae631f8267af639bd760396f28a35e303af3625f1fc5f160c2f08c8ac9b"),
				X:                hexToBytes(t, "29299c0aa183abcc170250464ce7495d78a0649e9eaa9426aa7fcd9f4dbd000b"),
				Y:                bls12Fr.Element{17, 18, 19, 20},
			},
		},
		{
			desc: "EmptyInputs",
			wantSchnarf: func() []byte {
				t.Helper()
				h := sha3.NewLegacyKeccak256()
				return h.Sum(nil)
			}(),
		},
		{
			desc: "VaryingLengthInputs",
			parts: Shnarf{
				OldShnarf:        hexToBytes(t, "d4d775835bab9bc0b03aa23778624da62169072de2005b34b2988d8a64fc857b"),
				SnarkHash:        hexToBytes(t, "e44d5326f398e2efc575739a159807546d7852943d3f63609ad61f56f0edb222"),
				NewStateRootHash: hexToBytes(t, "4e05eef8509d096d9d25d2488f729b9bffb6b04b74d2e67542cc81423712d1f6"),
				X:                hexToBytes(t, "20d4ecfd922dd4cd55da1492579bd2aaa77590541d16da418f6b22abd13d23eb"),
				Y:                bls12Fr.Element{12, 13, 14, 15},
			},
		},
	}

	for i, tt := range tests {
		tt.wantSchnarf = wantTestSchnarf(t, tt.parts)

		t.Run(tt.desc, func(t *testing.T) {
			got := tt.parts.Compute()
			if !bytes.Equal(got, tt.wantSchnarf) {
				t.Errorf("test case %d:\nwant: %x\ngot: %x", i, tt.wantSchnarf, got)
			}
		})
	}
}

// wantTestConcatHash is a helper function to compute the expected Keccak256 hash for a
// test case.
func wantTestConcatHash(t *testing.T, b1, b2 []byte) []byte {
	t.Helper()

	h := sha3.NewLegacyKeccak256()
	h.Write(b1)
	h.Write(b2)
	return h.Sum(nil)
}

// wantTestSchnarf is a helper function to compute the expected Shnarf hash for
// a test case.
func wantTestSchnarf(t *testing.T, parts Shnarf) []byte {
	t.Helper()

	var (
		h              = sha3.NewLegacyKeccak256()
		xBytes, yBytes = parts.X, parts.Y.Bytes()
	)
	h.Write(parts.OldShnarf)
	h.Write(parts.SnarkHash)
	h.Write(parts.NewStateRootHash)
	h.Write(xBytes[:])
	h.Write(yBytes[:])

	return h.Sum(nil)
}
