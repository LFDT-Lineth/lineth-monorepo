package recursion

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex/vortex_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
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
	// We assume recursion is done with KoalaBear
	if run.KoalaFS == nil {
		if run.BLSFS != nil {
			utils.Panic("wrong FS type: expected KoalaBear FS")
		}
		panic("no FS found in the prover runtime")
	}

	var (
		pcs               = run.Spec.PcsCtxs.(*vortex.Ctx)
		committedMatrices []vortex_koalabear.EncodedMatrix
		sisHashes         [][]field.Element
		trees             []*smt_koalabear.Tree
		mimcHashes        [][]field.Element
		lastRound         = run.Spec.QueriesParams.Round(pcs.Query.QueryID)
		pubs              = []fext.GenericFieldElem{}
	)

	for round := 0; round <= lastRound; round++ {

		var (
			committedMatrix, _ = run.State.TryGet(pcs.VortexProverStateName(round))
			sisHash, _         = run.State.TryGet(pcs.SisHashName(round))
			tree, _            = run.State.TryGet(pcs.MerkleTreeName(round))
			mimcHash, _        = run.State.TryGet(pcs.NoSisHashName(round))
		)

		if committedMatrix != nil {
			committedMatrices = append(committedMatrices, committedMatrix.(vortex_koalabear.EncodedMatrix))
		} else {
			committedMatrices = append(committedMatrices, nil)
		}

		if sisHash != nil {
			sisHashes = append(sisHashes, sisHash.([]field.Element))
		} else {
			sisHashes = append(sisHashes, nil)
		}

		if tree != nil {
			trees = append(trees, tree.(*smt_koalabear.Tree))
		} else {
			trees = append(trees, nil)
		}

		if mimcHash != nil {
			mimcHashes = append(mimcHashes, mimcHash.([]field.Element))
		} else {
			mimcHashes = append(mimcHashes, nil)
		}
	}

	for i, elem := range run.Spec.PublicInputs {
		if elem.Acc.IsBase() {
			pub := run.Spec.PublicInputs[i].Acc.GetVal(run)
			pubs = append(pubs, fext.NewGenFieldFromBase(pub))
		} else {
			pub := run.Spec.PublicInputs[i].Acc.GetValExt(run)
			pubs = append(pubs, fext.NewGenFieldFromExt(pub))
		}
	}

	return Witness{
		Proof:             run.ExtractProof(),
		CommittedMatrices: committedMatrices,
		SisHashes:         sisHashes,
		Poseidon2Hashes:   mimcHashes,
		Trees:             trees,
		FinalFS:           run.KoalaFS.State(),
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
		piWitness, err := column.GetColAssignmentBase(run, pis[i])
		if err != nil {
			return fmt.Errorf("proof no=%v, failed to get pi witness: %v", i, err)
		}

		circX, circYs, circMRoots, _ := SplitPublicInputs(cc.Ctx, piWitness)
		params := run.GetUnivariateParams(pcsCtx.Query.QueryID)
		pcsMRoot := pcsCtx.Items.MerkleRoots

		if circX[0] != params.ExtX.B0.A0 || circX[1] != params.ExtX.B0.A1 || circX[2] != params.ExtX.B1.A0 || circX[3] != params.ExtX.B1.A1 {
			return fmt.Errorf("proof no=%v, x value does not match %++v != %++v", i, circX, params.ExtX)
		}

		if len(circYs) != 4*len(params.ExtYs) {
			return fmt.Errorf("proof no=%v, number of Ys does not match; %v != %v", i, len(circYs), len(params.ExtYs))
		}

		for j := range params.ExtYs {
			if circYs[4*j] != params.ExtYs[j].B0.A0 || circYs[4*j+1] != params.ExtYs[j].B0.A1 || circYs[4*j+2] != params.ExtYs[j].B1.A0 || circYs[4*j+3] != params.ExtYs[j].B1.A1 {
				return fmt.Errorf("proof no=%v, Y[%v] does not match; %v != %v", i, j, circYs[4*j:4*j+4], params.ExtYs[j])
			}
		}

		if pcsCtx.IsNonEmptyPrecomputed() {
			for j := 0; j < blockSize; j++ {
				com := pcsCtx.Items.Precomputeds.MerkleRoot[j].GetColAssignmentAt(run, 0)
				if com != circMRoots[0] {
					return fmt.Errorf("proof no=%v, MRoot does not match; %v != %v", j, com.String(), circMRoots[0].String())
				}

				circMRoots = circMRoots[1:]
			}
		}

		nonEmptyCount := 0
		for j := range pcsMRoot {
			for k := 0; k < blockSize; k++ {
				if pcsMRoot[j][k] == nil {
					continue
				}

				com := pcsMRoot[j][k].GetColAssignmentAt(run, 0)
				if com != circMRoots[nonEmptyCount] {
					return fmt.Errorf("proof no=%v, MRoot does not match; %v != %v", i, com, circMRoots[nonEmptyCount])
				}

				nonEmptyCount++
			}
		}
	}

	return nil
}

