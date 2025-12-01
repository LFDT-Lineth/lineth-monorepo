package bls

import (
	"github.com/consensys/gnark/std/algebra/emulated/sw_bls12381"
)

// Type aliases for serialization registration
// These provide access to the concrete generic types needed by the serialization package

type (
	// G1 circuit types with sw_emulated.AffinePoint
	MultiAddCircuitG1        = multiAddCircuit[g1ElementWizard, sw_bls12381.G1Affine]
	MultiMulCircuitG1        = multiMulCircuit[g1ElementWizard, sw_bls12381.G1Affine]
	MultiMapCircuitG1        = multiMapCircuit[g1ElementWizard, sw_bls12381.G1Affine]
	MultiCheckableG1NonGroup = multiCheckableCircuit[nonGroupMembershipInstance[g1ElementWizard, sw_bls12381.G1Affine]]
	MultiCheckableG1NonCurve = multiCheckableCircuit[nonCurveMembershipInstance[g1ElementWizard, sw_bls12381.G1Affine]]
	// G2 circuit types
	MultiAddCircuitG2        = multiAddCircuit[g2ElementWizard, sw_bls12381.G2Affine]
	MultiMulCircuitG2        = multiMulCircuit[g2ElementWizard, sw_bls12381.G2Affine]
	MultiMapCircuitG2        = multiMapCircuit[g2ElementWizard, sw_bls12381.G2Affine]
	MultiCheckableG2NonGroup = multiCheckableCircuit[nonGroupMembershipInstance[g2ElementWizard, sw_bls12381.G2Affine]]
	MultiCheckableG2NonCurve = multiCheckableCircuit[nonCurveMembershipInstance[g2ElementWizard, sw_bls12381.G2Affine]]

	// Non-generic pairing and point eval circuits
	MultiMillerLoopMulCircuit      = multiMillerLoopMulCircuit
	MultiMillerLoopFinalExpCircuit = multiMillerLoopFinalExpCircuit
	MultiPointEvalCircuit          = multiPointEvalCircuit
	MultiPointEvalFailureCircuit   = multiPointEvalFailureCircuit
)
