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
// entryClaims is supplied by the caller rather than derived. The canonical entry
// ordering is defined by verifier-ray's PCS codegen (a separate Go module) and
// reproducing it here would mean two orderings that must agree — the exact bug
// factory this package was written to avoid. Pass nil while that seam is open;
// Project will say so rather than silently emitting a proof whose vanishing
// checks have nothing to re-slice from.
func Project(
	sys *wiop.System,
	proof wiop.Proof,
	pub wiop.PublicInput,
	entryClaims [][]Ext,
) (Proof, error) {
	if sys == nil {
		return Proof{}, fmt.Errorf("proofserialization: Project: nil system")
	}
	if len(pub) != len(sys.PublicInputs) {
		return Proof{}, fmt.Errorf("proofserialization: Project: %d public inputs, system declares %d",
			len(pub), len(sys.PublicInputs))
	}

	out := Proof{PcsOpening: PcsOpening{EntryClaims: entryClaims}}

	// publicInputAt maps a public-input cell to its slot in pub. Those values
	// live in pub rather than proof.Cells, but the verifier still absorbs them in
	// round order, so the round messages have to carry them.
	publicInputAt := make(map[wiop.ObjectID]int, len(sys.PublicInputs))
	for i, c := range sys.PublicInputs {
		publicInputAt[c.Context.ID] = i
	}

	rounds, err := projectRounds(sys, proof, pub, publicInputAt)
	if err != nil {
		return Proof{}, err
	}
	out.Rounds = rounds

	if out.ModuleSizes, err = projectModuleSizes(sys, proof); err != nil {
		return Proof{}, err
	}

	if proof.PCSOpeningProof != nil {
		out.PcsOpening.Proof = projectOpeningProof(*proof.PCSOpeningProof)
	}

	return out, nil
}

// projectRounds builds one RoundMessage per round, in round order.
func projectRounds(
	sys *wiop.System,
	proof wiop.Proof,
	pub wiop.PublicInput,
	publicInputAt map[wiop.ObjectID]int,
) ([]RoundMessage, error) {
	rounds := make([]RoundMessage, len(sys.Rounds))

	for _, r := range sys.Rounds {
		msg := RoundMessage{}

		// wiop no longer transports columns: every column is committed, and a
		// committed round contributes exactly its Merkle commitment. There is no
		// public-column case to project, so ColumnMessage's other variant is
		// currently unreachable from this path.
		if commitment, ok := proof.Commitments[r.ID]; ok {
			msg.Columns = []ColumnMessage{{Commitment: DigestFrom(commitment)}}
		}

		// Cells in declaration order, which is the order the transcript absorbs
		// them in. Public inputs are included: the verifier absorbs them too, and
		// leaving them out would desynchronise the replay.
		if len(r.Cells) > 0 {
			msg.Cells = make([]Scalar, len(r.Cells))
			for i, cell := range r.Cells {
				if slot, isPI := publicInputAt[cell.Context.ID]; isPI {
					msg.Cells[i] = ScalarFrom(pub[slot])
					continue
				}
				v, ok := proof.Cells[cell.Context.ID]
				if !ok {
					return nil, fmt.Errorf("proofserialization: Project: cell %q is in round %d "+
						"but absent from the proof", cell.Context.Path(), r.ID)
				}
				msg.Cells[i] = ScalarFrom(v)
			}
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
