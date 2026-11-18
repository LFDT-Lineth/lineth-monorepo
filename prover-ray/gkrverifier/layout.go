package gkrverifier

import (
	"fmt"
	"slices"

	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/gkr"
)

// slot is a coordinate that the block being filled holds: coordinate coord of the element elem, which
// is -1 for a zero element.
type slot struct{ elem, coord int }

// A block holds the coordinates of at most two elements, A and B, consecutive in the transcript: a block
// of eight coordinates starts at coordinate 0, 2 or 4 of A, and a block that a squeeze pads takes the last
// 6, 4 or 2 coordinates of the stream, those of one element. blockShapes lists the shapes, each as the
// number of zeros that pad the block on the left and the coordinate of A that follows them; the block
// then continues through A and into B.
var blockShapes = [...][2]int{{0, 0}, {0, 2}, {0, 4}, {2, 0}, {4, 2}, {6, 4}}

// blockSlot returns which element, 0 for A and 1 for B, and which coordinate of it slot i of a block of
// the given shape holds; the element is -1 for a zero of the padding.
func blockSlot(shape, i int) (elem, coord int) {
	pad, first := blockShapes[shape][0], blockShapes[shape][1]
	if i < pad {
		return -1, 0
	}
	t := first + i - pad
	return t / 6, t % 6
}

// transcriptRow is a compression of the transcript: its state is the previous row's new state, and
// its block has the given shape over the elements A and B.
type transcriptRow struct {
	// elems holds the ids of A and B, each -1 if the block holds nothing of it or only zeros.
	elems [2]int
	shape int
	// challenge is the id of the challenge that the row ends: the challenge is the first six elements of
	// the new state. It is -1 for a row that ends no challenge.
	challenge int
}

// levelLayout is a level of the schedule, with the positions of its elements in the flattened proof.
type levelLayout struct {
	skip bool
	// circuit is the circuit as the level sees it: level 0's identity view, and the circuit itself at
	// the others. unique holds the wires that the level's gates read, and inputIndices the position of
	// each input of each wire of the level in unique.
	circuit      gkr.SerializableCircuit
	unique       []int
	inputIndices [][]int
	// degree is the number of coefficients of each round polynomial that the proof holds: it is 0 at a
	// skip level.
	degree int
	// start is the position of the level's first element in the proof: its round polynomials, n of
	// them of degree coefficients each, then its final evaluations, which start at finals.
	start, finals int
	// absorbed holds the positions of the final evaluations that the transcript absorbs, in order.
	absorbed []int
	// foldChallenge is the id of the challenge c that batches the claims, or -1 below two claims and at
	// skip levels. roundChallenges holds the ids of the challenges of the rounds.
	foldChallenge   int
	roundChallenges []int
	// results holds, at a level that is not a skip level, the id of the value of each wire's gate on
	// the level's final evaluations, in the order of the level's wires.
	results []int
	// single says that the level has one source, so that its rounds are those of a single-source
	// zerocheck.
	single bool
	// sum and final are the ids of the sum that the level's sumcheck starts from, the claims folded by
	// the folding challenge, and of the value it ends on, the gates' results folded the same way and
	// weighted by the eq of the claims' points.
	sum, final int
	// eqs holds the ids of eq(p, r) for the point p of each claim source of each claim group, if the
	// level is not single.
	eqs [][]int
}

