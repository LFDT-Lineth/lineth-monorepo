package distributed_test

import (
	"testing"
	"time"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/sirupsen/logrus"
)

// runProverGLs executes the prover for each GL module segment. It takes in a list of
// compiled GL segments and corresponding witnesses, then runs the prover for each
// segment. The function logs the start and end times of the prover execution for each
// segment. It returns a slice of ProverRuntime instances, each representing the
// result of the prover execution for a segment.
func runProverGLs(
	t *testing.T,
	distWizard *distributed.DistributedWizard,
	witnessGLs []*distributed.ModuleWitnessGL,
) (proofs []*distributed.SegmentProof) {

	var (
		compiledGLs = distWizard.CompiledGLs
	)

	proofs = make([]*distributed.SegmentProof, len(witnessGLs))

	for i := range witnessGLs {

		var (
			witnessGL = witnessGLs[i]
			moduleGL  *distributed.RecursedSegmentCompilation
		)

		t.Logf("segment(total)=%v module=%v module.index=%v segment.index=%v", i, witnessGL.ModuleName, witnessGL.ModuleIndex, witnessGL.SegmentModuleIndex)
		for k := range distWizard.ModuleNames {
			if distWizard.ModuleNames[k] != witnessGLs[i].ModuleName {
				continue
			}
			moduleGL = compiledGLs[k]
		}

		if moduleGL == nil {
			t.Fatalf("module does not exists, module=%v, distWizard.ModuleNames=%v", witnessGL.ModuleName, distWizard.ModuleNames)
		}

		t.Logf("RUNNING THE GL PROVER: %v", time.Now())
		proofs[i] = moduleGL.ProveSegmentKoala(witnessGL)
		t.Logf("RUNNING THE GL PROVER - DONE: %v", time.Now())

	}

	return proofs
}

// runProverLPPs runs a prover for a LPP segment. It takes in a DistributedWizard
// object, a slice of RecursedSegmentCompilation objects, and a slice of
// ModuleWitnessLPP objects. It runs the prover for each segment and logs the
// time at which the prover starts and ends. It returns a slice of ProverRuntime
// instances, each representing the result of the prover execution for a segment.
func runProverLPPs(
	t *testing.T,
	distWizard *distributed.DistributedWizard,
	sharedRandomness field.Octuplet,
	witnessLPPs []*distributed.ModuleWitnessLPP,
) []*distributed.SegmentProof {

	var (
		proofs = make([]*distributed.SegmentProof, len(witnessLPPs))
	)

	for i := range witnessLPPs {

		var (
			witnessLPP  = witnessLPPs[i]
			moduleIndex = witnessLPP.ModuleIndex
			moduleLPP   = distWizard.CompiledLPPs[moduleIndex]
		)

		witnessLPP.InitialFiatShamirState = sharedRandomness

		t.Logf("segment(total)=%v module=%v module.index=%v segment.index=%v", i, witnessLPP.ModuleName, witnessLPP.ModuleIndex, witnessLPP.SegmentModuleIndex)
		t.Logf("RUNNING THE LPP PROVER: %v", time.Now())
		proofs[i] = moduleLPP.ProveSegmentKoala(witnessLPP)
		t.Logf("RUNNING THE LPP PROVER - DONE: %v", time.Now())
	}

	return proofs
}

// This function runs a prover for a conglomerator compilation. It takes in a
// ConglomeratorCompilation object and two slices of ProverRuntime objects,
// runGLs and runLPPs. It extracts witnesses from these runtimes, then uses the
// ConglomeratorCompilation object to prove the conglomerator, logging the start
// and end times of the proof process.
func runConglomerationProver(
	mt *distributed.VerificationKeyMerkleTree,
	cong *distributed.RecursedSegmentCompilation,
	runGLs, runLPPs []*distributed.SegmentProof,
) *distributed.SegmentProof {
	// The channel is used as a FIFO queue to store the remaining proofs to be
	// aggregated.

	remainingProofs := make(chan *distributed.SegmentProof, len(runGLs)+len(runLPPs))

	// This populates the queue
	for i := range runGLs {
		remainingProofs <- runGLs[i]
	}

	for i := range runLPPs {
		remainingProofs <- runLPPs[i]
	}

	// TryPopQueue attempts to consume a proof from the queue or return false
	// if the queue is empty.
	tryPopQueue := func() (*distributed.SegmentProof, bool) {
		select {
		case proof := <-remainingProofs:
			return proof, true
		default:
			return nil, false
		}
	}

	// This is the actual proof aggregation loop.
	for {
		a, ok := tryPopQueue()
		if !ok {
			panic("the queue cannot be empty here")
		}

		// If b cannot be found, it means that the queue contained only a single
		// proof to aggregate which means it was the result of the function.
		b, ok := tryPopQueue()
		if !ok {
			return a
		}

		logrus.Infof("AGGREGATING PROOF, remaining %v\n", len(remainingProofs))

		new := cong.ProveSegmentKoala(&distributed.ModuleWitnessConglo{
			SegmentProofs:             []distributed.SegmentProof{*a, *b},
			VerificationKeyMerkleTree: *mt,
		})

		logrus.Infof("AGGREGATED PROOFS\n")

		remainingProofs <- new
	}
}
