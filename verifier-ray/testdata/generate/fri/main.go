// Package main generates verifier-ray's FRI/PCS Zig test vectors from
// prover-ray.
//
// This is a separate Go module from testdata/generate (the wiop/vanishing
// fixture generator) specifically so its prover-ray dependency can be
// bumped independently: FRI/PCS support and the wiop-based vanishing
// pipeline evolve on different schedules in prover-ray, and coupling them
// to one pin means bumping for one can break the other for unrelated
// reasons. Every scenario here goes through prover-ray's exported API only
// (Commit/AddOpening/NewProverState/Fold/Open/Verify) and self-checks the
// resulting proof against the exported pcs.Verify before ever writing a Zig
// literal.
package main

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
)

func main() {
	if err := writePCSFixtures(); err != nil {
		panic(err)
	}
}

// ─── Zig-literal helpers ─────────────────────────────────────────────────
//
// Mirrors testdata/generate/main.go's own helpers of the same name and
// behavior; duplicated rather than shared because the two are separate Go
// modules with independently-pinned dependencies.

func elem(v uint64) field.Element {
	var e field.Element
	e.SetUint64(v)
	return e
}

func u(e field.Element) uint64 {
	return e.Uint64()
}

func ext6(e field.Ext) string {
	a0, a1, b0, b1, c0, c1 := field.ExtToUint64s(&e)
	return fmt.Sprintf(".{ %d, %d, %d, %d, %d, %d }", a0, a1, b0, b1, c0, c1)
}

func oct8(values field.Octuplet) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprintf("%d", u(value))
	}
	return ".{ " + strings.Join(parts, ", ") + " }"
}

func elemSlice(values []field.Element) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprintf("%d", u(value))
	}
	return ".{ " + strings.Join(parts, ", ") + " }"
}

func extSlice(values []field.Ext) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = ext6(value)
	}
	return ".{ " + strings.Join(parts, ", ") + " }"
}

func commitmentSlice(values []field.Octuplet) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = oct8(value)
	}
	return ".{ " + strings.Join(parts, ", ") + " }"
}

func intArrayLiteral(values []int) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprintf("%d", value)
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func runZigFmt(data []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "verifier-ray-vectors-*.zig")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	cmd := os.Getenv("ZIG")
	if cmd == "" {
		cmd = "zig"
	}
	if err := exec.Command(cmd, "fmt", tmp.Name()).Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(tmp.Name())
}

// ─── PCS fixtures: real end-to-end proofs through prover-ray's exported API ─
//
// Every scenario runs NewPCS/Commit/AddOpening/NewProverState/Fold/Open
// through prover-ray's exported surface only, computes its own claims (not
// via the unexported pcs.shiftedPoint, which only a caller inside package
// fri could reach), and self-checks the result with the exported pcs.Verify
// before ever writing a Zig literal. The canonical (batch,size,row) layout
// and its claim-offset schedule are re-derived from the frozen, documented
// ordering in prover-ray's pcs.go package doc (canonicalLayout itself is
// unexported and off-limits to this separate module).

type pcsDeepEntry struct {
	BatchIdx    int
	IsExt       bool
	RowIdx      int
	ClaimOffset int
	Shifts      []int
}

type pcsSizeBundle struct {
	SizeLog2 uint8
	Entries  []pcsDeepEntry
}

