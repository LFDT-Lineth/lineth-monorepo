package gkrverifier

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
)

// compression is a row of the transcript: the compression of the block into the state.
type compression struct {
	state, block, next [blockLen]field.Element
}

// transcriptWitness is what the layout's transcript gives for a seed and a proof.
type transcriptWitness struct {
	rows []compression
	// challenges holds the challenges by id, from layout.firstChallenge.
	challenges []field.Ext
}

// coordinate returns the base element that the id of a coordinate stands for.
func (l *layout) coordinate(id int, seed *[6]field.Element, proof []field.Ext) field.Element {
	value, k := (id-l.coordinateBase)/6, (id-l.coordinateBase)%6
	if value == l.seed {
		return seed[k]
	}
	var coordinates [6]field.Element
	e := proof[value]
	copy(coordinates[:], e.AppendBase(nil))
	return coordinates[k]
}

// evaluate runs the transcript of the layout on the seed and the proof.
func (l *layout) evaluate(seed [6]field.Element, proof []field.Ext) (*transcriptWitness, error) {
	if len(proof) != l.nbProof {
		return nil, fmt.Errorf("gkrverifier: the proof has %d elements, the layout needs %d", len(proof), l.nbProof)
	}
	compressor := poseidon2.NewCompressor(poseidon2.NewDefaultPermutation())
	w := &transcriptWitness{
		rows:       make([]compression, len(l.rows)),
		challenges: make([]field.Ext, l.nbChallenges),
	}
	var state [blockLen]field.Element
	for i, row := range l.rows {
		c := &w.rows[i]
		c.state = state
		for j, s := range row.slots {
			if s.read {
				c.block[j] = l.coordinate(s.coord, &seed, proof)
			}
		}
		compressor.CompressField(&c.next, &c.state, &c.block)
		state = c.next
		if row.challenge >= 0 {
			w.challenges[row.challenge-l.firstChallenge].SetBase(c.next[:6])
		}
	}
	return w, nil
}

// coordinates returns the six base coordinates of an element of E₆.
func coordinates(e field.Ext) (c [6]field.Element) {
	copy(c[:], e.AppendBase(nil))
	return c
}

// values returns the value of every id that is not a coordinate, by id: the proof's elements, the
// seed, and the challenges. The ids of the rows of the gates, the folds and the eqs follow; the
// evaluations of those rows fill them.
func (l *layout) values(seed [6]field.Element, proof []field.Ext, w *transcriptWitness) []field.Ext {
	values := make([]field.Ext, l.nbIDs)
	copy(values, proof)
	values[l.seed].SetBase(seed[:])
	copy(values[l.firstChallenge:], w.challenges)
	return values
}

// gateWitness holds, for each row of the gates, the values of its operands a and b, and of its result o.
type gateWitness struct {
	a, b, o []field.Ext
}

// evaluateGates runs the gate rows of the layout, writing their results to values.
func (l *layout) evaluateGates(values []field.Ext) *gateWitness {
	w := &gateWitness{
		a: make([]field.Ext, len(l.gateRows)),
		b: make([]field.Ext, len(l.gateRows)),
		o: make([]field.Ext, len(l.gateRows)),
	}
	for i := range l.gateRows {
		row := &l.gateRows[i]
		if row.a >= 0 {
			w.a[i] = values[row.a]
		}
		if row.b >= 0 {
			w.b[i] = values[row.b]
		}
		var previous field.Ext
		if i > 0 {
			previous = w.o[i-1]
		}
		w.o[i] = row.evaluate(w.a[i], w.b[i], previous)
		values[l.firstGate+i] = w.o[i]
	}
	return w
}
