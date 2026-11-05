package gkrverifier

import (
	"fmt"

	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/gkr"
)

// The layout is everything about the verifier that depends only on the circuit, the schedule and the
// number of variables n, not on the values: which element of the flattened proof is absorbed where,
// which squeeze gives which challenge, and so how the transcript's compressions are filled. The tables
// are defined from it and assigned from it, so they cannot disagree.

// slot is one of the eight base elements of a block that a compression takes.
type slot struct {
	// read says that the element is the coordinate coord of an absorbed value, and not zero.
	read  bool
	coord int
}

// transcriptRow is a compression of the transcript: its state is the previous row's new state, and
// its block is slots.
type transcriptRow struct {
	slots [blockLen]slot
	// challenge is the id of the challenge that the row ends: the challenge is the first six elements of
	// the new state. It is -1 for a row that ends no challenge.
	challenge int
}

// blockLen is the number of base elements in a block and in the state of the transcript.
const blockLen = 8

// levelLayout is a level of the schedule, with the positions of its elements in the flattened proof.
type levelLayout struct {
	skip bool
	// degree is the number of coefficients of each round polynomial that the proof holds: it is 0 at a
	// skip level.
	degree int
	// start is the position of the level's first element in the proof: its round polynomials, n of
	// them of degree coefficients each, then its final evaluations, which start at finals.
	start, finals int
	nbFinals      int
	// nbClaims is the number of claims of the level.
	nbClaims int
	// absorbed holds the positions of the final evaluations that the transcript absorbs, in order.
	absorbed []int
	// foldChallenge is the id of the challenge c that batches the claims, or -1 below two claims and at
	// skip levels. roundChallenges holds the ids of the challenges of the rounds.
	foldChallenge   int
	roundChallenges []int
}

// layout describes the verifier of a circuit and a schedule for n variables.
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
	// The ids of the values: the proof's elements have their positions, and the seed follows. The
	// transcript reads these by coordinate, so the next 6·(nbProof + 1) ids, from coordinateBase, are
	// those of their coordinates: coordinate k of the value id has the id coordinateBase + 6·id + k. The
	// challenges follow.
	seed           int
	coordinateBase int
	firstChallenge int
	nbChallenges   int
	// initialChallenges holds the ids of the n challenges of the initial point.
	initialChallenges []int
	rows              []transcriptRow
}

// coordinateID is the id under which coordinate k of the value id is exposed.
func (l *layout) coordinateID(id, k int) int { return l.coordinateBase + 6*id + k }

// newLayout builds the layout, walking the schedule as gnark's Verify does.
func newLayout(circuit gkr.SerializableCircuit, schedule constraint.GkrProvingSchedule, n int) (*layout, error) {
	if n < 1 {
		return nil, fmt.Errorf("gkrverifier: the number of variables is %d, it must be positive", n)
	}
	if len(schedule) == 0 {
		return nil, fmt.Errorf("gkrverifier: the schedule has no level")
	}
	l := &layout{n: n, circuit: circuit, schedule: schedule, levels: make([]levelLayout, len(schedule))}

	identity := gkr.SerializableGate{Evaluate: gkr.IdentityBytecode(), NbIn: 1, Degree: 1}
	consolidated := make([]bool, len(circuit))
	for _, group := range schedule[0].ClaimGroups() {
		for _, wireI := range group.Wires {
			consolidated[wireI] = true
		}
	}

	// the positions in the proof
	next := 0
	for levelI, level := range schedule {
		levelCircuit := circuit.LevelCircuit(schedule, levelI, identity)
		unique, _ := levelCircuit.InputMapping(level)
		_, skip := level.(*constraint.GkrSkipLevel)
		lv := levelLayout{
			skip:          skip,
			degree:        levelCircuit.ZeroCheckDegree(level),
			start:         next,
			nbFinals:      len(unique) * level.NbOutgoingEvalPoints(),
			nbClaims:      level.NbClaims(),
			foldChallenge: -1,
		}
		lv.finals = lv.start + n*lv.degree
		next = lv.finals + lv.nbFinals
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
	l.outputs = next
	for i, wireI := range circuit.Outputs() {
		if consolidated[wireI] {
			l.outputsAbsorbed = append(l.outputsAbsorbed, l.outputs+i)
		}
	}
	l.nbProof = l.outputs + len(circuit.Outputs())
	l.seed = l.nbProof
	l.coordinateBase = l.seed + 1
	l.firstChallenge = l.coordinateBase + 6*(l.seed+1)

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
			if lv.nbClaims >= 2 {
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
	return l, nil
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

// write appends the six coordinates of the value id, or of the zero element if id is -1, to the block
// being filled. It does not set the absorbed flag: the seed is written this way.
func (b *transcriptBuilder) write(id int) {
	for k := range 6 {
		s := slot{}
		if id >= 0 {
			s = slot{read: true, coord: b.layout.coordinateID(id, k)}
		}
		b.pending = append(b.pending, s)
		if len(b.pending) == blockLen {
			b.compress()
		}
	}
}

// compress ends the block being filled, padded on the left with zeros if it is not full.
func (b *transcriptBuilder) compress() {
	row := transcriptRow{challenge: -1}
	copy(row.slots[blockLen-len(b.pending):], b.pending)
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
