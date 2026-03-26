package logderivativesum_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	logderiv "github.com/consensys/linea-monorepo/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/stretchr/testify/require"
)

// It tests that the given expression for the LogDerivativeSum adds up to the given parameter.
func TestLogDerivativeSum(t *testing.T) {

	define := func(b *wizard.Builder) {
		var (
			comp = b.CompiledIOP
		)

		p0 := b.RegisterCommit("Num_0", 4)
		p1 := b.RegisterCommit("Num_1", 4)
		p2 := b.RegisterCommit("Num_2", 4)

		q0 := b.RegisterCommit("Den_0", 4)
		q1 := b.RegisterCommit("Den_1", 4)
		q2 := b.RegisterCommit("Den_2", 4)

		numerators := []*symbolic.Expression{
			symbolic.Mul(p0, -1),
			ifaces.ColumnAsVariable(p1),
			symbolic.Mul(p2, p0, 2),
		}

		denominators := []*symbolic.Expression{
			ifaces.ColumnAsVariable(q0),
			ifaces.ColumnAsVariable(q1),
			ifaces.ColumnAsVariable(q2),
		}

		size := 4
		zCat1 := query.LogDerivativeSumInput{}
		for i := range numerators {

			zCat1.Parts = append(zCat1.Parts, query.LogDerivativeSumPart{
				Size: size,
				Num:  numerators[i],
				Den:  denominators[i],
			})
		}
		comp.InsertLogDerivativeSum(0, "LogDerivSum_Test", zCat1)

	}

	prover := func(run *wizard.ProverRuntime) {

		run.AssignColumn("Num_0", smartvectors.ForTest(1, 1, 1, 1))
		run.AssignColumn("Num_1", smartvectors.ForTest(2, 3, 7, 9))
		run.AssignColumn("Num_2", smartvectors.ForTest(5, 6, 1, 1))

		run.AssignColumn("Den_0", smartvectors.ForTest(1, 1, 1, 1))
		run.AssignColumn("Den_1", smartvectors.ForTest(2, 3, 7, 9))
		run.AssignColumn("Den_2", smartvectors.ForTest(5, 6, 1, 1))

		run.AssignLogDerivSum("LogDerivSum_Test", fext.SetGenericInt64(8))

	}

	compiled := wizard.Compile(define, logderiv.CompileLogDerivativeSum, dummy.Compile)
	proof := wizard.Prove(compiled, prover)
	valid := wizard.Verify(compiled, proof)
	require.NoError(t, valid)
}

func TestLogDerivativeSumMixed(t *testing.T) {

	define := func(b *wizard.Builder) {
		var (
			comp = b.CompiledIOP
		)

		p0 := b.RegisterCommitExt("Num_0", 4)
		p1 := b.RegisterCommitExt("Num_1", 4)
		p2 := b.RegisterCommitExt("Num_2", 4)
		p3 := b.RegisterCommitExt("Num_3", 4)

		q0 := b.RegisterCommitExt("Den_0", 4)
		q1 := b.RegisterCommitExt("Den_1", 4)
		q2 := b.RegisterCommitExt("Den_2", 4)
		q3 := b.RegisterCommitExt("Den_3", 4)

		numerators := []*symbolic.Expression{
			ifaces.ColumnAsVariable(p0),
			symbolic.Mul(p0, p1),
			symbolic.Sub(p2, symbolic.Mul(p0, 4), symbolic.Mul(p0, p1)),
			ifaces.ColumnAsVariable(p3),
		}

		denominators := []*symbolic.Expression{
			ifaces.ColumnAsVariable(q0),
			ifaces.ColumnAsVariable(q1),
			ifaces.ColumnAsVariable(q2),
			ifaces.ColumnAsVariable(q3),
		}

		size := 4
		zCat1 := query.LogDerivativeSumInput{}

		for i := range numerators {
			zCat1.Parts = append(zCat1.Parts, query.LogDerivativeSumPart{
				Size: size,
				Num:  numerators[i],
				Den:  denominators[i],
			})
		}
		comp.InsertLogDerivativeSum(0, "LogDerivSum_Test", zCat1)

	}

	prover := func(run *wizard.ProverRuntime) {

		run.AssignColumn("Num_0", smartvectors.ForTest(1, 2, 3, 4))
		run.AssignColumn("Num_1", smartvectors.ForTest(2, 3, 7, 9))
		run.AssignColumn("Num_2", smartvectors.ForTest(3, 6, 9, 12))
		run.AssignColumn("Num_3", smartvectors.ForTest(1, 1, -1, -1))

		run.AssignColumn("Den_0", smartvectors.ForExtTest(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16))
		run.AssignColumn("Den_1", smartvectors.ForExtTest(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16))
		run.AssignColumn("Den_2", smartvectors.ForExtTest(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16))
		run.AssignColumn("Den_3", smartvectors.ForTest(3, 4, 3, 4))

		expectedResult := field.NewElement(0)
		run.AssignLogDerivSum("LogDerivSum_Test", fext.NewGenFieldFromBase(expectedResult))

	}

	compiled := wizard.Compile(define, logderiv.CompileLogDerivativeSum, dummy.Compile)
	proof := wizard.Prove(compiled, prover)
	valid := wizard.Verify(compiled, proof)
	require.NoError(t, valid)
}

func TestLogDerivativeSumMixed2(t *testing.T) {

	define := func(b *wizard.Builder) {
		var (
			comp = b.CompiledIOP
		)

		p0 := b.RegisterCommitExt("Num_0", 4)
		p1 := b.RegisterCommitExt("Num_1", 4)
		p2 := b.RegisterCommitExt("Num_2", 4)

		q0 := b.RegisterCommitExt("Den_0", 4)
		q1 := b.RegisterCommitExt("Den_1", 4)
		q2 := b.RegisterCommitExt("Den_2", 4)

		numerators := []*symbolic.Expression{
			symbolic.Mul(p0, -1),
			ifaces.ColumnAsVariable(p1),
			symbolic.Mul(p2, p0, 2),
		}

		denominators := []*symbolic.Expression{
			ifaces.ColumnAsVariable(q0),
			ifaces.ColumnAsVariable(q1),
			ifaces.ColumnAsVariable(q2),
		}

		size := 4
		zCat1 := query.LogDerivativeSumInput{}
		for i := range numerators {
			zCat1.Parts = append(zCat1.Parts, query.LogDerivativeSumPart{
				Size: size,
				Num:  numerators[i],
				Den:  denominators[i],
			})
		}

		comp.InsertLogDerivativeSum(0, "LogDerivSum_Test", zCat1)
	}

	prover := func(run *wizard.ProverRuntime) {

		run.AssignColumn("Num_0", smartvectors.ForTest(1, 1, 1, 1))
		run.AssignColumn("Num_1", smartvectors.ForTest(2, 3, 7, 9))
		run.AssignColumn("Num_2", smartvectors.ForTest(5, 6, 1, 1))

		run.AssignColumn("Den_0", smartvectors.ForExtTest(1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0))
		run.AssignColumn("Den_1", smartvectors.ForExtTest(2, 0, 0, 0, 3, 0, 0, 0, 7, 0, 0, 0, 9, 0, 0, 0))
		run.AssignColumn("Den_2", smartvectors.ForExtTest(5, 0, 0, 0, 6, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0))

		run.AssignLogDerivSum("LogDerivSum_Test", fext.SetGenericInt64(8))

	}

	compiled := wizard.Compile(define, logderiv.CompileLogDerivativeSum, dummy.Compile)
	proof := wizard.Prove(compiled, prover)
	valid := wizard.Verify(compiled, proof)
	require.NoError(t, valid)
}
