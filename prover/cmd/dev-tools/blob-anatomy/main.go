// blob-anatomy fetches a contiguous range of blocks from a live Linea RPC
// endpoint, re-encodes them with the production blob encoder
// (v1.EncodeBlockForCompression), and reports how the resulting payload
// bytes split across block metadata, block hashes, sender addresses and
// transaction RLP. It also writes the fetched blocks as an RLP list of
// consensus-encoded blocks, in the same framing as
// jvm-libs/linea/blob-compressor/src/testFixtures/resources/blocks_rlp.bin,
// so the corpus is reusable by existing fixtures and tooling.
//
// Fetching stops once the accumulated uncompressed payload (as measured by
// the real encoder) reaches --target-bytes, so windows are sized by the
// quantity that matters for compression economics rather than by block
// count, which varies enormously with traffic density.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	gethereum "github.com/ethereum/go-ethereum"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
)

// fieldSizes decomposes one transaction's RLP payload by field, matching the
// tx type's own field set. residual is whatever the individually-encoded
// fields don't account for (list headers, type envelope bytes).
type fieldSizes struct {
	nonce, gas, gasPrice, gasTipCap, gasFeeCap, to, value, dataLen, accessList, blobFields int
	txRLPTotal                                                                             int
	residual                                                                               int
}

func encodeLen(v any) int {
	b, err := rlp.EncodeToBytes(v)
	if err != nil {
		return 0
	}
	return len(b)
}

func measureTx(tx *ethtypes.Transaction) fieldSizes {
	var fs fieldSizes
	fs.nonce = encodeLen(tx.Nonce())
	fs.gas = encodeLen(tx.Gas())
	switch tx.Type() {
	case ethtypes.LegacyTxType:
		fs.gasPrice = encodeLen(tx.GasPrice())
	default:
		fs.gasTipCap = encodeLen(tx.GasTipCap())
		fs.gasFeeCap = encodeLen(tx.GasFeeCap())
	}
	if to := tx.To(); to != nil {
		fs.to = 20
	}
	fs.value = encodeLen(tx.Value())
	fs.dataLen = len(tx.Data())
	if al := tx.AccessList(); len(al) > 0 {
		fs.accessList = encodeLen(al)
	}
	if bh := tx.BlobHashes(); len(bh) > 0 {
		fs.blobFields = encodeLen(bh)
	}
	fs.txRLPTotal = len(ethereum.EncodeTxForSigning(tx))
	accounted := fs.nonce + fs.gas + fs.gasPrice + fs.gasTipCap + fs.gasFeeCap + fs.to + fs.value + fs.dataLen + fs.accessList + fs.blobFields
	fs.residual = fs.txRLPTotal - accounted
	return fs
}

type windowStats struct {
	FirstBlock, LastBlock uint64
	BlockCount            int
	TxCount               int
	FirstTimestamp        uint64
	LastTimestamp         uint64
	TotalPayloadBytes     int
	MetadataBytes         int // 6 bytes/block: numTxs(2) + timestamp(4)
	BlockHashBytes        int // 32 bytes/block
	FromAddressBytes      int // 20 bytes/tx
	TxRLPBytes            int
	TxFieldNonce          int
	TxFieldGas            int
	TxFieldGasPrice       int
	TxFieldGasTipCap      int
	TxFieldGasFeeCap      int
	TxFieldTo             int
	TxFieldValue          int
	TxFieldData           int
	TxFieldAccessList     int
	TxFieldBlob           int
	TxFieldResidual       int
	HashChecksPerformed   int
	HashChecksPassed      int
}

