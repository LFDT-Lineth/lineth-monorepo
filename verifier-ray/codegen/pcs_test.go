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

// newCommittedVanishing builds a size-4 module with one boundary vanishing
// constraint on a single committed oracle column, then compiles the global +
// PCS passes and proves it. It returns the compiled system and its runtime so a
// test can extract the PCS System the way a real protocol would.
//
// The constraint L_1 * (col - 99) forces col[1] == 99; the honest assignment
// {7,99,7,7} satisfies it. global.Compile turns the vanishing into quotient/eval
// rounds with LagrangeEval openings at zeta; pcs.Compile commits the column and
// produces the opening proof — exactly the shape BuildPcsSystem consumes.
func newCommittedVanishing(t *testing.T) (*wiop.System, *wiop.Runtime) {
	t.Helper()
	pcscompiler.SetFRINumQueriesForTest(1)

	sys := wiop.NewSystemf("pcs-codegen")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)

	// boundary: L_1 * (col - 99) == 0  →  col[1] == 99.
	expr := wiop.Mul(
		wiop.NewLagrangeSelector(mod, 1),
		wiop.Sub(col.View(), wiop.NewConstantField(field.NewFromString("99"))),
	)
	mod.NewVanishing(sys.Context.Childf("boundary"), expr)

	global.Compile(sys)
	pcscompiler.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, concretePcsBase(
		field.NewFromString("7"), field.NewFromString("99"),
		field.NewFromString("7"), field.NewFromString("7"),
	))
	runPcsProver(rt)
	return sys, rt
}

func concretePcsBase(vs ...field.Element) *wiop.ConcreteVector {
	return &wiop.ConcreteVector{Plain: field.VecFromBase(vs)}
}

func runPcsProver(rt *wiop.Runtime) {
	for _, action := range rt.CurrentRound().ProverActions {
		action.Run(rt)
	}
	for rt.CurrentRound().ID < len(rt.System.Rounds)-1 {
		rt.AdvanceRound()
		for _, action := range rt.CurrentRound().ProverActions {
			action.Run(rt)
		}
	}
}

func TestBuildPcsSystemShapeAndMaps(t *testing.T) {
	sys, rt := newCommittedVanishing(t)

	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	van, err := BuildVanishingSystem(sys, routing)
	if err != nil {
		t.Fatalf("BuildVanishingSystem() error = %v", err)
	}
	pcs, err := BuildPcsSystem(sys, rt, routing)
	if err != nil {
		t.Fatalf("BuildPcsSystem() error = %v", err)
	}

	// The whole soundness link: one claim map entry per vanishing claim, so
	// verifier.verify can re-slice every witness/quotient claim from the
	// PCS-authenticated entry_claims. A mismatch here is exactly the "unbuilt
	// link" the audit flagged.
	if len(pcs.WitnessMap) != van.TotalWitnessClaims {
		t.Fatalf("witness_map has %d refs, want TotalWitnessClaims %d", len(pcs.WitnessMap), van.TotalWitnessClaims)
	}
	if len(pcs.QuotientMap) != van.TotalQuotientClaims {
		t.Fatalf("quotient_map has %d refs, want TotalQuotientClaims %d", len(pcs.QuotientMap), van.TotalQuotientClaims)
	}

	// Every ref must land inside the flat entry layout. totalEntries is the sum
	// of every batch's base+ext widths across sizes.
	totalEntries := 0
	for _, shape := range pcs.Shapes {
		for _, s := range shape {
			totalEntries += s.BaseWidth + s.ExtWidth
		}
	}
	for i, ref := range append(append([]PcsClaimRef{}, pcs.WitnessMap...), pcs.QuotientMap...) {
		if ref.Entry < 0 || ref.Entry >= totalEntries {
			t.Fatalf("claim ref %d entry %d out of range [0,%d)", i, ref.Entry, totalEntries)
		}
		if ref.Shift < 0 {
			t.Fatalf("claim ref %d has negative shift %d", i, ref.Shift)
		}
	}

	// zeta must be the shared LagrangeEval coin, in range of the flat coins.
	if pcs.ZetaCoinIndex < 0 || pcs.ZetaCoinIndex >= routing.TotalRoundCoins {
		t.Fatalf("zeta_coin_index %d out of range [0,%d)", pcs.ZetaCoinIndex, routing.TotalRoundCoins)
	}

	// Params sanity: codeword strictly larger than plaintext, at least one query,
	// at least one committed batch.
	if pcs.LogCodewordSize <= pcs.LogPlaintextSize {
		t.Fatalf("log_codeword_size %d must exceed log_plaintext_size %d", pcs.LogCodewordSize, pcs.LogPlaintextSize)
	}
	if pcs.NumQueries < 1 {
		t.Fatalf("num_queries %d must be >= 1", pcs.NumQueries)
	}
	if pcs.NumBatches < 1 {
		t.Fatalf("num_batches %d must be >= 1", pcs.NumBatches)
	}

	// Batch roots: one per batch, each bound to the transcript. This committed-
	// vanishing protocol has no precomputed batch, so every root is an
	// interactive round reference whose index is a valid proof.rounds slot.
	if len(pcs.BatchRoots) != pcs.NumBatches {
		t.Fatalf("batch_roots has %d entries, want num_batches %d", len(pcs.BatchRoots), pcs.NumBatches)
	}
	for i, br := range pcs.BatchRoots {
		if br.Precomputed {
			t.Fatalf("batch %d unexpectedly marked precomputed", i)
		}
		if br.RoundIndex < 0 || br.RoundIndex >= len(sys.Rounds)-1 {
			// -1: the trailing opening round carries no message and is not in
			// proof.rounds, so a batch's round index must be strictly before it.
			t.Fatalf("batch %d round index %d out of proof.rounds range [0,%d)",
				i, br.RoundIndex, len(sys.Rounds)-1)
		}
	}
}

