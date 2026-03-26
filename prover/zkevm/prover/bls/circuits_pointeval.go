package bls

import (
	"fmt"
	"slices"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bls12381"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/emulated"
)

const (
	nbG1CompressedLimbs  = 24
	nbVersionedHashLimbs = 16
	nbRowsPerPointEval   = nbVersionedHashLimbs + 2*nbFrLimbs + 2*nbG1CompressedLimbs + nbFrLimbs + nbFrLimbs
)

type PointEvalWizardElement struct {
	VersionedHash        [nbVersionedHashLimbs]frontend.Variable
	EvaluationPoint      scalarElementWizard
	ClaimedValue         scalarElementWizard
	CommitmentCompressed [nbG1CompressedLimbs]frontend.Variable
	ProofCompressed      [nbG1CompressedLimbs]frontend.Variable
	ExpectedBlobSize     [nbFrLimbs]frontend.Variable
	ExpectedBlsModulus   [nbFrLimbs]frontend.Variable
}

type pointEvalGnarkInstance struct {
	VersionedHash        [nbVersionedHashLimbs]frontend.Variable
	EvaluationPoint      *sw_bls12381.Scalar
	ClaimedValue         *sw_bls12381.Scalar
	CommitmentCompressed [nbG1CompressedLimbs]frontend.Variable
	ProofCompressed      [nbG1CompressedLimbs]frontend.Variable
	ExpectedBlobSize     [nbFrLimbs]frontend.Variable
	ExpectedBlsModulus   [nbFrLimbs]frontend.Variable
}

// ToGnarkInstance converts the pointEvalInput into the appropriate gnark instance format
func (c PointEvalWizardElement) ToGnarkInstance(api frontend.API, fr *emulated.Field[sw_bls12381.ScalarField]) *pointEvalGnarkInstance {
	// We need to reorder limbs from MSB to LSB
	tEvaluationPoint := c.EvaluationPoint.ToElement(api, fr)
	tClaimedValue := c.ClaimedValue.ToElement(api, fr)
	var versionedHash [nbVersionedHashLimbs]frontend.Variable
	copy(versionedHash[0:8], c.VersionedHash[8:16])
	copy(versionedHash[8:16], c.VersionedHash[0:8])
	slices.Reverse(versionedHash[:])
	var commitmentCompressed [nbG1CompressedLimbs]frontend.Variable
	copy(commitmentCompressed[0:8], c.CommitmentCompressed[16:24])
	copy(commitmentCompressed[8:16], c.CommitmentCompressed[8:16])
	copy(commitmentCompressed[16:24], c.CommitmentCompressed[0:8])
	slices.Reverse(commitmentCompressed[:])
	var proofCompressed [nbG1CompressedLimbs]frontend.Variable
	copy(proofCompressed[0:8], c.ProofCompressed[16:24])
	copy(proofCompressed[8:16], c.ProofCompressed[8:16])
	copy(proofCompressed[16:24], c.ProofCompressed[0:8])
	slices.Reverse(proofCompressed[:])
	var expectedBlobSize [nbFrLimbs]frontend.Variable
	copy(expectedBlobSize[0:8], c.ExpectedBlobSize[8:16])
	copy(expectedBlobSize[8:16], c.ExpectedBlobSize[0:8])
	slices.Reverse(expectedBlobSize[:])
	var expectedBlsModulus [nbFrLimbs]frontend.Variable
	copy(expectedBlsModulus[0:8], c.ExpectedBlsModulus[8:16])
	copy(expectedBlsModulus[8:16], c.ExpectedBlsModulus[0:8])
	slices.Reverse(expectedBlsModulus[:])
	res := &pointEvalGnarkInstance{
		VersionedHash:        versionedHash,
		EvaluationPoint:      tEvaluationPoint,
		ClaimedValue:         tClaimedValue,
		CommitmentCompressed: commitmentCompressed,
		ProofCompressed:      proofCompressed,
		ExpectedBlobSize:     expectedBlobSize,
		ExpectedBlsModulus:   expectedBlsModulus,
	}
	return res
}

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
	PointEvalWizardElement
}

func (c *pointEvalInstance) Check(api frontend.API, fr *emulated.Field[sw_bls12381.ScalarField]) error {
	element := c.PointEvalWizardElement.ToGnarkInstance(api, fr)

	return evmprecompiles.KzgPointEvaluation16(
		api,
		element.VersionedHash,
		element.EvaluationPoint,
		element.ClaimedValue,
		element.CommitmentCompressed,
		element.ProofCompressed,
		element.ExpectedBlobSize,
		element.ExpectedBlsModulus,
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
	PointEvalWizardElement
}

func (c *pointEvalFailureInstance) Check(api frontend.API, fr *emulated.Field[sw_bls12381.ScalarField]) error {
	element := c.PointEvalWizardElement.ToGnarkInstance(api, fr)

	return evmprecompiles.KzgPointEvaluationFailure16(
		api,
		element.VersionedHash,
		element.EvaluationPoint,
		element.ClaimedValue,
		element.CommitmentCompressed,
		element.ProofCompressed,
		element.ExpectedBlobSize,
		element.ExpectedBlsModulus,
	)
}
