package gkr

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"

type Opening struct {
	EvaluationPoint []field.Ext
	Evaluation      field.Ext
}

// Verify checks the arithmetic relations of proof against the given challenges.
func Verify(c *Compiled, outputs Assignment, proof Proof, challenges []field.Ext) (map[Identifier]Opening, error) {

}