// computeLayout re-derives fri's frozen canonical layout: for each size in
// descending order, batches in declaration order, base rows then ext rows,
// row declaration order, with the claim offset accumulating across the
// whole layout (verifier_ray's flat claimed_values array).
func computeLayout(shapes []fri.Shape, shifts []fri.BatchShifts) []pcsSizeBundle {
	maxSizeLog2 := -1
	for _, s := range shapes {
		if len(s) > maxSizeLog2+1 {
			maxSizeLog2 = len(s) - 1
		}
	}

	var layout []pcsSizeBundle
	offset := 0
	for sizeLog2 := maxSizeLog2; sizeLog2 >= 0; sizeLog2-- {
		bundle := pcsSizeBundle{SizeLog2: uint8(sizeLog2)}
		for batchIdx := range shapes {
			if sizeLog2 >= len(shapes[batchIdx]) {
				continue
			}
			shape := shapes[batchIdx][sizeLog2]
			rowShifts := shifts[batchIdx][sizeLog2]
			for rowIdx := 0; rowIdx < shape.BaseWidth; rowIdx++ {
				bundle.Entries = append(bundle.Entries, pcsDeepEntry{
					BatchIdx: batchIdx, IsExt: false, RowIdx: rowIdx,
					ClaimOffset: offset, Shifts: rowShifts.Base[rowIdx],
				})
				offset += len(rowShifts.Base[rowIdx])
			}
			for rowIdx := 0; rowIdx < shape.ExtWidth; rowIdx++ {
				bundle.Entries = append(bundle.Entries, pcsDeepEntry{
					BatchIdx: batchIdx, IsExt: true, RowIdx: rowIdx,
					ClaimOffset: offset, Shifts: rowShifts.Ext[rowIdx],
				})
				offset += len(rowShifts.Ext[rowIdx])
			}
		}
		if len(bundle.Entries) > 0 {
			layout = append(layout, bundle)
		}
	}
	return layout
}

// flattenClaims reads claimed's per-(batch,size,row) values off in exactly
// layout's order: the flat array verifier_ray's query.pcs.VerifyInput expects.
func flattenClaims(layout []pcsSizeBundle, claimed []fri.BatchClaimedValues) []field.Ext {
	var flat []field.Ext
	for _, bundle := range layout {
		for _, e := range bundle.Entries {
			sized := claimed[e.BatchIdx][bundle.SizeLog2]
			if e.IsExt {
				flat = append(flat, sized.Ext[e.RowIdx]...)
			} else {
				flat = append(flat, sized.Base[e.RowIdx]...)
			}
		}
	}
	return flat
}

// pcsMakeEncoders builds n RSEncoders for sizes 2^0..2^(n-1) at a uniform
// inverse rate from exported fri.NewEncoder: the same construction
// prover-ray's own unexported test helper makeEncoders performs, done here
// with exported API since this generator cannot reach that helper.
func pcsMakeEncoders(n, invRate int) []*fri.RSEncoder {
	encoders := make([]*fri.RSEncoder, n)
	for i := range n {
		enc := fri.NewEncoder(uint64(invRate)*(1<<i), 1<<i)
		encoders[i] = &enc
	}
	return encoders
}

// shiftedPointExported computes zeta*omega_N^shift from exported API only.
// prover-ray's pcs.shiftedPoint is unexported, but this is exactly what the
// outer protocol (any real caller of AddOpening) computes independently to
// evaluate its own claims -- so this mirrors that caller, not an internal.
func shiftedPointExported(sizeLog2 uint8, shift int, zeta field.Ext) field.Ext {
	omega := field.RootOfUnityBy(1 << sizeLog2)
	var rotation field.Element
	rotation.Exp(omega, big.NewInt(int64(shift)))
	var point field.Ext
	point.MulByElement(&zeta, &rotation)
	return point
}

func evalRowExported(poly field.Vec, sizeLog2 uint8, shifts []int, zeta field.Ext) []field.Ext {
	values := make([]field.Ext, len(shifts))
	for i, shift := range shifts {
		point := shiftedPointExported(sizeLog2, shift, zeta)
		values[i] = polynomials.EvalLagrange(poly, field.ElemFromExt(point)).AsExt()
	}
	return values
}

