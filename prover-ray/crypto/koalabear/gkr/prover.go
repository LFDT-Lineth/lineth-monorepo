package gkr

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"

type ProverState struct {
	Proof Proof
}

func NewProverState(api *API) *ProverState {
	return nil
}

func (p *ProverState) HasNext() bool {

}

func (p *ProverState) Next(challenge field.Ext) []field.Ext {

}
