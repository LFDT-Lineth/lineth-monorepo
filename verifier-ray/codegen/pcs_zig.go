package codegen

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// PcsZigOptions configures imports and the constant naming for generated
// `pcs.System` data.
type PcsZigOptions struct {
	// PcsImport is the Zig module exposing `System` / `ColumnDesc` / `ClaimRef`
	// / `BatchRoot`. FriImport exposes `Params`. When empty, verifier-ray-relative
	// defaults are used.
	PcsImport string
	FriImport string
	// FieldImport is kept for parity with the other emitters' option structs and
	// the existing generator call sites. PCS roots are emitted inline, so the
	// current template does not need this import directly.
	FieldImport string
	// ConstName is the emitted `System` const name. Empty -> `pcs_system_<index>`.
	ConstName string
	// ConstPrefix namespaces the supporting consts (maps, roots) so several PCS
	// systems can coexist in one file. Empty -> no prefix.
	ConstPrefix string
	// EmitHeader, when true, prepends the import declarations.
	EmitHeader bool
}

func defaultPcsZigOptions() PcsZigOptions {
	return PcsZigOptions{
		PcsImport:   `@import("../query/pcs.zig")`,
		FriImport:   `@import("../query/fri.zig")`,
		FieldImport: `@import("../field/koalabear.zig")`,
		EmitHeader:  true,
	}
}

// WritePcsSystemZig writes the Zig source for one PcsSystem as a literal
// `pcs.System`.
func WritePcsSystemZig(w io.Writer, index int, system PcsSystem) error {
	return WritePcsSystemZigWithOptions(w, index, system, defaultPcsZigOptions())
}

