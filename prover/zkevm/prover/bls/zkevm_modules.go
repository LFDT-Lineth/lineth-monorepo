package bls

import (
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
)

func NewG1AddZkEvm(comp *wizard.CompiledIOP, limits *Limits, arith *arithmetization.Arithmetization) *BlsAdd {
	return newAdd(comp, G1, limits, newAddDataSource(comp, G1, arith)).
		WithAddCircuit(comp, query.PlonkRangeCheckOption(16, 1, true)).
		WithCurveMembershipCircuit(comp, query.PlonkRangeCheckOption(16, 1, true))
}

func NewG2AddZkEvm(comp *wizard.CompiledIOP, limits *Limits, arith *arithmetization.Arithmetization) *BlsAdd {
	return newAdd(comp, G2, limits, newAddDataSource(comp, G2, arith)).
		WithAddCircuit(comp, query.PlonkRangeCheckOption(16, 1, true)).
		WithCurveMembershipCircuit(comp, query.PlonkRangeCheckOption(16, 1, true))
}

func NewG1MsmZkEvm(comp *wizard.CompiledIOP, limits *Limits, arith *arithmetization.Arithmetization) *BlsMsm {
	return newMsm(comp, G1, limits, newMsmDataSource(comp, G1, arith)).
		WithMsmCircuit(comp, query.PlonkRangeCheckOption(16, 1, true)).
		WithGroupMembershipCircuit(comp, query.PlonkRangeCheckOption(16, 1, true))
}

func NewG2MsmZkEvm(comp *wizard.CompiledIOP, limits *Limits, arith *arithmetization.Arithmetization) *BlsMsm {
	return newMsm(comp, G2, limits, newMsmDataSource(comp, G2, arith)).
		WithMsmCircuit(comp, query.PlonkRangeCheckOption(16, 1, true)).
		WithGroupMembershipCircuit(comp, query.PlonkRangeCheckOption(16, 1, true))
}

func NewPairingZkEvm(comp *wizard.CompiledIOP, limits *Limits, arith *arithmetization.Arithmetization) *BlsPair {
	return newPair(comp, limits, newPairDataSource(comp, arith)).
		WithPairingCircuit(comp, query.PlonkRangeCheckOption(16, 1, true)).
		WithG1MembershipCircuit(comp, query.PlonkRangeCheckOption(16, 1, true)).
		WithG2MembershipCircuit(comp, query.PlonkRangeCheckOption(16, 1, true)) // << just added
}

func NewG1MapZkEvm(comp *wizard.CompiledIOP, limits *Limits, arith *arithmetization.Arithmetization) *BlsMap {
	return newMap(comp, G1, limits, newMapDataSource(comp, G1, arith)).
		WithMapCircuit(comp, query.PlonkRangeCheckOption(16, 1, true))
}

func NewG2MapZkEvm(comp *wizard.CompiledIOP, limits *Limits, arith *arithmetization.Arithmetization) *BlsMap {
	return newMap(comp, G2, limits, newMapDataSource(comp, G2, arith)).
		WithMapCircuit(comp, query.PlonkRangeCheckOption(16, 1, true))
}

func NewPointEvalZkEvm(comp *wizard.CompiledIOP, limits *Limits, arith *arithmetization.Arithmetization) *BlsPointEval {
	return newPointEval(comp, limits, newPointEvalDataSource(comp, arith)).
		WithPointEvalCircuit(comp, query.PlonkRangeCheckOption(16, 1, true)).
		WithPointEvalFailureCircuit(comp, query.PlonkRangeCheckOption(16, 1, true))
}
