package codegen

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

const pairTestBase = uint64(0x08800000)

// syntheticVerifyInput builds a small but structurally complete VerifyInput
// (rounds with cells and a commitment, module sizes, PCS opening with a nil
// and a present leaf, public inputs) so the pair encoder walks every layout
// branch. seed differentiates the two members of a pair.
func syntheticVerifyInput(seed uint32) proofserialization.VerifyInput {
	s := proofserialization.Element(seed)
	return proofserialization.VerifyInput{
		Proof: proofserialization.Proof{
			Rounds: []proofserialization.RoundMessage{
				{
					Cells:      []proofserialization.Scalar{{Value: proofserialization.Ext{s, 2, 3, 4, 5, 6}, IsExt: true}},
					Commitment: &proofserialization.Digest{s, 1, 2, 3, 4, 5, 6, 7},
				},
				{Cells: []proofserialization.Scalar{{Value: proofserialization.Ext{s + 1}, IsExt: false}}},
			},
			ModuleSizes: []uint64{4},
			PcsOpening: proofserialization.OpeningProof{
				InputQueries: [][]proofserialization.InputTreeOpening{{
					{
						Siblings: []proofserialization.Digest{{s, 9}},
						Leaves: []*proofserialization.RowPair{
							nil,
							{
								{Base: []proofserialization.Element{s}},
								{Ext: []proofserialization.Ext{{s, 1}}},
							},
						},
					},
				}},
				FriProof: proofserialization.FriProof{
					RoundRoots: []proofserialization.Digest{{s, 8}},
					FinalPoly:  []proofserialization.Ext{{s, 1, 2, 3, 4, 5}},
					RunningQueries: [][]proofserialization.Branch{{
						{Siblings: []proofserialization.Digest{{s, 7}}, Leaf: proofserialization.Digest{s, 6}},
					}},
				},
			},
		},
		PublicInputs: []proofserialization.Scalar{{Value: proofserialization.Ext{s}, IsExt: false}},
	}
}

// The pair image must be exactly: a 16-byte header of two absolute pointers,
// then byte-for-byte the images the single-proof encoder produces at the
// pointed-to bases. Pinning against Encode's own output keeps this test from
// ever disagreeing with the ABI-agreement tests that pin Encode itself.
func TestEncodeAggregatorPair_LayoutMatchesSingleEncoder(t *testing.T) {
	a := syntheticVerifyInput(11)
	b := syntheticVerifyInput(22)

	image, err := EncodeAggregatorPair(a, b, pairTestBase)
	if err != nil {
		t.Fatalf("EncodeAggregatorPair: %v", err)
	}

	ptrA := binary.LittleEndian.Uint64(image[0:8])
	ptrB := binary.LittleEndian.Uint64(image[8:16])
	if ptrA != pairTestBase+aggregatorHeaderSize {
		t.Fatalf("header a = %#x, want %#x", ptrA, pairTestBase+aggregatorHeaderSize)
	}
	if ptrB%8 != 0 || ptrB <= ptrA {
		t.Fatalf("header b = %#x: must be 8-aligned and after a (%#x)", ptrB, ptrA)
	}

	expectedA, err := proofserialization.Encode(a, ptrA)
	if err != nil {
		t.Fatalf("Encode(a): %v", err)
	}
	expectedB, err := proofserialization.Encode(b, ptrB)
	if err != nil {
		t.Fatalf("Encode(b): %v", err)
	}

	offsetB := ptrB - pairTestBase
	if want := alignUp8(aggregatorHeaderSize + uint64(len(expectedA))); offsetB != want {
		t.Fatalf("image B at offset %d, want aligned %d", offsetB, want)
	}
	if got := image[aggregatorHeaderSize : aggregatorHeaderSize+uint64(len(expectedA))]; !bytes.Equal(got, expectedA) {
		t.Fatal("image A region differs from single-encoder output")
	}
	if got := image[offsetB:]; !bytes.Equal(got, expectedB) {
		t.Fatal("image B region differs from single-encoder output")
	}
	if uint64(len(image)) != offsetB+uint64(len(expectedB)) {
		t.Fatalf("image is %d bytes, want %d", len(image), offsetB+uint64(len(expectedB)))
	}
}

// Each member must decode back from its region of the pair image, and the two
// must be the inputs in order (a first): a swapped or clobbered member would
// make the aggregator attest to the wrong pair.
func TestEncodeAggregatorPair_MembersRoundTrip(t *testing.T) {
	a := syntheticVerifyInput(11)
	b := syntheticVerifyInput(22)

	image, err := EncodeAggregatorPair(a, b, pairTestBase)
	if err != nil {
		t.Fatalf("EncodeAggregatorPair: %v", err)
	}

	ptrA := binary.LittleEndian.Uint64(image[0:8])
	ptrB := binary.LittleEndian.Uint64(image[8:16])

	decodedA, err := proofserialization.Decode(image[ptrA-pairTestBase:ptrB-pairTestBase], ptrA)
	if err != nil {
		t.Fatalf("Decode(a): %v", err)
	}
	decodedB, err := proofserialization.Decode(image[ptrB-pairTestBase:], ptrB)
	if err != nil {
		t.Fatalf("Decode(b): %v", err)
	}

	if !reflect.DeepEqual(decodedA, a) {
		t.Fatal("decoded proof A differs from input a")
	}
	if !reflect.DeepEqual(decodedB, b) {
		t.Fatal("decoded proof B differs from input b")
	}
	if reflect.DeepEqual(decodedA, decodedB) {
		t.Fatal("fixture error: the two synthetic members must differ")
	}
}

// A base the single-proof encoder rejects must be rejected here too, not
// silently shifted.
func TestEncodeAggregatorPair_PropagatesBadBase(t *testing.T) {
	a := syntheticVerifyInput(11)
	if _, err := EncodeAggregatorPair(a, a, 4); err == nil {
		t.Fatal("expected error for misaligned base")
	}
}
