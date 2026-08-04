package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/verifier-ray/codegen"
)

// Emits the runtime half of a full-pipeline PCS fixture into verify.zig: the
// `verifier.PcsOpening` (jagged entry_claims + the FRI opening proof), against
// the real verifier-ray types (`merkle.InputTreeOpening`/`RowPair`/`RowOpening`,
// `merkle.Branch`, `fri.Proof`, `pcs.OpeningProof`). The compile-time
// `pcs.System` itself is emitted by `codegen.WritePcsSystemZig`. The opening
// carries NO roots/zeta/fold-alphas/query-positions — the verifier rebuilds
// roots from `batch_roots`, derives zeta from `all_coins`, and squeezes the FRI
// challenges from the transcript.

// pcsOpeningZigLiteral renders `verifier.PcsOpening{ .entry_claims = ..., .proof
// = ... }`. entry_claims come from the codegen-extracted PcsSystem.EntryClaims
// (canonical EntryIdx order); the opening proof is the runtime PCSOpeningProof.
func pcsOpeningZigLiteral(sys *codegen.PcsSystem, proof fri.OpeningProof) string {
	var b strings.Builder
	b.WriteString("verifier.PcsOpening{ .entry_claims = &")
	b.WriteString(extJaggedLiteral(sys.EntryClaims))
	b.WriteString(", .proof = ")
	b.WriteString(pcsOpeningProofZigLiteral(proof))
	b.WriteString(" }")
	return b.String()
}

// pcsOpeningProofZigLiteral renders a `pcs.OpeningProof{...}` (input_queries +
// fri_proof) using merkle.InputTreeOpening / merkle.Branch / fri.Proof.
func pcsOpeningProofZigLiteral(proof fri.OpeningProof) string {
	var b strings.Builder
	b.WriteString("pcs.OpeningProof{ .input_queries = &.{ ")
	for q, iq := range proof.InputQueries {
		if q > 0 {
			b.WriteString(", ")
		}
		b.WriteString("&.{ ")
		for i, opening := range iq {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(inputTreeOpeningZigLiteral(opening))
		}
		b.WriteString(" }")
	}
	b.WriteString(" }, .fri_proof = fri.Proof{ ")
	fmt.Fprintf(&b, ".round_roots = &%s, ", commitmentSliceZig(proof.FRIProof.RoundRoots))
	fmt.Fprintf(&b, ".final_poly = &%s, ", extArrayLiteral(proof.FRIProof.FinalPoly))
	b.WriteString(".running_queries = &.{ ")
	for q, rq := range proof.FRIProof.RunningQueries {
		if q > 0 {
			b.WriteString(", ")
		}
		b.WriteString("&.{ ")
		for j, layer := range rq {
			if j > 0 {
				b.WriteString(", ")
			}
			branch := layer[0]
			fmt.Fprintf(&b, "merkle.Branch{ .leaf = %s, .siblings = &%s }",
				commitmentValueLiteral(branch.Leaf), commitmentSliceZig(branch.Siblings))
		}
		b.WriteString(" }")
	}
	b.WriteString(" } } }")
	return b.String()
}

func inputTreeOpeningZigLiteral(o fri.InputTreeOpening) string {
	var b strings.Builder
	fmt.Fprintf(&b, "merkle.InputTreeOpening{ .siblings = &%s, .leaves = &.{ ", commitmentSliceZig(o.Siblings))
	for i, l := range o.Leaves {
		if i > 0 {
			b.WriteString(", ")
		}
		if l == nil {
			b.WriteString("null")
		} else {
			fmt.Fprintf(&b, "merkle.RowPair{ %s, %s }", rowOpeningZigLiteral(l[0]), rowOpeningZigLiteral(l[1]))
		}
	}
	b.WriteString(" } }")
	return b.String()
}

func rowOpeningZigLiteral(r fri.RowOpening) string {
	return fmt.Sprintf("merkle.RowOpening{ .base = &%s, .ext = &%s }", elemArrayLiteral(r.Base), extArrayLiteral(r.Ext))
}

// commitmentSliceZig renders `[_]commitment.Commitment{ ... }` for a slice of
// octuplets (Merkle roots / siblings). commitment.Commitment == poseidon2.Digest
// == [8]field.Element, so the literal is assignable wherever a Digest is wanted.
func commitmentSliceZig(values []field.Octuplet) string {
	if len(values) == 0 {
		return "[_]commitment.Commitment{}"
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = commitmentValueLiteral(v)
	}
	return "[_]commitment.Commitment{ " + strings.Join(parts, ", ") + " }"
}

func extArrayLiteral(values []field.Ext) string {
	if len(values) == 0 {
		return "[_]ext.Ext{}"
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = extValueLiteral(v)
	}
	return "[_]ext.Ext{ " + strings.Join(parts, ", ") + " }"
}

// extJaggedLiteral renders a `[][]field.Ext` as `.{ &[_]ext.Ext{...}, ... }` for
// `[]const []const ext.Ext` (entry_claims).
func extJaggedLiteral(rows [][]field.Ext) string {
	if len(rows) == 0 {
		return ".{}"
	}
	parts := make([]string, len(rows))
	for i, row := range rows {
		parts[i] = "&" + extArrayLiteral(row)
	}
	return ".{ " + strings.Join(parts, ", ") + " }"
}

func elemArrayLiteral(values []field.Element) string {
	if len(values) == 0 {
		return "[_]field.Element{}"
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fieldValueLiteral(v)
	}
	return "[_]field.Element{ " + strings.Join(parts, ", ") + " }"
}

// writePcsSystemZig emits the compile-time `const <prefix>_pcs = pcs.System{...}`
// for a scenario, via the shared codegen emitter, and returns the const name.
func writePcsSystemZig(out *bytes.Buffer, prefix string, sys *codegen.PcsSystem) string {
	// WritePcsSystemZig names the emitted System const `ConstPrefix+ConstName` and
	// namespaces its supporting consts with ConstPrefix, so the System const is
	// `<prefix>_pcs_system` and it is what the caller references as `.pcs = ...`.
	constPrefix := prefix + "_pcs_"
	_ = codegen.WritePcsSystemZigWithOptions(out, 0, *sys, codegen.PcsZigOptions{
		PcsImport:   "pcs",
		FriImport:   "fri",
		FieldImport: "field",
		ConstName:   "system",
		ConstPrefix: constPrefix,
		EmitHeader:  false,
	})
	return constPrefix + "system"
}