// layout describes the verifier of a circuit and a schedule for n variables: everything about it that
// does not depend on the values. That is the ids of the values, which element of the flattened proof the
// transcript absorbs where and which squeeze gives which challenge, the rows of the gates, the folds,
// the eqs and the rounds, and the claims that gnark's Verify returns.
type layout struct {
	n        int
	circuit  gkr.SerializableCircuit
	schedule constraint.GkrProvingSchedule
	levels   []levelLayout
	// outputs is the position in the proof of the output evaluations, and outputsAbsorbed those of them
	// that are absorbed.
	outputs         int
	outputsAbsorbed []int
	// nbProof is the number of elements of the flattened proof.
	nbProof int
	// The ids of the values: the proof's elements have their positions, the seed follows, and the
	// challenges follow the seed.
	seed           int
	firstChallenge int
	nbChallenges   int
	// initialChallenges holds the ids of the n challenges of the initial point.
	initialChallenges []int
	rows              []transcriptRow
	// The gate rows have the ids from firstGate on, which follow the challenges: row i has the id
	// firstGate + i.
	firstGate int
	gateRows  []gateRow
	// consolidated marks the wires of level 0, and claimValueIndices is gnark's.
	consolidated      []bool
	claimValueIndices [][]int
	// points[levelI][k] holds the ids of the challenges of the k-th outgoing evaluation point of the
	// level, the output level being the schedule's length.
	points [][][]int
	// maxDegree is the largest degree of the levels' round polynomials.
	maxDegree int
	// the rows of the folds, the eqs and the rounds, and the claims that Verify returns
	foldRows  []foldRow
	eqRows    []eqRow
	roundRows []roundRow
	claims    []claimLayout
	// claimCounts is the number of claims of each wire.
	claimCounts map[int]int
	// nbIDs is the number of ids: the values are indexed by them.
	nbIDs int
}

// newLayout builds the layout, walking the schedule as gnark's Verify does.
func newLayout(circuit gkr.SerializableCircuit, schedule constraint.GkrProvingSchedule, n int) (*layout, error) {
	if n < 1 {
		return nil, fmt.Errorf("gkrverifier: the number of variables is %d, it must be positive", n)
	}
	if len(schedule) == 0 {
		return nil, fmt.Errorf("gkrverifier: the schedule has no level")
	}
	l := &layout{n: n, circuit: circuit, schedule: schedule, levels: make([]levelLayout, len(schedule))}

	consolidated := make([]bool, len(circuit))
	for _, group := range schedule[0].ClaimGroups() {
		for _, wireI := range group.Wires {
			consolidated[wireI] = true
		}
	}

	// the positions in the proof
	next := 0
	for levelI, level := range schedule {
		levelCircuit := circuit.LevelCircuit(schedule, levelI, identityGate())
		unique, inputIndices := levelCircuit.InputMapping(level)
		_, skip := level.(*constraint.GkrSkipLevel)
		_, single := level.(*constraint.GkrSingleSourceZeroCheckLevel)
		lv := levelLayout{
			skip:          skip,
			single:        single,
			circuit:       levelCircuit,
			unique:        unique,
			inputIndices:  inputIndices,
			degree:        levelCircuit.ZeroCheckDegree(level),
			start:         next,
			foldChallenge: -1,
		}
		lv.finals = lv.start + n*lv.degree
		next = lv.finals + len(unique)*level.NbOutgoingEvalPoints()
		if levelI != 0 {
			for uniqueI, wireI := range unique {
				if circuit.IsInput(wireI) && !consolidated[wireI] {
					continue
				}
				for pos := level.FinalEvalProofIndex(uniqueI, 0); pos < level.FinalEvalProofIndex(uniqueI+1, 0); pos++ {
					lv.absorbed = append(lv.absorbed, lv.finals+pos)
				}
			}
		}
		l.levels[levelI] = lv
	}
	l.consolidated = consolidated
	l.claimValueIndices = circuit.ClaimValueIndices(schedule)
	l.outputs = next
	for i, wireI := range circuit.Outputs() {
		if consolidated[wireI] {
			l.outputsAbsorbed = append(l.outputsAbsorbed, l.outputs+i)
		}
	}
	l.nbProof = l.outputs + len(circuit.Outputs())
	l.seed = l.nbProof
	l.firstChallenge = l.seed + 1

	// the transcript, in the order of Verify
	b := &transcriptBuilder{layout: l}
	b.write(l.seed)
	for range n {
		l.initialChallenges = append(l.initialChallenges, b.squeeze())
	}
	b.absorb(l.outputsAbsorbed)
	for levelI := len(schedule) - 1; levelI >= 0; levelI-- {
		lv := &l.levels[levelI]
		if !lv.skip {
			if schedule[levelI].NbClaims() >= 2 {
				lv.foldChallenge = b.squeeze()
			}
			for j := range n {
				first := lv.start + j*lv.degree
				b.absorb(rangeOf(first, lv.degree))
				lv.roundChallenges = append(lv.roundChallenges, b.squeeze())
			}
		}
		b.absorb(lv.absorbed)
	}
	l.rows = b.rows
	l.nbChallenges = b.nbChallenges
	l.firstGate = l.firstChallenge + l.nbChallenges
	l.lowerPoints()
	l.lowerGates()
	l.lowerSumchecks()
	l.lowerClaims()
	return l, nil
}

