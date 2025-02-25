package arithmetization

import (
	"reflect"
	"testing"

	"github.com/consensys/go-corset/pkg/mir"
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
		schema, _, errBin = ReadZkevmBin(&mir.DEFAULT_OPTIMISATION_LEVEL)
		limits            = &config.TracesLimits{}
		limitRefl         = reflect.ValueOf(limits).Elem()
	)

	for i := 0; i < limitRefl.NumField(); i++ {
		limitRefl.Field(i).SetInt(1 << 10)
	}

	require.NoError(t, errBin)
	Define(comp, schema, limits)
}
