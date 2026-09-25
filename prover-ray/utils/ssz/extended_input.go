package ssz

import "encoding/binary"

// This file encodes the extended l2-execution guest input (schema 0x0002),
// mirroring rollup_spec/l2_execution_ssz.py::encode_l2_execution_input. It wraps
// the per-payload 0x0001 vanilla stateless input (see EncodeStatelessInput) with
// the chain config, parent-FTX fields, and forced transactions. Golden-vector
// pinned against the Python reference in extended_input_test.go.

// extendedInputSchemaID is the two-byte big-endian schema id framing the 0x0002
// extended input.
var extendedInputSchemaID = []byte{0x00, 0x02}

const (
	// SSZ fixed-part sizes (little-endian uints, 4-byte offsets).
	chainConfigSize   = 20 + 20 + 8                  // l2MessageServiceAddress, coinbase, chainId
	privateInputFixed = 32 + 8 + chainConfigSize + 4 // hash, u64, chain config, payloads offset
	payloadFixed      = 4 + 4                        // statelessInput offset, forcedTransactions offset
	forcedTxFixed     = 8 + 4 + 1 + 8                // number, rlp offset, acceptance, deadline
)

// ChainConfig is the range-level chain identity in the extended input.
type ChainConfig struct {
	L2MessageServiceAddress [20]byte
	Coinbase                [20]byte
	ChainID                 uint64
}

// ForcedTransaction is one forced-transaction witness. Acceptance is the
// ForcedTransactionAcceptance enum value (0..4); callers map the request string
// to that value.
type ForcedTransaction struct {
	Number      uint64
	SignedTxRlp []byte
	Acceptance  uint8
	Deadline    uint64
}

// PayloadInput is one block's payload: the opaque 0x0001-framed vanilla
// stateless input plus its forced transactions.
type PayloadInput struct {
	StatelessInputSSZ  []byte
	ForcedTransactions []ForcedTransaction
}

// ExtendedInput is the decoded L2ExecutionProofPrivateInput ready to encode.
type ExtendedInput struct {
	ParentFtxRollingHash         [32]byte
	ParentLastProcessedFtxNumber uint64
	ChainConfig                  ChainConfig
	Payloads                     []PayloadInput
}

// EncodeExtendedInput encodes an ExtendedInput into framed SSZ bytes (0x0002
// schema id) that the l2-execution guest and native runner read.
func EncodeExtendedInput(in ExtendedInput) []byte {
	out := append([]byte(nil), extendedInputSchemaID...)
	return append(out, encodePrivateInput(in)...)
}

func encodePrivateInput(in ExtendedInput) []byte {
	payloads := make([][]byte, len(in.Payloads))
	for i, p := range in.Payloads {
		payloads[i] = encodePayloadInput(p)
	}
	payloadList := listWithOffsets(payloads)

	out := make([]byte, privateInputFixed, privateInputFixed+len(payloadList))
	copy(out[0:32], in.ParentFtxRollingHash[:])
	binary.LittleEndian.PutUint64(out[32:40], in.ParentLastProcessedFtxNumber)
	copy(out[40:60], in.ChainConfig.L2MessageServiceAddress[:])
	copy(out[60:80], in.ChainConfig.Coinbase[:])
	binary.LittleEndian.PutUint64(out[80:88], in.ChainConfig.ChainID)
	binary.LittleEndian.PutUint32(out[88:92], privateInputFixed)
	return append(out, payloadList...)
}

func encodePayloadInput(p PayloadInput) []byte {
	ftxs := make([][]byte, len(p.ForcedTransactions))
	for i, ftx := range p.ForcedTransactions {
		ftxs[i] = encodeForcedTransaction(ftx)
	}
	ftxList := listWithOffsets(ftxs)

	out := make([]byte, payloadFixed, payloadFixed+len(p.StatelessInputSSZ)+len(ftxList))
	binary.LittleEndian.PutUint32(out[0:4], payloadFixed)
	binary.LittleEndian.PutUint32(out[4:8], uint32(payloadFixed+len(p.StatelessInputSSZ)))
	out = append(out, p.StatelessInputSSZ...)
	return append(out, ftxList...)
}

func encodeForcedTransaction(ftx ForcedTransaction) []byte {
	out := make([]byte, forcedTxFixed, forcedTxFixed+len(ftx.SignedTxRlp))
	binary.LittleEndian.PutUint64(out[0:8], ftx.Number)
	binary.LittleEndian.PutUint32(out[8:12], forcedTxFixed)
	out[12] = ftx.Acceptance
	binary.LittleEndian.PutUint64(out[13:21], ftx.Deadline)
	return append(out, ftx.SignedTxRlp...)
}

// listWithOffsets encodes an SSZ list of variable-size elements: a table of
// uint32 offsets followed by the elements.
func listWithOffsets(elems [][]byte) []byte {
	table := 4 * len(elems)
	total := table
	for _, e := range elems {
		total += len(e)
	}
	out := make([]byte, table, total)
	off := table
	for i, e := range elems {
		binary.LittleEndian.PutUint32(out[i*4:i*4+4], uint32(off))
		off += len(e)
	}
	for _, e := range elems {
		out = append(out, e...)
	}
	return out
}
