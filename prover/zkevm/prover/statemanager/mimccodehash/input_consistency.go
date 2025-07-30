package mimccodehash

import (
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// inputModules is an optional sub-component of [Module] collecting the columns
// of the Rom and the RomLew module. It is optional in the sense that it may
// be omitted in tests but it is not optional in production.
type inputModules struct {
	RomInput    *RomInput
	RomLexInput *RomLexInput
}

// This function checks if the codehash module properly takes inputs from the
// Rom and RomLex module via a projection and two lookup queries
//
// @alex: since this module cannot currently be assigned without running this
// we should perhaps make this part of the main [NewModule] constructor as it
// is not actually optional.
func (mch *Module) ConnectToRom(comp *wizard.CompiledIOP,
	romInput *RomInput,
	romLexInput *RomLexInput) *Module {

	romInput.complete(comp)

	// Projection query between romInput and MiMCCodeHash module
	comp.InsertProjection(
		ifaces.QueryIDf("PROJECTION_ROM_MIMC_CODE_HASH_%v", mch.Inputs.Name),
		query.ProjectionInput{ColumnA: []ifaces.Column{romInput.CFI, romInput.Acc, romInput.CodeSize},
			ColumnB: []ifaces.Column{mch.CFI, mch.Limb, mch.CodeSize},
			FilterA: romInput.CounterIsEqualToNBytesMinusOne,
			FilterB: mch.IsActive})

	// Lookup between romLexInput and mch for
	// {CFI, codeHashHi, codeHashLo}
	comp.InsertInclusion(0,
		ifaces.QueryIDf("LOOKUP_MIMC_CODE_HASH_ROMLEX_%v", mch.Inputs.Name),
		[]ifaces.Column{mch.CFI, mch.CodeHashHi, mch.CodeHashLo},
		[]ifaces.Column{romLexInput.CFIRomLex, romLexInput.CodeHashHi, romLexInput.CodeHashLo})

	// And the reverse lookup
	comp.InsertInclusion(0,
		ifaces.QueryIDf("LOOKUP_ROMLEX_MIMC_CODE_HASH_%v", mch.Inputs.Name),
		[]ifaces.Column{romLexInput.CFIRomLex, romLexInput.CodeHashHi, romLexInput.CodeHashLo},
		[]ifaces.Column{mch.CFI, mch.CodeHashHi, mch.CodeHashLo})

	mch.InputModules = &inputModules{
		RomInput:    romInput,
		RomLexInput: romLexInput,
	}

	return mch
}
