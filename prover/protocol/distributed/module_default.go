package distributed

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// defaultConglomeratorRecursionWitness constructs a default recursion witness
// that can be used as filling for the witness of a conglomerated module that
// was not used in the proof.
//
// The default witness is generated from a dummy compiled-IOP and plays the role
// of a GL witness. Its wizard contains a single column satisfying the constraint
// P = 0. It also has the same public inputs as the other components of an actual
// segment proof. All set to the constant equal to zero.
//
// Its size is [initialCompilerSize] to match the spec of the compiler.
type DefaultModule struct {
	Column ifaces.Column
	Wiop   *wizard.CompiledIOP
}

// BuildDefaultModule returns a [DefaultModule]
func BuildDefaultModule(fmi *FilteredModuleInputs) *DefaultModule {

	var module *DefaultModule
	wizard.Compile(func(build *wizard.Builder) {
		module = DefineDefaultModule(build, fmi)
	})
	return module
}

// DefineDefaultModule defines a [DefaultModule] in the provided [Builder].
func DefineDefaultModule(builder *wizard.Builder, moduleInputs *FilteredModuleInputs) *DefaultModule {
	md := &DefaultModule{
		Column: builder.RegisterCommit("P", initialCompilerSize),
		Wiop:   builder.CompiledIOP,
	}

	builder.GlobalConstraint("P = 0", symbolic.NewVariable(md.Column))

	for i := range moduleInputs.PublicInputs {
		md.Wiop.InsertPublicInput(
			moduleInputs.PublicInputs[i].Name,
			accessors.NewConstant(field.Zero()),
		)
	}

	// These are the "dummy" public inputs that are only here so that the
	// moduleGL and moduleLPP have identical set of public inputs. The order
	// of declaration is also important. Namely, these needs to be declared before
	// the non-dummy ones.
	md.Wiop.InsertPublicInput(InitialRandomnessPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(IsFirstPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(IsLastPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(GlobalSenderPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(GlobalReceiverPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(LogDerivativeSumPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(GrandProductPublicInput, accessors.NewConstant(field.One()))
	md.Wiop.InsertPublicInput(HornerPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(HornerN0HashPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(HornerN1HashPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(IsGlPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(IsLppPublicInput, accessors.NewConstant(field.Zero()))
	md.Wiop.InsertPublicInput(NbActualLppPublicInput, accessors.NewConstant(field.Zero()))

	return md
}

// Prove sets the default witness in the provided [recursion.Witness].
func (md *DefaultModule) Assign(run *wizard.ProverRuntime) {
	run.AssignColumn(md.Column.GetColID(), smartvectors.NewConstant(field.Zero(), 1<<17))
}
