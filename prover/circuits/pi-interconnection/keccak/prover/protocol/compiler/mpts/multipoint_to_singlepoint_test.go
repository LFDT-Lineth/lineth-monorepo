package mpts

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/internal/testtools"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

func TestWithProfile(t *testing.T) {

	testcases := []struct {
		Profile []int
		UTC     *testtools.UnivariateTestcase
	}{
		{
			Profile: []int{3},
			UTC: &testtools.UnivariateTestcase{
				NameStr: "profile-3",
				Polys: []smartvectors.SmartVector{
					smartvectors.ForTest(1, 2, 3, 4),
				},
				QueryXs: []field.Element{
					field.NewElement(3),
				},
				QueryPols: [][]int{
					{0},
				},
			},
		},
	}

	for _, tc := range testcases {

		suite := []func(*wizard.CompiledIOP){
			Compile(WithNumColumnProfileOpt(tc.Profile, 0)),
			dummy.Compile,
		}

		t.Run(tc.UTC.NameStr, func(t *testing.T) {
			testtools.RunTestcase(t, tc.UTC, suite)
		})
	}

}

func TestCompiler(t *testing.T) {

	suite := []func(*wizard.CompiledIOP){
		Compile(),
		dummy.Compile,
	}

	for _, tc := range testtools.ListOfUnivariateTestcasesPositive {
		t.Run(tc.Name(), func(t *testing.T) {
			testtools.RunTestcase(t, tc, suite)
		})
	}
}

func TestCompilerWithGnark(t *testing.T) {

	suite := []func(*wizard.CompiledIOP){
		Compile(),
		dummy.Compile,
	}

	for _, tc := range testtools.ListOfUnivariateTestcasesPositive {
		t.Run(tc.Name(), func(t *testing.T) {
			testtools.RunTestShouldPassWithGnark(t, tc, suite)
		})
	}
}

func TestWithVerifierCol(t *testing.T) {

	suite := []func(*wizard.CompiledIOP){
		Compile(),
		dummy.Compile,
	}

	testcases := []*testtools.AnonymousTestcase{
		{
			NameStr: "with-constant-col",
			DefineFunc: func(comp *wizard.CompiledIOP) {
				u := verifiercol.NewConstantCol(field.Zero(), 8, "")
				comp.InsertUnivariate(0, "U", []ifaces.Column{u})
			},
			AssignFunc: func(run *wizard.ProverRuntime) {
				run.AssignUnivariate("U", field.Zero(), field.Zero())
			},
		},
		{
			NameStr: "with-constant-col-2",
			DefineFunc: func(comp *wizard.CompiledIOP) {
				u := verifiercol.NewConstantCol(field.NewElement(42), 8, "")
				comp.InsertUnivariate(0, "U", []ifaces.Column{u})
			},
			AssignFunc: func(run *wizard.ProverRuntime) {
				run.AssignUnivariate("U", field.Zero(), field.NewElement(42))
			},
		},
	}

	for i, tc := range testcases {
		t.Run(tc.NameStr, func(t *testing.T) {
			testtools.RunTestcase(t, testcases[i], suite)
		})
	}
}

func TestLdeOf(t *testing.T) {

	testcases := []struct {
		Name string
		Poly []field.Element
		LDE  []field.Element
	}{
		{
			Name: "constant-poly",
			Poly: vector.Repeat(field.NewElement(23), 8),
			LDE:  vector.Repeat(field.NewElement(23), 32),
		},
		{
			Name: "x-poly",
			Poly: vector.ForTest(1, -1),
			LDE:  vector.PowerVec(fft.GetOmega(8), 8),
		},
		{
			Name: "x-poly-2",
			Poly: vector.PowerVec(fft.GetOmega(4), 4),
			LDE:  vector.PowerVec(fft.GetOmega(8), 8),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {

			sizeBig := len(tc.LDE)
			res := make([]field.Element, sizeBig)
			copy(res, tc.Poly)
			_ldeOf(res, len(tc.Poly), sizeBig)

			for i := range tc.LDE {
				if !tc.LDE[i].Equal(&res[i]) {
					t.Errorf("mismatch res=%v, expected=%v", vector.Prettify(res), vector.Prettify(tc.LDE))
					return
				}
			}
		})
	}

}
