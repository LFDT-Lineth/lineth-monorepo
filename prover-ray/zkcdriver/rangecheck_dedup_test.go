package zkcdriver_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/ir/air"
	"github.com/LFDT-Lineth/zkc/pkg/schema"
	"github.com/LFDT-Lineth/zkc/pkg/schema/constraint/lookup"
	"github.com/LFDT-Lineth/zkc/pkg/schema/register"
	"github.com/LFDT-Lineth/zkc/pkg/util/collection/bit"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/util/source"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const inlineU8Program = `
fn main() {
    var acc:u8 = 0
    for i:u8 = 0; i < 10; i = i + 1 {
        acc = acc + i
    }
    if acc != 45 {
        fail
    }
}
`

func compileInlineZkc(t *testing.T, zkcSrc string) []byte {
	t.Helper()
	src := source.NewSourceFile("inline.zkc", []byte(zkcSrc))
	macroProgram, _, errs := compiler.Compile(zkcField, zkcCfg.GetMaxStaticHeight(), *src)
	require.Empty(t, errs)
	ir, errs := ast.Compile(macroProgram, zkcCfg)
	require.Empty(t, errs)
	binfile := constraints.NewBinaryFile[koalabear.Element, vm.Uint32](nil, nil, ir)
	serialized, err := binfile.MarshalBinary()
	require.NoError(t, err)
	return serialized
}

func u8ProgramInputs() *zkcdriver.PreReadInputs {
	return &zkcdriver.PreReadInputs{Inputs: map[string]bit.Array{}}
}

func makeU64Vec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

func countMatchingModules(sys *wiop.System, substr string) []*wiop.Module {
	var matched []*wiop.Module
	for _, m := range sys.Modules {
		if strings.Contains(m.Context.Path(), substr) {
			matched = append(matched, m)
		}
	}
	return matched
}

func tryProveAndVerify(t *testing.T, sys *wiop.System, assign func(*wiop.Runtime)) (ok bool) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Logf("rejected during Prove: %v", r)
			ok = false
		}
	}()
	proof, pub := sys.Prove(assign, wiop.ProveOptions{CheckUnreducedQueries: true})
	if err := sys.Verify(proof, pub); err != nil {
		t.Logf("rejected during Verify: %v", err)
		return false
	}
	return true
}

// TestZkcRangeU8DeduplicatedToRangeCheck verifies (a): ZkC static $range_u8
// lookups are emitted as wiop.RangeCheck queries without instantiating
// module-$range_u8, compile into range-mod-b256, accept honest traces, and
// reject out-of-range column values.
func TestZkcRangeU8DeduplicatedToRangeCheck(t *testing.T) {
	serialized := compileInlineZkc(t, inlineU8Program)

	sys := wiop.NewSystemf("u8-dedup")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(serialized))

	assert.Empty(t, countMatchingModules(sys, "module-$range_u"),
		"canonical $range_u<k> modules must not be instantiated by Define")

	var targetCol *wiop.Column
	for _, m := range sys.Modules {
		for _, rc := range m.RangeChecks {
			if rc.B == 256 {
				targetCol = rc.Handle
				break
			}
		}
	}
	require.NotNil(t, targetCol, "expected at least one RangeCheck with bound 256")

	proverCompilePipeline(sys)
	assert.Empty(t, countMatchingModules(sys, "module-$range_u"))
	rangeMods256 := countMatchingModules(sys, "rangecheck/range-mod-b256")
	require.Len(t, rangeMods256, 1, "expected a single range-mod-b256 module")

	shard := traceSingleShard(t, driver, u8ProgramInputs())
	require.True(t, tryProveAndVerify(t, sys, func(rt *wiop.Runtime) {
		driver.AssignTraceShard(rt, shard, field.Octuplet{})
	}), "honest trace must prove and verify")

	// Soundness: corrupting the range-checked ZkC column to 256 must be rejected.
	require.False(t, tryProveAndVerify(t, sys, func(rt *wiop.Runtime) {
		driver.AssignTraceShard(rt, shard, field.Octuplet{})
		orig := rt.GetColumnAssignment(targetCol).Plain.AsBase()
		bad := make([]field.Element, len(orig))
		copy(bad, orig)
		bad[0] = field.NewElement(256)
		rt.OverrideColumn(targetCol, &wiop.ConcreteVector{Plain: field.VecFromBase(bad)})
	}), "out-of-range witness value 256 must be rejected")
}

// TestZkcRangeU8SharedWithExplicitRangeCheck verifies (b): adding an explicit
// mod.NewRangeCheck(..., 256) alongside ZkC $range_u8 lookups shares the single
// range-mod-b256 precomputed column and multiplicity column M-0.
func TestZkcRangeU8SharedWithExplicitRangeCheck(t *testing.T) {
	serialized := compileInlineZkc(t, inlineU8Program)

	sys := wiop.NewSystemf("u8-shared")
	r0 := sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(serialized))

	gadgetMod := sys.NewDynamicModule(sys.Context.Childf("gadget"), wiop.PaddingDirectionRight)
	gadgetCol := gadgetMod.NewColumn(gadgetMod.Context.Childf("limb8"), r0)
	gadgetMod.NewRangeCheck(gadgetMod.Context.Childf("rc-limb8"), gadgetCol, 256)

	proverCompilePipeline(sys)

	assert.Empty(t, countMatchingModules(sys, "module-$range_u"))
	rangeMods256 := countMatchingModules(sys, "rangecheck/range-mod-b256")
	require.Len(t, rangeMods256, 1, "ZkC and explicit NewRangeCheck must share one range-mod-b256")
	// Full pipeline attaches 4 columns to a single-bucket range-mod-b256:
	// 1 precomputed (range-col-b256), 1 multiplicity (M-0), 1 log-derivative sum,
	// and 1 global quotient (7 columns if split into two buckets).
	require.Len(t, rangeMods256[0].Columns, 4,
		"range-mod-b256 must carry one shared lookup bucket (4 columns total)")

	shard := traceSingleShard(t, driver, u8ProgramInputs())
	assignWithGadget := func(vals ...uint64) func(*wiop.Runtime) {
		return func(rt *wiop.Runtime) {
			driver.AssignTraceShard(rt, shard, field.Octuplet{})
			rt.AssignColumn(gadgetCol, makeU64Vec(vals...))
		}
	}

	require.True(t, tryProveAndVerify(t, sys, assignWithGadget(0, 42, 127, 255)),
		"honest ZkC + gadget witness must prove and verify")
	require.False(t, tryProveAndVerify(t, sys, assignWithGadget(0, 42, 256, 255)),
		"out-of-range gadget value 256 must be rejected")
}

// buildSyntheticRangeSchema constructs an air.Schema with dynamic module "src"
// (registers "sel", "val") and static module "$range_u8" (1 register "value"
// populated by fillRow).
func buildSyntheticRangeSchema(
	fillRow func(j int) uint64,
	makeConstraints func(srcMod, rangeMod schema.ModuleId, selReg, valReg, tgtReg register.Id) []schema.Constraint[koalabear.Element],
) air.Schema[koalabear.Element] {
	srcTable := new(schema.Table[koalabear.Element]).Init("src", false, false, false, false, false)
	srcTable.AddRegisters(
		register.NewInput("sel", 1),
		register.NewInput("val", 16),
	)
	selReg, valReg := register.NewId(0), register.NewId(1)

	rangeTable := new(schema.Table[koalabear.Element]).Init("$range_u8", false, false, true, false, true)
	rangeTable.AddRegisters(register.NewComputed("value", 8))
	tgtReg := register.NewId(0)

	contents := make([][]koalabear.Element, 256)
	for j := range 256 {
		contents[j] = []koalabear.Element{koalabear.Element(field.NewElement(fillRow(j)))}
	}
	rangeTable.SetStaticContents(contents)

	const srcModID, rangeModID = schema.ModuleId(0), schema.ModuleId(1)
	srcTable.AddConstraints(makeConstraints(srcModID, rangeModID, selReg, valReg, tgtReg)...)
	return schema.NewUniformSchema([]*schema.Table[koalabear.Element]{srcTable, rangeTable})
}

// TestFilteredRangeLookupFallsBackToStaticModule verifies (c) and safety edge
// cases: filtered lookups, duplicate lookups on the same column, and
// non-canonical static contents.
func TestFilteredRangeLookupFallsBackToStaticModule(t *testing.T) {
	t.Run("filtered_lookup_keeps_static_module", func(t *testing.T) {
		sc := buildSyntheticRangeSchema(
			func(j int) uint64 { return uint64(j) },
			func(srcMod, rangeMod schema.ModuleId, selReg, valReg, tgtReg register.Id) []schema.Constraint[koalabear.Element] {
				src := lookup.FilteredVector(srcMod, selReg, valReg)
				tgt := lookup.UnfilteredVector(rangeMod, tgtReg)
				return []schema.Constraint[koalabear.Element]{
					lookup.NewConstraint[koalabear.Element]("filtered-u8", []lookup.Vector{tgt}, []lookup.Vector{src}),
				}
			},
		)

		sys := wiop.NewSystemf("filtered-fallback")
		zkcdriver.Define(sys, &sc, 0)
		proverCompilePipeline(sys)

		assert.Len(t, countMatchingModules(sys, "module-$range_u8"), 1,
			"filtered lookup must preserve module-$range_u8")
		assert.Empty(t, countMatchingModules(sys, "rangecheck/range-mod-b256"),
			"filtered fallback must not emit range-mod-b256")

		srcMod := countMatchingModules(sys, "module-src")[0]
		selCol, valCol := srcMod.Columns[0], srcMod.Columns[1]

		// Inactive rows (sel == 0) carry values >= 256 and must still pass.
		require.True(t, tryProveAndVerify(t, sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(selCol, makeU64Vec(1, 0, 1, 0))
			rt.AssignColumn(valCol, makeU64Vec(200, 999, 0, 5000))
		}), "inactive rows with out-of-range values must be accepted under filtered fallback")

		// Active row (sel == 1) with val == 256 must be rejected.
		require.False(t, tryProveAndVerify(t, sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(selCol, makeU64Vec(1, 0, 1, 0))
			rt.AssignColumn(valCol, makeU64Vec(256, 999, 0, 5000))
		}), "active row with val == 256 must be rejected")
	})

	t.Run("duplicate_lookup_same_column_unique_context", func(t *testing.T) {
		sc := buildSyntheticRangeSchema(
			func(j int) uint64 { return uint64(j) },
			func(srcMod, rangeMod schema.ModuleId, _, valReg, tgtReg register.Id) []schema.Constraint[koalabear.Element] {
				src := lookup.UnfilteredVector(srcMod, valReg)
				tgt := lookup.UnfilteredVector(rangeMod, tgtReg)
				return []schema.Constraint[koalabear.Element]{
					lookup.NewConstraint[koalabear.Element]("dup-u8", []lookup.Vector{tgt}, []lookup.Vector{src}),
					lookup.NewConstraint[koalabear.Element]("dup-u8", []lookup.Vector{tgt}, []lookup.Vector{src}),
				}
			},
		)

		sys := wiop.NewSystemf("dup-rc")
		zkcdriver.Define(sys, &sc, 0)

		srcMod := countMatchingModules(sys, "module-src")[0]
		require.Len(t, srcMod.RangeChecks, 2)
		assert.NotEqual(t, srcMod.RangeChecks[0].Context().Path(), srcMod.RangeChecks[1].Context().Path())

		proverCompilePipeline(sys)
		valCol := srcMod.Columns[0]

		require.True(t, tryProveAndVerify(t, sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(valCol, makeU64Vec(0, 7, 128, 255))
		}))
		require.False(t, tryProveAndVerify(t, sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(valCol, makeU64Vec(0, 7, 256, 255))
		}))
	})

	t.Run("non_canonical_static_contents_keeps_static_module", func(t *testing.T) {
		// Contents [1..256] instead of [0..255]: must NOT be rewritten to [0, 256).
		sc := buildSyntheticRangeSchema(
			func(j int) uint64 { return uint64(j + 1) },
			func(srcMod, rangeMod schema.ModuleId, _, valReg, tgtReg register.Id) []schema.Constraint[koalabear.Element] {
				src := lookup.UnfilteredVector(srcMod, valReg)
				tgt := lookup.UnfilteredVector(rangeMod, tgtReg)
				return []schema.Constraint[koalabear.Element]{
					lookup.NewConstraint[koalabear.Element]("shifted-u8", []lookup.Vector{tgt}, []lookup.Vector{src}),
				}
			},
		)

		sys := wiop.NewSystemf("non-canonical")
		zkcdriver.Define(sys, &sc, 0)
		proverCompilePipeline(sys)

		assert.Len(t, countMatchingModules(sys, "module-$range_u8"), 1)
		assert.Empty(t, countMatchingModules(sys, "rangecheck/range-mod-b256"))

		srcMod := countMatchingModules(sys, "module-src")[0]
		valCol := srcMod.Columns[0]

		require.True(t, tryProveAndVerify(t, sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(valCol, makeU64Vec(1, 2, 255, 256))
		}))
		require.False(t, tryProveAndVerify(t, sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(valCol, makeU64Vec(0, 1, 2, 3))
		}))
	})
}
