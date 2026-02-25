package bls

import (
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

func NewG1AddZkEvm(comp *wizard.CompiledIOP, limits *Limits) *BlsAdd {
	return newAdd(comp, G1, limits, newAddDataSource(comp, G1)).
		WithAddCircuit(comp, query.PlonkRangeCheckOption(16, 6, true)).
		WithCurveMembershipCircuit(comp, query.PlonkRangeCheckOption(16, 6, true))
}

func NewG2AddZkEvm(comp *wizard.CompiledIOP, limits *Limits) *BlsAdd {
	return newAdd(comp, G2, limits, newAddDataSource(comp, G2)).
		WithAddCircuit(comp, query.PlonkRangeCheckOption(16, 6, true)).
		WithCurveMembershipCircuit(comp, query.PlonkRangeCheckOption(16, 6, true))
}

func NewG1MsmZkEvm(comp *wizard.CompiledIOP, limits *Limits) *BlsMsm {
	return newMsm(comp, G1, limits, newMsmDataSource(comp, G1)).
		WithMsmCircuit(comp, query.PlonkRangeCheckOption(16, 6, true)).
		WithGroupMembershipCircuit(comp, query.PlonkRangeCheckOption(16, 6, true))
}

func NewG2MsmZkEvm(comp *wizard.CompiledIOP, limits *Limits) *BlsMsm {
	return newMsm(comp, G2, limits, newMsmDataSource(comp, G2)).
		WithMsmCircuit(comp, query.PlonkRangeCheckOption(16, 6, true)).
		WithGroupMembershipCircuit(comp, query.PlonkRangeCheckOption(16, 6, true))
}

func NewPairingZkEvm(comp *wizard.CompiledIOP, limits *Limits) *BlsPair {
	return newPair(comp, limits, newPairDataSource(comp)).
		WithPairingCircuit(comp, query.PlonkRangeCheckOption(16, 6, true)).
		WithG1MembershipCircuit(comp, query.PlonkRangeCheckOption(16, 6, true)).
		WithG2MembershipCircuit(comp, query.PlonkRangeCheckOption(16, 6, true))
}

func NewG1MapZkEvm(comp *wizard.CompiledIOP, limits *Limits) *BlsMap {
	return newMap(comp, G1, limits, newMapDataSource(comp, G1)).
		WithMapCircuit(comp, query.PlonkRangeCheckOption(16, 6, true))
}

func NewG2MapZkEvm(comp *wizard.CompiledIOP, limits *Limits) *BlsMap {
	return newMap(comp, G2, limits, newMapDataSource(comp, G2)).
		WithMapCircuit(comp, query.PlonkRangeCheckOption(16, 6, true))
}

func NewPointEvalZkEvm(comp *wizard.CompiledIOP, limits *Limits) *BlsPointEval {
	return newPointEval(comp, limits, newPointEvalDataSource(comp)).
		WithPointEvalCircuit(comp, query.PlonkRangeCheckOption(16, 6, true)).
		WithPointEvalFailureCircuit(comp, query.PlonkRangeCheckOption(16, 6, true))
}
