package zkevm

import (
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/serialization"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/bls"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/ecarith"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/ecdsa"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/ecpair"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/sha2"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/modexp"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/p256verify"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager"
)

// ZkEvm defines the wizard responsible for proving execution of the zk
type ZkEvm struct {
	// Arithmetization definition function. Generated during the compilation
	// process.
	Arithmetization *arithmetization.Arithmetization `json:"arithmetization"`
	// Keccak module in use. Generated during the compilation process.
	Keccak *keccak.KeccakZkEVM `json:"keccak"`
	// State manager module in use. Generated during the compilation process.
	StateManager *statemanager.StateManager `json:"stateManager"`
	// PublicInput gives access to the public inputs of the wizard-IOP and is
	// used to access them to define the outer-circuit.
	PublicInput *publicInput.PublicInput `json:"publicInput"`
	// Ecdsa is the module responsible for verifying the Ecdsa tx signatures and
	// ecrecover
	Ecdsa *ecdsa.EcdsaZkEvm `json:"ecdsa"`
	// Modexp is the module responsible for proving the calls to the Modexp
	// precompile
	Modexp *modexp.Module `json:"modexp"`
	// Ecadd is the module responsible for proving the calls to the Ecadd
	// precompile
	Ecadd *ecarith.EcAdd `json:"ecadd"`
	// Ecmul is the module responsible for proving the calls to the Ecmul
	// precompile
	Ecmul *ecarith.EcMul `json:"ecmul"`
	// Ecpair is the module responsible for the proving the calls the ecpairing
	// precompile
	Ecpair *ecpair.ECPair `json:"ecpair"`
	// Sha2 is the module responsible for doing the computation of the Sha2
	// precompile.
	Sha2 *sha2.Sha2SingleProvider `json:"sha2"`
	// BlsG1Add is responsible for BLS G1 addition precompile.
	BlsG1Add *bls.BlsAdd `json:"blsG1Add"`
	// BlsG2Add is responsible for BLS G2 addition precompile.
	BlsG2Add *bls.BlsAdd `json:"blsG2Add"`
	// BlsG1Msm is responsible for BLS G1 multi-scalar multiplicaton precompile.
	BlsG1Msm *bls.BlsMsm `json:"blsG1Msm"`
	// BlsG2Msm is responsible for BLS G2 multi-scalar multiplication precompile.
	BlsG2Msm *bls.BlsMsm `json:"blsG2Msm"`
	// BlsG1Map is responsible for BLS Fp map to G1 precompile.
	BlsG1Map *bls.BlsMap `json:"blsG1Map"`
	// BlsG2Map is responsible for BLS Fp2 map to G2 precompile.
	BlsG2Map *bls.BlsMap `json:"blsG2Map"`
	// BlsPairingCheck is responsible for BLS pairing check precompile.
	BlsPairingCheck *bls.BlsPair `json:"blsPairingCheck"`
	// PointEval is responsible for EIP-4844 point evaluation precompile.
	PointEval *bls.BlsPointEval `json:"pointEval"`
	// P256Verify is responsible for P256 signature verification precompile.
	P256Verify *p256verify.P256Verify `json:"p256Verify"`
	// Contains the actual wizard-IOP compiled object. This object is called to
	// generate the inner-proof.
	WizardIOP *wizard.CompiledIOP `json:"wizardIOP"`
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
		ser = func(wizardIOP *wizard.CompiledIOP) ([]byte, error) {
			return serialization.Serialize(wizardIOP)
		}
		wizardIOP = wizard.Compile(define, settings.CompilationSuite...).BootstrapFiatShamir(settings.Metadata, ser)
	)

	res.WizardIOP = wizardIOP
	return res
}

