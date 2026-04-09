package execution

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/sirupsen/logrus"

	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
)

// CircuitExecution for the outer-proof
type CircuitExecution struct {
	// LimitlessMode is set to true if the outer proof is generated for the
	// limitless prover mode.
	LimitlessMode bool `gnark:"-"`
	// CongloVK is used when the [LimitlessMode] is on and is helps checking
	// the validity of the inner-proofs verification-key public input.
	// Each VK is an octuplet (8 koalabear field elements).
	CongloVK [2]field.Octuplet
	// VKMerkleRoot is used when the [LimitlessMode] is on and is helps checking
	// the validity of the inner-proofs verification-key merkle root public
	// input. It is an octuplet (8 koalabear field elements).
	VKMerkleRoot field.Octuplet
	// The wizard verifier circuit
	WizardVerifier wizard.VerifierCircuit `gnark:",secret"`
	// The functional public inputs are the "actual" statement made by the
	// circuit. They are not part of the public input of the circuit for
	// a number of reasons involving efficiency and simplicity in the aggregation
	// process. What is the public input is their hash.
	FuncInputs FunctionalPublicInputSnark `gnark:",secret"`
	// The public input of the proof
	PublicInput frontend.Variable `gnark:",public"`
	// ExecDataPublicInputBytes is the execution data in byte form
	ExecDataBytes [1 << 17]frontend.Variable `gnark:",secret"`
}

// Allocates the outer-proof circuit
func Allocate(zkevm *zkevm.ZkEvm) CircuitExecution {
	wverifier := wizard.AllocateWizardCircuit(
		zkevm.RecursionCompiledIOP,
		zkevm.RecursionCompiledIOP.NumRounds(),
		true,
	)

	return CircuitExecution{
		WizardVerifier: *wverifier,
		FuncInputs: FunctionalPublicInputSnark{
			FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
				L2MessageHashes: L2MessageHashes{
					Values: make([][32]frontend.Variable, zkevm.Limits().BlockL2L1Logs()),
					Length: nil,
				},
			},
		},
	}
}

// AllocateLimitless allocates the outer-proof circuit in the context of a
// limitless execution. It works as [Allocate] but takes the conglomeration
// wizard as input and uses it to allocate the outer circuit. The trace-limits
// file is used to derive the maximal number of L2L1 logs.
//
// The proof generation can be done using the [MakeProof] function as we would
// do for the non-limitless execution proof.
func AllocateLimitless(congWiop *wizard.CompiledIOP, limits *config.TracesLimits, vkMerkleRoot field.Octuplet) CircuitExecution {
	logrus.Infof("Allocating the outer circuit with params: no_of_cong_wiop_rounds=%d "+
		"limits_block_l2l1_logs=%d", congWiop.NumRounds(), limits.BlockL2L1Logs())

	vk0 := congWiop.ExtraData[distributed.VerifyingKeyPublicInput].(field.Octuplet)
	vk1 := congWiop.ExtraData[distributed.VerifyingKey2PublicInput].(field.Octuplet)

	wverifier := wizard.AllocateWizardCircuit(congWiop, congWiop.NumRounds(), true)
	return CircuitExecution{
		LimitlessMode:  true,
		CongloVK:       [2]field.Octuplet{vk0, vk1},
		VKMerkleRoot:   vkMerkleRoot,
		WizardVerifier: *wverifier,
		FuncInputs: FunctionalPublicInputSnark{
			FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
				L2MessageHashes: L2MessageHashes{
					Values: make([][32]frontend.Variable, limits.BlockL2L1Logs()),
					Length: nil,
				},
			},
		},
	}
}

// Define of the wizard circuit
func (c *CircuitExecution) Define(api frontend.API) error {

	c.WizardVerifier.BLSFS = fiatshamir.NewGnarkFSBLS12377(api)

	c.WizardVerifier.Verify(api)

	extr := extractPIsFromWVC(api, &c.WizardVerifier)
	checkPublicInputs(
		api,
		&extr,
		c.FuncInputs,
		c.ExecDataBytes,
	)

	if c.LimitlessMode {
		c.checkLimitlessConglomerationCompletion(api)
	}

	api.AssertIsEqual(c.PublicInput, c.FuncInputs.Sum(api))

	c.FuncInputs.RangeCheck(api)

	return nil
}

func MakeProof(
	limits *config.TracesLimits,
	setup circuits.Setup,
	comp *wizard.CompiledIOP,
	wproof wizard.Proof,
	funcInputs public_input.Execution,
	execData []byte,
) string {

	assignment := assign(limits, comp, wproof, funcInputs, execData)

	proof, err := circuits.ProveCheck(
		&setup,
		&assignment,
		emPlonk.GetNativeProverOptions(ecc.BW6_761.ScalarField(), setup.Circuit.Field()),
		emPlonk.GetNativeVerifierOptions(ecc.BW6_761.ScalarField(), setup.Circuit.Field()),
	)

	if err != nil {
		panic(err)
	}

	logrus.Infof("generated outer-circuit proof `%++v` for input `%v`", proof, assignment.PublicInput.(*big.Int).String())

	// Write the serialized proof
	return circuits.SerializeProofRaw(proof)
}

// assign the wizard proof to the outer circuit
func assign(
	limits *config.TracesLimits,
	comp *wizard.CompiledIOP,
	proof wizard.Proof,
	funcInputs public_input.Execution,
	execData []byte,
) CircuitExecution {

	var (
		wizardVerifier = wizard.AssignVerifierCircuit(comp, proof, comp.NumRounds(), true)
		res            = CircuitExecution{
			WizardVerifier: *wizardVerifier,
			PublicInput:    new(big.Int).SetBytes(funcInputs.Sum()),
			FuncInputs: FunctionalPublicInputSnark{
				FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
					L2MessageHashes: L2MessageHashes{
						Values: make([][32]frontend.Variable, limits.BlockL2L1Logs()),
					},
				},
			},
		}
	)

	if len(execData) > len(res.ExecDataBytes) {
		utils.Panic("execData is too long: conflation contains too much data: %v > %v", len(execData), len(res.ExecDataBytes))
	}

	for i, b := range execData {
		res.ExecDataBytes[i] = b
	}

	for i := len(execData); i < len(res.ExecDataBytes); i++ {
		res.ExecDataBytes[i] = 0
	}

	if err := res.FuncInputs.Assign(&funcInputs); err != nil {
		panic(err)
	}
	return res
}
