// blob-calldata-probe reports the internal structure of transaction calldata
// in a blob-anatomy corpus: zero-byte density, how much is ABI-shaped
// (4-byte selector + a multiple of 32), calldata length distribution,
// selector concentration, and the per-lane zero rate across 32-byte word
// positions.
//
// Measured on Linea mainnet: ~70% of calldata bytes are zero, ~86-90% of
// calldata-bearing txs are ABI-shaped, and the per-lane zero rate runs 75-85%
// for lanes 0-11 down to ~32% at lane 31 - the signature of value-in-low-bytes
// ABI word padding. Note that exploiting this via transposition was measured
// and found counterproductive; see blob-streams.
//
// Usage: blob-calldata-probe <corpus.bin>
package main

import (
	"fmt"
	"os"
	"sort"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var raw [][]byte
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		panic(err)
	}

	var totalCalldata, zeroBytes int
	var nTx, nWithData, abiShaped int
	selCount := map[[4]byte]int{}
	var lens []int
	// zero-run histogram within 32-byte lanes
	laneZero := make([]int, 32)
	laneTotal := make([]int, 32)

	for _, rb := range raw {
		var blk ethtypes.Block
		if err := rlp.DecodeBytes(rb, &blk); err != nil {
			continue
		}
		for _, tx := range blk.Transactions() {
			nTx++
			d := tx.Data()
			if len(d) == 0 {
				continue
			}
			nWithData++
			totalCalldata += len(d)
			lens = append(lens, len(d))
			for _, b := range d {
				if b == 0 {
					zeroBytes++
				}
			}
			if len(d) >= 4 {
				var s [4]byte
				copy(s[:], d[:4])
				selCount[s]++
				if (len(d)-4)%32 == 0 {
					abiShaped++
				}
				// lane analysis on the argument region
				args := d[4:]
				for i, b := range args {
					lane := i % 32
					laneTotal[lane]++
					if b == 0 {
						laneZero[lane]++
					}
				}
			}
		}
	}
	sort.Ints(lens)
	fmt.Printf("txs=%d  withCalldata=%d  totalCalldata=%d bytes\n", nTx, nWithData, totalCalldata)
	fmt.Printf("zero bytes in calldata: %.1f%%\n", 100*float64(zeroBytes)/float64(totalCalldata))
	fmt.Printf("ABI-shaped (4+32k): %.1f%% of calldata-bearing txs\n", 100*float64(abiShaped)/float64(nWithData))
	if len(lens) > 0 {
		fmt.Printf("calldata len: median=%d p90=%d max=%d\n", lens[len(lens)/2], lens[len(lens)*9/10], lens[len(lens)-1])
	}
	fmt.Printf("distinct selectors: %d\n", len(selCount))
	type kv struct {
		s [4]byte
		n int
	}
	var top []kv
	for s, n := range selCount {
		top = append(top, kv{s, n})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].n > top[j].n })
	var cum int
	for i := 0; i < len(top) && i < 10; i++ {
		cum += top[i].n
	}
	fmt.Printf("top-10 selectors cover %.1f%% of calldata-bearing txs\n", 100*float64(cum)/float64(nWithData))
	fmt.Print("per-lane zero rate (32-byte word positions, arg region):\n  ")
	for i := 0; i < 32; i++ {
		if laneTotal[i] == 0 {
			continue
		}
		fmt.Printf("%2.0f ", 100*float64(laneZero[i])/float64(laneTotal[i]))
	}
	fmt.Println()
}
