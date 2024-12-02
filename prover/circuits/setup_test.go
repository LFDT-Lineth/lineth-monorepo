package circuits

import (
	"context"
	"errors"
	"fmt"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSetup(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		AssetsDir: dir,
	}
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &circuit{make([]frontend.Variable, 1)})
	require.NoError(t, err)

	canonicalSize, lagrangeSize := plonk.SRSSize(cs)
	canonical, lagrange, err := unsafekzg.NewSRS(cs)
	require.NoError(t, err)
	const srsFilenameTemplate = "kzg_srs_%s_%d_bn254_aleo.memdump"
	srsDir := filepath.Join(dir, "kzgsrs")
	require.NoError(t, os.Mkdir(srsDir, 0700))
	dumpToFile(t, canonical, filepath.Join(srsDir, fmt.Sprintf(srsFilenameTemplate, "canonical", canonicalSize)))
	dumpToFile(t, lagrange, filepath.Join(srsDir, fmt.Sprintf(srsFilenameTemplate, "lagrange", lagrangeSize)))

	const circuitName = "test"

	srsProvider := NewUnsafeSRSProvider()
	setup, err := MakeSetup(context.TODO(), circuitName, cs, srsProvider, map[string]any{})
	require.NoError(t, err)

	require.NoError(t, setup.WriteTo(filepath.Join(dir, circuitName)))

	_, err = LoadSetup(&cfg, circuitName)
	require.NoError(t, err)
}

type circuit struct {
	// this is a dummy circuit that does nothing
	// it is used to generate the SRS
	Input []frontend.Variable `gnark:",public"`
}

func (circuit *circuit) Define(api frontend.API) error {
	c, err := api.(frontend.Committer).Commit(circuit.Input...)
	if err != nil {
		return err
	}
	prod := frontend.Variable(1)
	for _, x := range circuit.Input {
		prod = api.Mul(prod, x)
	}
	api.AssertIsDifferent(c, prod)
	return nil
}

func dumpToFile(t *testing.T, o kzg.Serializable, path string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	require.NoError(t, err)

	err = errors.Join(o.WriteDump(f), f.Close())
	require.NoError(t, err)
}