func WritePcsSystemZigWithOptions(w io.Writer, index int, system PcsSystem, opts PcsZigOptions) error {
	data := newPcsTemplateData(index, system, opts)
	tmpl, err := template.New("pcs").Funcs(template.FuncMap{
		"octuplet": pcsOctupletLiteral,
		"shifts":   intArray,
	}).Parse(pcsZigTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}

type pcsTemplateData struct {
	Options      PcsZigOptions
	Index        int
	System       PcsSystem
	ConstName    string
	Prefix       string
	WitnessName  string
	QuotientName string
	RootsName    string
	// Flattened column payloads: every column's shifts / claim cells laid end
	// to end, with per-column offsets, so ColumnDesc carries u32/u8 offsets
	// instead of two 16-byte fat pointers.
	ShiftsName     string
	ClaimCellsName string
	FlatShifts     []int
	FlatClaimCells []PcsCellRef
	ColOffsets     []pcsColOffset
}

type pcsColOffset struct {
	ShiftsStart int
	ShiftsLen   int
	ClaimStart  int
}

const (
	u8Max  = 1<<8 - 1
	u16Max = 1<<16 - 1
)

// checkPcsIndex panics with the offending value rather than emitting a literal
// the verifier cannot represent, mirroring vanishing_zig.go's checkIndex.
func checkPcsIndex(v, max int, what string) int {
	if v < 0 || v > max {
		panic(fmt.Sprintf("pcs codegen: %s is %d; the verifier field holds 0..%d", what, v, max))
	}
	return v
}

func newPcsTemplateData(index int, system PcsSystem, opts PcsZigOptions) pcsTemplateData {
	def := defaultPcsZigOptions()
	if opts.PcsImport == "" {
		opts.PcsImport = def.PcsImport
	}
	if opts.FriImport == "" {
		opts.FriImport = def.FriImport
	}
	if opts.FieldImport == "" {
		opts.FieldImport = def.FieldImport
	}
	if opts.ConstName == "" {
		opts.ConstName = fmt.Sprintf("pcs_system_%d", index)
	}
	// The verifier narrows these into u8/u16 fields, and the R5 build is
	// ReleaseSmall, where `@intCast` safety checks are compiled out. Panic with
	// the offending value here rather than emitting a literal the verifier
	// cannot represent. `pcs.reconstruct` carries the matching comptime guards
	// for the counts this emitter does not own (columns.len, max_entries).
	for i, r := range system.WitnessMap {
		checkPcsIndex(r.ColDeclIdx, u16Max, fmt.Sprintf("witness_map[%d].col_decl_idx", i))
	}
	for i, r := range system.QuotientMap {
		checkPcsIndex(r.ColDeclIdx, u16Max, fmt.Sprintf("quotient_map[%d].col_decl_idx", i))
	}
	for i, b := range system.BatchRoots {
		if !b.Precomputed {
			checkPcsIndex(b.RoundIndex, u8Max, fmt.Sprintf("batch_roots[%d].round", i))
		}
	}

	flatShifts := []int{}
	flatClaims := []PcsCellRef{}
	offsets := make([]pcsColOffset, 0, len(system.Columns))
	for i, col := range system.Columns {
		checkPcsIndex(col.BatchIdx, u8Max, fmt.Sprintf("columns[%d].batch_idx", i))
		checkPcsIndex(len(col.Shifts), u8Max, fmt.Sprintf("columns[%d].shifts_len", i))
		for j, ref := range col.ClaimCells {
			checkPcsIndex(ref.Round, u8Max, fmt.Sprintf("columns[%d].claim_cells[%d].round", i, j))
		}
		offsets = append(offsets, pcsColOffset{
			ShiftsStart: len(flatShifts),
			ShiftsLen:   len(col.Shifts),
			ClaimStart:  len(flatClaims),
		})
		flatShifts = append(flatShifts, col.Shifts...)
		flatClaims = append(flatClaims, col.ClaimCells...)
	}
	return pcsTemplateData{
		Options:        opts,
		Index:          index,
		System:         system,
		ConstName:      opts.ConstPrefix + opts.ConstName,
		Prefix:         opts.ConstPrefix,
		WitnessName:    opts.ConstPrefix + "witness_map",
		QuotientName:   opts.ConstPrefix + "quotient_map",
		RootsName:      opts.ConstPrefix + "batch_roots",
		ShiftsName:     opts.ConstPrefix + "all_shifts",
		ClaimCellsName: opts.ConstPrefix + "all_claim_cells",
		FlatShifts:     flatShifts,
		FlatClaimCells: flatClaims,
		ColOffsets:     offsets,
	}
}

const pcsZigTemplate = `{{if .Options.EmitHeader}}// Code generated by verifier-ray/testdata/generate; DO NOT EDIT.

const pcs = {{.Options.PcsImport}};
const fri = {{.Options.FriImport}};

{{end}}const {{.WitnessName}} = [_]pcs.ClaimRef{
{{range .System.WitnessMap}}    .{ .col_decl_idx = {{.ColDeclIdx}}, .shift = {{.Shift}} },
{{end}}};
const {{.QuotientName}} = [_]pcs.ClaimRef{
{{range .System.QuotientMap}}    .{ .col_decl_idx = {{.ColDeclIdx}}, .shift = {{.Shift}} },
{{end}}};
const {{.RootsName}} = [_]pcs.BatchRoot{
{{range .System.BatchRoots}}{{if .Precomputed}}    .{ .precomputed = {{octuplet .Root}} },
{{else}}    .{ .round = {{.RoundIndex}} },
{{end}}{{end}}};

const {{.ShiftsName}} = [_]i32{ {{range .FlatShifts}}{{.}}, {{end}}};

const {{.ClaimCellsName}} = [_]pcs.CellRef{ {{range .FlatClaimCells}}.{ .round = {{.Round}}, .index = {{.Index}} }, {{end}}};

pub const {{.ConstName}} = pcs.System{
    .envelope_params = fri.Params{ .log_codeword_size = {{.System.LogCodewordSize}}, .log_plaintext_size = {{.System.LogPlaintextSize}}, .log_final_poly_size = {{.System.LogFinalPolySize}}, .num_queries = {{.System.NumQueries}} },
    .columns = &.{
{{range $i, $c := .System.Columns}}        .{ .batch_idx = {{$c.BatchIdx}}, .is_ext = {{$c.IsExt}}, .size = {{if $c.IsDynamic}}.{ .dynamic = .{ .index = {{$c.DynamicIndex}}, .min_size_log2 = {{$c.DynamicMinSizeLog2}} } }{{else}}.{ .static = {{$c.SizeLog2}} }{{end}}, .shifts_start = {{(index $.ColOffsets $i).ShiftsStart}}, .shifts_len = {{(index $.ColOffsets $i).ShiftsLen}}, .claim_start = {{(index $.ColOffsets $i).ClaimStart}} },
{{end}}    },
    .all_shifts = &{{.ShiftsName}},
    .all_claim_cells = &{{.ClaimCellsName}},
    .num_batches = {{.System.NumBatches}},
    .max_entries = {{.System.MaxEntries}},
    .max_size_log2 = {{.System.MaxSizeLog2}},
    .witness_map = &{{.WitnessName}},
    .quotient_map = &{{.QuotientName}},
    .batch_roots = &{{.RootsName}},
    .zeta_coin_index = {{.System.ZetaCoinIndex}},
};
`

// pcsOctupletLiteral emits a precomputed-batch root as an `[8]field.Element`
// literal `.{ .{ .value = v0 }, ... }`.
func pcsOctupletLiteral(o field.Octuplet) string {
	parts := make([]string, len(o))
	for i := range o {
		parts[i] = fmt.Sprintf(".{ .value = %d }", o[i].Uint64())
	}
	return ".{ " + strings.Join(parts, ", ") + " }"
}