func TestWritePcsSystemZigRenders(t *testing.T) {
	sys, rt := newCommittedVanishing(t)
	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	pcs, err := BuildPcsSystem(sys, rt, routing)
	if err != nil {
		t.Fatalf("BuildPcsSystem() error = %v", err)
	}

	var out bytes.Buffer
	// Use the short qualified imports the multi-system files use (matching the
	// committed coexist.zig header: pcs / layout / pcsverify), so the assertions
	// below check the exact module a real generated file references.
	if err := WritePcsSystemZig(&out, pcs, PcsZigOptions{
		PcsImport: "pcs", LayoutImport: "layout", VerifyImport: "pcsverify", ConstPrefix: "system_0_",
	}); err != nil {
		t.Fatalf("WritePcsSystemZig() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		// Params must be referenced through the pcs ROOT module (pcs.Params),
		// not verify.zig — verify.zig does not export Params, so `verify.Params`
		// would not compile. This asserts the correct qualified reference.
		"pub const system_0_params = pcs.Params{",
		"pub const system_0_shapes = ",
		"pub const system_0_shifts = ",
		"const system_0_witness_map = ",
		"const system_0_quotient_map = ",
		"const system_0_batch_roots = [_]pcsverify.BatchRoot{",
		"pub const system_0_pcs_system = ",
		".layout = ",
		".batch_roots = &system_0_batch_roots,",
		".zeta_coin_index = ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted Zig missing %q\n---\n%s", want, got)
		}
	}
	// Guard against the regression where Params is emitted as `verify.Params`
	// (the default VerifyImport) — that identifier does not exist and the
	// generated file would fail to compile.
	if strings.Contains(got, "verify.Params") || strings.Contains(got, "pcsverify.Params") {
		t.Errorf("Params must not be qualified with the verify module; got:\n%s", got)
	}
}

// TestBuildPcsSystemNoBatches confirms a protocol that commits nothing is
// rejected with a clear error rather than producing a degenerate System.
func TestBuildPcsSystemNoBatches(t *testing.T) {
	sys := wiop.NewSystemf("pcs-empty")
	sys.NewRound()
	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	if _, err := BuildPcsSystem(sys, wiop.NewRuntime(sys), routing); err == nil {
		t.Fatalf("BuildPcsSystem() on a system with no committed batches must error")
	}
}

// TestBuildPcsSystemMatchesCoexistFixture pins BuildPcsSystem's output for the
// exact protocol the committed coexist.zig fixture was hand-generated from, so
// the reusable codegen provably reproduces the known-good claim link (witness
// entry 0, quotient entry 1, zeta coin 1) rather than merely being internally
// consistent.
func TestBuildPcsSystemMatchesCoexistFixture(t *testing.T) {
	sys, rt := newCommittedVanishing(t)
	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	pcs, err := BuildPcsSystem(sys, rt, routing)
	if err != nil {
		t.Fatalf("BuildPcsSystem() error = %v", err)
	}

	// coexist.zig: witness_map = {.entry=0,.shift=0}, quotient_map = {.entry=1,.shift=0}.
	if len(pcs.WitnessMap) != 1 || pcs.WitnessMap[0] != (PcsClaimRef{Entry: 0, Shift: 0}) {
		t.Fatalf("witness_map = %+v, want [{0 0}]", pcs.WitnessMap)
	}
	if len(pcs.QuotientMap) != 1 || pcs.QuotientMap[0] != (PcsClaimRef{Entry: 1, Shift: 0}) {
		t.Fatalf("quotient_map = %+v, want [{1 0}]", pcs.QuotientMap)
	}
	// coexist.zig: zeta_coin_index = 1; params log_codeword=3 log_plain=2 nq=1.
	if pcs.ZetaCoinIndex != 1 {
		t.Fatalf("zeta_coin_index = %d, want 1", pcs.ZetaCoinIndex)
	}
	if pcs.LogCodewordSize != 3 || pcs.LogPlaintextSize != 2 || pcs.NumQueries != 1 {
		t.Fatalf("params = (codeword %d, plain %d, nq %d), want (3, 2, 1)",
			pcs.LogCodewordSize, pcs.LogPlaintextSize, pcs.NumQueries)
	}
	// coexist.zig: num_batches = 2 — the witness column round (batch 0, entry 0)
	// and the quotient-share round (batch 1, entry 1).
	if pcs.NumBatches != 2 {
		t.Fatalf("num_batches = %d, want 2 (witness round + quotient-share round)", pcs.NumBatches)
	}
	// coexist.zig: batch_roots = { .round = 0 }, { .round = 1 } — batch 0's root is
	// round 0's oracle commitment, batch 1's is round 1's. Neither is precomputed.
	want := []PcsBatchRoot{
		{Precomputed: false, RoundIndex: 0},
		{Precomputed: false, RoundIndex: 1},
	}
	if len(pcs.BatchRoots) != len(want) {
		t.Fatalf("batch_roots = %+v, want %+v", pcs.BatchRoots, want)
	}
	for i, w := range want {
		if pcs.BatchRoots[i].Precomputed != w.Precomputed || pcs.BatchRoots[i].RoundIndex != w.RoundIndex {
			t.Fatalf("batch_roots[%d] = %+v, want %+v", i, pcs.BatchRoots[i], w)
		}
	}
}
