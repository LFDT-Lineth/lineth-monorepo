package bls

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bls12381"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/emulated"
)

const (
	nbRowsPerC1Membership = nbG1Limbs // point
	nbRowsPerC2Membership = nbG2Limbs // point
	nbRowsPerG1Membership = nbG1Limbs // point
	nbRowsPerG2Membership = nbG2Limbs // point
)

func nbRowsPerCurveMembership(g Group) int {
	switch g {
	case G1:
		return nbRowsPerC1Membership
	case G2:
		return nbRowsPerC2Membership
	default:
		panic("unknown group for nbRowsPerCurveMembership")
	}
}

func nbRowsPerGroupMembership(g Group) int {
	switch g {
	case G1:
		return nbRowsPerG1Membership
	case G2:
		return nbRowsPerG2Membership
	default:
		panic("unknown group for nbRowsPerGroupMembership")
	}
}

type nonCurveMembershipInstance[C convertable[T], T element] struct {
	// P is the purported element
	P C `gnark:",public"`
}

func (c nonCurveMembershipInstance[C, T]) Check(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField], pairing *sw_bls12381.Pairing) error {
	switch v := any(c.P).(type) {
	case g1ElementWizard:
		P := v.ToElement(api, fp)
		res := pairing.IsOnCurve(P)
		api.AssertIsEqual(res, 0)
		return nil
	case g2ElementWizard:
		Q := v.ToElement(api, fp)
		res := pairing.IsOnTwist(Q)
		api.AssertIsEqual(res, 0)
		return nil
	default:
		return fmt.Errorf("unsupported element type %T for on-curve check", c.P)
	}
}

type nonGroupMembershipInstance[C convertable[T], T element] struct {
	// P is the purported element
	P C `gnark:",public"`
}

func (c nonGroupMembershipInstance[C, T]) Check(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField], pairing *sw_bls12381.Pairing) error {
	switch v := any(c.P).(type) {
	case g1ElementWizard:
		P := v.ToElement(api, fp)
		// We expect the element to be not on G1
		return evmprecompiles.ECPairBLSIsOnG1(api, P, 0) // 0 means we expect it to be not on G1
	case g2ElementWizard:
		Q := v.ToElement(api, fp)
		// We expect the element to be not on G2
		return evmprecompiles.ECPairBLSIsOnG2(api, Q, 0) // 0 means we expect it to be not on G2
	default:
		return fmt.Errorf("unsupported element type %T for non-group membership check", c.P)
	}
}

// -- circuit which performs multiple checks

type checkableInstance interface {
	nonCurveMembershipInstance[g1ElementWizard, sw_bls12381.G1Affine] | nonCurveMembershipInstance[g2ElementWizard, sw_bls12381.G2Affine] |
		nonGroupMembershipInstance[g1ElementWizard, sw_bls12381.G1Affine] | nonGroupMembershipInstance[g2ElementWizard, sw_bls12381.G2Affine]
	Check(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField], pairing *sw_bls12381.Pairing) error
}

type multiCheckableCircuit[T checkableInstance] struct {
	Instances []T
}

func (c *multiCheckableCircuit[T]) Define(api frontend.API) error {
	fp, err := emulated.NewField[sw_bls12381.BaseField](api)
	if err != nil {
		return fmt.Errorf("new field emulation: %w", err)
	}
	pairing, err := sw_bls12381.NewPairing(api)
	if err != nil {
		return fmt.Errorf("new pairing: %w", err)
	}
	for i := range c.Instances {
		if err := c.Instances[i].Check(api, fp, pairing); err != nil {
			return fmt.Errorf("instance %d check: %w", i, err)
		}
	}
	return nil
}

func newMultiCheckableCircuit[T checkableInstance](nbInstances int) *multiCheckableCircuit[T] {
	return &multiCheckableCircuit[T]{
		Instances: make([]T, nbInstances),
	}
}

func newCheckCircuit(g Group, membership membership, limits *Limits) frontend.Circuit {
	switch g {
	case G1:
		switch membership {
		case CURVE:
			return newMultiCheckableCircuit[nonCurveMembershipInstance[g1ElementWizard, sw_bls12381.G1Affine]](limits.NbC1MembershipInputInstances)
		case GROUP:
			return newMultiCheckableCircuit[nonGroupMembershipInstance[g1ElementWizard, sw_bls12381.G1Affine]](limits.NbG1MembershipInputInstances)
		default:
			panic(fmt.Sprintf("unknown membership type for G1: %v", membership))
		}
	case G2:
		switch membership {
		case CURVE:
			return newMultiCheckableCircuit[nonCurveMembershipInstance[g2ElementWizard, sw_bls12381.G2Affine]](limits.NbC2MembershipInputInstances)
		case GROUP:
			return newMultiCheckableCircuit[nonGroupMembershipInstance[g2ElementWizard, sw_bls12381.G2Affine]](limits.NbG2MembershipInputInstances)
		default:
			panic(fmt.Sprintf("unknown membership type for G2: %v", membership))
		}
	default:
		panic(fmt.Sprintf("unknown group for bls curve membership circuit: %v", g))
	}
}