// computeClaims evaluates every opened (size, row, shift) of witness at
// zeta*omega^shift via Lagrange interpolation -- exactly what AddOpening's
// caller is documented to supply, computed with no dependence on
// prover-ray's own (unexported) claim-computation test helper.
func computeClaims(witness fri.Batch, shifts fri.BatchShifts, zeta field.Ext) fri.BatchClaimedValues {
	claimed := make(fri.BatchClaimedValues, len(shifts))
	for sizeLog2, rowShifts := range shifts {
		sizedWitness := witness[sizeLog2]
		var sized fri.SizedClaimedValues
		sized.Base = make([][]field.Ext, len(rowShifts.Base))
		for rowIdx, s := range rowShifts.Base {
			sized.Base[rowIdx] = evalRowExported(field.VecFromBase(sizedWitness.Base[rowIdx]), uint8(sizeLog2), s, zeta)
		}
		sized.Ext = make([][]field.Ext, len(rowShifts.Ext))
		for rowIdx, s := range rowShifts.Ext {
			sized.Ext[rowIdx] = evalRowExported(field.VecFromExt(sizedWitness.Ext[rowIdx]), uint8(sizeLog2), s, zeta)
		}
		claimed[sizeLog2] = sized
	}
	return claimed
}

type pcsScenario struct {
	Name             string
	Params           fri.Params
	LogFinalPolySize uint8
	Encoders         []*fri.RSEncoder
	Witnesses        []fri.Batch
	Shifts           []fri.BatchShifts
	Zeta             field.Ext
	FoldAlphas       []field.Ext
	Positions        []int
}

type pcsCaseData struct {
	Name              string
	Params            fri.Params
	LogFinalPolySize  uint8
	Layout            []pcsSizeBundle
	Roots             []field.Octuplet
	ClaimedValues     []field.Ext
	Zeta              field.Ext
	FoldAlphas        []field.Ext
	QueryPositions    []int
	Proof             fri.OpeningProof
	ExpectVerifyError string
}

// buildPCSScenarioData runs the real end-to-end proof through exported API
// only, then self-checks it with the exported pcs.Verify before returning:
// if shiftedPointExported ever drifted from prover-ray's internal
// convention, this catches it here rather than shipping a wrong vector.
func buildPCSScenarioData(s pcsScenario) pcsCaseData {
	pcsInst, err := fri.NewPCS(s.Params, s.Encoders)
	if err != nil {
		panic(fmt.Sprintf("%s: NewPCS: %v", s.Name, err))
	}

	committed := make([]fri.CommitterState, len(s.Witnesses))
	claimed := make([]fri.BatchClaimedValues, len(s.Witnesses))
	roots := make([]field.Octuplet, len(s.Witnesses))
	shapes := make([]fri.Shape, len(s.Witnesses))
	for i, w := range s.Witnesses {
		committed[i] = pcsInst.Commit(w)
		claimed[i] = computeClaims(w, s.Shifts[i], s.Zeta)
		if err := pcsInst.AddOpening(committed[i], s.Zeta, s.Shifts[i], claimed[i]); err != nil {
			panic(fmt.Sprintf("%s: AddOpening: %v", s.Name, err))
		}
		roots[i] = committed[i].Tree.Root()
		shapes[i] = w.Shape()
	}

	state, err := pcsInst.NewProverState()
	if err != nil {
		panic(fmt.Sprintf("%s: NewProverState: %v", s.Name, err))
	}
	for round := 0; state.HasNext(); round++ {
		if round >= len(s.FoldAlphas) {
			panic(fmt.Sprintf("%s: not enough fold alphas for round %d", s.Name, round))
		}
		state.Fold(s.FoldAlphas[round])
	}
	proof := pcsInst.Open(state, s.Positions)

	if err := pcsInst.Verify(fri.VerifyInputs{
		Roots:         roots,
		Shapes:        shapes,
		Shifts:        s.Shifts,
		ClaimedValues: claimed,
		Zeta:          s.Zeta,
		Challenges:    fri.Challenges{FoldAlphas: s.FoldAlphas, QueryPositions: s.Positions},
	}, proof); err != nil {
		panic(fmt.Sprintf("%s: self-check via pcs.Verify failed: %v", s.Name, err))
	}

	layout := computeLayout(shapes, s.Shifts)

	return pcsCaseData{
		Name:             s.Name,
		Params:           s.Params,
		LogFinalPolySize: s.LogFinalPolySize,
		Layout:           layout,
		Roots:            roots,
		ClaimedValues:    flattenClaims(layout, claimed),
		Zeta:             s.Zeta,
		FoldAlphas:       s.FoldAlphas,
		QueryPositions:   s.Positions,
		Proof:            proof,
	}
}

