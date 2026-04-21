package circuits

import (
	"bytes"
	"fmt"

	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/sirupsen/logrus"

	plonk_bn254 "github.com/consensys/gnark/backend/plonk/bn254"
)

// Generates a PlonkProof and sanity-checks it against the verifying key. Can
// take a list of options which can of either backend.ProverOption of backend.
// VerifierOption.
func ProveCheck(setup *Setup, assignment frontend.Circuit, opts ...any) (plonk.Proof, error) {

	proverOpts := []backend.ProverOption{}
	verifierOpts := []backend.VerifierOption{}
	solverOpts := []solver.Option{}

	// @alex: we cannot incrementally pass the solver options to the prover
	// options (they are overridden at every call). That's why we need to collect
	// them first before passing them to the prover options.

	for _, opt := range opts {
		switch o := opt.(type) {
		case solver.Option:
			solverOpts = append(solverOpts, o)
		case backend.ProverOption:
			proverOpts = append(proverOpts, o)
		case backend.VerifierOption:
			verifierOpts = append(verifierOpts, o)
		default:
			return nil, fmt.Errorf("unknown option type to prove-check: %++v", o)
		}
	}

	proverOpts = append(proverOpts, backend.WithSolverOptions(solverOpts...))

	logrus.Infof("Creating the witness")
	witness, err := frontend.NewWitness(assignment, setup.Circuit.Field())
	if err != nil {
		return nil, fmt.Errorf("while generating the gnark witness: %w", err)
	}

	logrus.Infof("Generating the proof")
	var proof plonk.Proof

	proof, err = plonk.Prove(setup.Circuit, setup.ProvingKey, witness, proverOpts...)
	if err != nil {
		// The error returned by the Plonk prover is usually not helpful at
		// all. So, in order to get more details, we run the "test" Solver.
		logrus.Errorf("plonk.Prove returned an error, using the test.IsSolved to get more details: %s", err.Error())
		errDetail := test.IsSolved(
			assignment,
			assignment,
			setup.Circuit.Field(),
			// this test engine prover option was no-op before and it was removed
			// test.WithBackendProverOptions(proverOpts...),
		)
		return nil, fmt.Errorf("while running the plonk prover: %w", errDetail)
	}

	logrus.Infof("Sanity-checking the proof")
	// Sanity-check : the proof must pass
	{
		pubwitness, err := witness.Public()
		if err != nil {
			panic(err)
		}

		err = plonk.Verify(proof, setup.VerifyingKey, pubwitness, verifierOpts...)
		if err != nil {
			panic(err)
		}
		// logrus.Infof("the proof passed with\nproof=%++v\nwit=%++v\nvkey=%++v\n", proof, pubwitness, pp.VK)
	}

	return proof, nil
}

// Serializes the proof in an 0x prefixed hexstring
func SerializeProofRaw(proof plonk.Proof) string {
	var buf bytes.Buffer
	proof.WriteRawTo(&buf)
	return hexutil.Encode(buf.Bytes())
}

// Serializes the proof in an 0x prefixed hexstring
func SerializeProofSolidityBn254(proof plonk.Proof) string {
	buf := proof.(*plonk_bn254.Proof).MarshalSolidity()
	return hexutil.Encode(buf)
}
