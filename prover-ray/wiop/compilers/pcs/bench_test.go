package pcs_test

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
)

// selfAssignLagrange fills a LagrangeEval's claim cells from the committed
// assignments; in the full pipeline the global pass owns this.
type selfAssignLagrange struct{ le *wiop.LagrangeEval }

func (a *selfAssignLagrange) Run(rt *wiop.Runtime) { a.le.SelfAssign(rt) }

// rsBlowupBenchWorkload is a synthetic PCS workload: numCols base and numCols
// extension columns of 2^rowsLog2 rows committed in round 0, all opened at one
// round-1 coin.
type rsBlowupBenchWorkload struct {
	rowsLog2 int
	numCols  int
}

var rsBlowupBenchWorkloads = []rsBlowupBenchWorkload{
	{rowsLog2: 12, numCols: 8},
	{rowsLog2: 18, numCols: 8},
}

// newRSBlowupBenchSystem compiles w at cfg and returns the System with the
// honest assignment.
func newRSBlowupBenchSystem(w rsBlowupBenchWorkload, cfg wioptest.RSBlowupBenchConfig) (*wiop.System, func(*wiop.Runtime)) {
	var (
		sys   = wiop.NewSystemf("pcs-rs-blowup-bench")
		r0    = sys.NewRound()
		r1    = sys.NewRound()
		rows  = 1 << w.rowsLog2
		mod   = sys.NewSizedModule(sys.Context.Childf("mod"), rows, wiop.PaddingDirectionNone)
		cols  = make([]*wiop.Column, 0, 2*w.numCols)
		vecs  = make([]*wiop.ConcreteVector, 0, 2*w.numCols)
		views = make([]*wiop.ColumnView, 0, 2*w.numCols)
	)
	for i := range w.numCols {
		base := make([]field.Element, rows)
		ext := make([]field.Ext, rows)
		for j := range rows {
			base[j].SetUint64(uint64(i*rows + j))
			ext[j].B0.A0.SetUint64(uint64(i*rows + j + 1))
		}
		cols = append(cols,
			mod.NewColumn(sys.Context.Childf("base-%d", i), r0),
			mod.NewExtensionColumn(sys.Context.Childf("ext-%d", i), r0))
		vecs = append(vecs,
			&wiop.ConcreteVector{Plain: field.VecFromBase(base)},
			&wiop.ConcreteVector{Plain: field.VecFromExt(ext)})
	}
	for _, col := range cols {
		views = append(views, col.View())
	}
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), views, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	pcs.Compile(sys, cfg.PCSOptions()...)

	return sys, func(rt *wiop.Runtime) {
		for i, col := range cols {
			rt.AssignColumn(col, vecs[i])
		}
	}
}

// BenchmarkRSBlowup compares RS blowups, each at its paired query count, on
// synthetic PCS-only Systems. prove covers the round commitments (RS encoding)
// and the FRI opening; both phases report the proof image size:
//
//	go test -run=NONE -bench=BenchmarkRSBlowup -benchmem ./wiop/compilers/pcs/
func BenchmarkRSBlowup(b *testing.B) {
	for _, w := range rsBlowupBenchWorkloads {
		for _, cfg := range wioptest.RSBlowupBenchConfigs() {
			name := fmt.Sprintf("rows=2^%d/cols=%d/%s", w.rowsLog2, 2*w.numCols, cfg.Name())
			b.Run("prove/"+name, func(b *testing.B) {
				sys, assign := newRSBlowupBenchSystem(w, cfg)
				// The first proof at a blowup builds the process-wide FRI
				// envelope; keep that one-time cost out of the timed loop.
				proof, pub := sys.Prove(assign)
				b.ReportAllocs()
				for b.Loop() {
					proof, pub = sys.Prove(assign)
				}
				reportProofBytes(b, sys, proof, pub)
			})
			b.Run("verify/"+name, func(b *testing.B) {
				sys, assign := newRSBlowupBenchSystem(w, cfg)
				proof, pub := sys.Prove(assign)
				b.ReportAllocs()
				for b.Loop() {
					if err := sys.Verify(proof, pub); err != nil {
						b.Fatalf("honest proof must verify: %v", err)
					}
				}
				reportProofBytes(b, sys, proof, pub)
			})
		}
	}
}

func reportProofBytes(b *testing.B, sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) {
	b.Helper()
	b.ReportMetric(float64(proofserialization.Measure(sys, proof, pub).Total), "proof-bytes")
}
