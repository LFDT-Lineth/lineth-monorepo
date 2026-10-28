package fri

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

// ProverState drives the FRI commit and query phases as a coin-fed state
// machine. Instead of consuming every folding challenge up front, the caller
// feeds one challenge at a time via [ProverState.Fold]: each call folds the
// current layer into the next, commits the new layer, and returns its Merkle
// root so the caller can derive the next challenge from it (Fiat-Shamir). After
// numRounds folds the running polynomial has collapsed to the final polynomial,
// and [ProverState.Open] consumes the query positions to produce the Proof.
//
// Lifecycle:
//
//	st, _ := NewProverState(p, levels)
//	for st.HasNext() {              // numRounds iterations
//	    root := st.Fold(alpha_j)    // commits layer j+1, returns its root
//	    // absorb root, derive alpha_{j+1} …
//	}
//	proof := st.Open(positions)
//
// The embedded Proof is the in-progress proof; it is fully populated only once
// Open returns.
type ProverState struct {
	// Proof is the in-progress proof. Roots are filled as folds happen, the
	// final polynomial on the last fold, and the queries by Open.
	Proof

	p      Params
	plan   provePlan
	levels []Level

	// round is the next folding round to run; equivalently the number of folds
	// performed so far and the index of the current running layer.
	round uint8

	// running holds the evaluations of layer[round], nil for the zero
	// codeword: the running codeword is zero until the first level joins, and
	// is never materialized before (see zeroLayerTree).
	running []field.Ext
	trees   []layerTree // trees[1..min(round, numRounds-1)]; the final layer has no tree
}

// NewProverState validates the levels, builds the folding schedule, and seeds
// the machine with the zero codeword. The first level (at round 0, unless a
// FixedDomain PCS opens no size at its top) folds in as described in
// [ProverState.Fold] against this zero seed -- there being no round -1 to fold
// a real codeword from. The rounds before it fold the zero codeword.
func NewProverState(p Params, levels []Level) (*ProverState, error) {

	plan, err := buildProvePlan(p, levels)
	if err != nil {
		return nil, fmt.Errorf("fri: NewProverState: %w", err)
	}

	st := &ProverState{
		p:      p,
		plan:   plan,
		levels: levels,
		trees:  make([]layerTree, p.numRounds()),
	}

	if p.numRounds() > 1 {
		st.RoundRoots = make([]field.Octuplet, p.numRounds()-1)
	}

	// D=1: HasNext is false from the start so Fold is never called and
	// FinalPoly is never populated. Apply the top level directly (alphaDeep
	// is irrelevant when there is only one column) and extract coefficients
	// via IFFT, matching the logic in Fold's final-round branch.
	if p.numRounds() == 0 {
		if l, ok := plan.levelAtRound[0]; ok {
			var alphaDeep field.Ext
			st.running = st.levels[l].EvalsAt(alphaDeep, st.running)
		}
		coeffs := append([]field.Ext(nil), st.running...)
		p.domains[0].FFTInverseExt6(coeffs, fft.DIT)
		f := 1 << p.logFinalPolySize
		for i, c := range coeffs[f:] {
			if !c.IsZero() {
				panic(fmt.Sprintf("fri: ProverState: D=1 final layer has nonzero coefficient %d", f+i))
			}
		}
		st.FinalPoly = coeffs[:f]
	}

	return st, nil
}

// HasNext reports whether another folding challenge is expected.
func (st *ProverState) HasNext() bool {
	return st.round < st.p.numRounds()
}

// Fold consumes one folding challenge. If a level is introduced at this round,
// it is combined via alphaDeep (= alpha²) seeded by the running codeword
// from the preceding round (see EvalsAt); Fold commits the
// new layer and returns its Merkle root; on the final fold the running
// polynomial becomes the final polynomial — revealed in the clear rather
// than committed — and the returned root is the zero octuplet.
func (st *ProverState) Fold(alpha field.Ext) field.Octuplet {

	if !st.HasNext() {
		panic("fri: ProverState.Fold: all folding rounds already consumed")
	}

	j := st.round

	primary := st.running
	if l, ok := st.plan.levelAtRound[j]; ok {
		var alphaDeep field.Ext
		alphaDeep.Square(&alpha)
		primary = st.levels[l].EvalsAt(alphaDeep, st.running)
		if want := 1 << (st.p.LogCodewordSize - j); len(primary) != want {
			panic(fmt.Sprintf("fri: ProverState.Fold: levels[%d].EvalsAt returned %d values, want %d", l, len(primary), want))
		}
	}

	// The zero codeword (nil) folds into the zero codeword.
	st.running = nil
	if primary != nil {
		st.running = foldLayerInternally(primary, alpha, st.p.domains[j])
	}
	st.round = j + 1

	if j+1 == st.p.numRounds() && st.running == nil {
		st.FinalPoly = make([]field.Ext, 1<<st.p.logFinalPolySize)
		return field.Octuplet{}
	}
	if j+1 == st.p.numRounds() {
		// Final layer: revealed directly, no Merkle commitment, as its
		// 2^logFinalPolySize coefficients rather than its codeword. The
		// inverse FFT (DIT undoes the DIF forward encode without a separate
		// bit-reverse step -- see EncodeExt) must leave every higher
		// coefficient zero for a sufficiently low-degree witness.
		coeffs := append([]field.Ext(nil), st.running...)
		st.p.domains[j+1].FFTInverseExt6(coeffs, fft.DIT)
		f := 1 << st.p.logFinalPolySize
		for i, c := range coeffs[f:] {
			if !c.IsZero() {
				panic(fmt.Sprintf("fri: ProverState.Fold: final layer has nonzero coefficient %d, not low-degree enough", f+i))
			}
		}
		st.FinalPoly = coeffs[:f]
		return field.Octuplet{}
	}

	var tree layerTree
	if st.running == nil {
		tree = zeroLayerTree{height: int(st.p.LogCodewordSize - j - 1)}
	} else {
		tree = buildTreeExt(st.running)
	}
	st.trees[j+1] = tree
	root := tree.Root()
	st.RoundRoots[j] = root // root of layer j+1 → RoundRoots[(j+1)-1]
	return root
}

// Open runs the query phase for the given query positions and returns the
// completed Proof. It must be called after all numRounds folds.
func (st *ProverState) Open(openedPositions []int) Proof {

	if st.round != st.p.numRounds() {
		panic("fri: ProverState.Open: called before all folding rounds were consumed")
	}

	st.RunningQueries = make([]RunningQuery, st.p.NumQueries)
	capDepths := make([]int, st.p.numRounds())
	if st.p.numRounds() > 1 {
		st.RoundCaps = make([]MerkleCap, st.p.numRounds()-1)
		for j := uint8(1); j < st.p.numRounds(); j++ {
			depth := st.p.merkleCapDepth(int(st.p.LogCodewordSize - j))
			capDepths[j] = depth
			st.RoundCaps[j-1] = st.trees[j].OpenCap(depth)
		}
	}

	for k := range st.p.NumQueries {
		s := openedPositions[k]
		st.RunningQueries[k] = st.openRunningQueryExt(s, capDepths)
	}

	return st.Proof
}
