package fri

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
)

// GnarkOpeningProofShape returns the unassigned [GnarkOpeningProof] of the
// plan's opening: exactly what [AllocateGnarkOpeningProof] returns for an
// honest proof of it, with every slice length, every present or absent
// auxiliary level and every row width.
//
// It lets a verifier circuit be allocated from the protocol description alone,
// before any proof exists. The geometry is a function of the plan only: input
// trees are never merged (one per opened batch, [VerifierPlan.Trees]), and
// query positions, zeta and fold challenges select values, never shapes. It
// mirrors [PCS.Open] through the helpers that Open and Verify share (the
// plan's input cap infos, merkleCapDepth), without committing or folding
// anything. Every call returns fresh slices, sharing no backing array.
func (plan VerifierPlan) GnarkOpeningProofShape() GnarkOpeningProof {
	p := plan.Params
	numQueries := int(p.NumQueries)
	res := GnarkOpeningProof{
		InputQueries: make([]GnarkInputQuery, numQueries),
		InputCaps:    make([]GnarkInputCap, len(plan.Trees)),
	}
	for t, tree := range plan.Trees {
		res.InputCaps[t] = inputCapShape(p, tree.Shape, plan.capInfos[t])
	}
	for k := range res.InputQueries {
		res.InputQueries[k] = make(GnarkInputQuery, len(plan.Trees))
		for t, tree := range plan.Trees {
			res.InputQueries[k][t] = inputTreeOpeningShape(tree.Shape, plan.capInfos[t])
		}
	}

	numRounds := p.numRounds()
	var numRunning uint8
	if numRounds > 1 {
		numRunning = numRounds - 1
	}
	res.FRIProof = GnarkProof{
		RoundRoots:     make([]circuit.Octuplet, numRunning),
		RoundCaps:      make([]GnarkMerkleCap, numRunning),
		FinalPoly:      make([]circuit.Ext, 1<<p.logFinalPolySize),
		RunningQueries: make([]GnarkRunningQuery, numQueries),
	}
	// Running layer j is a plain binary tree over 2^(LogCodewordSize−j)
	// leaves, with no injections (see buildTreeExt): its cap is the frontier
	// alone, and a branch stops at it.
	for j := uint8(1); j < numRounds; j++ {
		depth := p.merkleCapDepth(int(p.LogCodewordSize - j))
		var nodes int
		if depth > 0 {
			nodes = 1 << depth
		}
		res.FRIProof.RoundCaps[j-1] = GnarkMerkleCap{Nodes: make([]circuit.Octuplet, nodes)}
	}
	for k := range res.FRIProof.RunningQueries {
		q := make(GnarkRunningQuery, numRunning)
		for j := uint8(1); j < numRounds; j++ {
			height := int(p.LogCodewordSize - j)
			q[j-1] = GnarkBranch{Siblings: make([]circuit.Octuplet, height-p.merkleCapDepth(height))}
		}
		res.FRIProof.RunningQueries[k] = q
	}
	return res
}

// inputTreeOpeningShape mirrors openInputTreeOpeningToDepth: the bottom level
// always carries its pair, the auxiliary levels at or below the cap carry
// theirs (a level without one is the zero pair), and the bottom sibling is
// derived rather than sent unless the leaves digest row pairs.
func inputTreeOpeningShape(shape Shape, info inputCapInfo) GnarkInputTreeOpening {
	numSiblings := info.treeHeight() - info.depth
	if !info.grouped {
		numSiblings--
	}
	res := GnarkInputTreeOpening{
		Siblings: make([]circuit.Octuplet, numSiblings),
		Leaves:   make([]GnarkRowPair, info.height),
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

// inputCapShape mirrors openInputCaps: an empty cap at depth zero, else the
// frontier and every revealed table in full.
func inputCapShape(p Params, shape Shape, info inputCapInfo) GnarkInputCap {
	if info.depth == 0 {
		return GnarkInputCap{Nodes: []circuit.Octuplet{}, Tables: []GnarkInputCapTable{}}
	}
	res := GnarkInputCap{
		Nodes:  make([]circuit.Octuplet, 1<<info.depth),
		Tables: make([]GnarkInputCapTable, len(info.revealed)),
	}
	rateLog := int(p.LogCodewordSize) - int(p.LogPlainTextSize)
	for i, sizeLog2 := range info.revealed {
		rows := make([]GnarkRowOpening, 1<<(sizeLog2+rateLog))
		for r := range rows {
			rows[r] = rowShape(shape[sizeLog2])
		}
		res.Tables[i] = GnarkInputCapTable{SizeLog2: uint8(sizeLog2), Rows: rows}
	}
	return res
}

func rowShape(sized SizedShape) GnarkRowOpening {
	return GnarkRowOpening{
		Base: make([]circuit.Element, sized.BaseWidth),
		Ext:  make([]circuit.Ext, sized.ExtWidth),
	}
}

func rowPairShape(sized SizedShape) GnarkRowPair {
	return GnarkRowPair{rowShape(sized), rowShape(sized)}
}
