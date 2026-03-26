package recursion

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/smt"
	vCom "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// AssignVortexUAlpha assigns the UAlpha column for all the subproofs. As
// for [PreVortexVerifierStep], this step should be run after the corresponding
// proofs have been added to the runtime states.
type AssignVortexUAlpha struct {
	Ctxs *Recursion
}

// AssignVortexOpenedCols assigns the OpenedCols for all the subproofs. As
// for [PreVortexVerifierStep], this step should be run after the corresponding
// proofs have been added to the runtime states.
type AssignVortexOpenedCols struct {
	Ctxs *Recursion
}

// ConsistencyCheck checks that the Vortex statements are consistents with
// the public-inputs of the plonk-in-wizard circuits.
type ConsistencyCheck struct {
	Ctx       *Recursion
	isSkipped bool `serde:"omit"`
	PIs       []ifaces.Column
}

// ExtractWitness extracts a [Witness] from a prover runtime toward being conglomerated.
func ExtractWitness(run *wizard.ProverRuntime) Witness {

	var (
		pcs               = run.Spec.PcsCtxs.(*vortex.Ctx)
		committedMatrices []vCom.EncodedMatrix
		sisHashes         [][]field.Element
		trees             []*smt.Tree
		mimcHashes        [][]field.Element
		lastRound         = run.Spec.QueriesParams.Round(pcs.Query.QueryID)
		pubs              = []field.Element{}
	)

	for round := 0; round <= lastRound; round++ {

		var (
			committedMatrix, _ = run.State.TryGet(pcs.VortexProverStateName(round))
			sisHash, _         = run.State.TryGet(pcs.SisHashName(round))
			tree, _            = run.State.TryGet(pcs.MerkleTreeName(round))
			mimcHash, _        = run.State.TryGet(pcs.MIMCHashName(round))
		)

		if committedMatrix != nil {
			committedMatrices = append(committedMatrices, committedMatrix.(vCom.EncodedMatrix))
		} else {
			committedMatrices = append(committedMatrices, nil)
		}

		if sisHash != nil {
			sisHashes = append(sisHashes, sisHash.([]field.Element))
		} else {
			sisHashes = append(sisHashes, nil)
		}

		if tree != nil {
			trees = append(trees, tree.(*smt.Tree))
		} else {
			trees = append(trees, nil)
		}

		if mimcHash != nil {
			mimcHashes = append(mimcHashes, mimcHash.([]field.Element))
		} else {
			mimcHashes = append(mimcHashes, nil)
		}
	}

	for i := range run.Spec.PublicInputs {
		pubs = append(pubs, run.Spec.PublicInputs[i].Acc.GetVal(run))
	}

	return Witness{
		Proof:             run.ExtractProof(),
		CommittedMatrices: committedMatrices,
		SisHashes:         sisHashes,
		MimcHashes:        mimcHashes,
		Trees:             trees,
		FinalFS:           run.FS.State()[0],
		Pub:               pubs,
	}
}

func (pa AssignVortexUAlpha) Run(run *wizard.ProverRuntime) {
	for _, ctx := range pa.Ctxs.PcsCtx {
		// Since all the context of the pcs is translated, this does not
		// need to run over a translated prover runtime.
		ctx.ComputeLinearCombFromRsMatrix(run)
	}
}

func (pa AssignVortexOpenedCols) Run(run *wizard.ProverRuntime) {
	for _, ctx := range pa.Ctxs.PcsCtx {
		// Since all the context of the pcs is translated, this does not
		// need to run over a translated prover runtime.
		pa := vortex.OpenSelectedColumnsProverAction{Ctx: ctx}
		pa.Run(run)
	}
}

