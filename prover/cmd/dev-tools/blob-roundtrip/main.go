// blob-roundtrip validates that a blob-anatomy corpus produces payloads whose
// CONTENT is correct, not merely whose byte counts add up.
//
// The fetcher's built-in self-check only confirms that the encoder's output
// length matches the sum of its parts. That would still pass if ethclient
// reconstruction produced subtly wrong transactions, which would leave payload
// length untouched while changing exactly the content that determines
// compressibility.
//
// Checks, all run over every block in the corpus:
//
//   - txRoot: DeriveSha(block.Transactions()) == header.TxHash. This is the
//     strongest available guarantee that the transaction list we hold is the
//     one the block actually commits to. It is purely local.
//   - scanLen: v1.ScanBlockByteLen agrees with the number of bytes the encoder
//     wrote. This is the routine that recovers block boundaries in production.
//   - decode: v1.DecodeBlockFromUncompressed succeeds.
//   - fields: every decoded transaction's nonce, gas, gas price / tip / fee cap,
//     recipient, value and calldata match the original, and the recovered
//     sender matches ethereum.GetFrom on the original.
//   - hash (only with --rpc): the locally recomputed block.Hash() matches the
//     hash the node reports, verifying ethclient's header reconstruction.
//     Fetched in batches.
//
// Deliberately NOT checked: equality of EncodeTxForSigning(original) against
// EncodeTxForSigning(rebuilt). For EIP-155 legacy transactions those differ,
// because EncodeTxForSigning emits nine fields (including chainID, 0, 0) while
// decodeLegacyTx reads back only the first six and never restores V. The
// rebuilt transaction therefore reports a nonsense ChainId() and re-encodes
// five bytes shorter. This does not affect the corpus — payload bytes are
// produced from the original transactions — but it does mean the encode/decode
// pair is lossy for legacy chainID, which matters to any scheme that stores
// decoded fields and re-encodes RLP.
//
// Usage: blob-roundtrip [--rpc URL] <corpus.bin>
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
)

type counts struct {
	blocks, txs                             int
	txRootBad, scanBad, decodeBad           int
	fieldBad, fromBad, hashBad, hashChecked int
}

func bigEq(a, b *big.Int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Cmp(b) == 0
}

