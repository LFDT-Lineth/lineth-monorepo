package fri

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// OpeningProofShape returns an [OpeningProof] with exactly the geometry an
// honest prover emits for the plan's opening: every slice length, every
// present or absent auxiliary level and every row width. Its values are zero
// and meaningless.
//
// It lets a verifier circuit be allocated from the protocol description alone,
// before any proof exists (see [AllocateGnarkOpeningProof]). The geometry is a
// function of the plan only: input trees are never merged (one per opened
// batch, [VerifierPlan.Trees]), and query positions, zeta and fold challenges
// select values, never shapes. It mirrors [PCS.Open] through the helpers that
// Open and Verify share (the plan's input cap infos, merkleCapDepth), without
// committing or folding anything.
func (plan VerifierPlan) OpeningProofShape() OpeningProof {
	p := plan.Params
	numQueries := int(p.NumQueries)
	res := OpeningProof{
		InputQueries: make([]InputQuery, numQueries),
		InputCaps:    make([]InputCap, len(plan.Trees)),
	}

	openings := make([]InputTreeOpening, len(plan.Trees))
	for t, tree := range plan.Trees {
		info := plan.capInfos[t]
		openings[t] = inputTreeOpeningShape(tree.Shape, info)
		res.InputCaps[t] = inputCapShape(p, tree.Shape, info)
	}
	for k := range res.InputQueries {
		res.InputQueries[k] = make(InputQuery, len(openings))
		for t, o := range openings {
			res.InputQueries[k][t] = cloneInputTreeOpening(o)
		}
	}

	res.FRIProof = Proof{
		FinalPoly:      make([]field.Ext, 1<<p.logFinalPolySize),
		RunningQueries: make([]RunningQuery, numQueries),
	}
	numRounds := p.numRounds()
	if numRounds > 1 {
		res.FRIProof.RoundRoots = make([]field.Octuplet, numRounds-1)
		res.FRIProof.RoundCaps = make([]MerkleCap, numRounds-1)
		for j := uint8(1); j < numRounds; j++ {
			// Running layer j is a plain binary tree over 2^(LogCodewordSize−j)
			// leaves, with no injections (see buildTreeExt).
			depth := p.merkleCapDepth(int(p.LogCodewordSize - j))
			if depth > 0 {
				res.FRIProof.RoundCaps[j-1] = MerkleCap{
					Nodes: make([]field.Octuplet, 1<<depth),
					Aux:   make([]*field.Octuplet, 1<<depth-1),
				}
			}
		}
		for k := range res.FRIProof.RunningQueries {
			q := make(RunningQuery, numRounds-1)
			for j := uint8(1); j < numRounds; j++ {
				height := int(p.LogCodewordSize - j)
				n := height - p.merkleCapDepth(height)
				q[j-1] = Branch{Siblings: make([]field.Octuplet, n), AuxSiblings: make([][]field.Octuplet, n)}
			}
			res.FRIProof.RunningQueries[k] = q
		}
	}
	return res
}

// inputTreeOpeningShape mirrors openInputTreeOpeningToDepth: the bottom level
// always carries its pair, the auxiliary levels at or below the cap carry
// theirs, and the bottom sibling is derived rather than sent unless the leaves
// digest row pairs.
func inputTreeOpeningShape(shape Shape, info inputCapInfo) InputTreeOpening {
	numSiblings := info.treeHeight() - info.depth
	if !info.grouped {
		numSiblings--
	}
	res := InputTreeOpening{
		Siblings: make([]field.Octuplet, numSiblings),
		Leaves:   make([]*RowPair, info.height),
	}
	bottom := len(shape) - 1
	for bottom >= 0 && shape[bottom].BaseWidth+shape[bottom].ExtWidth == 0 {
		bottom--
	}
	res.Leaves[info.height-1] = rowPairShape(shape[bottom])
	for sizeLog2, sized := range shape[:bottom] {
		if sized.BaseWidth+sized.ExtWidth == 0 {
			continue
		}
		auxDepth, isAux := inputAuxDepth(info.rateLog, sizeLog2, bottom)
		if isAux && auxDepth >= info.depth {
			res.Leaves[auxDepth] = rowPairShape(sized)
		}
	}
	return res
}

// inputCapShape mirrors openInputCaps: no cap at depth zero, else the
// frontier and every revealed table in full.
func inputCapShape(p Params, shape Shape, info inputCapInfo) InputCap {
	if info.depth == 0 {
		return InputCap{}
	}
	res := InputCap{
		Nodes:  make([]field.Octuplet, 1<<info.depth),
		Tables: make([]InputCapTable, len(info.revealed)),
	}
	rateLog := int(p.LogCodewordSize) - int(p.LogPlainTextSize)
	for i, sizeLog2 := range info.revealed {
		rows := make([]RowOpening, 1<<(sizeLog2+rateLog))
		for r := range rows {
			rows[r] = rowShape(shape[sizeLog2])
		}
		res.Tables[i] = InputCapTable{SizeLog2: uint8(sizeLog2), Rows: rows}
	}
	return res
}

func rowShape(sized SizedShape) RowOpening {
	return RowOpening{
		Base: make([]field.Element, sized.BaseWidth),
		Ext:  make([]field.Ext, sized.ExtWidth),
	}
}

func rowPairShape(sized SizedShape) *RowPair {
	return &RowPair{rowShape(sized), rowShape(sized)}
}

// cloneInputTreeOpening deep-copies o, so that the queries of a shaped proof
// share no backing arrays.
func cloneInputTreeOpening(o InputTreeOpening) InputTreeOpening {
	res := InputTreeOpening{
		Siblings: make([]field.Octuplet, len(o.Siblings)),
		Leaves:   make([]*RowPair, len(o.Leaves)),
	}
	for i, pair := range o.Leaves {
		if pair != nil {
			res.Leaves[i] = &RowPair{
				{Base: make([]field.Element, len(pair[0].Base)), Ext: make([]field.Ext, len(pair[0].Ext))},
				{Base: make([]field.Element, len(pair[1].Base)), Ext: make([]field.Ext, len(pair[1].Ext))},
			}
		}
	}
	return res
}
