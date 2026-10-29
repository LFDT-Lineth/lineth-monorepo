package dummy_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/dummy"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/stretchr/testify/require"
)

func base(v uint64) field.Gen {
	var e field.Element
	e.SetUint64(v)
	return field.ElemFromBase(e)
}

// samplePub returns deterministic public inputs: 1000 + i.
func samplePub() wiop.PublicInput {
	pub := make(wiop.PublicInput, dummy.NumPublicInputs)
	for i := range pub {
		pub[i] = base(uint64(1000 + i))
	}
	return pub
}

func TestWitness(t *testing.T) {
	require.Equal(t, base(3125), dummy.Witness(base(5)), "the witness must be x^5")

	var e field.Ext
	for i, l := range []*field.Element{&e.B0.A0, &e.B0.A1, &e.B1.A0, &e.B1.A1, &e.B2.A0, &e.B2.A1} {
		l.SetUint64(uint64(i + 1))
	}
	got := dummy.Witness(field.ElemFromExt(e)).AsExt()
	for i, l := range []field.Element{got.B0.A0, got.B0.A1, got.B1.A0, got.B1.A1, got.B2.A0, got.B2.A1} {
		require.Equal(t, uint64((i+1)*(i+1)*(i+1)*(i+1)*(i+1)), l.Uint64(), "an extension witness is x^5 limb by limb")
	}
}

func TestProveVerify(t *testing.T) {
	sys := dummy.NewSystem()
	proof, pub := dummy.Prove(sys, samplePub())
	require.Equal(t, samplePub(), pub, "the statement is the public inputs given to Prove")
	require.NoError(t, dummy.Verify(sys, proof, pub), "an honest proof must verify")
}

func TestVerify_ChangedPublicInput(t *testing.T) {
	sys := dummy.NewSystem()
	proof, pub := dummy.Prove(sys, samplePub())

	changed := append(wiop.PublicInput(nil), pub...)
	changed[3] = changed[3].Add(field.ElemOne())
	require.Error(t, dummy.Verify(sys, proof, changed), "a changed public input must be rejected")
	require.Error(t, dummy.Verify(sys, proof, pub[:1]), "a missing public input must be rejected")
	require.Panics(t, func() { dummy.Prove(sys, pub[:1]) }, "Prove must refuse a statement of the wrong length")
}

func TestEncode(t *testing.T) {
	sys := dummy.NewSystem()
	proof, pub := dummy.Prove(sys, samplePub())

	img, err := dummy.Encode(sys, proof, pub, dummy.GuestBase)
	require.NoError(t, err)

	decoded, err := ps.Decode(img, dummy.GuestBase)
	require.NoError(t, err)
	require.Len(t, decoded.Proof.Rounds, 1, "a dummy proof has one round message")
	require.Nil(t, decoded.Proof.Rounds[0].Commitment, "a dummy proof commits to nothing")
	require.Empty(t, decoded.Proof.ModuleSizes)
	require.Len(t, decoded.Proof.Rounds[0].Cells, dummy.NumPublicInputs)
	require.Len(t, decoded.PublicInputs, dummy.NumPublicInputs)
	for i := range pub {
		require.Equal(t, ps.ScalarFrom(dummy.Witness(pub[i])), decoded.Proof.Rounds[0].Cells[i], "round 0's cell %d is Y_%d", i, i)
		require.Equal(t, ps.ScalarFrom(pub[i]), decoded.PublicInputs[i], "public input %d is X_%d", i, i)
	}
}

func TestEncodePair(t *testing.T) {
	sys := dummy.NewSystem()
	proof, pub := dummy.Prove(sys, samplePub())

	img, err := dummy.EncodePair(sys, proof, pub, proof, pub, dummy.GuestBase)
	require.NoError(t, err)

	a := binary.LittleEndian.Uint64(img[0:]) - dummy.GuestBase
	b := binary.LittleEndian.Uint64(img[8:]) - dummy.GuestBase
	require.Equal(t, uint64(dummy.SizePairHeader), a, "proof A starts right after the header")
	require.Zero(t, b%8, "proof B is 8-byte aligned")

	single, err := dummy.Encode(sys, proof, pub, dummy.GuestBase+a)
	require.NoError(t, err)
	require.Equal(t, single, img[a:a+uint64(len(single))], "each member is encoded at its own absolute start")
}

// zigTestdata is where the Zig verifier reads its fixtures from.
const zigTestdata = "zig/testdata"

// TestZigFixturesAreUpToDate keeps the Zig verifier's committed proof images
// in sync with this package. They are committed so the Zig suite needs no Go
// toolchain. Regenerate with UPDATE_DUMMY_FIXTURES=1, then run `zig build test`
// in zig/.
//
//   - dummy_image.bin: an honest proof
//   - dummy_image_tampered.bin: an honest proof with a public input changed afterwards
//   - dummy_pair_image.bin: two honest proofs of the same public inputs
//   - dummy_pair_image_mismatch.bin: two honest proofs of different public inputs
func TestZigFixturesAreUpToDate(t *testing.T) {
	sys := dummy.NewSystem()
	proof, pub := dummy.Prove(sys, samplePub())

	tampered := append(wiop.PublicInput(nil), pub...)
	tampered[3] = tampered[3].Add(field.ElemOne())

	other := append(wiop.PublicInput(nil), pub...)
	other[0] = other[0].Add(field.ElemOne())
	otherProof, other := dummy.Prove(sys, other)

	encode := func(proof wiop.Proof, pub wiop.PublicInput) []byte {
		img, err := dummy.Encode(sys, proof, pub, dummy.GuestBase)
		require.NoError(t, err)
		return img
	}
	encodePair := func(proofB wiop.Proof, pubB wiop.PublicInput) []byte {
		img, err := dummy.EncodePair(sys, proof, pub, proofB, pubB, dummy.GuestBase)
		require.NoError(t, err)
		return img
	}

	fixtures := map[string][]byte{
		"dummy_image.bin":               encode(proof, pub),
		"dummy_image_tampered.bin":      encode(proof, tampered),
		"dummy_pair_image.bin":          encodePair(proof, pub),
		"dummy_pair_image_mismatch.bin": encodePair(otherProof, other),
	}

	for name, content := range fixtures {
		path := filepath.Join(zigTestdata, name)
		if os.Getenv("UPDATE_DUMMY_FIXTURES") != "" {
			require.NoError(t, os.WriteFile(path, content, 0o644))
			continue
		}
		committed, err := os.ReadFile(path)
		require.NoError(t, err, "reading %s", path)
		require.Equal(t, committed, content,
			"%s is stale: regenerate with UPDATE_DUMMY_FIXTURES=1 go test ./dummy/ "+
				"and re-run `zig build test` in prover-ray/dummy/zig", path)
	}
}
