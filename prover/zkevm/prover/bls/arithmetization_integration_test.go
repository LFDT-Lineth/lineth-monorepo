package bls

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/consensys/go-corset/pkg/asm"
	"github.com/consensys/go-corset/pkg/binfile"
	"github.com/consensys/go-corset/pkg/ir"
	"github.com/consensys/go-corset/pkg/ir/air"
	"github.com/consensys/go-corset/pkg/ir/mir"
	"github.com/consensys/go-corset/pkg/schema/module"
	"github.com/consensys/go-corset/pkg/trace"
	"github.com/consensys/go-corset/pkg/util/field/bls12_377"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/plonkinwizard"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
)

// integrationTestCompiler is the compiler used for integration tests
// it can be switched to dummy.Compile for faster tests that do not check
// the actual validity of the proof. But in that case, we do not use the
// wrapped gnark circuit builder externalizing range checks.
var integrationTestCompiler = plonkinwizard.Compile

// var integrationTestCompiler = dummy.Compile

const (
	zkevmBin = "../../arithmetization/zkevm.bin"
)

func parseZkEvmBin(t *testing.T, path string) (*binfile.BinaryFile, *air.Schema[bls12_377.Element], module.LimbsMap) {
	zkevm, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	zkbinf, _, err := arithmetization.UnmarshalZkEVMBin(zkevm)
	if err != nil {
		t.Fatal(err)
	}
	zkSchema, mapping := arithmetization.CompileZkevmBin(zkbinf, &mir.OptimisationConfig{})
	return zkbinf, zkSchema, mapping
}

func parseExpandedTrace(
	t *testing.T,
	path string,
	zkbinf *binfile.BinaryFile,
	zkSchema *air.Schema[bls12_377.Element],
	mapping module.LimbsMap,
) trace.Trace[bls12_377.Element] {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rawTrace, _, err := arithmetization.ReadLtTraces(f)
	if err != nil {
		t.Fatal(err)
	}
	rawTrace, errs := asm.Propagate(zkbinf.Schema, rawTrace)
	if err := errors.Join(errs...); err != nil {
		t.Fatal(err)
	}
	expandedTrace, errs := ir.NewTraceBuilder[bls12_377.Element]().
		WithValidation(true).
		WithExpansion(true).
		// WithDefensivePadding(true).
		WithRegisterMapping(mapping).
		WithParallelism(true).
		WithBatchSize(1024).
		Build(zkSchema, rawTrace)
	if err := errors.Join(errs...); err != nil {
		t.Fatal(err)
	}
	return expandedTrace
}

func parseColumns(
	t *testing.T,
	expandedTrace trace.Trace[bls12_377.Element],
	neededColumns []string,
) (cols map[string]trace.Column[bls12_377.Element], maxLen uint) {
	foundCols := 0
	modId := uint(0)
	moduleFound := false
	for ; modId < expandedTrace.Width(); modId++ {
		if expandedTrace.Module(modId).Name().String() == moduleName {
			moduleFound = true
			break
		}
	}
	if !moduleFound {
		t.Fatal("module not found")
		return nil, 0
	}
	mod := expandedTrace.Module(modId)
	cols = make(map[string]trace.Column[bls12_377.Element])
	for colId := uint(0); colId < mod.Width(); colId++ {
		col := mod.Column(colId)
		if slices.Contains(neededColumns, col.Name()) {
			cols[col.Name()] = col
			maxLen = max(maxLen, col.Data().Len())
			foundCols++
		}
	}
	if foundCols == len(neededColumns) {
		return cols, utils.NextPowerOfTwo(maxLen)
	}
	t.Fatal("not all columns found")
	return nil, 0
}

func registerColumns(_ *testing.T, builder *wizard.Builder, cols map[string]trace.Column[bls12_377.Element], maxLen uint) {
	for k := range cols {
		builder.RegisterCommit(colNameFn(k), int(maxLen))
	}
}

func assignColumns(_ *testing.T, run *wizard.ProverRuntime, cols map[string]trace.Column[bls12_377.Element], maxLen uint) {
	for colName, col := range cols {
		data := col.Data()
		plain := make([]field.Element, data.Len())
		for i := range plain {
			plain[i] = data.Get(uint(i)).Element
		}
		run.AssignColumn(ifaces.ColID(colNameFn(colName)), smartvectors.RightZeroPadded(plain, int(maxLen)))
	}
}
