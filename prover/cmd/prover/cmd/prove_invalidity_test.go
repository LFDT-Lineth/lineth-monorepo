package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestProveInvalidityFromRequestFile runs the full Prove flow using an
// invalidity request JSON file (dev-mode: generates mock proof, no real setup).
func TestProveInvalidityFromRequestFile(t *testing.T) {
	requestFile := filepath.Join("..", "..", "..", "backend", "invalidity", "testdata", "5-1-getZkInvalidityProof-v2.json")
	configFile := filepath.Join("..", "..", "..", "config", "config-devnet-full.toml")

	_, err := os.Stat(requestFile)
	require.NoError(t, err, "request file must exist: %s", requestFile)

	_, err = os.Stat(configFile)
	require.NoError(t, err, "config file must exist: %s", configFile)

	// Override assets_dir so the config can find the SRS from the test's CWD
	viper.Set("assets_dir", filepath.Join("..", "..", "..", "prover-assets"))

	outputFile := filepath.Join(t.TempDir(), "invalidity-response.json")

	args := ProverArgs{
		Input:      requestFile,
		Output:     outputFile,
		ConfigFile: configFile,
	}

	err = Prove(args)
	require.NoError(t, err, "Prove should succeed for invalidity request")

	info, err := os.Stat(outputFile)
	require.NoError(t, err, "output file should exist")
	require.Greater(t, info.Size(), int64(0), "output file should not be empty")

	t.Logf("Prove completed successfully. Output written to %s (%d bytes)", outputFile, info.Size())
}