func extLift(v uint64) field.Ext { return field.Lift(elem(v)) }

// buildNormalPCSScenario mirrors two sizes in one batch, multiple ext rows
// and shifts, two folding rounds.
func buildNormalPCSScenario() pcsCaseData {
	params, err := fri.NewParams(3, 2, 1)
	if err != nil {
		panic(err)
	}

	witness := make(fri.Batch, 3)
	witness[1] = fri.SizedTable{Ext: [][]field.Ext{{extLift(101), extLift(102)}}}
	witness[2] = fri.SizedTable{Ext: [][]field.Ext{
		{extLift(201), extLift(202), extLift(203), extLift(204)},
		{extLift(211), extLift(212), extLift(213), extLift(214)},
	}}

	shifts := make(fri.BatchShifts, 3)
	shifts[1] = fri.SizedShifts{Ext: [][]int{{0}}}
	shifts[2] = fri.SizedShifts{Ext: [][]int{{0}, {1}}}

	return buildPCSScenarioData(pcsScenario{
		Name:      "normal_flow",
		Params:    params,
		Encoders:  pcsMakeEncoders(3, 2),
		Witnesses: []fri.Batch{witness},
		Shifts:    []fri.BatchShifts{shifts},
		Zeta:      field.UintsToExt(19, 2, 3, 5, 7, 11),
		FoldAlphas: []field.Ext{
			field.UintsToExt(29, 1, 0, 0, 0, 0),
			field.UintsToExt(31, 0, 1, 0, 0, 0),
		},
		Positions: []int{3},
	})
}

// buildD1PCSScenario exercises the num_rounds==0 special case: a single
// size-1 batch, no folding rounds at all, verified directly against the
// (Horner-evaluated) FinalPoly.
func buildD1PCSScenario() pcsCaseData {
	params, err := fri.NewParams(2, 0, 1)
	if err != nil {
		panic(err)
	}

	witness := make(fri.Batch, 1)
	witness[0] = fri.SizedTable{Ext: [][]field.Ext{{extLift(42)}}}

	shifts := make(fri.BatchShifts, 1)
	shifts[0] = fri.SizedShifts{Ext: [][]int{{0}}}

	return buildPCSScenarioData(pcsScenario{
		Name:       "d1_top_level",
		Params:     params,
		Encoders:   pcsMakeEncoders(1, 4),
		Witnesses:  []fri.Batch{witness},
		Shifts:     []fri.BatchShifts{shifts},
		Zeta:       field.UintsToExt(7, 1, 0, 0, 0, 0),
		FoldAlphas: nil,
		Positions:  []int{1},
	})
}

// buildBoundaryPCSScenario exercises a level introduced at the boundary
// round: a main batch folding normally, plus a second batch at
// log_final_poly_size whose intro round equals numRounds() -- authenticated
// but never folded, its DEEP quotient checked for being constant across the
// conjugate pair instead.
func buildBoundaryPCSScenario() pcsCaseData {
	params, err := fri.NewParams(4, 3, 1, fri.LogFinalPolySize(1))
	if err != nil {
		panic(err)
	}

	main := make(fri.Batch, 4)
	main[3] = fri.SizedTable{Ext: [][]field.Ext{{
		extLift(301), extLift(302), extLift(303), extLift(304),
		extLift(305), extLift(306), extLift(307), extLift(308),
	}}}
	mainShifts := make(fri.BatchShifts, 4)
	mainShifts[3] = fri.SizedShifts{Ext: [][]int{{0}}}

	boundary := make(fri.Batch, 2)
	boundary[1] = fri.SizedTable{Ext: [][]field.Ext{{extLift(401), extLift(402)}}}
	boundaryShifts := make(fri.BatchShifts, 2)
	boundaryShifts[1] = fri.SizedShifts{Ext: [][]int{{0}}}

	return buildPCSScenarioData(pcsScenario{
		Name:             "boundary_round",
		Params:           params,
		LogFinalPolySize: 1,
		Encoders:         pcsMakeEncoders(4, 2),
		Witnesses:        []fri.Batch{main, boundary},
		Shifts:           []fri.BatchShifts{mainShifts, boundaryShifts},
		Zeta:             field.UintsToExt(23, 3, 5, 7, 11, 13),
		FoldAlphas: []field.Ext{
			field.UintsToExt(29, 1, 0, 0, 0, 0),
			field.UintsToExt(31, 0, 1, 0, 0, 0),
		},
		Positions: []int{5},
	})
}