func main() {
	rpcURL := flag.String("rpc", "https://rpc.linea.build", "JSON-RPC endpoint")
	start := flag.Uint64("start", 0, "first block number to fetch (required)")
	targetBytes := flag.Int("target-bytes", 20*780_000, "stop once accumulated uncompressed payload reaches this many bytes")
	maxBlocks := flag.Int("max-blocks", 200_000, "safety cap on blocks fetched")
	workers := flag.Int("workers", 16, "concurrent block fetches; 1 is the old sequential behaviour")
	outDir := flag.String("out", "testdata/blob-anatomy", "output directory")
	name := flag.String("name", "", "window name, e.g. 2025-09-25_busy (required)")
	label := flag.String("label", "", "human-readable traffic characterization, e.g. busy/median/quiet/current")
	flag.Parse()

	if *start == 0 || *name == "" {
		fmt.Fprintln(os.Stderr, "usage: blob-anatomy --start <block> --name <window-name> [--label busy|median|quiet|current] [--target-bytes N] [--rpc URL] [--out DIR]")
		os.Exit(2)
	}

	ctx := context.Background()
	rc, err := gethrpc.DialContext(ctx, *rpcURL)
	must(err)
	ec := ethclient.NewClient(rc)

	chainID, err := ec.ChainID(ctx)
	must(err)

	var (
		stats      windowStats
		blockBytes [][]byte
		fatal      error
	)

	fmt.Fprintf(os.Stderr, "fetching from block %d, target %d payload bytes...\n", *start, *targetBytes)

	reachedHead := false
	blocks, stopPrefetch := prefetch(ctx, ec, *start, *workers)
	defer stopPrefetch()
	for n := *start; stats.TotalPayloadBytes < *targetBytes && int(n-*start) < *maxBlocks; n++ {
		res, ok := <-blocks
		if !ok {
			break
		}
		blk, err := res.blk, res.err
		if err != nil {
			if errors.Is(err, gethereum.NotFound) {
				fmt.Fprintf(os.Stderr, "reached chain head at block %d\n", n)
				reachedHead = true
				break
			}
			// Anything else is a transport failure that survived retries. Do not
			// treat it as a stopping point: that is how a short corpus ends up
			// looking internally valid.
			fmt.Fprintf(os.Stderr, "fatal: block %d failed after retries: %v\n", n, err)
			fatal = err
			break
		}

		if stats.BlockCount == 0 {
			stats.FirstBlock = n
			stats.FirstTimestamp = blk.Time()
			verifyHash(rc, ctx, n, blk, &stats)
		}
		stats.LastBlock = n
		stats.LastTimestamp = blk.Time()

		raw, err := rlp.EncodeToBytes(blk)
		must(err)
		blockBytes = append(blockBytes, raw)

		stats.BlockCount++
		stats.MetadataBytes += 6
		stats.BlockHashBytes += 32
		stats.TotalPayloadBytes += 6 + 32

		for _, tx := range blk.Transactions() {
			stats.TxCount++
			stats.FromAddressBytes += 20
			stats.TotalPayloadBytes += 20

			fs := measureTx(tx)
			stats.TxRLPBytes += fs.txRLPTotal
			stats.TotalPayloadBytes += fs.txRLPTotal
			stats.TxFieldNonce += fs.nonce
			stats.TxFieldGas += fs.gas
			stats.TxFieldGasPrice += fs.gasPrice
			stats.TxFieldGasTipCap += fs.gasTipCap
			stats.TxFieldGasFeeCap += fs.gasFeeCap
			stats.TxFieldTo += fs.to
			stats.TxFieldValue += fs.value
			stats.TxFieldData += fs.dataLen
			stats.TxFieldAccessList += fs.accessList
			stats.TxFieldBlob += fs.blobFields
			stats.TxFieldResidual += fs.residual
		}

		if stats.BlockCount%2000 == 0 {
			fmt.Fprintf(os.Stderr, "  %d blocks, %d payload bytes so far\n", stats.BlockCount, stats.TotalPayloadBytes)
		}
	}
	// Cross-check: replay the real encoder on the last fetched block and
	// confirm our from/hash/metadata accounting matches its output length
	// exactly. This is a self-consistency check on the tool, not on the data.
	if stats.BlockCount > 0 {
		lastRaw := blockBytes[len(blockBytes)-1]
		var lastBlk ethtypes.Block
		must(rlp.DecodeBytes(lastRaw, &lastBlk))
		verifyHash(rc, ctx, stats.LastBlock, &lastBlk, &stats)

		var buf strings.Builder
		must(v1.EncodeBlockForCompression(&lastBlk, sbWriter{&buf}))
		want := 6 + 32 + 20*len(lastBlk.Transactions())
		for _, tx := range lastBlk.Transactions() {
			want += len(ethereum.EncodeTxForSigning(tx))
		}
		if buf.Len() != want {
			fmt.Fprintf(os.Stderr, "ANOMALY: encoder self-check mismatch on block %d: EncodeBlockForCompression wrote %d bytes, accounting predicted %d\n", lastBlk.NumberU64(), buf.Len(), want)
		} else {
			fmt.Fprintf(os.Stderr, "encoder self-check OK on block %d (%d bytes)\n", lastBlk.NumberU64(), buf.Len())
		}
	}

	must(os.MkdirAll(*outDir, 0o755))
	binPath := filepath.Join(*outDir, *name+".bin")
	manifestPath := filepath.Join(*outDir, *name+".manifest.json")

	corpus, err := rlp.EncodeToBytes(blockBytes)
	must(err)
	must(os.WriteFile(binPath, corpus, 0o644))

	manifest := buildManifest(*rpcURL, chainID.Uint64(), *label, stats)
	manifest["targetBytes"] = *targetBytes
	manifest["targetReached"] = stats.TotalPayloadBytes >= *targetBytes
	manifest["reachedChainHead"] = reachedHead
	if fatal != nil {
		manifest["fetchError"] = fatal.Error()
	}
	mb, err := json.MarshalIndent(manifest, "", "  ")
	must(err)
	must(os.WriteFile(manifestPath, mb, 0o644))

	fmt.Fprintf(os.Stderr, "\nwrote %s (%d bytes, %d blocks, %d txs)\n", binPath, len(corpus), stats.BlockCount, stats.TxCount)
	fmt.Fprintf(os.Stderr, "wrote %s\n", manifestPath)
	if stats.HashChecksPerformed > 0 && stats.HashChecksPassed != stats.HashChecksPerformed {
		fmt.Fprintf(os.Stderr, "ANOMALY: %d/%d block-hash checks failed\n", stats.HashChecksPerformed-stats.HashChecksPassed, stats.HashChecksPerformed)
	}
	if fatal != nil {
		fmt.Fprintf(os.Stderr, "INCOMPLETE: wrote %d of %d target payload bytes; see fetchError in the manifest\n",
			stats.TotalPayloadBytes, *targetBytes)
		os.Exit(1)
	}
}

