package lookup

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

// assignmentStrat is a function responsible for assigning a list of column in
// a prover runtime. It is used to specify how the table should be assigned.
type assignmentStrat func(run *wizard.ProverRuntime, cols ...ifaces.Column)

// lookupTestCase represents a generic test case for the lookup compiler. It
// can feature multiple different tables in the same test.
type lookupTestCase struct {
	// Title provides an explainer of what the test case is about
	Title string
	// PerTableCases gives a list of queries for a list of including tables.
	PerTableCases []perTableCase
	// MustPanic specifies whether the test should panic.
	MustPanic bool
}

// perTableCase specifies how a test table and included tables should be
// constructed.
type perTableCase struct {
	// The number of columns in the table (excluding the conditional filters)
	NumCol int
	// StratIncluding specifies how to assign each fragment of the including
	// table.
	StratIncluding []assignmentStrat
	// StratIncluded specifies how to assign each included table
	StratIncluded []assignmentStrat
	// StratCondIncluding specifies the filter to employ on each fragment of the
	// including table. It can be left as nil if the test case does not use
	// filters on the including table.
	StratCondIncluding []assignmentStrat
	// StratCondIncluded specifies the filter to employ on each included table.
	// `nil` indicates that no filters are specified for the query.
	StratCondIncluded []assignmentStrat
}

