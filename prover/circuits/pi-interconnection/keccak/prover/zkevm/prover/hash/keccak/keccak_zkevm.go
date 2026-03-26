// The keccak package accumulates the providers from different zkEVM modules,
//
//	and proves the hash consistency over the unified provider.
//
// The provider encodes the inputs and outputs of the hash from different modules.
package keccak

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	gen_acc "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/keccak/acc_module"
)

// ShakiraProverAction is a ProverAction for the SHAKIRA module, serializable for
// KeccakZkEVM. It assigns the ManuallyShifted column.
type ShakiraProverAction struct {
	SelectorKeccakResLo *dedicated.ManuallyShifted
}

// Run implements the wizard.ProverAction interface.
func (s *ShakiraProverAction) Run(run *wizard.ProverRuntime) {
	s.SelectorKeccakResLo.Assign(run)
}

type KeccakZkEVM struct {
	Settings *Settings

	// SuppProverSteps is a list of prover steps to be run
	// by the KeccakZKEvm at the beginning of the assignment.
	SuppProverSteps []wizard.ProverAction

	// The [wizard.ProverAction] for submodules.
	Pa_accData wizard.ProverAction
	Pa_accInfo wizard.ProverAction
	Pa_keccak  wizard.ProverAction
}

func NewKeccakZkEVM(comp *wizard.CompiledIOP, settings Settings, providersFromEcdsa []generic.GenericByteModule) *KeccakZkEVM {

	shakira, isHashLoManualShf := getShakiraArithmetization(comp)

	res := newKeccakZkEvm(
		comp,
		settings, append(
			providersFromEcdsa,
			shakira,
			getRlpAddArithmetization(comp),
		),
	)

	shakiraProverAction := &ShakiraProverAction{SelectorKeccakResLo: isHashLoManualShf}
	res.SuppProverSteps = append(res.SuppProverSteps, shakiraProverAction)

	return res
}

func newKeccakZkEvm(comp *wizard.CompiledIOP, settings Settings, providers []generic.GenericByteModule) *KeccakZkEVM {

	// create the list of  [generic.GenDataModule] and [generic.GenInfoModule]
	var (
		gdm = make([]generic.GenDataModule, 0, len(providers))
		gim = make([]generic.GenInfoModule, 0, len(providers))
	)

	for i := range providers {
		gdm = append(gdm, providers[i].Data)
		gim = append(gim, providers[i].Info)
	}

	var (
		inpAcc = gen_acc.GenericAccumulatorInputs{
			MaxNumKeccakF: settings.MaxNumKeccakf,
			ProvidersData: gdm,
			ProvidersInfo: gim,
		}

		// unify the data from different providers in a single provider
		accData = gen_acc.NewGenericDataAccumulator(comp, inpAcc)
		// unify the info from different providers in a single provider
		accInfo = gen_acc.NewGenericInfoAccumulator(comp, inpAcc)

		keccakInp = KeccakSingleProviderInput{
			Provider: generic.GenericByteModule{
				Data: accData.Provider,
				Info: accInfo.Provider,
			},
			MaxNumKeccakF: settings.MaxNumKeccakf,
		}
		keccak = NewKeccakSingleProvider(comp, keccakInp)
	)

	res := &KeccakZkEVM{
		Pa_accData: accData,
		Pa_accInfo: accInfo,
		Pa_keccak:  keccak,
		Settings:   &settings,
	}
	return res

}

func (k *KeccakZkEVM) Run(run *wizard.ProverRuntime) {

	for _, action := range k.SuppProverSteps {
		action.Run(run)
	}

	k.Pa_accData.Run(run)
	k.Pa_accInfo.Run(run)
	k.Pa_keccak.Run(run)
}

// getShakiraArithmetization returns a [generic.GenericByteModule] representing
// the data to hash using SHA3 in the SHAKIRA module of the arithmetization. The
// returned module contains an [Info.IsHashLo] that is a column to assign: e.g
// it does not come from the arithmetization directly but is derived from.
func getShakiraArithmetization(comp *wizard.CompiledIOP) (generic.GenericByteModule, *dedicated.ManuallyShifted) {

	res := generic.GenericByteModule{
		Data: generic.GenDataModule{
			HashNum: comp.Columns.GetHandle("shakiradata.ID"),
			Index:   comp.Columns.GetHandle("shakiradata.INDEX"),
			Limb:    comp.Columns.GetHandle("shakiradata.LIMB"),
			NBytes:  comp.Columns.GetHandle("shakiradata.nBYTES"),
			ToHash:  comp.Columns.GetHandle("shakiradata.IS_KECCAK_DATA"),
		},
		Info: generic.GenInfoModule{
			HashNum: comp.Columns.GetHandle("shakiradata.ID"),
			HashLo:  comp.Columns.GetHandle("shakiradata.LIMB"),
			HashHi:  comp.Columns.GetHandle("shakiradata.LIMB"),
			// Before, we usse to pass column.Shift(IsHashHi, -1) but this does
			// not work with the prover distribution as the column is used as
			// a filter for a projection query.
			IsHashHi: comp.Columns.GetHandle("shakiradata.SELECTOR_KECCAK_RES_HI"),
		},
	}

	supp := dedicated.ManuallyShift(comp, res.Info.IsHashHi, -1, "shakiradata.SELECTOR_KECCAK_RES_LO")
	pragmas.MarkLeftPadded(supp.Natural)
	res.Info.IsHashLo = supp.Natural
	return res, supp
}

func getRlpAddArithmetization(comp *wizard.CompiledIOP) generic.GenericByteModule {
	return generic.GenericByteModule{
		Data: generic.GenDataModule{
			HashNum: comp.Columns.GetHandle("rlpaddr.STAMP"),
			Index:   comp.Columns.GetHandle("rlpaddr.INDEX"),
			Limb:    comp.Columns.GetHandle("rlpaddr.LIMB"),
			NBytes:  comp.Columns.GetHandle("rlpaddr.nBYTES"),
			ToHash:  comp.Columns.GetHandle("rlpaddr.LC"),
		},
		Info: generic.GenInfoModule{
			HashNum:  comp.Columns.GetHandle("rlpaddr.STAMP"),
			HashLo:   comp.Columns.GetHandle("rlpaddr.DEP_ADDR_LO"),
			HashHi:   comp.Columns.GetHandle("rlpaddr.RAW_ADDR_HI"),
			IsHashLo: comp.Columns.GetHandle("rlpaddr.SELECTOR_KECCAK_RES"),
			IsHashHi: comp.Columns.GetHandle("rlpaddr.SELECTOR_KECCAK_RES"),
		},
	}
}
