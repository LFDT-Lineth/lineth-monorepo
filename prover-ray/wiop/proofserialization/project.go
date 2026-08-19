package proofserialization

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Project turns a [wiop.Proof] into the verifier's shape, ready for [Encode].
//
// This is the step that makes serialization possible at all: wiop.Proof is keyed
// by ObjectID through Go maps, and the verifier's Proof is round-major and dense.
// The maps disappear here, which is why they were never an obstacle to a
// zero-decode dump. See README.md section 3.
//
// PCS entry claims are not part of the projected shape: they are ordinary
// LagrangeEval cells, already present in the round messages [projectRounds]
// builds, and the verifier derives its canonical-entry-order view of them from
// those cells at verify time.
func Project(
	sys *wiop.System,
	proof wiop.Proof,
	pub wiop.PublicInput,
) (VerifyInput, error) {
	if sys == nil {
		return VerifyInput{}, fmt.Errorf("proofserialization: Project: nil system")
	}
	if len(pub) != len(sys.PublicInputs) {
		return VerifyInput{}, fmt.Errorf("proofserialization: Project: %d public inputs, system declares %d",
			len(pub), len(sys.PublicInputs))
	}

	out := VerifyInput{Proof: Proof{}}

	// The flat public-input statement, in registration order. These are absorbed
	// separately from the round messages, so the round cells below must not
	// repeat them.
	if len(pub) > 0 {
		out.PublicInputs = make([]Scalar, len(pub))
		for i, v := range pub {
			out.PublicInputs[i] = ScalarFrom(v)
		}
	}

	// publicInputAt marks which cells are public inputs, so the round messages can
	// skip them: verifier.Proof documents that rounds[*].cells omits public-input
	// cells and PublicInputs supplies them instead.
	publicInputAt := make(map[wiop.ObjectID]bool, len(sys.PublicInputs))
	for _, c := range sys.PublicInputs {
		publicInputAt[c.Context.ID] = true
	}

	rounds, err := projectRounds(sys, proof, publicInputAt)
	if err != nil {
		return VerifyInput{}, err
	}
	out.Proof.Rounds = rounds

	if out.Proof.ModuleSizes, err = projectModuleSizes(sys, proof); err != nil {
		return VerifyInput{}, err
	}

	if proof.PCSOpeningProof != nil {
		out.Proof.PcsOpening = projectOpeningProof(*proof.PCSOpeningProof)
	}

	return out, nil
}

// projectRounds builds one RoundMessage per round, in round order.
func projectRounds(
	sys *wiop.System,
	proof wiop.Proof,
	isPublicInput map[wiop.ObjectID]bool,
) ([]RoundMessage, error) {
	rounds := make([]RoundMessage, len(sys.Rounds))

	for _, r := range sys.Rounds {
		msg := RoundMessage{}

		// wiop does not transport columns: every column is committed, and a
		// committed round contributes exactly its Merkle root.
		if commitment, ok := proof.Commitments[r.ID]; ok {
			digest := DigestFrom(commitment)
			msg.Commitment = &digest
		}

		// Non-public-input cells, in declaration order. Public inputs are skipped:
		// they travel in VerifyInput.PublicInputs, and repeating them here would
		// absorb them twice and desynchronise the transcript replay.
		for _, cell := range r.Cells {
			if isPublicInput[cell.Context.ID] {
				continue
			}
			v, ok := proof.Cells[cell.Context.ID]
			if !ok {
				return nil, fmt.Errorf("proofserialization: Project: cell %q is in round %d "+
					"but absent from the proof", cell.Context.Path(), r.ID)
			}
			msg.Cells = append(msg.Cells, ScalarFrom(v))
		}

		rounds[r.ID] = msg
	}

	return rounds, nil
}

// projectModuleSizes flattens the dynamic-module sizes into canonical order:
// ascending module index, restricted to dynamic modules. That is the order the
// verifier's SizeSource.dynamic indices refer to.
func projectModuleSizes(sys *wiop.System, proof wiop.Proof) ([]uint64, error) {
	var sizes []uint64
	for k := range sys.Modules {
		if !sys.Modules[k].IsDynamic() {
			continue
		}
		v, ok := proof.DynamicSizes[k]
		if !ok {
			return nil, fmt.Errorf("proofserialization: Project: dynamic module %d has no size "+
				"in the proof", k)
		}
		if v <= 0 {
			return nil, fmt.Errorf("proofserialization: Project: dynamic module %d has size %d",
				k, v)
		}
		sizes = append(sizes, uint64(v))
	}
	return sizes, nil
}

func projectOpeningProof(op fri.OpeningProof) OpeningProof {
	out := OpeningProof{FriProof: projectFriProof(op.FRIProof)}

	if len(op.InputQueries) > 0 {
		out.InputQueries = make([][]InputTreeOpening, len(op.InputQueries))
		for q, iq := range op.InputQueries {
			if len(iq) == 0 {
				continue
			}
			out.InputQueries[q] = make([]InputTreeOpening, len(iq))
			for i, open := range iq {
				out.InputQueries[q][i] = projectInputTreeOpening(open)
			}
		}
	}

	return out
}

func projectInputTreeOpening(o fri.InputTreeOpening) InputTreeOpening {
	out := InputTreeOpening{Siblings: DigestsFrom(o.Siblings)}

	if len(o.Leaves) > 0 {
		out.Leaves = make([]*RowPair, len(o.Leaves))
		for i, l := range o.Leaves {
			if l == nil {
				continue // stays nil: Zig's `null` for that level
			}
			pair := RowPair{projectRowOpening(l[0]), projectRowOpening(l[1])}
			out.Leaves[i] = &pair
		}
	}

	return out
}

func projectRowOpening(r fri.RowOpening) RowOpening {
	return RowOpening{Base: ElementsFrom(r.Base), Ext: ExtsFrom(r.Ext)}
}

func projectFriProof(p fri.Proof) FriProof {
	out := FriProof{
		RoundRoots: DigestsFrom(p.RoundRoots),
		FinalPoly:  ExtsFrom(p.FinalPoly),
	}

	if len(p.RunningQueries) > 0 {
		out.RunningQueries = make([][]Branch, len(p.RunningQueries))
		for q, rq := range p.RunningQueries {
			if len(rq) == 0 {
				continue
			}
			out.RunningQueries[q] = make([]Branch, len(rq))
			for j, layer := range rq {
				if len(layer) == 0 {
					continue
				}
				// The verifier reads one Branch per fold round. Go's QueryLayer is
				// a slice, but only layer[0] is consumed, and AuxSiblings has no
				// counterpart in Zig's merkle.Branch at all -- measured to be
				// entirely nil, so dropping it loses nothing.
				out.RunningQueries[q][j] = Branch{
					Siblings: DigestsFrom(layer[0].Siblings),
					Leaf:     DigestFrom(layer[0].Leaf),
				}
			}
		}
	}

	return out
}
