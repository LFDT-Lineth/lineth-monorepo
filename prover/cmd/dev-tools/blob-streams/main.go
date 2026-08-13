// blob-streams reads a blob-anatomy corpus and emits the encoded blob payload
// alongside per-field-class streams, so the payload anatomy can be measured in
// the COMPRESSED domain rather than the uncompressed one.
//
// This distinction matters: calldata is ~90% of uncompressed payload but only
// ~74% of compressed payload, while block hashes are ~0.8% uncompressed and
// ~6.7% compressed (they are incompressible). Decisions about which fields are
// worth optimising must be made on the compressed shares.
//
// It also emits a stride-32 transpose of the ABI argument regions
// (calldata_lanes.bin) so the value of columnar re-encoding can be tested
// directly. Measured result: the transpose roughly DOUBLES compressed size,
// because it destroys the cross-transaction literal matches that LZ relies on.
//
// Usage: blob-streams <corpus.bin> <outDir>
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: blob-streams <corpus.bin> <outDir>")
		os.Exit(2)
	}
	corpus, outDir := os.Args[1], os.Args[2]
	data, err := os.ReadFile(corpus)
	if err != nil {
		panic(err)
	}
	var raw [][]byte
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}

	var payload bytes.Buffer             // exactly what v1.EncodeBlockForCompression produces
	var blockEnds []int                  // cumulative payload offset after each block
	var meta, hashes, froms bytes.Buffer // metadata classes
	var selectors, calldataRaw, irregular bytes.Buffer
	var structFields bytes.Buffer // nonce/gas/fees/to/value, RLP-encoded per field
	var lanes [32]bytes.Buffer    // stride-32 transpose of ABI argument regions

	for _, rb := range raw {
		var blk ethtypes.Block
		if err := rlp.DecodeBytes(rb, &blk); err != nil {
			continue
		}
		if err := v1.EncodeBlockForCompression(&blk, &payload); err != nil {
			panic(err)
		}
		// Blob filling searches over BLOCK counts, not bytes: production cannot
		// split a block across blobs. Record the boundaries so a driver can slice
		// the payload at legal points without re-encoding.
		blockEnds = append(blockEnds, payload.Len())

		var m [6]byte
		n := uint16(len(blk.Transactions()))
		m[0], m[1] = byte(n>>8), byte(n)
		t := uint32(blk.Time())
		m[2], m[3], m[4], m[5] = byte(t>>24), byte(t>>16), byte(t>>8), byte(t)
		meta.Write(m[:])
		h := blk.Hash()
		hashes.Write(h[:])

		for _, tx := range blk.Transactions() {
			f := ethereum.GetFrom(tx)
			froms.Write(f[:])

			for _, v := range []any{tx.Nonce(), tx.Gas(), tx.GasTipCap(), tx.GasFeeCap(), tx.Value()} {
				b, _ := rlp.EncodeToBytes(v)
				structFields.Write(b)
			}
			if to := tx.To(); to != nil {
				structFields.Write(to[:])
			}

			d := tx.Data()
			calldataRaw.Write(d)
			if len(d) >= 4 && (len(d)-4)%32 == 0 {
				selectors.Write(d[:4])
				for i, b := range d[4:] {
					lanes[i%32].WriteByte(b)
				}
			} else {
				irregular.Write(d)
			}
		}
	}

	write := func(name string, b []byte) {
		if err := os.WriteFile(filepath.Join(outDir, name), b, 0o644); err != nil {
			panic(err)
		}
		fmt.Printf("%-22s %10d\n", name, len(b))
	}
	fmt.Println("stream                    bytes")
	write("payload.bin", payload.Bytes())
	write("meta.bin", meta.Bytes())
	write("hashes.bin", hashes.Bytes())
	write("froms.bin", froms.Bytes())
	write("structfields.bin", structFields.Bytes())
	write("selectors.bin", selectors.Bytes())
	write("calldata_raw.bin", calldataRaw.Bytes())
	write("calldata_irregular.bin", irregular.Bytes())
	var lanesCat bytes.Buffer
	for i := range lanes {
		lanesCat.Write(lanes[i].Bytes())
	}
	write("calldata_lanes.bin", lanesCat.Bytes())

	var offs bytes.Buffer
	for _, e := range blockEnds {
		fmt.Fprintf(&offs, "%d\n", e)
	}
	if err := os.WriteFile(filepath.Join(outDir, "block_ends.txt"), offs.Bytes(), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("%-22s %10d  (block boundaries)\n", "block_ends.txt", len(blockEnds))
}
