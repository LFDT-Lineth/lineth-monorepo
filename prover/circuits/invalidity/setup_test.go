package invalidity_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	keccakDummy "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/config"
	smt_koalabear "github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	invalidityPI "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/invalidity_pi"
	"github.com/stretchr/testify/require"
)

// TestSetupRoundTripNonceBalance verifies that the invalidity-nonce-balance
// circuit can be compiled, set up, written to disk, and read back with
// matching checksums. This catches bugs in the setup/keygen pipeline without
// the cost of generating a real proof.
func TestSetupRoundTripNonceBalance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping setup round-trip test in short mode")
	}

	const maxRlpByteSize = 1023

	keccakComp := invalidity.MakeKeccakCompiledIOP(maxRlpByteSize, keccakDummy.Compile)
	builder := invalidity.NewBuilder(
		invalidity.Config{
			Depth:             smt_koalabear.DefaultDepth,
			KeccakCompiledIOP: keccakComp,
			MaxRlpByteSize:    maxRlpByteSize,
		},
		&invalidity.BadNonceBalanceCircuit{},
	)

	verifySetupRoundTrip(t, builder, circuits.InvalidityNonceBalanceCircuitID)
}

// TestSetupRoundTripPrecompileLogs verifies the same round-trip for the
// invalidity-precompile-logs circuit, using a lightweight mock zkEvm that
// provides only the public input columns needed by BadPrecompileCircuit.
func TestSetupRoundTripPrecompileLogs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping setup round-trip test in short mode")
	}

	comp, _ := invalidity.MockZkEvmPI(rng, invalidityPI.Inputs{
		FixedInputs: fixedInputs,
		CaseInputs: invalidityPI.CaseInputs{
			HasBadPrecompile: true,
			NumL2Logs:        5,
		},
	}, nil)

	builder := invalidity.NewBuilder(
		invalidity.Config{
			ZkEvmComp: comp,
		},
		&invalidity.BadPrecompileCircuit{},
	)

	verifySetupRoundTrip(t, builder, circuits.InvalidityPrecompileLogsCircuitID)
}

// TestSetupRoundTripFilteredAddress verifies the same round-trip for the
// invalidity-filtered-address circuit.
func TestSetupRoundTripFilteredAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping setup round-trip test in short mode")
	}

	const maxRlpByteSize = 1023

	keccakComp := invalidity.MakeKeccakCompiledIOP(maxRlpByteSize, keccakDummy.Compile)
	builder := invalidity.NewBuilder(
		invalidity.Config{
			KeccakCompiledIOP: keccakComp,
			MaxRlpByteSize:    maxRlpByteSize,
		},
		&invalidity.FilteredAddressCircuit{},
	)

	verifySetupRoundTrip(t, builder, circuits.InvalidityFilteredAddressCircuitID)
}

// verifySetupRoundTrip compiles a circuit via the builder, generates a setup
// with an unsafe SRS, writes the assets to a temp directory, then reads
// back the manifest and verifying key to check that checksums are consistent.
func verifySetupRoundTrip(t *testing.T, builder circuits.Builder, circuitID circuits.CircuitID) {
	t.Helper()

	t.Logf("Compiling circuit %s...", circuitID)
	ccs, err := builder.Compile()
	require.NoError(t, err)
	t.Logf("Compiled: %d constraints", ccs.GetNbConstraints())

	t.Log("Running MakeSetup with unsafe SRS...")
	srsProvider := circuits.NewUnsafeSRSProvider()
	setup, err := circuits.MakeSetup(
		context.Background(),
		circuitID,
		ccs,
		srsProvider,
		map[string]any{},
	)
	require.NoError(t, err)

	vkDigest := setup.VerifyingKeyDigest()
	require.NotEmpty(t, vkDigest)
	t.Logf("VK digest: %s", vkDigest)

	dir := t.TempDir()
	setupPath := filepath.Join(dir, string(circuitID))
	t.Logf("Writing setup to %s", setupPath)
	require.NoError(t, setup.WriteTo(setupPath))

	manifestPath := filepath.Join(setupPath, config.ManifestFileName)
	manifest, err := circuits.ReadSetupManifest(manifestPath)
	require.NoError(t, err)

	require.Equal(t, string(circuitID), manifest.CircuitName)
	require.Equal(t, ecc.BLS12_377.String(), manifest.CurveID)
	require.Equal(t, ccs.GetNbConstraints(), manifest.NbConstraints)

	vkPath := filepath.Join(setupPath, config.VerifyingKeyFileName)
	vk := plonk.NewVerifyingKey(ecc.BLS12_377)
	require.NoError(t, circuits.ReadVerifyingKey(vkPath, vk))

	require.Equal(t, setup.Manifest.Checksums.VerifyingKey, manifest.Checksums.VerifyingKey,
		"manifest VK checksum mismatch after round-trip")

	circuitDigest, err := circuits.CircuitDigest(ccs)
	require.NoError(t, err)
	require.Equal(t, circuitDigest, manifest.Checksums.Circuit,
		"manifest circuit checksum mismatch")

	t.Logf("Round-trip passed for %s", circuitID)
}
