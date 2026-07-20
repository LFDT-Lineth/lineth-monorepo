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

	running []field.Ext   // evaluations of layer[round]
	layers  [][]field.Ext // layers[0..round]; layers[numRounds] is the final polynomial
	trees   []*Tree       // trees[0..min(round, numRounds-1)]; the final layer has no tree
}

// NewProverState validates the levels, builds the folding schedule, and seeds
// the machine with the zero codeword: round 0 always introduces a level (the
// main degree-D polynomial), which folds in as described in [ProverState.Fold]
// against this zero seed -- there being no round -1 to fold a real codeword
// from.
func NewProverState(p Params, levels []Level) (*ProverState, error) {

	plan, err := buildProvePlan(p, levels)
	if err != nil {
		return nil, fmt.Errorf("fri: NewProverState: %w", err)
	}

	st := &ProverState{
		p:       p,
		plan:    plan,
		levels:  levels,
		running: make([]field.Ext, 1<<p.LogCodewordSize),
		layers:  make([][]field.Ext, p.numRounds()+1),
		trees:   make([]*Tree, p.numRounds()),
	}
	st.layers[0] = st.running

	if p.numRounds() > 1 {
		st.RoundRoots = make([]field.Octuplet, p.numRounds()-1)
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

	st.running = foldLayerInternally(primary, alpha, st.p.domains[j])
	st.layers[j+1] = st.running
	st.round = j + 1

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

	tree := buildTreeExt(st.running)
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

	for k := range st.p.NumQueries {
		s := openedPositions[k]
		st.RunningQueries[k] = st.openRunningQueryExt(s)
	}

	return st.Proof
}
