// Package nativerunner runs the native l2_execution_runner on an extended
// (0x0002) input and parses its --json output into the guest's public inputs.
package nativerunner

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
)

// Output is the parsed native-runner result for one l2-execution request.
type Output struct {
	StartBlockNumber uint64
	PublicInputs     backend.PublicInputs
}

// Run writes the extended input to a temp file, runs `<binPath> <file> --json`,
// and parses stdout. binPath is the native l2_execution_runner binary.
func Run(ctx context.Context, binPath string, extendedInput []byte) (Output, error) {
	out, err := run(ctx, binPath, extendedInput, "--json")
	if err != nil {
		return Output{}, err
	}
	return Parse(out)
}

// RunSSZ runs `<binPath> <file> --ssz` and returns the raw 0x0003 output. It is
// byte-identical to what the guest writes to guest_output, so dev-zkvm compares
// it against the ZkC Execute output to cross-check the native runner.
func RunSSZ(ctx context.Context, binPath string, extendedInput []byte) ([]byte, error) {
	return run(ctx, binPath, extendedInput, "--ssz")
}

// run writes the extended input to a temp file and runs the native runner with
// the given output flag, returning its stdout.
func run(ctx context.Context, binPath string, extendedInput []byte, flag string) ([]byte, error) {
	tmp, err := os.CreateTemp("", "l2-exec-input-*.ssz")
	if err != nil {
		return nil, fmt.Errorf("creating temp input: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(extendedInput); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("writing temp input: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("closing temp input: %w", err)
	}

	//nolint:gosec // G204: binPath is an operator-configured trusted path, not user input
	out, err := exec.CommandContext(ctx, binPath, tmp.Name(), flag).Output()
	if err != nil {
		return nil, fmt.Errorf("running native runner %q: %w", binPath, err)
	}
	return out, nil
}

// jsonOutput mirrors the runner's --json shape: the getZkL2ExecutionProofV1
// response minus the prover-attached proverVersion/proof/programVk fields.
// Parse() decodes it strictly, so any field the spec adds or renames fails here.
type jsonOutput struct {
	StartBlockNumber uint64           `json:"startBlockNumber"`
	PublicInputs     jsonPublicInputs `json:"publicInputs"`
}

type jsonPublicInputs struct {
	ParentBlockHash                       string   `json:"parentBlockHash"`
	EndBlockHash                          string   `json:"endBlockHash"`
	EndBlockNumber                        uint64   `json:"endBlockNumber"`
	EndBlockTimestamp                     uint64   `json:"endBlockTimestamp"`
	L2L1Messages                          []string `json:"l2L1Messages"`
	EndL1L2BridgeRollingHash              string   `json:"endL1L2BridgeRollingHash"`
	EndL1L2BridgeRollingHashMessageNumber uint64   `json:"endL1L2BridgeRollingHashMessageNumber"`
	DynamicChainConfigHash                string   `json:"dynamicChainConfigHash"`
	ParentFtxRollingHash                  string   `json:"parentFtxRollingHash"`
	ParentFtxNumber                       uint64   `json:"parentFtxNumber"`
	EndFtxRollingHash                     string   `json:"endFtxRollingHash"`
	EndProcessedFtxNumber                 uint64   `json:"endProcessedFtxNumber"`
	FilteredAddresses                     []string `json:"filteredAddresses"`
	TxFromsHash                           string   `json:"txFromsHash"`
	BlockCount                            uint64   `json:"blockCount"`
	L2MessagingBlocksOffsets              []uint16 `json:"l2MessagingBlocksOffsets"`
}

// Parse converts the runner's --json output into an Output.
func Parse(data []byte) (Output, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var j jsonOutput
	if err := dec.Decode(&j); err != nil {
		return Output{}, fmt.Errorf("parsing runner output: %w", err)
	}

	pi := j.PublicInputs
	var out Output
	out.StartBlockNumber = j.StartBlockNumber
	out.PublicInputs.EndBlockNumber = pi.EndBlockNumber
	out.PublicInputs.EndBlockTimestamp = pi.EndBlockTimestamp
	out.PublicInputs.EndL1L2BridgeRollingHashMessageNumber = pi.EndL1L2BridgeRollingHashMessageNumber
	out.PublicInputs.ParentFtxNumber = pi.ParentFtxNumber
	out.PublicInputs.EndProcessedFtxNumber = pi.EndProcessedFtxNumber

	hashes := []struct {
		src string
		dst *[32]byte
		key string
	}{
		{pi.ParentBlockHash, &out.PublicInputs.ParentBlockHash, "parentBlockHash"},
		{pi.EndBlockHash, &out.PublicInputs.EndBlockHash, "endBlockHash"},
		{pi.EndL1L2BridgeRollingHash, &out.PublicInputs.EndL1L2BridgeRollingHash, "endL1L2BridgeRollingHash"},
		{pi.DynamicChainConfigHash, &out.PublicInputs.DynamicChainConfigHash, "dynamicChainConfigHash"},
		{pi.ParentFtxRollingHash, &out.PublicInputs.ParentFtxRollingHash, "parentFtxRollingHash"},
		{pi.EndFtxRollingHash, &out.PublicInputs.EndFtxRollingHash, "endFtxRollingHash"},
		{pi.TxFromsHash, &out.PublicInputs.TxFromsHash, "txFromsHash"},
	}
	for _, h := range hashes {
		b, err := decodeFixed(h.src, 32, "publicInputs."+h.key)
		if err != nil {
			return Output{}, err
		}
		copy(h.dst[:], b)
	}

	out.PublicInputs.BlockCount = pi.BlockCount
	out.PublicInputs.L2MessagingBlocksOffsets = slices.Clone(pi.L2MessagingBlocksOffsets)
	if out.PublicInputs.L2MessagingBlocksOffsets == nil {
		out.PublicInputs.L2MessagingBlocksOffsets = []uint16{}
	}

	var err error
	if out.PublicInputs.L2L1Messages, err = decodeHash32List(pi.L2L1Messages, "l2L1Messages"); err != nil {
		return Output{}, err
	}
	if out.PublicInputs.FilteredAddresses, err = decodeAddressList(pi.FilteredAddresses, "filteredAddresses"); err != nil {
		return Output{}, err
	}
	return out, nil
}

func decodeFixed(s string, n int, ctx string) ([]byte, error) {
	if !strings.HasPrefix(s, "0x") {
		return nil, fmt.Errorf("%s: missing 0x prefix", ctx)
	}
	b, err := hex.DecodeString(s[2:])
	if err != nil {
		return nil, fmt.Errorf("%s: invalid hex: %w", ctx, err)
	}
	if len(b) != n {
		return nil, fmt.Errorf("%s: expected %d bytes, got %d", ctx, n, len(b))
	}
	return b, nil
}

func decodeHash32List(ss []string, ctx string) ([][32]byte, error) {
	out := make([][32]byte, len(ss))
	for i, s := range ss {
		b, err := decodeFixed(s, 32, fmt.Sprintf("%s[%d]", ctx, i))
		if err != nil {
			return nil, err
		}
		copy(out[i][:], b)
	}
	return out, nil
}

func decodeAddressList(ss []string, ctx string) ([][20]byte, error) {
	out := make([][20]byte, len(ss))
	for i, s := range ss {
		b, err := decodeFixed(s, 20, fmt.Sprintf("%s[%d]", ctx, i))
		if err != nil {
			return nil, err
		}
		copy(out[i][:], b)
	}
	return out, nil
}