// Prove assigns and runs the inner-prover of the zkEVM and then, it returns the
// inner-proof
func (z *ZkEvm) ProveInner(input *Witness) wizard.Proof {
	return wizard.Prove(z.WizardIOP, z.GetMainProverStep(input))
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
		comp            = b.CompiledIOP
		arith           = arithmetization.NewArithmetization(b, s.Arithmetization)
		ecdsa           = ecdsa.NewEcdsaZkEvm(comp, &s.Ecdsa)
		stateManager    = statemanager.NewStateManager(comp, s.Statemanager)
		keccak          = keccak.NewKeccakZkEVM(comp, s.Keccak, ecdsa.GetProviders())
		modexp          = modexp.NewModuleZkEvm(comp, s.Modexp)
		ecadd           = ecarith.NewEcAddZkEvm(comp, &s.Ecadd)
		ecmul           = ecarith.NewEcMulZkEvm(comp, &s.Ecmul)
		ecpair          = ecpair.NewECPairZkEvm(comp, &s.Ecpair)
		sha2            = sha2.NewSha2ZkEvm(comp, s.Sha2)
		blsG1Add        = bls.NewG1AddZkEvm(comp, &s.Bls)
		blsG1Msm        = bls.NewG1MsmZkEvm(comp, &s.Bls)
		blsG1Map        = bls.NewG1MapZkEvm(comp, &s.Bls)
		blsG2Add        = bls.NewG2AddZkEvm(comp, &s.Bls)
		blsG2Msm        = bls.NewG2MsmZkEvm(comp, &s.Bls)
		blsG2Map        = bls.NewG2MapZkEvm(comp, &s.Bls)
		blsPairingCheck = bls.NewPairingZkEvm(comp, &s.Bls)
		pointEval       = bls.NewPointEvalZkEvm(comp, &s.Bls)
		p256verify      = p256verify.NewP256VerifyZkEvm(comp, &s.P256Verify)
		publicInput     = publicInput.NewPublicInputZkEVM(comp, &s.PublicInput, &stateManager.StateSummary)
	)

	return &ZkEvm{
		Arithmetization: arith,
		Ecdsa:           ecdsa,
		StateManager:    stateManager,
		Keccak:          keccak,
		Modexp:          modexp,
		Ecadd:           ecadd,
		Ecmul:           ecmul,
		Ecpair:          ecpair,
		Sha2:            sha2,
		BlsG1Add:        blsG1Add,
		BlsG2Add:        blsG2Add,
		BlsG1Msm:        blsG1Msm,
		BlsG2Msm:        blsG2Msm,
		BlsG1Map:        blsG1Map,
		BlsG2Map:        blsG2Map,
		BlsPairingCheck: blsPairingCheck,
		PointEval:       pointEval,
		P256Verify:      p256verify,
		PublicInput:     &publicInput,
	}
}

// Returns a prover function for the zkEVM module. The resulting function is
// aimed to be passed to the wizard.Prove function.
func (z *ZkEvm) GetMainProverStep(input *Witness) (prover wizard.MainProverStep) {
	return func(run *wizard.ProverRuntime) {

		// Assigns the arithmetization module. From Corset. Must be done first
		// because the following modules use the content of these columns to
		// assign themselves.
		z.Arithmetization.Assign(run, input.ExecTracesFPath)

		// Assign the state-manager module
		z.Ecdsa.Assign(run, input.TxSignatureGetter, len(input.TxSignatures))
		z.StateManager.Assign(run, input.SMTraces)
		z.Keccak.Run(run)
		z.Modexp.Assign(run)
		z.Ecadd.Assign(run)
		z.Ecmul.Assign(run)
		z.Ecpair.Assign(run)
		z.Sha2.Run(run)
		z.BlsG1Add.Assign(run)
		z.BlsG2Add.Assign(run)
		z.BlsG1Msm.Assign(run)
		z.BlsG2Msm.Assign(run)
		z.BlsG1Map.Assign(run)
		z.BlsG2Map.Assign(run)
		z.BlsPairingCheck.Assign(run)
		z.PointEval.Assign(run)
		z.P256Verify.Assign(run)
		z.PublicInput.Assign(run, input.L2BridgeAddress, input.BlockHashList)
	}
}

// Limits returns the configuration limits used to instantiate the current
// zk-EVM.
func (z *ZkEvm) Limits() *config.TracesLimits {
	return z.Arithmetization.Settings.Limits
}