func (cc *ConsistencyCheck) RunGnark(api frontend.API, run wizard.GnarkRuntime) {
	pis := cc.PIs
	koalaApi := koalagnark.NewAPI(api)

	for i := range pis {

		var (
			pcsCtx                       = cc.Ctx.PcsCtx[i]
			piWitness                    = pis[i].GetColAssignmentGnark(run)
			circX, circYs, circMRoots, _ = SplitPublicInputs(cc.Ctx, piWitness)
			params                       = run.GetUnivariateParams(pcsCtx.Query.QueryID)
			pcsMRoot                     = pcsCtx.Items.MerkleRoots
		)

		koalaApi.AssertIsEqual(circX[0], params.ExtX.B0.A0)
		koalaApi.AssertIsEqual(circX[1], params.ExtX.B0.A1)
		koalaApi.AssertIsEqual(circX[2], params.ExtX.B1.A0)
		koalaApi.AssertIsEqual(circX[3], params.ExtX.B1.A1)

		if len(circYs) != 4*len(params.ExtYs) {
			utils.Panic("proof no=%v, number of Ys does not match; %v != %v", i, len(circYs), len(params.ExtYs))
		}

		for j := range params.ExtYs {
			koalaApi.AssertIsEqual(circYs[4*j], params.ExtYs[j].B0.A0)
			koalaApi.AssertIsEqual(circYs[4*j+1], params.ExtYs[j].B0.A1)
			koalaApi.AssertIsEqual(circYs[4*j+2], params.ExtYs[j].B1.A0)
			koalaApi.AssertIsEqual(circYs[4*j+3], params.ExtYs[j].B1.A1)
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
			for j := 0; j < blockSize; j++ {
				mRootName := pcsCtx.Items.Precomputeds.MerkleRoot[j].GetColID()
				if cc.Ctx.InputCompiledIOP.Precomputed.Exists(mRootName) {
					com := pcsCtx.Items.Precomputeds.MerkleRoot[j].GetColAssignmentGnarkAt(run, 0)
					koalaApi.AssertIsEqual(com, circMRoots[0])
				}

				circMRoots = circMRoots[1:]
			}
		}

		nonEmptyCount := 0
		for j := range pcsMRoot {

			for k := 0; k < blockSize; k++ {
				if pcsMRoot[j][k] == nil {
					continue
				}

				com := pcsMRoot[j][k].GetColAssignmentGnarkAt(run, 0)
				koalaApi.AssertIsEqual(com, circMRoots[nonEmptyCount])
				nonEmptyCount++
			}
		}
	}
}

func (cc *ConsistencyCheck) Skip() {
	cc.isSkipped = true
}

func (cc *ConsistencyCheck) IsSkipped() bool {
	return cc.isSkipped
}
