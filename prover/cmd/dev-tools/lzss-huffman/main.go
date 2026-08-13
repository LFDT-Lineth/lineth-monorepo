// lzss-huffman derives hardcoded canonical Huffman code lengths for a combined
// literal/length alphabet over the deployed LZSS symbol stream.
//
// Design being tabulated:
//
//	alphabet = 256 literals + 256 backref lengths = 512 symbols
//	after a length symbol: 1 flag bit (0 = near, 1 = far), then a raw
//	14- or 21-bit address
//
// Folding near/far into the alphabet instead (768 symbols) codes 0.11% better
// but raises the maximum code length from 11 to 19 bits, which is ~70% more
// iterations in a table-free decoder, paid on every literal. Near/far is
// 48.4/51.6 across the corpus, so a flag bit carries 0.9992 bits and wastes
// essentially nothing.
//
// # Why a minimum code length, and why it removes the need for an EOF symbol
//
// The deployed scheme has no end marker: every symbol is exactly 8 bits and
// byte-aligned, so exhausting the input terminates the stream unambiguously.
// Huffman would normally break that, since up to 7 bits of padding remain in
// the final byte and could decode as a spurious symbol. Requiring every codeword
// to be at least 8 bits restores the property: fewer than 8 trailing bits can
// never complete a codeword, so the stream still self-terminates and no
// end-of-stream symbol is required.
//
// This is feasible. Kraft requires sum(2^-len) <= 1; with every length >= 8 each
// term is at most 2^-8, so the constraint is easily met - indeed reaching a
// complete code (sum == 1) requires at *least* 256 codewords, and we have 512.
//
// # Why the table is hardcoded
//
// Measured cost is 0.51% (cross-entropy against tables trained only on other
// windows). In exchange the decoder never builds a table, so under zkC it is a
// precomputed column rather than a per-proof committed one; there is no
// untrusted table to validate, removing a class of malleability bug that DEFLATE
// and zstd decoders all carry; and the encoder loses its frequency-counting
// pass.
//
// Only code LENGTHS are emitted. Canonical Huffman derives codes from lengths
// deterministically (DEFLATE 3.2.2: order by length, then by symbol).
//
// Usage:
//
//	lzss-huffman --payloads <dir> --dict <file> [--emit-go <file>]
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/consensys/compress/lzss"
)

const (
	chunkSize  = 780_000
	nLiterals  = 256
	nLengths   = 256 // LZSS backref lengths are 1..256
	alphabet   = nLiterals + nLengths
	symShort   = 0xFE
	symDynamic = 0xFF
)

// bitReader reads MSB-first, matching icza/bitio as used by consensys/compress.
type bitReader struct {
	buf []byte
	pos int
}

func (r *bitReader) bits(n int) uint64 {
	var v uint64
	for i := 0; i < n; i++ {
		b := r.buf[r.pos>>3]
		v = v<<1 | uint64(b>>(7-(r.pos&7))&1)
		r.pos++
	}
	return v
}

func (r *bitReader) left() int { return len(r.buf)*8 - r.pos }

// countSymbols walks a compressed LZSS stream and tallies the combined
// literal/length alphabet. A literal is a raw byte; 0xFE and 0xFF are
// delimiters introducing an 8-bit length-1 and then a 14- or 21-bit address.
// Collapsing the delimiter into the length symbol is where most of the coding
// gain comes from: today every backref pays a whole byte just to announce
// itself.
func countSymbols(stream []byte, counts []uint64) {
	r := &bitReader{buf: stream}
	for r.left() >= 8 {
		sym := byte(r.bits(8))
		if sym == symShort || sym == symDynamic {
			width := 14
			if sym == symDynamic {
				width = 21
			}
			if r.left() < 8+width {
				return
			}
			length := int(r.bits(8)) + 1
			r.bits(width) // address stays raw, not entropy-coded
			counts[nLiterals+length-1]++
			continue
		}
		counts[sym]++
	}
}

