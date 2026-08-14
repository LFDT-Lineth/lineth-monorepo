package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

func TestCompileToZigWritesAllSubsystems(t *testing.T) {
	sys, _ := newBooleanPcsSystem(t)

	var buf bytes.Buffer
	opts := CompileToZigOptions{
		CompiledSystemZigOptions: CompiledSystemZigOptions{
			EmitHeader:         true,
			ProtocolImport:     `@import("verifier_ray").protocol`,
			FieldImport:        `@import("verifier_ray").field.koalabear`,
			VanishingImport:    `@import("verifier_ray").query.vanishing`,
			LogDerivImport:     `@import("verifier_ray").query.logderivativesum`,
			GrandProductImport: `@import("verifier_ray").query.grandproduct`,
			RowLimitImport:     `@import("verifier_ray").query.rowlimit`,
		},
		Pcs: PcsZigOptions{
			PcsImport:   `@import("verifier_ray").query.pcs`,
			FriImport:   `@import("verifier_ray").query.fri`,
			FieldImport: `@import("verifier_ray").field.koalabear`,
			EmitHeader:  false,
		},
	}
	if err := CompileToZig(sys, 0, &buf, opts); err != nil {
		t.Fatalf("CompileToZig() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"pub const system_0_spec = protocol.Spec{",
		"const system_0 = vanishing.System{",
		"const system_0_grandproduct = grandproduct.System{",
		"const system_0_rowlimit = rowlimit.System{",
		"pub const pcs_system_0 = pcs.System{",
		"pub const system_0_systems = verifier.Systems{",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("CompileToZig() output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestCompileToZigFailsClosedOnUnhandledAction(t *testing.T) {
	// A system with no PCS/vanishing/etc at all is still "handled" (an empty
	// system has no unhandled actions), but BuildPcsSystem should fail since
	// pcs.Compile never ran — CompileToZig must surface that error rather
	// than silently emit an incomplete system.
	sys := wiop.NewSystemf("no-pcs")
	sys.NewRound()
	sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	var buf bytes.Buffer
	if err := CompileToZig(sys, 0, &buf, CompileToZigOptions{}); err == nil {
		t.Fatalf("CompileToZig() error = nil, want an error for a non-PCS-compiled system")
	}
}
