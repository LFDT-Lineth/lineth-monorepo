package jobadapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/ssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file run for both modes that run the guest on the extended
// input: dev-zkvm and dev-partial.

// guestOutput stands in for the guest output. The runner does not read it; it
// only compares it byte for byte with the native runner's output.
var guestOutput = []byte("guest output")

// extendedModes maps each mode to the proof bytes its prover returns.
var extendedModes = map[backend.ProverMode][]byte{
	backend.ProverModeDevZkVM:    backend.DevMarkerProof(backend.ProverModeDevZkVM),
	backend.ProverModeDevPartial: []byte("devmode proof"),
}

// guestPublicInputs are the public inputs of a job of blockCount blocks ending
// at endBlock.
func guestPublicInputs(endBlock, blockCount uint64) backend.PublicInputs {
	return backend.PublicInputs{EndBlockNumber: endBlock, BlockCount: blockCount, ParentBlockHash: [32]byte{0xee}}
}

// extendedProver returns what the mode's prover returns: the proof, the public
// inputs decoded from the guest output, changed by edit if set, and that output.
func extendedProver(mode backend.ProverMode, output []byte, edit func(*backend.PublicInputs)) *mockProver {
	return &mockProver{result: func(job backend.Job) backend.Result {
		pi := guestPublicInputs(job.EndBlock, job.EndBlock-job.StartBlock+1)
		if edit != nil {
			edit(&pi)
		}
		return backend.Result{
			JobID:        job.ID,
			Status:       backend.ResultStatusOK,
			ProofBytes:   extendedModes[mode],
			PublicInputs: pi,
			GuestOutput:  output,
		}
	}}
}

// fakeNativeRunner builds a native runner stub that answers --ssz with the
// given raw bytes and fails on any other call.
func fakeNativeRunner(t *testing.T, sszBytes []byte) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell runner is POSIX-only")
	}
	var octal strings.Builder
	for _, b := range sszBytes {
		fmt.Fprintf(&octal, "\\%03o", b)
	}
	script := "#!/bin/sh\n" +
		"if [ \"$2\" = \"--ssz\" ]; then\n" +
		"  printf '" + octal.String() + "'\n" +
		"else\n" +
		"  echo \"unexpected call: $*\" >&2\n" +
		"  exit 1\n" +
		"fi\n"
	bin := filepath.Join(t.TempDir(), "fake-runner")
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o700)) //nolint:gosec // G306: test stub must be executable
	return bin
}

// The response comes from the prover's result, and the guest runs on the same
// extended input as the native runner. In the multi-block request the start
// and end blocks differ.
func TestRunner_Extended_ResponseFromResult(t *testing.T) {
	for _, fixture := range []string{"request_single_block.json", "request_multi_block.json"} {
		body := readFixture(t, fixture)
		req, err := DecodeL2ExecutionRequest(body)
		require.NoError(t, err)
		startBlock := req.Payloads[0].BlockNumber
		endBlock := req.Payloads[len(req.Payloads)-1].BlockNumber
		for mode, proof := range extendedModes {
			t.Run(fixture+"/"+string(mode), func(t *testing.T) {
				mock := extendedProver(mode, guestOutput, nil)
				runner, err := NewRunner(mock, testProverVersion,
					WithMode(mode), WithNativeRunnerBin(fakeNativeRunner(t, guestOutput)))
				require.NoError(t, err)

				result := runner.Run(context.Background(), l2ExecutionRunRequest("e-1", body))

				require.Equal(t, RunStatusSuccess, result.Status, "run err: %v", result.Err)
				require.Len(t, mock.jobs, 1)
				assert.Equal(t, ssz.EncodeExtendedInput(buildExtendedInput(req)), mock.jobs[0].Payload,
					"the guest should run on the extended input")
				assert.Equal(t, requestGuestProgramID(t, body), mock.jobs[0].GuestProgramID)

				resp, ok := result.ResponseBody.(executionResponse)
				require.True(t, ok)
				assert.Equal(t, testProverVersion+"-"+string(mode), resp.ProverVersion)
				assert.Equal(t, hexBytes(proof), resp.ProofHex)
				assert.Equal(t, startBlock, resp.StartBlockNumber, "the start block should be the request's first block")
				assert.Equal(t, endBlock, resp.PublicInputs.EndBlockNumber)
				assert.Equal(t, uint64(len(req.Payloads)), resp.PublicInputs.BlockCount)
				assert.Equal(t, hexHash([32]byte{0xee}), resp.PublicInputs.ParentBlockHash,
					"the public inputs should come from the prover's result")
				assert.Equal(t, guestProgramID, resp.ProgramVk)
			})
		}
	}
}

