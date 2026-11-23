package codegen

import (
	"fmt"
	"io"
	"strings"

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
	return executeZigTemplate(w, "pcs", data)
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
	// ManifestsName is the per-batch manifest locator array; emitted only when
	// the system carries manifests (BatchManifests non-empty).
	ManifestsName string
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

	// maxMerkleCapHeight mirrors pcs.zig's `max_merkle_cap_height`: verify
	// keeps a cap's interior nodes in a stack buffer of 2^height digests, so
	// the baked height is bounded by what the guest stack can hold.
	maxMerkleCapHeight = 10
)

// checkPcsIndex panics with the offending value rather than emitting a literal
// the verifier cannot represent, mirroring vanishing_zig.go's checkIndex.
func checkPcsIndex(v, limit int, what string) {
	if v < 0 || v > limit {
		panic(fmt.Sprintf("pcs codegen: %s is %d; the verifier field holds 0..%d", what, v, limit))
	}
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
	checkPcsIndex(system.MerkleCapHeight, maxMerkleCapHeight, "merkle_cap_height")
	if len(system.BatchManifests) > 0 && len(system.BatchManifests) != system.NumBatches {
		panic(fmt.Sprintf("pcs codegen: %d batch_manifests for %d batches", len(system.BatchManifests), system.NumBatches))
	}
	for i, m := range system.BatchManifests {
		if m == nil {
			continue
		}
		checkPcsIndex(m.Round, u8Max, fmt.Sprintf("batch_manifests[%d].round", i))
		checkPcsIndex(m.CellStart, u16Max, fmt.Sprintf("batch_manifests[%d].cell_start", i))
		checkPcsIndex(m.ColStart, u16Max, fmt.Sprintf("batch_manifests[%d].col_start", i))
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
		ManifestsName:  opts.ConstPrefix + "batch_manifests",
		ShiftsName:     opts.ConstPrefix + "all_shifts",
		ClaimCellsName: opts.ConstPrefix + "all_claim_cells",
		FlatShifts:     flatShifts,
		FlatClaimCells: flatClaims,
		ColOffsets:     offsets,
	}
}

// pcsOctupletLiteral emits a precomputed-batch root as an `[8]field.Element`
// literal `.{ .{ .value = v0 }, ... }`.
func pcsOctupletLiteral(o field.Octuplet) string {
	parts := make([]string, len(o))
	for i := range o {
		parts[i] = fmt.Sprintf(".{ .value = %d }", o[i].Uint64())
	}
	return ".{ " + strings.Join(parts, ", ") + " }"
}
