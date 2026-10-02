package backend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_DevMock_NeedsNoArtifacts(t *testing.T) {
	// dev-mock must construct with neither a circuit bin nor a guest ELF.
	c, err := New(Config{Mode: ProverModeDevMock})
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, ProverModeDevMock, c.mode)
	assert.Nil(t, c.driver, "dev-mock builds no driver")
}

func TestNew_InvalidMode(t *testing.T) {
	_, err := New(Config{Mode: "bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid prover mode")
}

func TestNew_EmptyModeDefaultsToFull(t *testing.T) {
	// Empty mode resolves to full, which fails opening the empty circuit bin path.
	_, err := New(Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circuit bin")
}

func TestProve_DevMock_ReturnsPlaceholder(t *testing.T) {
	c, err := New(Config{Mode: ProverModeDevMock})
	require.NoError(t, err)

	result := c.Prove(context.Background(), Job{ID: "job-1", StartBlock: 10, EndBlock: 14})
	assert.Equal(t, ResultStatusOK, result.Status)
	assert.Equal(t, "job-1", result.JobID)
	require.NoError(t, result.Err)
	assert.Equal(t, []byte("dev-proof:dev-mock"), result.ProofBytes)
	assert.Equal(t, PublicInputs{}, result.PublicInputs, "public inputs stay zero placeholders")
}

func TestProve_UnwiredModes_ReturnBlockerError(t *testing.T) {
	// dev-zkvm's Core.Prove runs the guest under Execute, so it is not here.
	for _, mode := range []ProverMode{ProverModePartial} {
		c := &Core{mode: mode}
		result := c.Prove(context.Background(), Job{ID: "job"})
		assert.Equal(t, ResultStatusFailed, result.Status, mode)
		require.Error(t, result.Err, mode)
		assert.ErrorIs(t, result.Err, ErrNotImplemented, mode)
	}
}

func TestProverMode_Helpers(t *testing.T) {
	assert.True(t, ProverModeDevMock.IsDev())
	assert.True(t, ProverModeDevZkVM.IsDev())
	assert.False(t, ProverModeFull.IsDev())
	assert.False(t, ProverModePartial.IsDev())

	assert.False(t, ProverModeDevMock.needsArtifacts())
	assert.True(t, ProverModeFull.needsArtifacts())

	assert.True(t, ProverModeFull.Valid())
	assert.False(t, ProverMode("nope").Valid())
}
