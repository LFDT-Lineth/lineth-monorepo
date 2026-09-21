package gkr

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"

type ProverState struct {
	Proof Proof
}

// NewProverState starts a proof of c over the instances given by a.
// The number of instances is the common length of a's value slices.
func NewProverState(c *Compiled, a Assignment) *ProverState {
	return nil
}

func (p *ProverState) HasNext() bool {

}

// Next consumes a challenge and returns the resulting round message, a copy owned
// by the caller. The same message is appended to p.Proof, which is authoritative.
func (p *ProverState) Next(challenge field.Ext) []field.Ext {

}
