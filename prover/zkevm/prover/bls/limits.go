package bls

type Limits struct {
	// LimitG1AddCalls on the total number of G1 additions that can be performed across
	// all addition circuits.
	LimitG1AddCalls int
	// LimitG2AddCalls on the total number of G2 additions that can be performed
	// across all addition circuits.
	LimitG2AddCalls int

	// Number of G1 addition input instances. Single instance is approx 11000
	// PLONK constraints
	NbG1AddInputInstances int
	// Number of G2 addition input instances. Single instance is approx 25000
	// PLONK constraints.
	NbG2AddInputInstances int

	// LimitG1MsmCalls on the total number of G1 scalar multiplications that can be
	// performed across all scalar multiplication circuits.
	LimitG1MsmCalls int
	// LimitG2MsmCalls on the total number of G2 scalar multiplications that can be
	// performed across all scalar multiplication circuits.
	LimitG2MsmCalls int

	// Number of G1 scalar multiplication input instances. Single instance is
	// approx 596000 PLONK constraints
	NbG1MulInputInstances int
	// Number of G2 scalar multiplication input instances. Single instance is
	// approx 1.2 million PLONK constraints
	NbG2MulInputInstances int

	// LimitMillerLoopCalls on the total number of Miller loops that can be performed
	// across all Miller loop circuits.
	LimitMillerLoopCalls int
	// LimitFinalExpCalls on the total number of final exponentiations that can be
	// performed across all final exponentiation circuits.
	LimitFinalExpCalls int

	// Number of inputs per Miller loop circuits. Counted without the last
	// Miller loop which is done in the final exponentiation part. Single
	// instance is approx 1.68M PLONK constraints
	NbMillerLoopInputInstances int
	// Number of inputs per final exponentiation circuits. Single instance is
	// approx 1.74M PLONK constraints.
	NbFinalExpInputInstances int

	// LimitG1MembershipCalls on the total number of G1 subgroup membership checks that can
	// be performed across all subgroup membership circuits.
	LimitG1MembershipCalls int
	// LimitG2MembershipCalls on the total number of G2 subgroup membership checks that
	// can be performed across all subgroup membership circuits.
	LimitG2MembershipCalls int

	// Number of inputs per G1 subgroup membership circuits. Single instance is
	// approx 317000 PLONK constraints.
	NbG1MembershipInputInstances int
	// Number of inputs per G2 subgroup membership circuits. Single instance is
	// approx 378000 PLONK constraints.
	NbG2MembershipInputInstances int

	// LimitMapFpToG1Calls on the total number of G1 maps to curve that can be performed
	// across all map to curve circuits.
	LimitMapFpToG1Calls int
	// LimitMapFp2ToG2Calls on the total number of G2 maps to curve that can be performed
	// across all map to curve circuits.
	LimitMapFp2ToG2Calls int

	// Number of G1 map to curve input instances. Single instance is approx
	// 217000 PLONK constraints.
	NbG1MapToInputInstances int
	// Number of G2 map to curve input instances. Single instance is approx
	// 797000 PLONK constraints.
	NbG2MapToInputInstances int

	// LimitC1MembershipCalls on the total number of C1 curve membership checks that can
	// be performed across all curve membership circuits.
	LimitC1MembershipCalls int
	// LimitC2MembershipCalls on the total number of C2 curve membership checks that can
	// be performed across all curve membership circuits.
	LimitC2MembershipCalls int

	// Number of C1 membership input instances. Single instance is approx 4000
	// PLONK constraints
	NbC1MembershipInputInstances int
	// Number of C2 membership input instances. Single instance is approx 8000
	// PLONK constraints.
	NbC2MembershipInputInstances int

	// LimitPointEvalCalls on the total number of point evaluations that can be performed
	// across all point evaluation circuits.
	LimitPointEvalCalls int
	// LimitPointEvalFailureCalls on the total number of point evaluation failures that
	// can be performed across all point evaluation failure circuits.
	LimitPointEvalFailureCalls int

	// Number of point evaluation input instances. Single instance is approx
	// 3.19M PLONK constraints.
	NbPointEvalInputInstances int
	// Number of point evaluation failure input instances. Single instance is approx
	// 5.34M PLONK constraints.
	NbPointEvalFailureInputInstances int
}

func (l *Limits) limitAddCalls(g Group) int {
	switch g {
	case G1:
		return l.LimitG1AddCalls
	case G2:
		return l.LimitG2AddCalls
	default:
		panic("unknown group for bls add calls")
	}
}

func (l *Limits) nbAddInputInstances(g Group) int {
	switch g {
	case G1:
		return l.NbG1AddInputInstances
	case G2:
		return l.NbG2AddInputInstances
	default:
		panic("unknown group for bls add input instances")
	}
}

func (l *Limits) limitMapCalls(g Group) int {
	switch g {
	case G1:
		return l.LimitMapFpToG1Calls
	case G2:
		return l.LimitMapFp2ToG2Calls
	default:
		panic("unknown group for bls map calls")
	}
}

func (l *Limits) nbMapInputInstances(g Group) int {
	switch g {
	case G1:
		return l.NbG1MapToInputInstances
	case G2:
		return l.NbG2MapToInputInstances
	default:
		panic("unknown group for bls map input instances")
	}
}

func (l *Limits) limitMulCalls(g Group) int {
	switch g {
	case G1:
		return l.LimitG1MsmCalls
	case G2:
		return l.LimitG2MsmCalls
	default:
		panic("unknown group for bls mul calls")
	}
}

func (l *Limits) nbMulInputInstances(g Group) int {
	switch g {
	case G1:
		return l.NbG1MulInputInstances
	case G2:
		return l.NbG2MulInputInstances
	default:
		panic("unknown group for bls mul input instances")
	}
}

func (l *Limits) limitCurveMembershipCalls(g Group) int {
	switch g {
	case G1:
		return l.LimitC1MembershipCalls
	case G2:
		return l.LimitC2MembershipCalls
	default:
		panic("unknown group for bls curve membership calls")
	}
}

func (l *Limits) nbCurveMembershipInputInstances(g Group) int {
	switch g {
	case G1:
		return l.NbC1MembershipInputInstances
	case G2:
		return l.NbC2MembershipInputInstances
	default:
		panic("unknown group for bls curve membership input instances")
	}
}

func (l *Limits) limitGroupMembershipCalls(g Group) int {
	switch g {
	case G1:
		return l.LimitG1MembershipCalls
	case G2:
		return l.LimitG2MembershipCalls
	default:
		panic("unknown group for bls group membership calls")
	}
}

func (l *Limits) nbGroupMembershipInputInstances(g Group) int {
	switch g {
	case G1:
		return l.NbG1MembershipInputInstances
	case G2:
		return l.NbG2MembershipInputInstances
	default:
		panic("unknown group for bls group membership input instances")
	}
}
