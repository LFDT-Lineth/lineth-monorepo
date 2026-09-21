package gkr

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"

// Proof is the sequence of round messages the prover emits, one per Next.
type Proof [][]field.Ext

type ProverState struct {
	Proof Proof
}

// NewProverState starts a proof of c over the instances given by a.
// The number of instances is the common length of a's value slices.
func NewProverState(c *Compiled, a Assignment) *ProverState {
	return nil
}

func (p *ProverState) HasNext() bool {
	panic("gkr: not implemented")
}

// Next consumes a challenge and returns the resulting round message, a copy owned
// by the caller. The same message is appended to p.Proof, which is authoritative.
func (p *ProverState) Next(challenge field.Ext) []field.Ext {
	panic("gkr: not implemented")
}
