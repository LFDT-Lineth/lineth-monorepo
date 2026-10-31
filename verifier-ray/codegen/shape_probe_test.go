package codegen

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

// TestR5SystemShape compiles the real R5 arithmetization entrypoint and prints
// the shape of the resulting CompiledSystem. It proves nothing — only
// runCompilePipeline + BuildCompiledSystem run — so it needs no witness, no ELF
// and about a second.
//
// The numbers it prints are the inputs to the recursion row budget in
// wiop-agg-design.md: a verifier of one shard proof pays
//
//	Q * 2 * (opened felt width)   rows of proof reading and Poseidon2 input,
//	Q * 2 * (columns + shifts)    rows of extension-field multiplication,
//	(expression nodes)            rows of vanishing-constraint evaluation,
//
// so `num_queries` and `opened felt width` are the two figures that decide
// whether in-circuit recursion fits a 2^22-row shard. Re-run this whenever the
// arithmetization changes and update that document if they move.
func TestR5SystemShape(t *testing.T) {
	binF, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("compiling embedded binary: %v", err)
	}
	compiled, err := binF.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	sys := wiop.NewSystemf("probe")
	sys.NewRound()
	_ = zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiled))
	runCompilePipeline(sys)

	cs, err := BuildCompiledSystem(sys)
	if err != nil {
		t.Fatalf("BuildCompiledSystem: %v", err)
	}

	// PCS: committed columns, split base/ext, and total opened felt width.
	var baseCols, extCols, totalShifts, feltWidth int
	sizeHist := map[int]int{}
	for _, c := range cs.Pcs.Columns {
		if c.IsExt {
			extCols++
			feltWidth += 6
		} else {
			baseCols++
			feltWidth++
		}
		totalShifts += len(c.Shifts)
		if c.IsDynamic {
			sizeHist[-1-c.DynamicIndex]++
		} else {
			sizeHist[c.SizeLog2]++
		}
	}

	// Vanishing: modules, expression nodes, constraints, buckets.
	var exprNodes, vanishings, buckets, opNodes, selNodes, cancelled int
	maxExprInModule := 0
	for _, m := range cs.Vanishing.Modules {
		exprNodes += len(m.Expressions)
		if len(m.Expressions) > maxExprInModule {
			maxExprInModule = len(m.Expressions)
		}
		for _, e := range m.Expressions {
			switch e.Kind {
			case ExprOp:
				opNodes++
			case ExprLagrangeSelector:
				selNodes++
			}
		}
		buckets += len(m.Buckets)
		for _, b := range m.Buckets {
			vanishings += len(b.Vanishings)
			for _, v := range b.Vanishings {
				cancelled += len(v.CancelledPositions)
			}
		}
	}

	var ldRefs, gpRefs int
	for _, q := range cs.LogDeriv.Queries {
		ldRefs += len(q.ZFinalRefs)
	}
	for _, q := range cs.GrandProduct.Queries {
		gpRefs += len(q.ZFinalRefs)
	}
	var rlMods int
	for _, c := range cs.RowLimit.Checks {
		rlMods += len(c.IncludedModules) + len(c.IncludingsModules)
	}
	totalCells := 0
	for _, n := range cs.PublicInput.RoundCellCounts {
		totalCells += n
	}

	fmt.Printf("\n=== R5 COMPILED SYSTEM SHAPE ===\n")
	fmt.Printf("rounds                 %d\n", len(cs.Routing.RoundCoinCounts)-1)
	fmt.Printf("total coins            %d\n", cs.Routing.TotalRoundCoins)
	fmt.Printf("dynamic modules        %d\n", cs.Routing.DynamicModuleCount)
	fmt.Printf("transcript cells       %d  (per-round %v)\n", totalCells, cs.PublicInput.RoundCellCounts)
	fmt.Printf("public inputs          %d\n", len(cs.PublicInput.Refs))
	fmt.Printf("--- PCS ---\n")
	fmt.Printf("num_queries            %d\n", cs.Pcs.NumQueries)
	fmt.Printf("log_codeword/plaintext %d / %d  (final_poly %d)\n", cs.Pcs.LogCodewordSize, cs.Pcs.LogPlaintextSize, cs.Pcs.LogFinalPolySize)
	fmt.Printf("batches                %d\n", cs.Pcs.NumBatches)
	fmt.Printf("committed columns      %d  (base %d, ext %d)\n", len(cs.Pcs.Columns), baseCols, extCols)
	fmt.Printf("opened felt width      %d   <-- felts per opened row set\n", feltWidth)
	fmt.Printf("total (col,shift)      %d\n", totalShifts)
	fmt.Printf("size histogram         %v   (negative key = -1-dynamicIndex)\n", sizeHist)
	fmt.Printf("witness/quotient claims %d / %d\n", cs.Vanishing.TotalWitnessClaims, cs.Vanishing.TotalQuotientClaims)
	fmt.Printf("--- VANISHING ---\n")
	fmt.Printf("modules                %d\n", len(cs.Vanishing.Modules))
	fmt.Printf("expression nodes       %d   <-- eval_expr rows per proof\n", exprNodes)
	fmt.Printf("  of which op / sel    %d / %d\n", opNodes, selNodes)
	fmt.Printf("max nodes in a module  %d\n", maxExprInModule)
	fmt.Printf("buckets / vanishings   %d / %d\n", buckets, vanishings)
	fmt.Printf("cancelled positions    %d\n", cancelled)
	fmt.Printf("--- SCALAR CHECKS ---\n")
	fmt.Printf("logderiv queries/refs  %d / %d\n", len(cs.LogDeriv.Queries), ldRefs)
	fmt.Printf("grandprod queries/refs %d / %d\n", len(cs.GrandProduct.Queries), gpRefs)
	fmt.Printf("rowlimit checks/mods   %d / %d\n", len(cs.RowLimit.Checks), rlMods)
	fmt.Printf("shared-randomness refs %d (commitment round %d, hasCommitment %v)\n",
		len(cs.SharedRandomness.ContributionRefs),
		cs.SharedRandomness.CommitmentRound.RoundIndex,
		cs.SharedRandomness.CommitmentRound.HasCommitment)
	for i, q := range cs.GrandProduct.Queries {
		fmt.Printf("  gp[%d] hasExpected=%v expected=%d zrefs=%d\n", i, q.HasExpected, q.Expected, len(q.ZFinalRefs))
	}
	for i, q := range cs.LogDeriv.Queries {
		fmt.Printf("  ld[%d] resultIsZero=%v zrefs=%d\n", i, q.ResultIsZero, len(q.ZFinalRefs))
	}

	// Cross-shard readiness. Both of these are false/absent for a single-shard
	// R5 compilation, and an aggregator can only bind shards together once they
	// are not. See wiop-agg-design.md section 6.
	deferred := 0
	for _, q := range cs.GrandProduct.Queries {
		if !q.HasExpected {
			deferred++
		}
	}
	contributionBinds := len(cs.SharedRandomness.ContributionRefs) > 0 &&
		cs.SharedRandomness.CommitmentRound.HasCommitment
	fmt.Printf("--- CROSS-SHARD READINESS ---\n")
	fmt.Printf("contribution binds a commitment  %v\n", contributionBinds)
	if !contributionBinds && len(cs.SharedRandomness.ContributionRefs) > 0 {
		fmt.Printf("  (coin round %d has no commitment, so both sides hash a zero\n"+
			"   octuplet and the check is a tautology — see design doc section 6.1)\n",
			cs.SharedRandomness.CommitmentRound.RoundIndex)
	}
	fmt.Printf("grandproduct handles deferred    %d of %d\n", deferred, len(cs.GrandProduct.Queries))
	fmt.Printf("=== END ===\n\n")
}
