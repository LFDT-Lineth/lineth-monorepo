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

// gateWitness holds, for each row of the gates, the values of its operands a and b, and of its result o.
type gateWitness struct {
	a, b, o []field.Ext
}

// evaluateGates runs the gate rows of the layout on the proof.
func (l *layout) evaluateGates(proof []field.Ext) *gateWitness {
	w := &gateWitness{
		a: make([]field.Ext, len(l.gateRows)),
		b: make([]field.Ext, len(l.gateRows)),
		o: make([]field.Ext, len(l.gateRows)),
	}
	value := func(id int) field.Ext {
		if id < l.nbProof {
			return proof[id]
		}
		return w.o[id-l.firstGate]
	}
	for i := range l.gateRows {
		row := &l.gateRows[i]
		if row.a >= 0 {
			w.a[i] = value(row.a)
		}
		if row.b >= 0 {
			w.b[i] = value(row.b)
		}
		var previous field.Ext
		if i > 0 {
			previous = w.o[i-1]
		}
		w.o[i] = row.evaluate(w.a[i], w.b[i], previous)
	}
	return w
}