// packageMerge computes optimal prefix-code lengths subject to a maximum
// (Larmore-Hirschberg). Plain Huffman is simpler and on this corpus peaks at 11
// bits, but "usually under the limit" is not a guarantee once smoothing adds
// near-zero-frequency symbols, and the usual fixup heuristics can emit a code
// that violates Kraft. This is exact.
func packageMerge(weights []uint64, limit int) []uint8 {
	n := len(weights)
	type item struct {
		w       uint64
		members []int
	}
	base := make([]item, n)
	for i, w := range weights {
		base[i] = item{w, []int{i}}
	}
	sort.Slice(base, func(i, j int) bool { return base[i].w < base[j].w })

	current := append([]item(nil), base...)
	for level := 0; level < limit-1; level++ {
		var packaged []item
		for j := 0; j+1 < len(current); j += 2 {
			packaged = append(packaged, item{
				w:       current[j].w + current[j+1].w,
				members: append(append([]int{}, current[j].members...), current[j+1].members...),
			})
		}
		current = append(append([]item(nil), base...), packaged...)
		sort.Slice(current, func(i, j int) bool { return current[i].w < current[j].w })
	}

	lengths := make([]uint8, n)
	for _, it := range current[:2*n-2] {
		for _, m := range it.members {
			lengths[m]++
		}
	}
	return lengths
}

func kraft(lengths []uint8) float64 {
	s := 0.0
	for _, l := range lengths {
		s += math.Exp2(-float64(l))
	}
	return s
}