// prefetch runs a bounded worker pool that fetches blocks ahead of the consumer
// and delivers them in order. ethclient has no batch API for full blocks, and
// one sequential BlockByNumber per block is latency-bound at ~10 blocks/s;
// overlapping `workers` requests reaches ~100 blocks/s against the same
// endpoint, which is the difference between a minute and a quarter of an hour
// per window.
//
// Ordering is preserved by giving each worker a fixed slot and reading the
// result channels in sequence, so the caller still sees a contiguous chain and
// the existing stop-on-NotFound logic is unchanged.
func prefetch(ctx context.Context, ec *ethclient.Client, start uint64, workers int) (<-chan blockResult, func()) {
	out := make(chan blockResult, workers)
	ctx, cancel := context.WithCancel(ctx)
	slots := make([]chan blockResult, workers)
	for i := range slots {
		slots[i] = make(chan blockResult, 1)
	}
	for i := 0; i < workers; i++ {
		go func(i int) {
			for n := start + uint64(i); ; n += uint64(workers) {
				blk, err := fetchBlock(ctx, ec, n)
				select {
				case slots[i] <- blockResult{blk, err}:
				case <-ctx.Done():
					return
				}
				if err != nil {
					return
				}
			}
		}(i)
	}
	go func() {
		defer close(out)
		for i := 0; ; i = (i + 1) % workers {
			select {
			case r := <-slots[i]:
				select {
				case out <- r:
				case <-ctx.Done():
					return
				}
				if r.err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, cancel
}

type blockResult struct {
	blk *ethtypes.Block
	err error
}

// fetchBlock retrieves one block, retrying transient failures with backoff.
// A "not found" is passed straight back: it means we have caught up with the
// head of the chain, which is a legitimate stopping point. Everything else —
// gateway errors, resets, timeouts — is transient and must not be allowed to
// silently truncate the corpus.
func fetchBlock(ctx context.Context, ec *ethclient.Client, n uint64) (*ethtypes.Block, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		blk, err := ec.BlockByNumber(ctx, new(big.Int).SetUint64(n))
		if err == nil {
			return blk, nil
		}
		if errors.Is(err, gethereum.NotFound) {
			return nil, err
		}
		lastErr = err
		fmt.Fprintf(os.Stderr, "  block %d attempt %d failed (%v), retrying\n", n, attempt+1, err)
		time.Sleep(time.Duration(1<<attempt) * 250 * time.Millisecond)
	}
	return nil, lastErr
}

// sbWriter adapts strings.Builder to io.Writer without importing bytes just
// for this.
type sbWriter struct{ *strings.Builder }

func (w sbWriter) Write(p []byte) (int, error) { return w.Builder.Write(p) }

func verifyHash(rc *gethrpc.Client, ctx context.Context, n uint64, blk *ethtypes.Block, stats *windowStats) {
	var raw json.RawMessage
	if err := rc.CallContext(ctx, &raw, "eth_getBlockByNumber", fmt.Sprintf("0x%x", n), false); err != nil {
		fmt.Fprintf(os.Stderr, "hash-check RPC failed for block %d: %v\n", n, err)
		return
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return
	}
	reported, _ := m["hash"].(string)
	stats.HashChecksPerformed++
	if strings.EqualFold(reported, blk.Hash().Hex()) {
		stats.HashChecksPassed++
	} else {
		fmt.Fprintf(os.Stderr, "ANOMALY: block %d hash mismatch: RPC reports %s, recomputed %s\n", n, reported, blk.Hash().Hex())
	}
}

func toolCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func buildManifest(rpcURL string, chainID uint64, label string, s windowStats) map[string]any {
	hours := float64(s.LastTimestamp-s.FirstTimestamp) / 3600.0
	if hours <= 0 {
		hours = 1.0 / 3600.0
	}
	avgTxPerBlock := 0.0
	if s.BlockCount > 0 {
		avgTxPerBlock = float64(s.TxCount) / float64(s.BlockCount)
	}
	return map[string]any{
		"rpcEndpoint":         rpcURL,
		"chainId":             chainID,
		"fetchedAtUTC":        time.Now().UTC().Format(time.RFC3339),
		"toolCommit":          toolCommit(),
		"label":               label,
		"blockRange":          map[string]uint64{"first": s.FirstBlock, "last": s.LastBlock},
		"blockCount":          s.BlockCount,
		"txCount":             s.TxCount,
		"firstTimestamp":      s.FirstTimestamp,
		"lastTimestamp":       s.LastTimestamp,
		"windowHours":         hours,
		"avgTxPerBlock":       avgTxPerBlock,
		"avgPayloadBPerBlock": float64(s.TotalPayloadBytes) / float64(max(1, s.BlockCount)),
		"payloadBytesPerHour": float64(s.TotalPayloadBytes) / hours,
		"totalPayloadBytes":   s.TotalPayloadBytes,
		"byteBreakdown": map[string]int{
			"blockMetadata": s.MetadataBytes,
			"blockHash":     s.BlockHashBytes,
			"fromAddresses": s.FromAddressBytes,
			"txRLP":         s.TxRLPBytes,
		},
		"txFieldBreakdown": map[string]int{
			"nonce":      s.TxFieldNonce,
			"gas":        s.TxFieldGas,
			"gasPrice":   s.TxFieldGasPrice,
			"gasTipCap":  s.TxFieldGasTipCap,
			"gasFeeCap":  s.TxFieldGasFeeCap,
			"to":         s.TxFieldTo,
			"value":      s.TxFieldValue,
			"data":       s.TxFieldData,
			"accessList": s.TxFieldAccessList,
			"blobFields": s.TxFieldBlob,
			"residual":   s.TxFieldResidual,
		},
		"hashChecks": map[string]int{
			"performed": s.HashChecksPerformed,
			"passed":    s.HashChecksPassed,
		},
		"note": "byteBreakdown sums to totalPayloadBytes exactly (it is the same accounting v1.EncodeBlockForCompression uses). txFieldBreakdown decomposes txRLP only; residual accounts for RLP list headers and type-envelope bytes not attributable to a single field.",
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}
