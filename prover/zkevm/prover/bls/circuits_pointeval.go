package bls

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bls12381"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/emulated"
)

const (
	nbG1CompressedLimbs  = 3
	nbVersionedHashLimbs = 2
	nbRowsPerPointEval   = nbVersionedHashLimbs + 2*nbFrLimbs + 2*nbG1CompressedLimbs + 2 + nbFrLimbs
)

type multiPointEvalCircuit struct {
	Instances []pointEvalInstance `gnark:",public"`
}

func newMultiPointEvalCircuit(limits *Limits) *multiPointEvalCircuit {
	return &multiPointEvalCircuit{
		Instances: make([]pointEvalInstance, limits.NbPointEvalInputInstances),
	}
}

func (c *multiPointEvalCircuit) Define(api frontend.API) error {
	fr, err := emulated.NewField[sw_bls12381.ScalarField](api)
	if err != nil {
		return fmt.Errorf("new field: %w", err)
	}
	for i := range c.Instances {
		if err := c.Instances[i].Check(api, fr); err != nil {
			return fmt.Errorf("check instance %d: %w", i, err)
		}
	}
	return nil
}

type pointEvalInstance struct {
	VersionedHash        [nbVersionedHashLimbs]frontend.Variable
	EvaluationPoint      scalarElementWizard
	ClaimedValue         scalarElementWizard
	CommitmentCompressed [nbG1CompressedLimbs]frontend.Variable
	ProofCompressed      [nbG1CompressedLimbs]frontend.Variable
	ExpectedBlobSize     [2]frontend.Variable
	ExpectedBlsModulus   [nbFrLimbs]frontend.Variable
}

func (c *pointEvalInstance) Check(api frontend.API, fr *emulated.Field[sw_bls12381.ScalarField]) error {
	tEvaluationPoint := c.EvaluationPoint.ToElement(api, fr)
	tClaimedValue := c.ClaimedValue.ToElement(api, fr)

	return evmprecompiles.KzgPointEvaluation(
		api,
		c.VersionedHash,
		tEvaluationPoint,
		tClaimedValue,
		c.CommitmentCompressed,
		c.ProofCompressed,
		c.ExpectedBlobSize,
		c.ExpectedBlsModulus,
	)
}

type multiPointEvalFailureCircuit struct {
	Instances []pointEvalFailureInstance `gnark:",public"`
}

func newMultiPointEvalFailureCircuit(limits *Limits) *multiPointEvalFailureCircuit {
	return &multiPointEvalFailureCircuit{
		Instances: make([]pointEvalFailureInstance, limits.NbPointEvalFailureInputInstances),
	}
}

func (c *multiPointEvalFailureCircuit) Define(api frontend.API) error {
	fr, err := emulated.NewField[sw_bls12381.ScalarField](api)
	if err != nil {
		return fmt.Errorf("new field: %w", err)
	}
	for i := range c.Instances {
		if err := c.Instances[i].Check(api, fr); err != nil {
			return fmt.Errorf("check instance %d: %w", i, err)
		}
	}
	return nil
}

type pointEvalFailureInstance struct {
	VersionedHash        [nbVersionedHashLimbs]frontend.Variable
	EvaluationPoint      scalarElementWizard
	ClaimedValue         scalarElementWizard
	CommitmentCompressed [nbG1CompressedLimbs]frontend.Variable
	ProofCompressed      [nbG1CompressedLimbs]frontend.Variable
	ExpectedBlobSize     [2]frontend.Variable
	ExpectedBlsModulus   [nbFrLimbs]frontend.Variable
}

func (c *pointEvalFailureInstance) Check(api frontend.API, fr *emulated.Field[sw_bls12381.ScalarField]) error {
	tEvaluationPoint := c.EvaluationPoint.ToElement(api, fr)
	tClaimedValue := c.ClaimedValue.ToElement(api, fr)

	return evmprecompiles.KzgPointEvaluationFailure(
		api,
		c.VersionedHash,
		tEvaluationPoint,
		tClaimedValue,
		c.CommitmentCompressed,
		c.ProofCompressed,
		c.ExpectedBlobSize,
		c.ExpectedBlsModulus,
	)
}
