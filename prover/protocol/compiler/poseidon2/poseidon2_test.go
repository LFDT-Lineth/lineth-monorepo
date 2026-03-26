package poseidon2

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logdata"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

func TestPoseidon2(t *testing.T) {

	define := func(builder *wizard.Builder) {

		a := [8]ifaces.Column{
			builder.RegisterCommit("A0", 1024),
			builder.RegisterCommit("A1", 1024),
			builder.RegisterCommit("A2", 1024),
			builder.RegisterCommit("A3", 1024),
			builder.RegisterCommit("A4", 1024),
			builder.RegisterCommit("A5", 1024),
			builder.RegisterCommit("A6", 1024),
			builder.RegisterCommit("A7", 1024),
		}

		b := [8]ifaces.Column{
			builder.RegisterCommit("B0", 1024),
			builder.RegisterCommit("B1", 1024),
			builder.RegisterCommit("B2", 1024),
			builder.RegisterCommit("B3", 1024),
			builder.RegisterCommit("B4", 1024),
			builder.RegisterCommit("B5", 1024),
			builder.RegisterCommit("B6", 1024),
			builder.RegisterCommit("B7", 1024),
		}

		c := [8]ifaces.Column{
			builder.RegisterCommit("C0", 1024),
			builder.RegisterCommit("C1", 1024),
			builder.RegisterCommit("C2", 1024),
			builder.RegisterCommit("C3", 1024),
			builder.RegisterCommit("C4", 1024),
			builder.RegisterCommit("C5", 1024),
			builder.RegisterCommit("C6", 1024),
			builder.RegisterCommit("C7", 1024),
		}

		builder.CompiledIOP.InsertPoseidon2(0, "POSEIDON2", a, b, c, nil)
	}

	comp := wizard.Compile(define, CompilePoseidon2)
	stats := logdata.GetWizardStats(comp)
	fmt.Printf("stats=%+v\n", stats)
}