func (cc *ConsistencyCheck) Run(run wizard.Runtime) error {

	pis := cc.PIs

	for i := range pis {

		pcsCtx := cc.Ctx.PcsCtx[i]
		piWitness := pis[i].GetColAssignment(run).IntoRegVecSaveAlloc()
		circX, circYs, circMRoots, _ := SplitPublicInputs(cc.Ctx, piWitness)
		params := run.GetUnivariateParams(pcsCtx.Query.QueryID)
		pcsMRoot := pcsCtx.Items.MerkleRoots

		if circX != params.X {
			return fmt.Errorf("proof no=%v, x value does not match %v != %v", i, circX.String(), params.X.String())
		}

		if len(circYs) != len(params.Ys) {
			return fmt.Errorf("proof no=%v, number of Ys does not match; %v != %v", i, len(circYs), len(params.Ys))
		}

		for i := range circYs {
			if circYs[i] != params.Ys[i] {
				return fmt.Errorf("proof no=%v, Y[%v] does not match; %v != %v", i, i, circYs[i].String(), params.Ys[i].String())
			}
		}

		if pcsCtx.IsNonEmptyPrecomputed() {

			com := pcsCtx.Items.Precomputeds.MerkleRoot.GetColAssignmentAt(run, 0)
			if com != circMRoots[0] {
				return fmt.Errorf("proof no=%v, MRoot does not match; %v != %v", i, com.String(), circMRoots[0].String())
			}

			circMRoots = circMRoots[1:]
		}

		nonEmptyCount := 0
		for j := range pcsMRoot {

			if pcsMRoot[j] == nil {
				continue
			}

			com := pcsMRoot[j].GetColAssignmentAt(run, 0)
			if com != circMRoots[nonEmptyCount] {
				return fmt.Errorf("proof no=%v, MRoot does not match; %v != %v", i, com.String(), circMRoots[nonEmptyCount].String())
			}

			nonEmptyCount++
		}
	}

	return nil
}

func (cc *ConsistencyCheck) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	pis := cc.PIs

	for i := range pis {

		var (
			pcsCtx                       = cc.Ctx.PcsCtx[i]
			piWitness                    = pis[i].GetColAssignmentGnark(run)
			circX, circYs, circMRoots, _ = SplitPublicInputs(cc.Ctx, piWitness)
			params                       = run.GetUnivariateParams(pcsCtx.Query.QueryID)
			pcsMRoot                     = pcsCtx.Items.MerkleRoots
		)

		api.AssertIsEqual(circX, params.X)

		if len(circYs) != len(params.Ys) {
			utils.Panic("proof no=%v, number of Ys does not match; %v != %v", i, len(circYs), len(params.Ys))
		}

		for i := range circYs {
			api.AssertIsEqual(circYs[i], params.Ys[i])
		}

		if pcsCtx.IsNonEmptyPrecomputed() {

			// Note alex: for the conglomeration use-case. The precomputed Merkle-root
			// of the comp used to build the circuit and the one of the comp used to
			// build the proof may be different.
			//
			// When this feature is activated, then the precomputed column is "elevated"
			// to a round "0" Proof column. And its value is removed from the
			// comp.Precomputed table. We use that fact to check if we can deactivate the
			// equality assertion.
			mRootName := pcsCtx.Items.Precomputeds.MerkleRoot.GetColID()
			if cc.Ctx.InputCompiledIOP.Precomputed.Exists(mRootName) {
				com := pcsCtx.Items.Precomputeds.MerkleRoot.GetColAssignmentGnarkAt(run, 0)
				api.AssertIsEqual(com, circMRoots[0])
			}

			circMRoots = circMRoots[1:]
		}

		nonEmptyCount := 0
		for j := range pcsMRoot {

			if pcsMRoot[j] == nil {
				continue
			}

			com := pcsMRoot[j].GetColAssignmentGnarkAt(run, 0)
			api.AssertIsEqual(com, circMRoots[nonEmptyCount])
			nonEmptyCount++
		}
	}
}

func (cc *ConsistencyCheck) Skip() {
	cc.isSkipped = true
}

func (cc *ConsistencyCheck) IsSkipped() bool {
	return cc.isSkipped
}