// corruptBoundaryClaimData tampers the boundary batch's only claimed value
// (last in flattened order, since layout is size-descending and the
// boundary batch is the smallest size): the boundary-round DEEP quotient
// stops being constant across the conjugate pair, and checkFolds must
// reject it. No prover-ray access needed: this mutates the already-emitted
// honest case.
func corruptBoundaryClaimData(base pcsCaseData) pcsCaseData {
	c := base
	c.Name = "boundary_round_corrupted_claim"
	c.ClaimedValues = append([]field.Ext(nil), base.ClaimedValues...)
	c.ClaimedValues[len(c.ClaimedValues)-1] = extLift(999_999)
	c.ExpectVerifyError = "BoundaryAuxNotConstant"
	return c
}

// ─── Zig literal emission ───────────────────────────────────────────────────

func pcsSystemLiteral(params fri.Params, logFinalPolySize uint8, layout []pcsSizeBundle) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pcs.System{ .params = fri.Params{ .log_codeword_size = %d, .log_plaintext_size = %d, .log_final_poly_size = %d, .num_queries = %d }, .layout = &.{ ",
		params.LogCodewordSize, params.LogPlainTextSize, logFinalPolySize, params.NumQueries)
	for i, bundle := range layout {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, ".{ .size_log2 = %d, .entries = &.{ ", bundle.SizeLog2)
		for j, e := range bundle.Entries {
			if j > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, ".{ .batch_idx = %d, .is_ext = %t, .row_idx = %d, .claim_offset = %d, .shifts = &[_]usize%s }",
				e.BatchIdx, e.IsExt, e.RowIdx, e.ClaimOffset, intArrayLiteral(e.Shifts))
		}
		b.WriteString(" } }")
	}
	b.WriteString(" } }")
	return b.String()
}

func rowOpeningLiteral(r fri.RowOpening) string {
	return fmt.Sprintf("RowOpeningData{ .base = &%s, .ext = &%s }", elemSlice(r.Base), extSlice(r.Ext))
}

func inputTreeOpeningLiteral(o fri.InputTreeOpening) string {
	var b strings.Builder
	fmt.Fprintf(&b, "InputTreeOpeningData{ .siblings = &%s, .leaves = &.{ ", commitmentSlice(o.Siblings))
	for i, l := range o.Leaves {
		if i > 0 {
			b.WriteString(", ")
		}
		if l == nil {
			b.WriteString("null")
		} else {
			fmt.Fprintf(&b, "RowPairData{ %s, %s }", rowOpeningLiteral(l[0]), rowOpeningLiteral(l[1]))
		}
	}
	b.WriteString(" } }")
	return b.String()
}

