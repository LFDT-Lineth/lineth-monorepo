package test_utils

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	typesLinea "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

// CheckSameTx checks if the most essential fields in two transactions are equal
// TODO cover type-specific fields
func CheckSameTx(t *testing.T, orig, decoded *types.Transaction, decodedFrom common.Address) {
	assert.Equal(t, orig.Type(), decoded.Type())
	assert.Equal(t, orig.To(), decoded.To())
	assert.Equal(t, orig.Nonce(), decoded.Nonce())
	assert.Equal(t, orig.Data(), decoded.Data())
	assert.Equal(t, orig.Value(), decoded.Value())
	assert.Equal(t, orig.Cost(), decoded.Cost())
	assert.Equal(t, ethereum.GetFrom(orig), typesLinea.EthAddress(decodedFrom))
}