func main() {
	payloads := flag.String("payloads", os.ExpandEnv("$HOME/linea-blob-corpus/payloads"),
		"directory of *.payload.bin")
	dictPath := flag.String("dict", "", "compression dictionary (required)")
	chunksPer := flag.Int("chunks-per-window", 4, "chunks sampled per payload")
	minBits := flag.Int("min-bits", 8,
		"minimum code length; 8 keeps the stream self-terminating without an EOF symbol")
	maxBits := flag.Int("max-bits", 15,
		"maximum code length; the decode loop iterates once per level")
	emitGo := flag.String("emit-go", "", "write the table as a Go source file")
	flag.Parse()

	if *dictPath == "" {
		fmt.Fprintln(os.Stderr, "usage: lzss-huffman --dict <file> [--payloads <dir>] [--emit-go <file>]")
		os.Exit(2)
	}
	rawDict, err := os.ReadFile(*dictPath)
	must(err)
	dict := lzss.AugmentDict(rawDict)

	files, err := filepath.Glob(filepath.Join(*payloads, "*.payload.bin"))
	must(err)
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no payloads in %s\n", *payloads)
		os.Exit(1)
	}
	sort.Strings(files)

	counts := make([]uint64, alphabet)
	var total uint64
	for _, f := range files {
		data, err := os.ReadFile(f)
		must(err)
		for i := 0; i < *chunksPer; i++ {
			lo, hi := i*chunkSize, (i+1)*chunkSize
			if hi > len(data) {
				break
			}
			c, err := lzss.NewCompressor(dict)
			must(err)
			_, err = c.Write(data[lo:hi])
			must(err)
			countSymbols(c.Bytes(), counts)
		}
		total = 0
		for _, v := range counts {
			total += v
		}
		fmt.Fprintf(os.Stderr, "  %s: running total %d symbols\n", filepath.Base(f), total)
	}

	// Laplace smoothing. A hardcoded table has to encode input it has never
	// seen; a literal byte absent from the corpus still needs a codeword.
	weights := make([]uint64, alphabet)
	unseen := 0
	for i := range weights {
		if counts[i] == 0 {
			unseen++
		}
		weights[i] = counts[i] + 1
	}

	lengths := packageMerge(weights, *maxBits)

	// Raising a length only shrinks its Kraft term, so clamping the minimum
	// upward can never invalidate the code. It does leave the code incomplete
	// (sum < 1); the slack is reported below and is the price of dropping the
	// EOF symbol.
	raised := 0
	for i, l := range lengths {
		if int(l) < *minBits {
			lengths[i] = uint8(*minBits)
			raised++
		}
	}

	k := kraft(lengths)
	if k > 1.0+1e-9 {
		fmt.Fprintf(os.Stderr, "fatal: Kraft sum %.12f > 1, not a prefix code\n", k)
		os.Exit(1)
	}
	maxLen := 0
	for _, l := range lengths {
		if int(l) > maxLen {
			maxLen = int(l)
		}
		if int(l) < *minBits {
			fmt.Fprintln(os.Stderr, "fatal: minimum length not enforced")
			os.Exit(1)
		}
	}

	var ideal, actual float64
	for i, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / float64(total)
		ideal -= p * math.Log2(p)
		actual += p * float64(lengths[i])
	}

	fmt.Printf("\n%d symbols over %d distinct (%d of %d unseen, smoothed in)\n",
		total, alphabet-unseen, unseen, alphabet)
	fmt.Printf("  ideal entropy    %7.4f bits/symbol\n", ideal)
	fmt.Printf("  this fixed table %7.4f bits/symbol   (+%.2f%% over adaptive-ideal)\n",
		actual, 100*(actual/ideal-1))
	fmt.Printf("  code lengths     [%d, %d]   (%d raised to the minimum)\n",
		*minBits, maxLen, raised)
	fmt.Printf("  Kraft sum        %.12f   (slack %.6f = cost of no EOF symbol)\n", k, 1-k)

	hist := map[uint8]int{}
	for _, l := range lengths {
		hist[l]++
	}
	fmt.Println("\n  code length histogram:")
	for b := uint8(1); b <= uint8(maxLen); b++ {
		if hist[b] > 0 {
			fmt.Printf("    %2d bits: %4d symbols\n", b, hist[b])
		}
	}

	if *emitGo != "" {
		var b strings.Builder
		fmt.Fprintf(&b, "// Code generated by cmd/dev-tools/lzss-huffman. DO NOT EDIT.\n\npackage lzss\n\n")
		fmt.Fprintf(&b, "// SymbolCodeLengths holds canonical Huffman code lengths for the combined\n")
		fmt.Fprintf(&b, "// literal/length alphabet: 0..255 are literal bytes, 256..511 are backref\n")
		fmt.Fprintf(&b, "// lengths 1..256. Codes follow the canonical construction (DEFLATE 3.2.2):\n")
		fmt.Fprintf(&b, "// order by length, then by symbol.\n//\n")
		fmt.Fprintf(&b, "// A backref symbol is followed by one flag bit (0 = near, 14-bit address;\n")
		fmt.Fprintf(&b, "// 1 = far, 21-bit address) and then the raw address bits.\n//\n")
		fmt.Fprintf(&b, "// Every code is at least %d bits, so fewer than 8 trailing pad bits can never\n", *minBits)
		fmt.Fprintf(&b, "// complete a codeword and the stream self-terminates without an EOF symbol.\n//\n")
		fmt.Fprintf(&b, "// Derived from %d symbols; %.4f bits/symbol against an ideal %.4f.\n",
			total, actual, ideal)
		fmt.Fprintf(&b, "var SymbolCodeLengths = [%d]uint8{\n", alphabet)
		for row := 0; row < alphabet; row += 16 {
			end := row + 16
			if end > alphabet {
				end = alphabet
			}
			b.WriteString("\t")
			for s := row; s < end; s++ {
				fmt.Fprintf(&b, "%d, ", lengths[s])
			}
			b.WriteString("\n")
		}
		b.WriteString("}\n")
		must(os.WriteFile(*emitGo, []byte(b.String()), 0o644))
		fmt.Printf("\nwrote %s\n", *emitGo)
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}