// A guest output that differs from the native runner's is refused.
func TestRunner_Extended_NativeCheckFails(t *testing.T) {
	wrong := append([]byte(nil), guestOutput...)
	wrong[2] ^= 0xff
	for mode := range extendedModes {
		t.Run(string(mode), func(t *testing.T) {
			runner, err := NewRunner(extendedProver(mode, wrong, nil), testProverVersion,
				WithMode(mode), WithNativeRunnerBin(fakeNativeRunner(t, guestOutput)))
			require.NoError(t, err)

			result := runner.Run(context.Background(),
				l2ExecutionRunRequest("e-2", readFixture(t, "request_single_block.json")))

			assert.Equal(t, RunStatusFailed, result.Status)
			assert.ErrorContains(t, result.Err, "native runner check failed")
		})
	}
}

// The guest's end block and block count must match the request.
func TestRunner_Extended_PublicInputsMatchRequest(t *testing.T) {
	req, err := DecodeL2ExecutionRequest(readFixture(t, "request_single_block.json"))
	require.NoError(t, err)
	end := req.Payloads[0].BlockNumber
	cases := map[string]struct {
		edit    func(*backend.PublicInputs)
		wantErr string
	}{
		"end block": {
			func(pi *backend.PublicInputs) { pi.EndBlockNumber++ },
			fmt.Sprintf("end block %d, block count 1, want %d and 1", end+1, end),
		},
		"block count": {
			func(pi *backend.PublicInputs) { pi.BlockCount = 2 },
			fmt.Sprintf("end block %d, block count 2, want %d and 1", end, end),
		},
	}
	for mode := range extendedModes {
		for name, tc := range cases {
			t.Run(string(mode)+"/"+name, func(t *testing.T) {
				runner, err := NewRunner(extendedProver(mode, guestOutput, tc.edit), testProverVersion,
					WithMode(mode), WithNativeRunnerBin(fakeNativeRunner(t, guestOutput)))
				require.NoError(t, err)

				result := runner.Run(context.Background(),
					l2ExecutionRunRequest("e-3", readFixture(t, "request_single_block.json")))

				assert.Equal(t, RunStatusFailed, result.Status)
				require.ErrorContains(t, result.Err, "guest public inputs do not match the request")
				assert.ErrorContains(t, result.Err, tc.wantErr)
			})
		}
	}
}

// A native runner that cannot run fails the job before the guest runs.
func TestRunner_Extended_NativeRunnerFailsFirst(t *testing.T) {
	for mode := range extendedModes {
		t.Run(string(mode), func(t *testing.T) {
			mock := extendedProver(mode, guestOutput, nil)
			runner, err := NewRunner(mock, testProverVersion, WithMode(mode),
				WithNativeRunnerBin(filepath.Join(t.TempDir(), "missing-runner")))
			require.NoError(t, err)

			result := runner.Run(context.Background(),
				l2ExecutionRunRequest("e-4", readFixture(t, "request_single_block.json")))

			assert.Equal(t, RunStatusFailed, result.Status)
			require.ErrorContains(t, result.Err, "running native runner")
			assert.Empty(t, mock.jobs, "the guest should not run")
		})
	}
}