func TestExhaustive(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	const (
		sizeS int = 1 << 3
		sizeT int = 1 << 4
	)

	var (
		// #nosec G404 -- we don't need a cryptographic PRNG for testing purposes
		rng          = rand.New(rand.NewChaCha8([32]byte{}))
		smallNumbers = smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15)
		xorTable     = [3]smartvectors.SmartVector{
			smartvectors.ForTest(0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3),
			smartvectors.ForTest(0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3),
			smartvectors.ForTest(0, 1, 2, 3, 1, 0, 3, 2, 2, 3, 0, 1, 3, 2, 1, 0),
		}

		// only suitable for T
		assignSmallNumbers = func(run *wizard.ProverRuntime, cols ...ifaces.Column) {
			if len(cols) != 1 {
				panic("only works with 1 columns")
			}
			run.AssignColumn(cols[0].GetColID(), smallNumbers)
		}

		// only suitable for T
		assignXorTable = func(run *wizard.ProverRuntime, cols ...ifaces.Column) {
			if len(cols) != 3 {
				panic("only works with 3 columns")
			}
			for i := range cols {
				run.AssignColumn(cols[i].GetColID(), xorTable[i])
			}
		}

		// suitable for any table or column
		assignZeroes = func(run *wizard.ProverRuntime, cols ...ifaces.Column) {
			size := cols[0].Size()
			for i := range cols {
				run.AssignColumn(cols[i].GetColID(), smartvectors.NewConstant(field.Zero(), size))
			}
		}

		// suitable for any table or column
		assignOnes = func(run *wizard.ProverRuntime, cols ...ifaces.Column) {
			size := cols[0].Size()
			for i := range cols {
				run.AssignColumn(cols[i].GetColID(), smartvectors.NewConstant(field.One(), size))
			}
		}

		// suitable for any table or column
		assignRandoms = func(run *wizard.ProverRuntime, cols ...ifaces.Column) {
			size := cols[0].Size()
			for i := range cols {
				run.AssignColumn(cols[i].GetColID(), smartvectors.PseudoRand(rng, size))
			}
		}

		// suitable for any table or column
		assignRandom1Bit = func(run *wizard.ProverRuntime, cols ...ifaces.Column) {
			size := cols[0].Size()
			for i := range cols {
				vec := make([]int, size)
				for j := range vec {
					vec[j] = rng.IntN(2)
				}
				run.AssignColumn(cols[i].GetColID(), smartvectors.ForTest(vec...))
			}
		}

		// suitable for any table or column
		assignRandom4Bit = func(run *wizard.ProverRuntime, cols ...ifaces.Column) {
			size := cols[0].Size()
			for i := range cols {
				vec := make([]int, size)
				for j := range vec {
					vec[j] = rng.IntN(4)
				}
				run.AssignColumn(cols[i].GetColID(), smartvectors.ForTest(vec...))
			}
		}

		// suitable for tables of 3 columns
		randomXor = func(run *wizard.ProverRuntime, cols ...ifaces.Column) {
			if len(cols) != 3 {
				panic("only works with 3 columns")
			}
			size := cols[0].Size()
			vecs := [3][]int{}
			for j := 0; j < size; j++ {
				x, y := rng.IntN(4), rng.IntN(4)
				z := x ^ y
				vecs[0] = append(vecs[0], x)
				vecs[1] = append(vecs[1], y)
				vecs[2] = append(vecs[2], z)
			}
			for i := range cols {
				run.AssignColumn(cols[i].GetColID(), smartvectors.ForTest(vecs[i]...))
			}
		}
	)

	testCases := []lookupTestCase{
		{
			Title: "range check for one column",
			PerTableCases: []perTableCase{
				{
					NumCol:         1,
					StratIncluding: []assignmentStrat{assignSmallNumbers},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
					},
				},
			},
		},
		{
			Title: "range check for one column (negative)",
			PerTableCases: []perTableCase{
				{
					NumCol:         1,
					StratIncluding: []assignmentStrat{assignSmallNumbers},
					StratIncluded: []assignmentStrat{
						assignRandoms,
					},
				},
			},
			MustPanic: true,
		},
		{
			Title: "range check but the including column is fully masked",
			PerTableCases: []perTableCase{
				{
					NumCol:             1,
					StratIncluding:     []assignmentStrat{assignSmallNumbers},
					StratCondIncluding: []assignmentStrat{assignZeroes},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
					},
				},
			},
			MustPanic: true,
		},
		{
			Title: "range check for one column with conditional (full ones)",
			PerTableCases: []perTableCase{
				{
					NumCol:         1,
					StratIncluding: []assignmentStrat{assignSmallNumbers},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
					},
					StratCondIncluded: []assignmentStrat{
						assignOnes,
					},
				},
			},
		},
		{
			Title: "range check for a full random column with conditional (full zeroes)",
			PerTableCases: []perTableCase{
				{
					NumCol:         1,
					StratIncluding: []assignmentStrat{assignSmallNumbers},
					StratIncluded: []assignmentStrat{
						assignRandoms,
					},
					StratCondIncluded: []assignmentStrat{
						assignZeroes,
					},
				},
			},
		},
		{
			Title: "range check for a full random column with conditional (negative)",
			PerTableCases: []perTableCase{
				{
					NumCol:         1,
					StratIncluding: []assignmentStrat{assignSmallNumbers},
					StratIncluded: []assignmentStrat{
						assignRandoms,
					},
					StratCondIncluded: []assignmentStrat{
						assignOnes,
					},
				},
			},
			MustPanic: true,
		},
		{
			Title: "range checks with multiple conditional and one non conditional",
			PerTableCases: []perTableCase{
				{
					NumCol:         1,
					StratIncluding: []assignmentStrat{assignSmallNumbers},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandoms,
					},
					StratCondIncluded: []assignmentStrat{
						nil,
						assignOnes,
						assignZeroes,
					},
				},
			},
		},
		{
			Title: "range checks with multiple conditional and one non conditional (negative)",
			PerTableCases: []perTableCase{
				{
					NumCol:         1,
					StratIncluding: []assignmentStrat{assignSmallNumbers},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandoms,
					},
					StratCondIncluded: []assignmentStrat{
						nil,
						assignOnes,
						nil,
					},
				},
			},
			MustPanic: true,
		},
		{
			Title: "xor check",
			PerTableCases: []perTableCase{
				{
					NumCol:         3,
					StratIncluding: []assignmentStrat{assignXorTable},
					StratIncluded: []assignmentStrat{
						randomXor,
					},
				},
			},
		},
		{
			Title: "xor check with random conditionals",
			PerTableCases: []perTableCase{
				{
					NumCol:         3,
					StratIncluding: []assignmentStrat{assignXorTable},
					StratIncluded: []assignmentStrat{
						randomXor,
					},
					StratCondIncluded: []assignmentStrat{
						assignRandom1Bit,
					},
				},
			},
		},
		{
			Title: "xor check with zeroed conditional and random included",
			PerTableCases: []perTableCase{
				{
					NumCol:         3,
					StratIncluding: []assignmentStrat{assignXorTable},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
					},
					StratCondIncluded: []assignmentStrat{
						assignZeroes,
					},
				},
			},
		},
		{
			Title: "xor check with fully masked including table",
			PerTableCases: []perTableCase{
				{
					NumCol:         3,
					StratIncluding: []assignmentStrat{assignXorTable},
					StratIncluded: []assignmentStrat{
						randomXor,
					},
					StratCondIncluding: []assignmentStrat{assignZeroes},
				},
			},
			MustPanic: true,
		},
		{
			Title: "range check for 2 by 2 column (fragmented)",
			PerTableCases: []perTableCase{
				{
					NumCol:         1,
					StratIncluding: []assignmentStrat{assignSmallNumbers, assignRandom4Bit},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
						assignRandom1Bit,
					},
				},
			},
		},
		{
			Title: "range check for 2 by 2 column with filters to 1 on the including side",
			PerTableCases: []perTableCase{
				{
					NumCol:         1,
					StratIncluding: []assignmentStrat{assignSmallNumbers, assignRandom4Bit},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
						assignRandom1Bit,
					},
					StratCondIncluding: []assignmentStrat{
						assignOnes,
						assignOnes,
					},
				},
			},
		},
		{
			Title: "range check for 2 by 2 column with filters to 1 on the included side",
			PerTableCases: []perTableCase{
				{
					NumCol:         1,
					StratIncluding: []assignmentStrat{assignSmallNumbers, assignRandom4Bit},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
						assignRandom1Bit,
					},
					StratCondIncluded: []assignmentStrat{
						assignOnes,
						assignOnes,
					},
				},
			},
		},
		{
			Title: "range check for 2 by 2 column with two conditions on either side of the query",
			PerTableCases: []perTableCase{
				{
					NumCol: 1,
					StratIncluding: []assignmentStrat{
						assignSmallNumbers,
						assignRandom4Bit,
					},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
						assignRandom1Bit,
					},
					StratCondIncluding: []assignmentStrat{
						assignOnes,
						assignZeroes,
					},
					StratCondIncluded: []assignmentStrat{
						assignRandom1Bit,
						assignRandom1Bit,
					},
				},
			},
		},
		{
			Title: "plenty of range checks",
			PerTableCases: []perTableCase{
				{
					NumCol: 1,
					StratIncluding: []assignmentStrat{
						assignSmallNumbers,
					},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
					},
				},
			},
		},
		{
			Title: "plenty of range checks with conditional",
			PerTableCases: []perTableCase{
				{
					NumCol: 1,
					StratIncluding: []assignmentStrat{
						assignSmallNumbers,
					},
					StratIncluded: []assignmentStrat{
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
						assignRandom4Bit,
					},
					StratCondIncluded: []assignmentStrat{
						assignRandom1Bit,
						assignRandom1Bit,
						assignRandom1Bit,
						assignRandom1Bit,
						assignRandom1Bit,
						assignRandom1Bit,
						assignRandom1Bit,
						assignRandom1Bit,
					},
				},
			},
		},
	}

	for tcID, testCase := range testCases {

		t.Run(
			fmt.Sprintf("testcase-%v-title=%v", tcID, testCase.Title),
			func(t *testing.T) {

				// def is the definition function for the test case. It also
				// internally schedules the assignment so that we do not have
				// additionally declare a prove function.
				def := func(b *wizard.Builder) {
					for tabID, tabCase := range testCase.PerTableCases {

						// This declare the table and its conditional
						table := make([][]ifaces.Column, len(tabCase.StratIncluding))
						var condTable []ifaces.Column
						if tabCase.StratCondIncluding != nil {
							condTable = make([]ifaces.Column, len(tabCase.StratCondIncluding))
						}

						for frag := range table {
							table[frag] = make([]ifaces.Column, tabCase.NumCol)
							for col := range table[frag] {
								table[frag][col] = b.InsertCommit(
									0,
									ifaces.ColIDf("TAB_%v_FRAG_%v_COL_%v", tabID, frag, col),
									sizeT,
								)
							}

							if tabCase.StratCondIncluding != nil && tabCase.StratCondIncluding[frag] != nil {
								condTable[frag] = b.InsertCommit(
									0,
									ifaces.ColIDf("TAB_%v_FRAG_%v_COND", tabID, frag),
									sizeT,
								)
							}
						}

						b.SubProvers.AppendToInner(0, func(run *wizard.ProverRuntime) {
							for frag := range tabCase.StratIncluding {
								tabCase.StratIncluding[frag](run, table[frag]...)
								if condTable != nil {
									tabCase.StratCondIncluding[frag](run, condTable[frag])
								}
							}
						})

						// This declare the included ones
						for incID := range tabCase.StratIncluded {
							included := make([]ifaces.Column, tabCase.NumCol)
							for i := range included {
								included[i] = b.RegisterCommit(
									ifaces.ColIDf("TAB_%v_SUB_%v_COL_%v", tabID, incID, i),
									sizeS,
								)
							}

							var condInc ifaces.Column
							if tabCase.StratCondIncluded != nil && tabCase.StratCondIncluded[incID] != nil {
								condInc = b.InsertCommit(
									0,
									ifaces.ColIDf("TAB_%v_SUB_%v_COND", tabID, incID),
									sizeS,
								)
							}

							b.SubProvers.AppendToInner(0, func(run *wizard.ProverRuntime) {
								tabCase.StratIncluded[incID](run, included...)
								if tabCase.StratCondIncluded != nil && tabCase.StratCondIncluded[incID] != nil {
									tabCase.StratCondIncluded[incID](run, condInc)
								}
							})

							b.GenericFragmentedConditionalInclusion(
								0,
								ifaces.QueryIDf("INCLUSION_%v_%v", tabID, incID),
								table,
								included,
								condTable,
								condInc,
							)

						}
					}
				}

				comp := wizard.Compile(def, CompileLogDerivative, dummy.Compile)

				if testCase.MustPanic {
					defer func() {
						if r := recover(); r == nil {
							t.Fatalf("The test did not panic")
						}
					}()
				}

				proof := wizard.Prove(comp, func(_ *wizard.ProverRuntime) {})

				err := wizard.Verify(comp, proof)
				if err != nil {
					t.Fatalf("The prover output a proof but it was invalid: %v", err)
				}
			})
	}
}
