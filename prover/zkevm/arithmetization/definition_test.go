package arithmetization

import (
	"testing"

	"github.com/consensys/go-corset/pkg/ir/mir"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/stretchr/testify/require"
)

func TestDefine(t *testing.T) {

	var (
		comp = &wizard.CompiledIOP{
			Columns:         column.NewStore(),
			QueriesParams:   wizard.NewRegister[ifaces.QueryID, ifaces.Query](),
			QueriesNoParams: wizard.NewRegister[ifaces.QueryID, ifaces.Query](),
			Coins:           wizard.NewRegister[coin.Name, coin.Info](),
			Precomputed:     collection.NewMapping[ifaces.ColID, ifaces.ColAssignment](),
		}
		binf, _, errBin = ReadZkevmBin()
		limits          = config.GetTestTracesLimits()
	)
	// Compile binary file into an air.Schema
	schema, _ := CompileZkevmBin(binf, &mir.DEFAULT_OPTIMISATION_LEVEL)

	require.NoError(t, errBin)
	Define(comp, schema, limits)
}
