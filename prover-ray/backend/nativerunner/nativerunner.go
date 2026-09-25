// Package nativerunner runs the native l2-execution-runner on an extended
// (0x0002) input and parses its --json output into the guest's real public
// inputs and revealed preimage arrays.
package nativerunner

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
)

// Output is the parsed native-runner result for one l2-execution request.
type Output struct {
	StartBlockNumber  uint64
	PublicInputs      backend.PublicInputs
	L2L1Messages      [][32]byte
	TxFroms           [][20]byte
	FilteredAddresses [][20]byte
}

// Run writes the extended input to a temp file, runs `<binPath> <file> --json`,
// and parses stdout. binPath is the native l2-execution-runner binary.
func Run(ctx context.Context, binPath string, extendedInput []byte) (Output, error) {
	out, err := run(ctx, binPath, extendedInput, "--json")
	if err != nil {
		return Output{}, err
	}
	return Parse(out)
}

// RunSSZ runs `<binPath> <file> --ssz` and returns the raw 0x0003 wire output
// (the 2-byte schema id followed by keccak256(SSZ(public inputs))). It is
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

// jsonOutput mirrors the runner's --json shape: getZkL2ExecutionProofV1.response
// minus the prover-attached proverVersion/proof/programVk fields.
type jsonOutput struct {
	StartBlockNumber  uint64           `json:"startBlockNumber"`
	PublicInputs      jsonPublicInputs `json:"publicInputs"`
	L2L1Messages      []string         `json:"l2L1Messages"`
	TxFroms           []string         `json:"txFroms"`
	FilteredAddresses []string         `json:"filteredAddresses"`
}

type jsonPublicInputs struct {
	ParentBlockHash                          string `json:"parentBlockHash"`
	EndBlockHash                             string `json:"endBlockHash"`
	EndBlockNumber                           uint64 `json:"endBlockNumber"`
	EndBlockTimestamp                        uint64 `json:"endBlockTimestamp"`
	L2L1MessagesHash                         string `json:"l2L1MessagesHash"`
	ParentL1L2BridgeRollingHash              string `json:"parentL1L2BridgeRollingHash"`
	ParentL1L2BridgeRollingHashMessageNumber uint64 `json:"parentL1L2BridgeRollingHashMessageNumber"`
	EndL1L2BridgeRollingHash                 string `json:"endL1L2BridgeRollingHash"`
	EndL1L2BridgeRollingHashMessageNumber    uint64 `json:"endL1L2BridgeRollingHashMessageNumber"`
	DynamicChainConfigHash                   string `json:"dynamicChainConfigHash"`
	ParentFtxRollingHash                     string `json:"parentFtxRollingHash"`
	ParentFtxNumber                          uint64 `json:"parentFtxNumber"`
	EndFtxRollingHash                        string `json:"endFtxRollingHash"`
	EndProcessedFtxNumber                    uint64 `json:"endProcessedFtxNumber"`
	FilteredAddressesHash                    string `json:"filteredAddressesHash"`
	TxFromsHash                              string `json:"txFromsHash"`
}

// Parse converts the runner's --json output into an Output.
func Parse(data []byte) (Output, error) {
	var j jsonOutput
	if err := json.Unmarshal(data, &j); err != nil {
		return Output{}, fmt.Errorf("parsing runner output: %w", err)
	}

	pi := j.PublicInputs
	var out Output
	out.StartBlockNumber = j.StartBlockNumber
	out.PublicInputs.EndBlockNumber = pi.EndBlockNumber
	out.PublicInputs.EndBlockTimestamp = pi.EndBlockTimestamp
	out.PublicInputs.ParentL1L2BridgeRollingHashMessageNumber = pi.ParentL1L2BridgeRollingHashMessageNumber
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
		{pi.L2L1MessagesHash, &out.PublicInputs.L2L1MessagesHash, "l2L1MessagesHash"},
		{pi.ParentL1L2BridgeRollingHash, &out.PublicInputs.ParentL1L2BridgeRollingHash, "parentL1L2BridgeRollingHash"},
		{pi.EndL1L2BridgeRollingHash, &out.PublicInputs.EndL1L2BridgeRollingHash, "endL1L2BridgeRollingHash"},
		{pi.DynamicChainConfigHash, &out.PublicInputs.DynamicChainConfigHash, "dynamicChainConfigHash"},
		{pi.ParentFtxRollingHash, &out.PublicInputs.ParentFtxRollingHash, "parentFtxRollingHash"},
		{pi.EndFtxRollingHash, &out.PublicInputs.EndFtxRollingHash, "endFtxRollingHash"},
		{pi.FilteredAddressesHash, &out.PublicInputs.FilteredAddressesHash, "filteredAddressesHash"},
		{pi.TxFromsHash, &out.PublicInputs.TxFromsHash, "txFromsHash"},
	}
	for _, h := range hashes {
		b, err := decodeFixed(h.src, 32, "publicInputs."+h.key)
		if err != nil {
			return Output{}, err
		}
		copy(h.dst[:], b)
	}

	var err error
	if out.L2L1Messages, err = decodeHash32List(j.L2L1Messages, "l2L1Messages"); err != nil {
		return Output{}, err
	}
	if out.TxFroms, err = decodeAddressList(j.TxFroms, "txFroms"); err != nil {
		return Output{}, err
	}
	if out.FilteredAddresses, err = decodeAddressList(j.FilteredAddresses, "filteredAddresses"); err != nil {
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