// claimPosition is the position in the proof of the value with the given index among the final
// evaluations of the level of a claim source, the output level being the schedule's length.
func (l *layout) claimPosition(source constraint.GkrClaimSource, index int) int {
	if source.Level == len(l.schedule) {
		return l.outputs + index
	}
	return l.levels[source.Level].finals + index
}

// rangeOf returns first, first + 1, …, first + count - 1.
func rangeOf(first, count int) []int {
	r := make([]int, count)
	for i := range r {
		r[i] = first + i
	}
	return r
}

// transcriptBuilder replays the writes and squeezes of gnark's transcript, over the hasher
// poseidon2.NewHasher, and records its compressions. An element of E₆ is written as its six
// coordinates; a block of eight coordinates is compressed into the state when complete; a squeeze
// writes the zero element if nothing was absorbed since the last, pads the pending coordinates on the
// left with zeros and compresses them if there are any, and returns the first six elements of the
// state.
type transcriptBuilder struct {
	layout       *layout
	rows         []transcriptRow
	pending      []slot
	absorbed     bool
	nbChallenges int
}

// write appends the six coordinates of the element id, or of the zero element if id is -1, to the block
// being filled. It does not set the absorbed flag: the seed is written this way.
func (b *transcriptBuilder) write(id int) {
	for k := range 6 {
		b.pending = append(b.pending, slot{elem: id, coord: k})
		if len(b.pending) == poseidon2.BlockLen {
			b.compress()
		}
	}
}

// compress ends the block being filled, padded on the left with zeros if it is not full.
func (b *transcriptBuilder) compress() {
	pad := poseidon2.BlockLen - len(b.pending)
	shape := slices.Index(blockShapes[:], [2]int{pad, b.pending[0].coord})
	if shape < 0 {
		panic("gkrverifier: a block of the transcript has no known shape")
	}
	row := transcriptRow{elems: [2]int{-1, -1}, shape: shape, challenge: -1}
	for i, s := range b.pending {
		which, _ := blockSlot(shape, pad+i)
		row.elems[which] = s.elem
	}
	b.rows = append(b.rows, row)
	b.pending = b.pending[:0]
}

// absorb writes the elements at the given positions of the proof, and sets the absorbed flag if there
// are any.
func (b *transcriptBuilder) absorb(positions []int) {
	if len(positions) == 0 {
		return
	}
	for _, pos := range positions {
		b.write(pos)
	}
	b.absorbed = true
}

// squeeze returns the id of the challenge it derives.
func (b *transcriptBuilder) squeeze() int {
	if !b.absorbed {
		b.write(-1)
	}
	b.absorbed = false
	if len(b.pending) != 0 {
		b.compress()
	}
	last := &b.rows[len(b.rows)-1]
	if last.challenge != -1 {
		panic("gkrverifier: a row of the transcript ends two challenges")
	}
	id := b.layout.firstChallenge + b.nbChallenges
	b.nbChallenges++
	last.challenge = id
	return id
}
