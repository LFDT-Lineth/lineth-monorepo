package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

func TestSerializeProofEmpty(t *testing.T) {
	got, err := serializeProof(vanishingProofView{})
	if err != nil {
		t.Fatal(err)
	}
	if want := make([]byte, 16); !bytes.Equal(got, want) {
		t.Fatalf("empty proof bytes mismatch:\n got %x\nwant %x", got, want)
	}
}

func TestSerializeProofLayout(t *testing.T) {
	proof := vanishingProofView{
		rounds: []runtimeTraceRound{{
			columns: []runtimeTraceColumn{
				{commitments: []field.Octuplet{octuplet(1, 2, 3, 4, 5, 6, 7, 8)}},
				{publicBaseValues: elems(9, 10)},
				{publicExtValues: []field.Ext{field.UintsToExt(11, 12, 13, 14, 15, 16)}},
			},
			cells: []runtimeTraceCell{
				baseTraceCell(elem(17)),
				extTraceCell(field.UintsToExt(18, 19, 20, 21, 22, 23)),
			},
		}},
		witnessClaims:  []field.Ext{field.UintsToExt(24, 25, 26, 27, 28, 29)},
		quotientClaims: []field.Ext{field.UintsToExt(30, 31, 32, 33, 34, 35)},
		moduleSizes:    []int{4},
	}

	got, err := serializeProof(proof)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 181 {
		t.Fatalf("serialized proof length = %d, want 181", len(got))
	}
	layout := proofLayout(proof)
	if layout.roundCount != 1 || layout.columnCount != 3 || layout.cellCount != 2 ||
		layout.publicBaseValueCount != 2 || layout.publicExtValueCount != 1 ||
		layout.witnessClaimCount != 1 || layout.quotientClaimCount != 1 ||
		layout.moduleSizeCount != 1 || layout.encodedSize != len(got) {
		t.Fatalf("proof layout = %+v, serialized length = %d", layout, len(got))
	}

	// Offsets below are cumulative byte positions in the v0 wire format:
	//
	//   0  round_count: u32
	//   4  first round column_count: u32
	//   8  oracle commitment tag: u8
	//   9  oracle commitment: 8 limbs * u32 = 32 bytes, ending at offset 41
	//   41 public base column tag: u8
	//   42 public base length: u32
	//   46 public base values: 2 limbs * u32 = 8 bytes, ending at offset 54
	//   54 public extension column tag: u8
	//   55 public extension length: u32
	//   59 public extension values: 1 ext * 6 limbs * u32 = 24 bytes, ending at offset 83
	//   83 cell_count: u32
	//   87 base scalar tag: u8
	//   88 base scalar: u32, ending at offset 92
	//   92 extension scalar tag: u8
	//   93 extension scalar: 1 ext * 6 limbs * u32 = 24 bytes, ending at offset 117
	//   117 witness_claim_count: u32
	//   121 witness claims: 1 ext * 6 limbs * u32 = 24 bytes, ending at offset 145
	//   145 quotient_claim_count: u32
	//   149 quotient claims: 1 ext * 6 limbs * u32 = 24 bytes, ending at offset 173
	//   173 module_size_count: u32
	//   177 module sizes: 1 * u32 = 4 bytes, ending at offset 181
	expectU32(t, got, 0, 1)   // round_count
	expectU32(t, got, 4, 3)   // column_count
	expectByte(t, got, 8, 0)  // oracle commitment tag
	expectU32(t, got, 9, 1)   // first commitment limb
	expectU32(t, got, 37, 8)  // last commitment limb
	expectByte(t, got, 41, 1) // public base column
	expectU32(t, got, 42, 2)  // public base length
	expectU32(t, got, 46, 9)  // first public base value
	expectByte(t, got, 54, 2) // public extension column
	expectU32(t, got, 55, 1)  // public extension length
	expectU32(t, got, 59, 11) // first public extension limb
	expectU32(t, got, 83, 2)  // cell_count
	expectByte(t, got, 87, 0) // base scalar
	expectU32(t, got, 88, 17) // base scalar value
	expectByte(t, got, 92, 1) // extension scalar
	expectU32(t, got, 93, 18) // first extension scalar limb
	expectU32(t, got, 117, 1) // witness_claim_count
	expectU32(t, got, 121, 24)
	expectU32(t, got, 145, 1) // quotient_claim_count
	expectU32(t, got, 149, 30)
	expectU32(t, got, 173, 1) // module_size_count
	expectU32(t, got, 177, 4)
}

func expectByte(t *testing.T, data []byte, offset int, want byte) {
	t.Helper()
	if got := data[offset]; got != want {
		t.Fatalf("byte at offset %d = %d, want %d", offset, got, want)
	}
}

func expectU32(t *testing.T, data []byte, offset int, want uint32) {
	t.Helper()
	if got := binary.LittleEndian.Uint32(data[offset:]); got != want {
		t.Fatalf("u32 at offset %d = %d, want %d", offset, got, want)
	}
}
