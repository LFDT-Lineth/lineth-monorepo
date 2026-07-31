// Real-protocol FRI/PCS fixture generator.
//
// This is the deployment of PCS on a REAL protocol (as opposed to `coexist.go`'s
// minimal hand-built System): it takes a genuine prover-ray `wioptest`
// vanishing scenario, runs the FULL arithmetization pipeline PLUS `pcs.Compile`,
// proves it, and emits a PCS-enabled `verifier.Systems` — vanishing + PCS both
// active, sharing one transcript — into `testdata/generated/realpcs.zig`.
//
// It reuses `coexist.go`'s generic `extractPcsFixture` + `emitCoexist`: those
// read only the compiled System, the LagrangeEval openings, the coin routing,
// and the runtime's opening proof, so they work unchanged on a real scenario.
// The ONLY realpcs-specific part is choosing the scenario and running the
// pipeline with PCS enabled.
//
// Regenerate with: cd testdata/generate && go run . -realpcs
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
)

// compilePipelineWithPCS is compilePipelineWithoutPCS (main.go) followed by the
// PCS commitment pass. pcs.Compile MUST run last, after every arithmetization
// pass has registered its columns and LagrangeEval queries (see the pcs package
// doc).
//
// It reuses compilePipelineWithoutPCS rather than repeating the arithmetization
// passes so the two pipelines cannot drift: a pass added to one is added to both.
func compilePipelineWithPCS(sys *wiop.System) {
	compilePipelineWithoutPCS(sys)
	pcscompiler.Compile(sys)
}

// pickRealPCSScenario returns the first vanishing scenario that, after the full
// PCS pipeline, actually commits columns and opens LagrangeEval claims — i.e. a
// genuine protocol the PCS layer has something to authenticate. Falls back to a
// clear panic if none qualifies (so a prover-ray change that removes committed
// columns fails loudly rather than emitting an empty fixture).
func pickRealPCSScenario() *wioptest.VanishingScenario {
	for _, factory := range wioptest.VanishingScenarios() {
		sc := factory()
		probe := factory() // independent System: compiling mutates it
		compilePipelineWithPCS(probe.Sys)
		if len(pcscompiler.CommittedBatches(probe.Sys)) > 0 && len(probe.Sys.LagrangeEvals) > 0 {
			return sc
		}
	}
	panic("realpcs: no vanishing scenario commits columns + opens LagrangeEvals")
}

func buildRealPCSFixture() coexistFixture {
	// Match the coexist query count so the fixture stays small and fast; the
	// real protocol's soundness parameters are a deployment concern, not a
	// codegen one, and the verifier checks whatever the proof declares.
	pcscompiler.SetFRINumQueriesForTest(1)

	sc := pickRealPCSScenario()
	compilePipelineWithPCS(sc.Sys)
	rt := runProver(sc.Sys, sc.AssignHonest)
	return extractPcsFixture(sc.Sys, rt)
}

func writeRealPCSFixture() error {
	fx := buildRealPCSFixture()
	var out bytes.Buffer
	emitCoexist(&out, fx)
	data := out.Bytes()
	if formatted, err := runZigFmt(data); err == nil {
		data = formatted
	}
	return os.WriteFile(filepath.Join("..", "generated", "realpcs.zig"), data, 0o644)
}

// realPCSScenarioName reports which scenario was selected, for the -realpcs
// run's stdout (so a regen shows what real protocol backs the fixture).
func realPCSScenarioName() string {
	sc := pickRealPCSScenario()
	return fmt.Sprintf("realpcs scenario: %q", sc.Name)
}
