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
	require.ErrorContains(t, run([]string{cmdVerifierAssets, flagZig, out}), "--system is required")
	require.ErrorContains(t, run([]string{cmdVerifierAssets, flagSystem, "devmode"}), "--zig is required")
	require.ErrorContains(t,
		run([]string{cmdVerifierAssets, flagSystem, "nope", flagZig, out}), `unknown system "nope"`)
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
	require.NoError(t, runVerifierAssets(args, &log))

	system, err := os.ReadFile(zigPath)
	require.NoError(t, err)
	assert.Contains(t, string(system), "pub const system_0_systems = verifier.Systems{")

	image, err := os.ReadFile(imagePath)
	require.NoError(t, err)
	input, err := proofserialization.Decode(image, proofserialization.GuestBase)
	require.NoError(t, err)
	assert.Len(t, input.PublicInputs, devmode.NumPublicInputs)
	assert.Equal(t, 2, strings.Count(log.String(), "wrote "))
}

func TestRunDevmodeProve(t *testing.T) {
	hash := strings.Repeat("01", devmodeHashLen)
	vk := strings.Repeat("2a", devmodeVKLen)
	in := strings.NewReader(hash + " " + vk + "\n" + hash + " " + vk + "\n")
	var out bytes.Buffer
	require.NoError(t, runDevmodeProve(nil, in, &out))

	for range 2 {
		var length [4]byte
		_, err := io.ReadFull(&out, length[:])
		require.NoError(t, err)
		image := make([]byte, binary.BigEndian.Uint32(length[:]))
		_, err = io.ReadFull(&out, image)
		require.NoError(t, err)
		input, err := proofserialization.Decode(image, devmode.GuestBase)
		require.NoError(t, err)
		require.Len(t, input.PublicInputs, devmode.NumPublicInputs)
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
			require.ErrorContains(t, err, "request")
		})
	}
	require.ErrorContains(t, runDevmodeProve([]string{"extra"}, strings.NewReader(""), io.Discard),
		"unexpected arguments")
}
