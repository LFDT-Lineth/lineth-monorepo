package zkevm

import (
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/serialization"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/ecarith"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/ecdsa"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/sha2"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/modexp"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager"
)

// ZkEvm defines the wizard responsible for proving execution of the zk
type ZkEvm struct {
	// Arithmetization definition function. Generated during the compilation
	// process.
	arithmetization *arithmetization.Arithmetization
	// Keccak module in use. Generated during the compilation process.
	keccak *keccak.KeccakZkEVM
	// State manager module in use. Generated during the compilation process.
	stateManager *statemanager.StateManager
	// PublicInput gives access to the public inputs of the wizard-IOP and is
	// used to access them to define the outer-circuit.
	PublicInput *publicInput.PublicInput
	// ecdsa is the module responsible for verifying the ecdsa tx signatures and
	// ecrecover
	ecdsa *ecdsa.EcdsaZkEvm

	// modexp is the module responsible for proving the calls to the modexp
	// precompile
	modexp *modexp.Module
	// deactivated pending the resolution of: https://github.com/Consensys/linea-tracer/issues/954
	//
	// ecadd is the module responsible for proving the calls to the ecadd
	// precompile
	// ecadd *ecarith.EcAdd
	// ecmul is the module responsible for proving the calls to the ecmul
	// precompile
	ecmul *ecarith.EcMul
	// ecpair is the module responsible for the proving the calls the ecpairing
	// precompile
	// ecpair *ecpair.ECPair
	// sha2 is the module responsible for doing the computation of the sha2
	// precompile.
	sha2 *sha2.Sha2SingleProvider

	// Contains the actual wizard-IOP compiled object. This object is called to
	// generate the inner-proof.
	WizardIOP *wizard.CompiledIOP
}

// NewZkEVM instantiates a new ZkEvm instance. The function returns a fully
// initialized and compiled zkEVM object tuned with the caller's parameters and
// the input compilation suite.
//
// The function can take a bit of time to complete. It will populate the zkEVM
// struct and needs to be called before running the prover of the inner-proof.
func NewZkEVM(
	settings Settings, // Settings for the zkEVM
) *ZkEvm {

	var (
		res    *ZkEvm
		define = func(b *wizard.Builder) {
			res = newZkEVM(b, &settings)
		}
		wizardIOP = wizard.Compile(define, settings.CompilationSuite...).BootstrapFiatShamir(settings.Metadata, serialization.SerializeCompiledIOP)
	)

	res.WizardIOP = wizardIOP
	return res
}

// Prove assigns and runs the inner-prover of the zkEVM and then, it returns the
// inner-proof
func (z *ZkEvm) ProveInner(input *Witness) wizard.Proof {
	return wizard.Prove(z.WizardIOP, z.prove(input))
}

// Verify verifies the inner-proof of the zkEVM
func (z *ZkEvm) VerifyInner(proof wizard.Proof) error {
	return wizard.Verify(z.WizardIOP, proof)
}

// newZkEVM is the main define function of the zkEVM module. This function is
// unexported and should not be exported. The user should instead use the
// "NewZkEvm" function. This function is meant to be passed as a closure to the
// wizard.Compile function. Thus, this is an internal.
func newZkEVM(b *wizard.Builder, s *Settings) *ZkEvm {

	var (
		comp         = b.CompiledIOP
		arith        = arithmetization.NewArithmetization(b, s.Arithmetization)
		ecdsa        = ecdsa.NewEcdsaZkEvm(comp, &s.Ecdsa)
		stateManager = statemanager.NewStateManagerNoHub(comp, s.Statemanager)
		keccak       = keccak.NewKeccakZkEVM(comp, s.Keccak, ecdsa.GetProviders())
		modexp       = modexp.NewModuleZkEvm(comp, s.Modexp)
		// deactivated pending the resolution of: https://github.com/Consensys/linea-tracer/issues/954
		//
		// ecadd        = ecarith.NewEcAddZkEvm(comp, &s.Ecadd)
		ecmul = ecarith.NewEcMulZkEvm(comp, &s.Ecmul)
		// ecpair      = ecpair.NewECPairZkEvm(comp, &s.Ecpair)
		sha2        = sha2.NewSha2ZkEvm(comp, s.Sha2)
		publicInput = publicInput.NewPublicInputZkEVM(comp, &s.PublicInput, &stateManager.StateSummary)
	)

	return &ZkEvm{
		arithmetization: arith,
		ecdsa:           ecdsa,
		stateManager:    stateManager,
		keccak:          keccak,
		modexp:          modexp,
		// deactivated pending the resolution of: https://github.com/Consensys/linea-tracer/issues/954
		//
		// ecadd:           ecadd,
		ecmul: ecmul,
		// ecpair:      ecpair,
		sha2:        sha2,
		PublicInput: &publicInput,
	}
}

// Returns a prover function for the zkEVM module. The resulting function is
// aimed to be passed to the wizard.Prove function.
func (z *ZkEvm) prove(input *Witness) (prover wizard.ProverStep) {
	return func(run *wizard.ProverRuntime) {

		// Assigns the arithmetization module. From Corset. Must be done first
		// because the following modules use the content of these columns to
		// assign themselves.
		z.arithmetization.Assign(run, input.ExecTracesFPath)

		// Assign the state-manager module
		z.ecdsa.Assign(run, input.TxSignatureGetter, len(input.TxSignatures))
		z.stateManager.Assign(run, input.SMTraces)
		z.keccak.Run(run)
		z.modexp.Assign(run)
		// deactivated pending the resolution of: https://github.com/Consensys/linea-tracer/issues/954
		//
		// z.ecadd.Assign(run)
		z.ecmul.Assign(run)
		// z.ecpair.Assign(run)
		z.sha2.Run(run)
		z.PublicInput.Assign(run, input.L2BridgeAddress)
	}
}

// Limits returns the configuration limits used to instantiate the current
// zk-EVM.
func (z *ZkEvm) Limits() *config.TracesLimits {
	return z.arithmetization.Settings.Limits
}
