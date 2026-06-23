package fri

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// tableOfSize builds a SizedTable holding one base row and one ext row of the
// given size (a power of two), drawing deterministic values from ctr. A negative
// size yields an empty table.
func tableOfSize(size int, ctr *uint64) SizedTable {
	if size < 0 {
		return SizedTable{}
	}
	base := make([]field.Element, size)
	ext := make([]field.Ext, size)
	for i := range base {
		base[i] = field.NewElement(*ctr)
		*ctr++
	}
	for i := range ext {
		ext[i] = field.IntsToExt(int64(*ctr), 0, 0, 0, 0, 0)
		*ctr++
	}
	return SizedTable{Base: [][]field.Element{base}, Ext: [][]field.Ext{ext}}
}

// makeEncoders builds n encoders with plaintext sizes 2^i and a shared inverse
// rate (which must be >= 2 so plaintext size < codeword size).
func makeEncoders(n, invRate int) []*RSEncoder {
	encoders := make([]*RSEncoder, n)
	for i := 0; i < n; i++ {
		enc := NewEncoder(uint64(invRate)*(1<<i), 1<<i)
		encoders[i] = &enc
	}
	return encoders
}

func requirePanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: expected a panic, got none", name)
		}
	}()
	fn()
}

// TestMultiSizeTableWellFormedness covers SizedTable.Size and
// MultiSizeTable.checkWellFormedness across valid and malformed shapes.
func TestMultiSizeTableWellFormedness(t *testing.T) {

	cases := []struct {
		name    string
		sizes   []int // -1 means an empty sub-table
		wantK   int
		wantErr bool
	}{
		{"k=1 ascending", []int{1, 2, 4}, 1, false},
		{"k=2", []int{2, 4}, 2, false},
		{"empty middle", []int{1, -1, 4}, 1, false},
		{"last entry empty", []int{1, -1}, 0, true},
		{"inconsistent size", []int{1, 4}, 0, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var ctr uint64
			table := make(MultiSizeTable, len(c.sizes))
			for i, s := range c.sizes {
				table[i] = tableOfSize(s, &ctr)
			}

			k, err := table.checkWellFormedness()
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got k=%d nil", k)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if k != c.wantK {
				t.Fatalf("k = %d, want %d", k, c.wantK)
			}
		})
	}
}

// TestAssertValidMultiEncoder checks the encoder-list validation: matching rate
// and plaintext sizes 2^i pass, a rate mismatch or a wrong plaintext size panic.
func TestAssertValidMultiEncoder(t *testing.T) {

	assertValidMultiEncoder(makeEncoders(3, 2)) // must not panic

	requirePanic(t, "rate mismatch", func() {
		encoders := makeEncoders(3, 2)
		bad := NewEncoder(4*2, 2) // plaintext 2 but inverse rate 4
		encoders[1] = &bad
		assertValidMultiEncoder(encoders)
	})

	requirePanic(t, "wrong plaintext size", func() {
		encoders := makeEncoders(3, 2)
		bad := NewEncoder(2*3, 3) // plaintext 3 != 2^1
		encoders[1] = &bad
		assertValidMultiEncoder(encoders)
	})
}

// TestMultiSizeTableEncode checks that Encode delegates each row to its encoder
// and blows the size up by the inverse rate.
func TestMultiSizeTableEncode(t *testing.T) {

	const (
		n       = 3
		invRate = 2
	)

	var ctr uint64
	witness := MultiSizeTable{
		tableOfSize(1, &ctr),
		tableOfSize(2, &ctr),
		tableOfSize(4, &ctr),
	}
	encoders := makeEncoders(n, invRate)

	encoded := witness.Encode(encoders)

	for i := range witness {
		if got, want := encoded[i].Size(), invRate*(1<<i); got != want {
			t.Fatalf("level %d: encoded size %d, want %d", i, got, want)
		}
		for k, row := range witness[i].Base {
			ref := encoders[i].Encode(row)
			for j := range ref {
				if !encoded[i].Base[k][j].Equal(&ref[j]) {
					t.Fatalf("level %d base row %d pos %d: encode mismatch", i, k, j)
				}
			}
		}
		for k, row := range witness[i].Ext {
			ref := encoders[i].EncodeExt(row)
			for j := range ref {
				if !encoded[i].Ext[k][j].Equal(&ref[j]) {
					t.Fatalf("level %d ext row %d pos %d: encode mismatch", i, k, j)
				}
			}
		}
	}
}

// TestCommit exercises the full commitment path: Commit on a well-formed K=1
// witness produces a Merkle tree whose every leaf opens back to the root, and
// whose leaf count is invRate times the largest committed plaintext. Covers
// several blowups and an empty middle level.
func TestCommit(t *testing.T) {

	cases := []struct {
		name    string
		sizes   []int // plaintext sizes per level (2^i, or -1 for an empty level)
		invRate int
	}{
		{"three-levels", []int{1, 2, 4}, 2},
		{"four-levels-rate4", []int{1, 2, 4, 8}, 4},
		{"empty-middle", []int{1, -1, 4}, 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			var ctr uint64
			witness := make(MultiSizeTable, len(c.sizes))
			for i, s := range c.sizes {
				witness[i] = tableOfSize(s, &ctr)
			}
			encoders := makeEncoders(len(c.sizes), c.invRate)

			cs := Commit(encoders, witness)
			if cs.Tree == nil {
				t.Fatalf("Commit returned a nil tree")
			}

			wantLeaves := c.invRate * (1 << (len(c.sizes) - 1))
			if cs.Tree.NumLeaves() != wantLeaves {
				t.Fatalf("NumLeaves = %d, want %d", cs.Tree.NumLeaves(), wantLeaves)
			}

			root := cs.Tree.Root()
			for idx := 0; idx < cs.Tree.NumLeaves(); idx++ {
				branch := cs.Tree.OpenBranch(idx)
				got, err := branch.RecoverRoot(idx)
				if err != nil {
					t.Fatalf("idx %d: RecoverRoot: %v", idx, err)
				}
				if got != root {
					t.Fatalf("idx %d: recovered root != tree root", idx)
				}
			}
		})
	}
}
