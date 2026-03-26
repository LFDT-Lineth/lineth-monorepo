package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	gnarkio "github.com/consensys/gnark/io"
	"github.com/consensys/gnark/test/unsafekzg"
	"github.com/consensys/linea-monorepo/prover/circuits"
)

// trivialCircuit is a minimal gnark circuit for generating real VKs in tests.
type trivialCircuit struct {
	X frontend.Variable `gnark:",public"`
}

func (c *trivialCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(c.X, c.X)
	return nil
}

// makeTestVK compiles a trivial circuit for the given curve, runs plonk.Setup,
// and returns the verifying key. This produces a real gnark VK suitable for
// checksum testing.
func makeTestVK(t *testing.T, curveID ecc.ID) plonk.VerifyingKey {
	t.Helper()
	ccs, err := frontend.Compile(curveID.ScalarField(), scs.NewBuilder, &trivialCircuit{})
	if err != nil {
		t.Fatalf("failed to compile trivial circuit for %s: %v", curveID, err)
	}

	srs, srsLagrange, err := unsafekzg.NewSRS(ccs)
	if err != nil {
		t.Fatalf("failed to generate SRS for %s: %v", curveID, err)
	}

	_, vk, err := plonk.Setup(ccs, srs, srsLagrange)
	if err != nil {
		t.Fatalf("plonk.Setup failed for %s: %v", curveID, err)
	}

	return vk
}

// TestListOfChecksumsMatchesObjectChecksum verifies that listOfChecksums
// (used at setup time to write allowedVkForAggregationDigests) produces
// the same hash as circuits.ObjectChecksum (used at prove time to verify
// sub-proof VK compatibility).
//
// This is a regression test for a bug where listOfChecksums used WriteTo
// (compressed point encoding) while ObjectChecksum used WriteRawTo
// (uncompressed/raw encoding), producing different hashes for the same VK.
func TestListOfChecksumsMatchesObjectChecksum(t *testing.T) {
	curves := []ecc.ID{
		ecc.BLS12_377, // used by execution/decompression payload circuits
		ecc.BW6_761,   // used by aggregation circuits
		ecc.BN254,     // used by emulation circuits
	}

	for _, curveID := range curves {
		t.Run(curveID.String(), func(t *testing.T) {
			vk := makeTestVK(t, curveID)

			// Compute hash via listOfChecksums (setup-time path)
			setupHashes := listOfChecksums([]plonk.VerifyingKey{vk})
			setupHash := setupHashes[0]

			// Compute hash via ObjectChecksum (prove-time path)
			proveHash, err := circuits.ObjectChecksum(vk)
			if err != nil {
				t.Fatalf("ObjectChecksum failed: %v", err)
			}

			if setupHash != proveHash {
				t.Errorf("VK checksum mismatch for curve %s:\n"+
					"  listOfChecksums (setup-time): %s\n"+
					"  ObjectChecksum  (prove-time): %s\n"+
					"This means setup would write one hash to allowedVkForAggregationDigests,\n"+
					"but prove-time would compute a different hash from the same VK,\n"+
					"causing aggregation to reject all sub-proofs.",
					curveID, setupHash, proveHash)
			}
		})
	}
}

// TestListOfChecksumsUsesWriteRawTo verifies that listOfChecksums uses
// WriteRawTo (not WriteTo) by comparing its output against a direct
// WriteRawTo hash and a direct WriteTo hash.
func TestListOfChecksumsUsesWriteRawTo(t *testing.T) {
	vk := makeTestVK(t, ecc.BLS12_377)

	// Hash via listOfChecksums
	setupHash := listOfChecksums([]plonk.VerifyingKey{vk})[0]

	// Hash via direct WriteRawTo
	h := sha256.New()
	raw, ok := vk.(gnarkio.WriterRawTo)
	if !ok {
		t.Fatal("VK does not implement gnarkio.WriterRawTo")
	}
	if _, err := raw.WriteRawTo(h); err != nil {
		t.Fatalf("WriteRawTo failed: %v", err)
	}
	rawHash := "0x" + hex.EncodeToString(h.Sum(nil))

	// Hash via direct WriteTo (compressed — the old buggy behavior)
	h2 := sha256.New()
	if _, err := vk.WriteTo(h2); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	compressedHash := "0x" + hex.EncodeToString(h2.Sum(nil))

	// listOfChecksums must match WriteRawTo
	if setupHash != rawHash {
		t.Errorf("listOfChecksums does not match WriteRawTo:\n"+
			"  listOfChecksums: %s\n"+
			"  WriteRawTo:      %s", setupHash, rawHash)
	}

	// Verify listOfChecksums does NOT match the old buggy WriteTo path
	// (for BLS12-377, WriteTo and WriteRawTo produce different output)
	if rawHash != compressedHash && setupHash == compressedHash {
		t.Errorf("listOfChecksums matches WriteTo (compressed) instead of WriteRawTo (raw):\n"+
			"  listOfChecksums: %s\n"+
			"  WriteTo:         %s\n"+
			"  WriteRawTo:      %s\n"+
			"This is the original bug — setup-time and prove-time hashes will differ!",
			setupHash, compressedHash, rawHash)
	}
}

// TestListOfChecksumsPanicsWithoutWriteRawTo verifies that listOfChecksums
// panics if the asset does not implement WriteRawTo (rather than silently
// falling back to WriteTo which would produce wrong hashes).
func TestListOfChecksumsPanicsWithoutWriteRawTo(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("listOfChecksums did not panic for an asset without WriteRawTo")
		}
	}()

	// fakeWriterTo implements io.WriterTo but NOT gnarkio.WriterRawTo
	listOfChecksums([]fakeWriterTo{{data: []byte("test")}})
}

// fakeWriterTo is a test double that implements io.WriterTo but NOT gnarkio.WriterRawTo.
type fakeWriterTo struct {
	data []byte
}

func (f fakeWriterTo) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(f.data)
	return int64(n), err
}
