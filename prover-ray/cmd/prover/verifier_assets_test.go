package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	cmdVerifierAssets = "verifier-assets"
	flagSystem        = "--system"
	flagZig           = "--zig"
)

func TestRunVerifierAssets_RequiresFlags(t *testing.T) {
	out := filepath.Join(t.TempDir(), "system.zig")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no system", []string{flagZig, out}, "--system is required"},
		{"no zig", []string{flagSystem, "devmode"}, "--zig is required"},
		{"unknown system", []string{flagSystem, "nope", flagZig, out}, `unknown system "nope"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(append([]string{cmdVerifierAssets}, tc.args...))
			require.ErrorContains(t, err, tc.want, "verifier-assets must reject %v", tc.args)
		})
	}
}

// TestRunVerifierAssets_Devmode writes the devmode assets. It needs `zig` (or ZIG) to format the
// system, as the verifier-ray and rollup builds that call the command do.
func TestRunVerifierAssets_Devmode(t *testing.T) {
	if os.Getenv("ZIG") == "" {
		if _, err := exec.LookPath("zig"); err != nil {
			t.Skip("zig not on PATH")
		}
	}
	dir := t.TempDir()
	zigPath := filepath.Join(dir, "generated", "devmode_system.zig")
	imagePath := filepath.Join(dir, "devmode_proof_image.bin")

	var log bytes.Buffer
	args := []string{flagSystem, "devmode", flagZig, zigPath, "--image", imagePath}
	require.NoError(t, runVerifierAssets(args, &log), "generating the devmode assets")

	system, err := os.ReadFile(zigPath)
	require.NoError(t, err, "reading the generated system")
	assert.Contains(t, string(system), "pub const system_0_systems = verifier.Systems{",
		"the generated system must declare its verifier.Systems")

	image, err := os.ReadFile(imagePath)
	require.NoError(t, err, "reading the generated proof image")
	input, err := proofserialization.Decode(image, proofserialization.GuestBase)
	require.NoError(t, err, "the proof image must decode at GuestBase")
	assert.Len(t, input.PublicInputs, devmode.NumPublicInputs, "public inputs of the devmode proof")
	assert.Equal(t, 2, strings.Count(log.String(), "wrote "), "the log must report both written files")
}

func TestRunDevmodeProve(t *testing.T) {
	hash := strings.Repeat("01", devmodeHashLen)
	vk := strings.Repeat("2a", devmodeVKLen)
	in := strings.NewReader(hash + " " + vk + "\n" + hash + " " + vk + "\n")
	var out bytes.Buffer
	require.NoError(t, runDevmodeProve(nil, in, &out), "serving two well-formed requests")

	for i := range 2 {
		var length [4]byte
		_, err := io.ReadFull(&out, length[:])
		require.NoError(t, err, "reading the length prefix of proof %d", i)
		image := make([]byte, binary.BigEndian.Uint32(length[:]))
		_, err = io.ReadFull(&out, image)
		require.NoError(t, err, "reading the image of proof %d", i)
		input, err := proofserialization.Decode(image, devmode.GuestBase)
		require.NoError(t, err, "proof %d must decode at GuestBase", i)
		require.Len(t, input.PublicInputs, devmode.NumPublicInputs, "public inputs of proof %d", i)
	}
	assert.Zero(t, out.Len(), "trailing bytes after the two proofs")
}

func TestRunDevmodeProve_RejectsMalformedRequests(t *testing.T) {
	hash := strings.Repeat("01", devmodeHashLen)
	for name, line := range map[string]string{
		"one field":     hash,
		"short hash":    "01 " + strings.Repeat("2a", devmodeVKLen),
		"non-hex VK":    hash + " " + strings.Repeat("zz", devmodeVKLen),
		"VK limb >= p":  hash + " " + strings.Repeat("ff", devmodeVKLen),
		"trailing args": hash + " " + strings.Repeat("2a", devmodeVKLen) + " x",
	} {
		t.Run(name, func(t *testing.T) {
			err := runDevmodeProve(nil, strings.NewReader(line+"\n"), io.Discard)
			require.ErrorContains(t, err, "request", "a malformed request must be rejected")
		})
	}
	require.ErrorContains(t, runDevmodeProve([]string{"extra"}, strings.NewReader(""), io.Discard),
		"unexpected arguments", "devmode-prove takes no arguments")
}