func main() {
	rpcURL := flag.String("rpc", "", "optional JSON-RPC endpoint; if set, verifies every block hash against the node")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: blob-roundtrip [--rpc URL] <corpus.bin>")
		os.Exit(2)
	}

	data, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		panic(err)
	}
	var raw [][]byte
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		panic(err)
	}

	var c counts
	reported := map[uint64]string{}
	if *rpcURL != "" {
		reported = fetchHashes(*rpcURL, raw)
	}

	shown := 0
	report := func(format string, args ...any) {
		if shown < 5 {
			fmt.Fprintf(os.Stderr, format, args...)
			shown++
		}
	}

	for _, rb := range raw {
		var blk ethtypes.Block
		if err := rlp.DecodeBytes(rb, &blk); err != nil {
			continue
		}
		c.blocks++

		if ethtypes.DeriveSha(blk.Transactions(), trie.NewStackTrie(nil)) != blk.TxHash() {
			c.txRootBad++
			report("block %d: txRoot mismatch\n", blk.NumberU64())
		}

		if want, ok := reported[blk.NumberU64()]; ok {
			c.hashChecked++
			if !bytes.EqualFold([]byte(want), []byte(blk.Hash().Hex())) {
				c.hashBad++
				report("block %d: hash mismatch, node=%s local=%s\n", blk.NumberU64(), want, blk.Hash().Hex())
			}
		}

		var buf bytes.Buffer
		if err := v1.EncodeBlockForCompression(&blk, &buf); err != nil {
			panic(err)
		}
		enc := buf.Bytes()

		if n, err := v1.ScanBlockByteLen(enc); err != nil || n != len(enc) {
			c.scanBad++
			report("block %d: ScanBlockByteLen=%d err=%v, wrote %d\n", blk.NumberU64(), n, err, len(enc))
			continue
		}

		dec, err := v1.DecodeBlockFromUncompressed(bytes.NewReader(enc))
		if err != nil {
			c.decodeBad++
			report("block %d: decode failed: %v\n", blk.NumberU64(), err)
			continue
		}
		orig := blk.Transactions()
		if len(dec.Txs) != len(orig) {
			c.decodeBad++
			report("block %d: tx count %d != %d\n", blk.NumberU64(), len(dec.Txs), len(orig))
			continue
		}

		for i, td := range dec.Txs {
			c.txs++
			o, r := orig[i], ethtypes.NewTx(td)
			ok := o.Nonce() == r.Nonce() &&
				o.Gas() == r.Gas() &&
				bigEq(o.Value(), r.Value()) &&
				bytes.Equal(o.Data(), r.Data()) &&
				((o.To() == nil) == (r.To() == nil)) &&
				(o.To() == nil || *o.To() == *r.To())
			if ok && o.Type() == ethtypes.LegacyTxType {
				ok = bigEq(o.GasPrice(), r.GasPrice())
			} else if ok {
				ok = bigEq(o.GasTipCap(), r.GasTipCap()) && bigEq(o.GasFeeCap(), r.GasFeeCap())
			}
			if !ok {
				c.fieldBad++
				report("block %d tx %d: field mismatch (type %d)\n", blk.NumberU64(), i, o.Type())
			}
			origFrom := ethereum.GetFrom(o)
			if !bytes.Equal(dec.Froms[i][:], origFrom[:]) {
				c.fromBad++
				report("block %d tx %d: from mismatch %x vs %x\n", blk.NumberU64(), i, dec.Froms[i], origFrom)
			}
		}
	}

	fmt.Printf("blocks=%d txs=%d hashChecked=%d\n", c.blocks, c.txs, c.hashChecked)
	fmt.Printf("txRootBad=%d scanBad=%d decodeBad=%d fieldBad=%d fromBad=%d hashBad=%d\n",
		c.txRootBad, c.scanBad, c.decodeBad, c.fieldBad, c.fromBad, c.hashBad)
	if c.txRootBad+c.scanBad+c.decodeBad+c.fieldBad+c.fromBad+c.hashBad == 0 {
		fmt.Println("ALL CHECKS PASS")
		return
	}
	fmt.Println("FAILURES PRESENT")
	os.Exit(1)
}

// fetchHashes retrieves the node-reported hash for every block in the corpus,
// in batches, using header-only responses.
func fetchHashes(url string, raw [][]byte) map[uint64]string {
	ctx := context.Background()
	rc, err := gethrpc.DialContext(ctx, url)
	if err != nil {
		panic(err)
	}
	var nums []uint64
	for _, rb := range raw {
		var blk ethtypes.Block
		if rlp.DecodeBytes(rb, &blk) == nil {
			nums = append(nums, blk.NumberU64())
		}
	}
	out := make(map[uint64]string, len(nums))
	const batch = 100
	for i := 0; i < len(nums); i += batch {
		end := min(i+batch, len(nums))
		elems := make([]gethrpc.BatchElem, 0, end-i)
		results := make([]map[string]any, end-i)
		for j := i; j < end; j++ {
			elems = append(elems, gethrpc.BatchElem{
				Method: "eth_getBlockByNumber",
				Args:   []any{fmt.Sprintf("0x%x", nums[j]), false},
				Result: &results[j-i],
			})
		}
		if err := rc.BatchCallContext(ctx, elems); err != nil {
			fmt.Fprintf(os.Stderr, "hash batch %d failed: %v\n", i, err)
			continue
		}
		for j, e := range elems {
			if e.Error != nil || results[j] == nil {
				continue
			}
			if h, ok := results[j]["hash"].(string); ok {
				out[nums[i+j]] = h
			}
		}
		if (i/batch)%20 == 0 {
			fmt.Fprintf(os.Stderr, "  fetched %d/%d hashes\n", end, len(nums))
		}
	}
	return out
}
