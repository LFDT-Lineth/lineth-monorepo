package execution

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/config"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/sirupsen/logrus"

	"github.com/consensys/gnark/std/hash/mimc"
	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
)

// CircuitExecution for the outer-proof
type CircuitExecution struct {
	// The wizard verifier circuit
	WizardVerifier wizard.WizardVerifierCircuit `gnark:",secret"`
	// The functional public inputs are the "actual" statement made by the
	// circuit. They are not part of the public input of the circuit for
	// a number of reasons involving efficiency and simplicity in the aggregation
	// process. What is the public input is their hash.
	FuncInputs FunctionalPublicInputSnark `gnark:",secret"`
	// The public input of the proof
	PublicInput frontend.Variable `gnark:",public"`
}

// Allocates the outer-proof circuit
func Allocate(zkevm *zkevm.ZkEvm) CircuitExecution {
	wverifier, err := wizard.AllocateWizardCircuit(zkevm.WizardIOP)
	if err != nil {
		panic(err)
	}
	return CircuitExecution{
		WizardVerifier: *wverifier,
		FuncInputs: FunctionalPublicInputSnark{
			FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
				L2MessageHashes: L2MessageHashes{
					Values: make([][32]frontend.Variable, zkevm.Limits().BlockL2L1Logs),
					Length: nil,
				},
			},
		},
	}
}

// assign the wizard proof to the outer circuit
func assign(
	limits *config.TracesLimits,
	comp *wizard.CompiledIOP,
	proof wizard.Proof,
	funcInputs public_input.Execution,
) CircuitExecution {

	var (
		wizardVerifier = wizard.GetWizardVerifierCircuitAssignment(comp, proof)
		res            = CircuitExecution{
			WizardVerifier: *wizardVerifier,
			FuncInputs: FunctionalPublicInputSnark{
				FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
					L2MessageHashes: L2MessageHashes{Values: make([][32]frontend.Variable, limits.BlockL2L1Logs)}, // TODO use a maximum from config
				},
			},
			PublicInput: new(big.Int).SetBytes(funcInputs.Sum(nil)),
		}
	)

	res.FuncInputs.Assign(&funcInputs)

	return res
}

// Define of the wizard circuit
func (c *CircuitExecution) Define(api frontend.API) error {
	c.WizardVerifier.Verify(api)
	checkPublicInputs(
		api,
		&c.WizardVerifier,
		c.FuncInputs,
	)

	// Add missing public input check
	mimcHasher, _ := mimc.NewMiMC(api)
	api.AssertIsEqual(c.PublicInput, c.FuncInputs.Sum(api, &mimcHasher))
	return nil
}

func MakeProof(
	limits *config.TracesLimits,
	setup circuits.Setup,
	comp *wizard.CompiledIOP,
	wproof wizard.Proof,
	funcInputs public_input.Execution,
) string {

	assignment := assign(limits, comp, wproof, funcInputs)

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
