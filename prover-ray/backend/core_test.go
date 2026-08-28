package backend

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildInputs_MultiBlock_NotImplemented rejects multi-block jobs.
func TestBuildInputs_MultiBlock_NotImplemented(t *testing.T) {
	c := &Core{cfg: Config{}}
	_, err := c.buildInputs(Job{StartBlock: 1, EndBlock: 2, Type: ProofTypeL2Execution})
	require.ErrorIs(t, err, ErrNotImplemented)
}

// TestBuildInputs_InvertedRange rejects malformed block ranges.
func TestBuildInputs_InvertedRange(t *testing.T) {
	c := &Core{cfg: Config{}}
	_, err := c.buildInputs(Job{StartBlock: 5, EndBlock: 3, Type: ProofTypeL2Execution})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNotImplemented)
	require.Contains(t, err.Error(), "invalid block range")
}
