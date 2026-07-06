package arithmetization

import (
	"strings"
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

// TestAllTraceLimitsAreUsed verifies that every module limit entry defined in
// the config is actually reachable by at least one module in the corset schema.
// This catches stale/dead limit entries that should be renamed or removed.
//
// Some limit entries are consumed via typed accessors (precompiles, block
// metadata, reference tables) rather than corset module name matching — those
// are reported as warnings, not failures.
func TestAllTraceLimitsAreUsed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping heavy test in short mode")
	}

	binf, _, errBin := ReadZkevmBin()
	require.NoError(t, errBin)
	schema, _ := CompileZkevmBin(binf, &mir.DEFAULT_OPTIMISATION_LEVEL)

	limits := config.GetTestTracesLimits()

	// Collect all module names from the corset schema (lowercased).
	modules := schema.Modules().Collect()
	moduleNames := make([]string, 0, len(modules))
	for _, mod := range modules {
		name := strings.ToLower(mod.Name().String())
		if name != "" {
			moduleNames = append(moduleNames, name)
		}
	}

	// Entries consumed via typed Go accessors, not corset module matching.
	typedAccessorPrefixes := []string{
		"precompile_", "block_", "shomei_",
		"bin_reference_table", "shf_reference_table", "instruction_decoder",
		"u20", "u32", "u36", "u64", "u128",
	}

	isTypedAccessor := func(name string) bool {
		for _, prefix := range typedAccessorPrefixes {
			if strings.HasPrefix(name, prefix) || name == prefix {
				return true
			}
		}
		return false
	}

	// For each limit entry, check if at least one schema module matches it.
	for _, ml := range limits.Modules {
		if ml.Module == "" {
			continue // default fallback
		}
		if isTypedAccessor(ml.Module) {
			continue // accessed via typed methods, not corset prefix match
		}
		matched := false
		for _, modName := range moduleNames {
			if strings.HasPrefix(modName, ml.Module) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("trace limit %q (limit=%d) has no matching corset module — rename or remove it", ml.Module, ml.Limit)
		}
	}
}
