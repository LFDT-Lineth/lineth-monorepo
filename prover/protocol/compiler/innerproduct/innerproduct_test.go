package innerproduct

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

func TestInnerProduct(t *testing.T) {
	define := func(b *wizard.Builder) {
		for i, c := range testCases {
			bs := make([]ifaces.Column, len(c.bName))
			a := b.RegisterCommit(c.aName, c.size)
			for i, name := range c.bName {
				bs[i] = b.RegisterCommit(name, c.size)
			}
			b.InnerProduct(c.qName, a, bs...)
			// go to the next round
			_ = b.RegisterRandomCoin(coin.Namef("Coin_%v", i), coin.Field)
		}
	}
	prover := func(run *wizard.ProverRuntime) {
		for j, c := range testCases {
			run.AssignColumn(c.aName, c.a)
			for i, name := range c.bName {
				run.AssignColumn(name, c.b[i])
			}
			run.AssignInnerProduct(c.qName, c.expected...)
			run.GetRandomCoinField(coin.Namef("Coin_%v", j))
		}
	}

	comp := wizard.Compile(define, Compile(), dummy.Compile)
	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

var testCases = []struct {
	qName    ifaces.QueryID
	aName    ifaces.ColID
	bName    []ifaces.ColID
	size     int
	a        smartvectors.SmartVector
	b        []smartvectors.SmartVector
	expected []field.Element
}{
	{qName: "Quey1",
		aName: "ColA1",
		bName: []ifaces.ColID{"ColB1"},
		size:  4,
		a:     smartvectors.ForTest(1, 1, 1, 1),
		b: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2),
		},
		expected: []field.Element{field.NewElement(5)},
	},
	{qName: "Quey2",
		aName: "ColA2",
		bName: []ifaces.ColID{"ColB2_0", "ColB2_1"},
		size:  4,
		a:     smartvectors.ForTest(1, 1, 1, 1),
		b: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2),
			smartvectors.ForTest(1, 0, 0, 2),
		},
		expected: []field.Element{field.NewElement(5), field.NewElement(3)},
	},
	{qName: "Quey3",
		aName: "ColA3",
		bName: []ifaces.ColID{"ColB3_0", "ColB3_1"},
		size:  8,
		a:     smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0),
		b: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0),
			smartvectors.ForTest(1, 0, 0, 2, 1, 0, 0, 0),
		},
		expected: []field.Element{field.NewElement(7), field.NewElement(5)},
	},
	{qName: "Quey4",
		aName: "ColA4",
		bName: []ifaces.ColID{"ColB4"},
		size:  16,
		a:     smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		b: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		},
		expected: []field.Element{field.NewElement(15)},
	},

	{qName: "Quey",

		aName: "ColA",
		bName: []ifaces.ColID{"ColB"},
		size:  32,
		a:     smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		b: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		},
		expected: []field.Element{field.NewElement(30)},
	},
}