func pcsProofLiteral(proof fri.OpeningProof) string {
	var b strings.Builder
	b.WriteString("OpeningProofData{ .input_queries = &.{ ")
	for q, iq := range proof.InputQueries {
		if q > 0 {
			b.WriteString(", ")
		}
		b.WriteString("&.{ ")
		for i, opening := range iq {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(inputTreeOpeningLiteral(opening))
		}
		b.WriteString(" }")
	}
	b.WriteString(" }, .fri_proof = FriProofData{ ")
	fmt.Fprintf(&b, ".round_roots = &%s, ", commitmentSlice(proof.FRIProof.RoundRoots))
	fmt.Fprintf(&b, ".final_poly = &%s, ", extSlice(proof.FRIProof.FinalPoly))
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
			fmt.Fprintf(&b, "BranchData{ .leaf = %s, .siblings = &%s }", oct8(branch.Leaf), commitmentSlice(branch.Siblings))
		}
		b.WriteString(" }")
	}
	b.WriteString(" } } }")
	return b.String()
}

func writePCSCase(out *bytes.Buffer, c pcsCaseData) {
	fmt.Fprintf(out, "    .{\n")
	fmt.Fprintf(out, "        .name = \"%s\",\n", c.Name)
	fmt.Fprintf(out, "        .system = %s,\n", pcsSystemLiteral(c.Params, c.LogFinalPolySize, c.Layout))
	fmt.Fprintf(out, "        .roots = &%s,\n", commitmentSlice(c.Roots))
	fmt.Fprintf(out, "        .claimed_values = &%s,\n", extSlice(c.ClaimedValues))
	fmt.Fprintf(out, "        .zeta = %s,\n", ext6(c.Zeta))
	fmt.Fprintf(out, "        .fold_alphas = &%s,\n", extSlice(c.FoldAlphas))
	fmt.Fprintf(out, "        .query_positions = &[_]usize%s,\n", intArrayLiteral(c.QueryPositions))
	fmt.Fprintf(out, "        .proof = %s,\n", pcsProofLiteral(c.Proof))
	if c.ExpectVerifyError != "" {
		fmt.Fprintf(out, "        .expect_verify_error = \"%s\",\n", c.ExpectVerifyError)
	}
	fmt.Fprintf(out, "    },\n")
}

func writePCSFixtures() error {
	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by verifier-ray/testdata/generate/fri; DO NOT EDIT.")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "const verifier_ray = @import(\"verifier_ray\");")
	fmt.Fprintln(&out, "const pcs = verifier_ray.query.pcs;")
	fmt.Fprintln(&out, "const fri = verifier_ray.query.fri;")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "pub const RowOpeningData = struct { base: []const u32, ext: []const [6]u32 };")
	fmt.Fprintln(&out, "pub const RowPairData = [2]RowOpeningData;")
	fmt.Fprintln(&out, "pub const InputTreeOpeningData = struct { siblings: []const [8]u32, leaves: []const ?RowPairData };")
	fmt.Fprintln(&out, "pub const BranchData = struct { leaf: [8]u32, siblings: []const [8]u32 };")
	fmt.Fprintln(&out, "pub const FriProofData = struct { round_roots: []const [8]u32, final_poly: []const [6]u32, running_queries: []const []const BranchData };")
	fmt.Fprintln(&out, "pub const OpeningProofData = struct { input_queries: []const []const InputTreeOpeningData, fri_proof: FriProofData };")
	fmt.Fprintln(&out, "pub const PcsCase = struct { name: []const u8, system: pcs.System, roots: []const [8]u32, claimed_values: []const [6]u32, zeta: [6]u32, fold_alphas: []const [6]u32, query_positions: []const usize, proof: OpeningProofData, expect_verify_error: []const u8 = \"\" };")
	fmt.Fprintln(&out)

	boundary := buildBoundaryPCSScenario()
	cases := []pcsCaseData{
		buildNormalPCSScenario(),
		buildD1PCSScenario(),
		boundary,
		corruptBoundaryClaimData(boundary),
	}

	fmt.Fprintln(&out, "pub const pcs_cases = [_]PcsCase{")
	for _, c := range cases {
		writePCSCase(&out, c)
	}
	fmt.Fprintln(&out, "};")

	data := out.Bytes()
	zigfmt, err := runZigFmt(data)
	if err == nil {
		data = zigfmt
	}
	return os.WriteFile(filepath.Join("..", "..", "generated", "pcs.zig"), data, 0o644)
}
