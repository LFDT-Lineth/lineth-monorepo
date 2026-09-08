package messagebus_test

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// makeVec builds a base-field ConcreteVector from uint64 literals.
func makeVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// declareBusCoins declares the α and β the message bus folds rows with, as
// early as they can be declared: round 1, the round the fixtures put their bus
// columns on. Round 0 cannot carry them — [wiop.Runtime.AdvanceRound] samples
// the coins of the round it enters, nothing ever enters round 0, and an
// unsampled coin panics the fold.
func declareBusCoins(sys *wiop.System) (alpha, beta *wiop.CoinField) {
	coinRound := sys.Rounds[0].EnsureNext()
	return coinRound.NewCoinField(sys.Context.Childf("alpha")),
		coinRound.NewCoinField(sys.Context.Childf("beta"))
}
