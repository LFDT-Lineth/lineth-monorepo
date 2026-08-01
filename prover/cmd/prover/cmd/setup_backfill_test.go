package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/test/unsafekzg"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/stretchr/testify/require"
)

// TestUpdateSetupBackfillsMissingLagrangeDump pins the persist hook's placement
// and gate in updateSetup: the hook runs before the skip-if-already-setup
// check, so a setup re-run against current assets still backfills a missing
// derived dump, and persist_derived_srs = false keeps setup from writing at
// all. Nothing else exercises updateSetup, so a regression here only surfaces
// as an hours-long re-derivation at prover start.
func TestUpdateSetupBackfillsMissingLagrangeDump(t *testing.T) {
	assert := require.New(t)

	builder := dummy.NewBuilder(circuits.MockCircuitIDEmulation, ecc.BN254.ScalarField())
	ccs, err := builder.Compile()
	assert.NoError(err)
	canonicalSize, lagrangeSize := plonk.SRSSize(ccs)

	cfg := &config.Config{AssetsDir: t.TempDir(), Version: "0.0.1", PersistDerivedSRS: true}
	srsDir := cfg.PathForSRS()
	assert.NoError(os.MkdirAll(srsDir, 0o700))

	canonical, _, err := unsafekzg.NewSRS(ccs)
	assert.NoError(err)
	f, err := os.Create(filepath.Join(srsDir, fmt.Sprintf("kzg_srs_canonical_%d_bn254_aleo.memdump", canonicalSize)))
	assert.NoError(err)
	assert.NoError(canonical.WriteDump(f))
	assert.NoError(f.Close())

	derived := filepath.Join(srsDir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_derived.memdump", lagrangeSize))
	newStore := func() *circuits.SRSStore {
		s, err := circuits.NewSRSStore(srsDir)
		assert.NoError(err)
		return s
	}
	const circuitID = circuits.CircuitID("backfill-test")

	// a fresh setup publishes the assets and the derived dump
	assert.NoError(updateSetup(context.TODO(), cfg, false, newStore(), circuitID, builder, nil))
	assert.FileExists(derived, "a fresh setup must persist the derived dump")

	// the headline behaviour: assets current (the skip path), dump missing — a
	// fresh setup run must still backfill it
	assert.NoError(os.Remove(derived))
	assert.NoError(updateSetup(context.TODO(), cfg, false, newStore(), circuitID, builder, nil))
	assert.FileExists(derived, "setup against current assets must backfill a missing dump")

	// and the gate: opting out keeps setup from writing
	assert.NoError(os.Remove(derived))
	cfg.PersistDerivedSRS = false
	assert.NoError(updateSetup(context.TODO(), cfg, false, newStore(), circuitID, builder, nil))
	_, err = os.Stat(derived)
	assert.True(os.IsNotExist(err), "persist_derived_srs = false must keep setup from writing")
}
