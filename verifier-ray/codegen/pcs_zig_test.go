package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

func TestWritePcsSystemZig(t *testing.T) {
	system := PcsSystem{
		LogCodewordSize:  23,
		LogPlaintextSize: 22,
		LogFinalPolySize: 0,
		NumQueries:       2,
		NumBatches:       2,
		Columns: []PcsColumnDesc{
			{BatchIdx: 0, IsExt: false, SizeLog2: 3, Shifts: []int{0, 7}, ClaimCells: []PcsCellRef{{Round: 1, Index: 0}, {Round: 1, Index: 1}}},
			{BatchIdx: 1, IsExt: true, IsDynamic: true, DynamicIndex: 1, DynamicMinSizeLog2: 3, Shifts: []int{1}, ClaimCells: []PcsCellRef{{Round: 2, Index: 0}}},
		},
		MaxEntries:    2,
		MaxSizeLog2:   22,
		WitnessMap:    []PcsClaimRef{{ColDeclIdx: 0, Shift: 0}},
		QuotientMap:   []PcsClaimRef{{ColDeclIdx: 1, Shift: 0}},
		ZetaCoinIndex: 5,
		BatchRoots: []PcsBatchRoot{
			{RoundIndex: 0},
			{Precomputed: true, Root: octupletForTest(1, 2, 3, 4, 5, 6, 7, 8)},
		},
		BatchManifests: []*PcsBatchManifest{
			{Round: 0, CellStart: 4, ColStart: 0},
			nil,
		},
	}

	var buf bytes.Buffer
	if err := WritePcsSystemZig(&buf, 7, system); err != nil {
		t.Fatalf("WritePcsSystemZig() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		`const pcs = @import("../query/pcs.zig");`,
		`const fri = @import("../query/fri.zig");`,
		`const witness_map = [_]pcs.ClaimRef{`,
		`const quotient_map = [_]pcs.ClaimRef{`,
		`const batch_roots = [_]pcs.BatchRoot{`,
		`const batch_manifests = [_]?pcs.BatchManifest{`,
		`    .{ .round = 0, .cell_start = 4, .col_start = 0 },`,
		`    null,`,
		`.batch_manifests = &batch_manifests,`,
		`pub const pcs_system_7 = pcs.System{`,
		`const all_shifts = [_]i32{ 0, 7, 1, }`,
		`const all_claim_cells = [_]pcs.CellRef{ .{ .round = 1, .index = 0 }, .{ .round = 1, .index = 1 }, .{ .round = 2, .index = 0 }, }`,
		`.{ .batch_idx = 0, .is_ext = false, .size = .{ .static = 3 }, .shifts_start = 0, .shifts_len = 2, .claim_start = 0 },`,
		`.{ .batch_idx = 1, .is_ext = true, .size = .{ .dynamic = .{ .index = 1, .min_size_log2 = 3 } }, .shifts_start = 2, .shifts_len = 1, .claim_start = 2 },`,
		`.zeta_coin_index = 5,`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated pcs missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestWritePcsSystemZigWithOptions(t *testing.T) {
	system := PcsSystem{
		LogCodewordSize:  1,
		LogPlaintextSize: 0,
		Columns:          []PcsColumnDesc{},
		WitnessMap:       []PcsClaimRef{},
		QuotientMap:      []PcsClaimRef{},
		BatchRoots:       []PcsBatchRoot{},
	}

	var buf bytes.Buffer
	if err := WritePcsSystemZigWithOptions(&buf, 0, system, PcsZigOptions{
		PcsImport:   "pcs",
		FriImport:   "fri",
		ConstName:   "system",
		ConstPrefix: "case_0_",
		EmitHeader:  false,
	}); err != nil {
		t.Fatalf("WritePcsSystemZigWithOptions() error = %v", err)
	}

	out := buf.String()
	for _, unwanted := range []string{
		`const pcs = `,
		`const fri = `,
	} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("generated pcs unexpectedly contains %q\n--- got ---\n%s", unwanted, out)
		}
	}
	for _, want := range []string{
		`const case_0_witness_map = [_]pcs.ClaimRef{`,
		`const case_0_quotient_map = [_]pcs.ClaimRef{`,
		`const case_0_batch_roots = [_]pcs.BatchRoot{`,
		`pub const case_0_system = pcs.System{`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated pcs missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func octupletForTest(values ...uint64) field.Octuplet {
	if len(values) != 8 {
		panic("octupletForTest requires 8 values")
	}
	var out field.Octuplet
	for i, value := range values {
		out[i].SetUint64(value)
	}
	return out
}

// DynamicModuleOrder must follow sys.Modules order (module-index order), matching
// prover-ray Runtime.AdvanceRound's dynamic-size absorption. If it followed
// verifier-registration order instead, a multi-dynamic-module protocol whose
// verifier actions are encountered in a different order than sys.Modules would
// absorb sizes in the wrong sequence and derive different Fiat-Shamir coins,
// rejecting honest proofs. (P1b regression guard.)
func TestDynamicModuleOrderFollowsSysModules(t *testing.T) {
	sys := wiop.NewSystemf("dynorder")
	r0 := sys.NewRound()
	// Create modules in a known order: dynA, static, dynB.
	dynA := sys.NewDynamicModule(sys.Context.Childf("dynA"), wiop.PaddingDirectionRight)
	sys.NewSizedModule(sys.Context.Childf("static"), 4, wiop.PaddingDirectionNone)
	dynB := sys.NewDynamicModule(sys.Context.Childf("dynB"), wiop.PaddingDirectionRight)
	_ = dynA.NewColumn(sys.Context.Childf("colA"), r0)
	_ = dynB.NewColumn(sys.Context.Childf("colB"), r0)

	order := DynamicModuleOrder(sys)
	if len(order) != 2 {
		t.Fatalf("DynamicModuleOrder len = %d, want 2", len(order))
	}
	// Must be sys.Modules order (dynA before dynB), skipping the static module.
	if order[0] != dynA || order[1] != dynB {
		t.Fatalf("DynamicModuleOrder = [%s %s], want [dynA dynB] in sys.Modules order",
			order[0].Context.Path(), order[1].Context.Path())
	}

	idx := DynamicModuleIndex(sys)
	if idx[dynA] != 0 || idx[dynB] != 1 {
		t.Fatalf("DynamicModuleIndex = {dynA:%d dynB:%d}, want {0,1}", idx[dynA], idx[dynB])
	}
}

// A dynamic column's runtime size must be >= its baked min_size_log2, or two
// distinct raw shifts alias mod the runtime size. That bound is enforced by
// `src/query/pcs.zig`'s `reconstruct`, which returns
// `error.DynamicModuleSizeBelowMinimum` for an aliasing size — covered by
// verifier-ray's own `test/pcs_test.zig`. See
// TestBuildPcsSystemRecordsMinimumSafeDynamicSize below for the
// DynamicMinSizeLog2 computation itself.

// BuildPcsSystem now supports dynamic multi-shift columns across a RANGE of
// runtime sizes by baking the minimum safe size_log2 into the column metadata.
// Offsets 1 and 5 are distinct at size 8, but collide at size 4; the baked
// System must therefore ACCEPT the size-8 proof while recording min_size_log2=3
// so the verifier rejects proofs below size 8 for that column.
func TestBuildPcsSystemRecordsMinimumSafeDynamicSize(t *testing.T) {
	sys := wiop.NewSystemf("dyn-alias-crosssize")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	mod.NewVanishing(
		sys.Context.Childf("alias"),
		wiop.Sub(col.View().Shift(1), col.View().Shift(5)),
	)

	global.Compile(sys)
	pcscompiler.Compile(sys)

	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	pcs, err := BuildPcsSystem(sys, routing)
	if err != nil {
		t.Fatalf("BuildPcsSystem() error = %v", err)
	}
	if len(pcs.Columns) == 0 {
		t.Fatalf("BuildPcsSystem returned no columns")
	}
	if !pcs.Columns[0].IsDynamic {
		t.Fatalf("pcs.Columns[0] is not dynamic")
	}
	if pcs.Columns[0].DynamicMinSizeLog2 != 3 {
		t.Fatalf("pcs.Columns[0].DynamicMinSizeLog2 = %d, want 3", pcs.Columns[0].DynamicMinSizeLog2)
	}
}

// The manifest locator must point at the prover's manifest cells: contiguous,
// in the committed round, one per column, starting at the batch's first
// declaration index.
func TestBuildPcsSystemLocatesManifestCells(t *testing.T) {
	sys := wiop.NewSystemf("manifest-cells")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionRight)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	b := mod.NewColumn(sys.Context.Childf("b"), r0)
	mod.NewVanishing(sys.Context.Childf("eq"), wiop.Sub(a.View(), b.View()))

	global.Compile(sys)
	pcscompiler.Compile(sys)

	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	pcs, err := BuildPcsSystem(sys, routing)
	if err != nil {
		t.Fatalf("BuildPcsSystem() error = %v", err)
	}
	if len(pcs.BatchManifests) != pcs.NumBatches {
		t.Fatalf("len(BatchManifests) = %d, want %d", len(pcs.BatchManifests), pcs.NumBatches)
	}
	cells := pcscompiler.ManifestCells(r0)
	if len(cells) != 2 {
		t.Fatalf("ManifestCells(r0) has %d cells, want 2", len(cells))
	}
	got := pcs.BatchManifests[0]
	if got == nil {
		t.Fatalf("BatchManifests[0] is nil for the interactive batch")
	}
	want := PcsBatchManifest{Round: r0.ID, CellStart: cells[0].Context.ID.Position(), ColStart: 0}
	if *got != want {
		t.Fatalf("BatchManifests[0] = %+v, want %+v", *got, want)
	}
}

// Size 1 is handled through that same minimum-size metadata. Offsets 0 and 1
// are distinct at size 8 and size 2, but both normalize to the only row at size
// 1, so the baked dynamic column must carry min_size_log2=1 instead of being
// rejected outright.
func TestBuildPcsSystemRecordsMinimumSafeSizeAboveOne(t *testing.T) {
	sys := wiop.NewSystemf("dyn-alias-size-one")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	mod.NewVanishing(
		sys.Context.Childf("alias"),
		wiop.Sub(col.View(), col.View().Shift(1)),
	)

	global.Compile(sys)
	pcscompiler.Compile(sys)

	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	pcs, err := BuildPcsSystem(sys, routing)
	if err != nil {
		t.Fatalf("BuildPcsSystem() error = %v", err)
	}
	if len(pcs.Columns) == 0 {
		t.Fatalf("BuildPcsSystem returned no columns")
	}
	if pcs.Columns[0].DynamicMinSizeLog2 != 1 {
		t.Fatalf("pcs.Columns[0].DynamicMinSizeLog2 = %d, want 1", pcs.Columns[0].DynamicMinSizeLog2)
	}
}

// A generated value that does not fit its verifier field must fail loudly at
// generation time. The R5 build is ReleaseSmall, where `@intCast` safety checks
// are compiled out, so a value that slipped through here would be silent UB in
// the verifier rather than a rejected build.
func TestPcsZigRejectsOutOfRangeIndices(t *testing.T) {
	for _, tc := range []struct {
		name string
		sys  PcsSystem
		want string
	}{
		{
			name: "batch_idx above u8",
			sys:  PcsSystem{Columns: []PcsColumnDesc{{BatchIdx: 1 << 8}}},
			want: "batch_idx is 256",
		},
		{
			name: "shifts_len above u8",
			sys:  PcsSystem{Columns: []PcsColumnDesc{{Shifts: make([]int, 1<<8)}}},
			want: "shifts_len is 256",
		},
		{
			name: "claim cell round above u8",
			sys:  PcsSystem{Columns: []PcsColumnDesc{{ClaimCells: []PcsCellRef{{Round: 1 << 8}}}}},
			want: "round is 256",
		},
		{
			name: "witness col_decl_idx above u16",
			sys:  PcsSystem{WitnessMap: []PcsClaimRef{{ColDeclIdx: 1 << 16}}},
			want: "col_decl_idx is 65536",
		},
		{
			name: "quotient col_decl_idx above u16",
			sys:  PcsSystem{QuotientMap: []PcsClaimRef{{ColDeclIdx: 1 << 16}}},
			want: "col_decl_idx is 65536",
		},
		{
			name: "batch root round above u8",
			sys:  PcsSystem{BatchRoots: []PcsBatchRoot{{Precomputed: false, RoundIndex: 1 << 8}}},
			want: "round is 256",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("newPcsTemplateData did not panic on an out-of-range %s", tc.name)
				}
				msg, ok := r.(string)
				if !ok {
					t.Fatalf("panic value = %v (%T), want string", r, r)
				}
				if !strings.Contains(msg, tc.want) {
					t.Fatalf("panic = %q, want substring %q", msg, tc.want)
				}
			}()
			_ = newPcsTemplateData(0, tc.sys, PcsZigOptions{})
		})
	}
}
