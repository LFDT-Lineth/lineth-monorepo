package codegen

import (
	"fmt"
	"io"
	"strings"
)

// PcsZigOptions configures imports and the constant naming for a generated Zig
// pcs.verify.System. The default imports assume the standard verifier-ray source
// layout; multi-system files set EmitHeader=false and provide qualified imports.
type PcsZigOptions struct {
	// Import prefixes for the Zig `pcs` root module and its `layout` / `verify`
	// submodules. When empty, sensible verifier-ray-relative defaults are used.
	PcsImport    string
	LayoutImport string
	VerifyImport string
	// ConstPrefix namespaces the emitted consts (params/shapes/shifts/…), so
	// several PCS systems can coexist in one file. Empty → the fixture-compatible
	// unprefixed names (params, shapes, shifts, pcs_system).
	ConstPrefix string
}

func defaultPcsZigOptions() PcsZigOptions {
	return PcsZigOptions{
		PcsImport:    `@import("../pcs/root.zig")`,
		LayoutImport: `@import("../pcs/layout.zig")`,
		VerifyImport: `@import("../pcs/verify.zig")`,
	}
}

// WritePcsSystemZig writes the Zig source for one PcsSystem: params, per-batch
// shapes and shift schedules, the witness/quotient claim maps, and the
// pcs.verify.System literal that ties them together. It emits data only — Zig
// owns buildLayout and the verifier itself.
//
// The emitted `<prefix>pcs_system` const is a `verify.System` ready to drop into
// a `verifier.Systems{ .pcs = ... }`. `WriteCompiledSystemZig` uses this to make
// the PCS↔vanishing claim link concrete for a real protocol.
func WritePcsSystemZig(w io.Writer, system PcsSystem, opts PcsZigOptions) error {
	if opts.PcsImport == "" {
		opts.PcsImport = defaultPcsZigOptions().PcsImport
	}
	if opts.LayoutImport == "" {
		opts.LayoutImport = defaultPcsZigOptions().LayoutImport
	}
	if opts.VerifyImport == "" {
		opts.VerifyImport = defaultPcsZigOptions().VerifyImport
	}
	p := opts.ConstPrefix

	var b strings.Builder

	// Params. log_final_poly_size is fixed at 0 for the current PCS (fold to a
	// constant); it is surfaced as a field so the shape check stays explicit.
	fmt.Fprintf(&b, "pub const %sparams = %s.Params{ .log_codeword_size = %d, .log_plaintext_size = %d, .num_queries = %d, .log_final_poly_size = %d };\n\n",
		p, opts.VerifyImport, system.LogCodewordSize, system.LogPlaintextSize, system.NumQueries, system.LogFinalPolySize)

	// Shapes: one Shape per batch.
	for bi, shape := range system.Shapes {
		fmt.Fprintf(&b, "const %sbatch%d_shape = [_]%s.SizedShape{\n", p, bi, opts.LayoutImport)
		for _, s := range shape {
			fmt.Fprintf(&b, "    .{ .base_width = %d, .ext_width = %d },\n", s.BaseWidth, s.ExtWidth)
		}
		fmt.Fprintln(&b, "};")
	}
	fmt.Fprintf(&b, "pub const %sshapes = [_]%s.Shape{ ", p, opts.LayoutImport)
	for bi := range system.Shapes {
		if bi > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "&%sbatch%d_shape", p, bi)
	}
	fmt.Fprintln(&b, " };")

	// Shifts: per batch, per size, per row a []usize; then the SizedShifts list.
	for bi, bs := range system.Shifts {
		for sizeLog2, ss := range bs {
			for r, row := range ss.Base {
				fmt.Fprintf(&b, "const %sb%d_s%d_base_%d = [_]usize{ %s };\n", p, bi, sizeLog2, r, pcsIntList(row))
			}
			for r, row := range ss.Ext {
				fmt.Fprintf(&b, "const %sb%d_s%d_ext_%d = [_]usize{ %s };\n", p, bi, sizeLog2, r, pcsIntList(row))
			}
		}
		fmt.Fprintf(&b, "const %sbatch%d_shifts = [_]%s.SizedShifts{\n", p, bi, opts.LayoutImport)
		for sizeLog2, ss := range bs {
			b.WriteString("    .{ .base = &.{")
			for r := range ss.Base {
				if r > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, " &%sb%d_s%d_base_%d", p, bi, sizeLog2, r)
			}
			b.WriteString(" }, .ext = &.{")
			for r := range ss.Ext {
				if r > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, " &%sb%d_s%d_ext_%d", p, bi, sizeLog2, r)
			}
			fmt.Fprintln(&b, " } },")
		}
		fmt.Fprintln(&b, "};")
	}
	fmt.Fprintf(&b, "pub const %sshifts = [_]%s.BatchShifts{ ", p, opts.LayoutImport)
	for bi := range system.Shifts {
		if bi > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "&%sbatch%d_shifts", p, bi)
	}
	fmt.Fprintln(&b, " };")

	// Claim maps.
	fmt.Fprintf(&b, "const %switness_map = [_]%s.ClaimRef{ ", p, opts.VerifyImport)
	pcsWriteClaimRefs(&b, system.WitnessMap)
	fmt.Fprintln(&b, " };")
	fmt.Fprintf(&b, "const %squotient_map = [_]%s.ClaimRef{ ", p, opts.VerifyImport)
	pcsWriteClaimRefs(&b, system.QuotientMap)
	fmt.Fprintln(&b, " };")

	// The System literal. `layout` is reconstructed by Zig from shapes+shifts, so
	// the canonical enumeration lives in exactly one place (buildLayout).
	fmt.Fprintf(&b, "pub const %spcs_system = %s.System{\n", p, opts.VerifyImport)
	fmt.Fprintf(&b, "    .params = %sparams,\n", p)
	fmt.Fprintf(&b, "    .layout = %s.layout.buildLayout(&%sshapes, &%sshifts) catch unreachable,\n", opts.PcsImport, p, p)
	fmt.Fprintf(&b, "    .shapes = &%sshapes,\n", p)
	fmt.Fprintf(&b, "    .num_batches = %d,\n", system.NumBatches)
	fmt.Fprintf(&b, "    .witness_map = &%switness_map,\n", p)
	fmt.Fprintf(&b, "    .quotient_map = &%squotient_map,\n", p)
	fmt.Fprintf(&b, "    .zeta_coin_index = %d,\n", system.ZetaCoinIndex)
	fmt.Fprintln(&b, "};")

	_, err := io.WriteString(w, b.String())
	return err
}

func pcsWriteClaimRefs(b *strings.Builder, refs []PcsClaimRef) {
	for i, r := range refs {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, ".{ .entry = %d, .shift = %d }", r.Entry, r.Shift)
	}
}

func pcsIntList(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprintf("%d", x)
	}
	return strings.Join(parts, ", ")
}
