package main

import (
	"context"
	"fmt"

	"github.com/consensys/gnark-crypto/ecc"
	frBls "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	frBn254 "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	frBw6 "github.com/consensys/gnark-crypto/ecc/bw6-761/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/aggregation"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/emulation"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/test_utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func main() {

	var t test_utils.FakeTestingT

	// Mock circuits to aggregate
	var innerSetups []circuits.Setup
	const nCircuits = 10
	logrus.Infof("Initializing many inner-circuits of %v\n", nCircuits)
	srsProvider := circuits.NewUnsafeSRSProvider() // This is a dummy SRS provider, not to use in prod.
	for i := 0; i < nCircuits; i++ {
		logrus.Infof("\t%d/%d\n", i+1, nCircuits)
		pp, _ := dummy.MakeUnsafeSetup(srsProvider, circuits.MockCircuitID(i), ecc.BLS12_377.ScalarField())
		innerSetups = append(innerSetups, pp)
	}

	// This collects the verifying keys from the public parameters
	vkeys := make([]plonk.VerifyingKey, 0, len(innerSetups))
	for _, setup := range innerSetups {
		vkeys = append(vkeys, setup.VerifyingKey)
	}

	ncs := []int{1, 5, 10, 20}

	// At this step, we will collect several proofs for the BW6 circuit and
	// several verification keys.
	var (
		bw6Vkeys  = make([]plonk.VerifyingKey, 0, len(ncs))
		bw6Proofs = make([]plonk.Proof, 0, len(ncs))
	)

	aggregationPIBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	var aggregationPI frBw6.Element
	aggregationPI.SetBytes(aggregationPIBytes)

	logrus.Infof("Compiling interconnection circuit")
	maxNC := utils.Max(ncs...)

	piConfig := config.PublicInput{
		MaxNbDataAvailability: maxNC,
		MaxNbExecution:        maxNC,
	}

	piCircuit := pi_interconnection.DummyCircuit{
		ExecutionPublicInput:     make([]frontend.Variable, piConfig.MaxNbExecution),
		ExecutionFPI:             make([]frontend.Variable, piConfig.MaxNbExecution),
		DecompressionPublicInput: make([]frontend.Variable, piConfig.MaxNbDataAvailability),
		DecompressionFPI:         make([]frontend.Variable, piConfig.MaxNbDataAvailability),
	}

	piCs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &piCircuit)
	assert.NoError(t, err)

	piSetup, err := circuits.MakeSetup(context.TODO(), circuits.PublicInputInterconnectionCircuitID, piCs, srsProvider, nil)
	assert.NoError(t, err)

	for _, nc := range ncs {
		// Compile PI Interconnection sub-circuit

		// Building aggregation circuit for max `nc` proofs
		logrus.Infof("Building aggregation circuit for size of %v\n", nc)
		ccs, err := aggregation.MakeCS(
			nc,
			piSetup,
			vkeys)
		assert.NoError(t, err)

		// Generate the setup
		logrus.Infof("Generating setup, will take a while to complete..")
		ppBw6, err := circuits.MakeSetup(context.Background(), circuits.CircuitID(fmt.Sprintf("aggregation-%d", nc)), ccs, circuits.NewUnsafeSRSProvider(), nil)
		assert.NoError(t, err)

		// Generate proofs claims to aggregate
		nProofs := utils.Ite(nc == 0, 0, utils.Max(nc-3, 1))
		logrus.Infof("Generating a witness, %v dummy-proofs to aggregates", nProofs)
		innerProofClaims := make([]aggregation.ProofClaimAssignment, nProofs)
		innerPI := make([]frBls.Element, nc)
		for i := range innerProofClaims {

			// Assign the dummy circuit for a random value
			circID := i % nCircuits
			_, err = innerPI[i].SetRandom()
			assert.NoError(t, err)
			a := dummy.Assign(circuits.MockCircuitID(circID), innerPI[i])

			// Stores the inner-proofs for later
			proof, err := circuits.ProveCheck(
				&innerSetups[circID], a,
				emPlonk.GetNativeProverOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
				emPlonk.GetNativeVerifierOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
			)
			assert.NoError(t, err)

			innerProofClaims[i] = aggregation.ProofClaimAssignment{
				CircuitID:   circID,
				Proof:       proof,
				PublicInput: innerPI[i],
			}
		}

		logrus.Info("generating witness for the interconnection circuit")
		// assign public input circuit
		circuitTypes := make([]pi_interconnection.InnerCircuitType, nc)
		for i := range circuitTypes {
			circuitTypes[i] = pi_interconnection.InnerCircuitType(test_utils.RandIntN(2)) // #nosec G115 -- value already constrained
		}

		innerPiPartition := utils.RightPad(utils.Partition(innerPI, circuitTypes), 2)
		execPI := utils.RightPad(innerPiPartition[typeExec], len(piCircuit.ExecutionPublicInput))
		decompPI := utils.RightPad(innerPiPartition[typeDecomp], len(piCircuit.DecompressionPublicInput))

		piAssignment := pi_interconnection.DummyCircuit{
			AggregationPublicInput:   [2]frontend.Variable{aggregationPIBytes[:16], aggregationPIBytes[16:]},
			ExecutionPublicInput:     utils.ToVariableSlice(execPI),
			DecompressionPublicInput: utils.ToVariableSlice(decompPI),
			DecompressionFPI:         utils.ToVariableSlice(pow5(decompPI)),
			ExecutionFPI:             utils.ToVariableSlice(pow5(execPI)),
			NbExecution:              len(innerPiPartition[typeExec]),
			NbDecompression:          len(innerPiPartition[typeDecomp]),
		}

		logrus.Infof("Generating PI proof")
		piW, err := frontend.NewWitness(&piAssignment, ecc.BLS12_377.ScalarField())
		assert.NoError(t, err)
		piProof, err := circuits.ProveCheck(
			&piSetup, &piAssignment,
			emPlonk.GetNativeProverOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
			emPlonk.GetNativeVerifierOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
		)
		assert.NoError(t, err)
		piPubW, err := piW.Public()
		assert.NoError(t, err)
		piInfo := aggregation.PiInfo{
			Proof:         piProof,
			PublicWitness: piPubW,
			ActualIndexes: pi_interconnection.InnerCircuitTypesToIndexes(&piConfig, circuitTypes),
		}

		// Assigning the BW6 circuit
		logrus.Infof("Generating the aggregation proof for arity %v", nc)

		bw6Proof, err := aggregation.MakeProof(&ppBw6, nc, innerProofClaims, piInfo, aggregationPI)
		assert.NoError(t, err)

		bw6Proofs = append(bw6Proofs, bw6Proof)
		bw6Vkeys = append(bw6Vkeys, ppBw6.VerifyingKey)
	}

	// Then create the BN254 circuit
	logrus.Infof("Generating the emulation circuit")
	ccsEmulation, err := emulation.MakeCS(bw6Vkeys)
	assert.NoError(t, err)

	logrus.Infof("Generating the setup for the emulation circuit")
	setupEmulation, err := circuits.MakeSetup(context.Background(), circuits.EmulationCircuitID, ccsEmulation, circuits.NewUnsafeSRSProvider(), nil)
	assert.NoError(t, err)

	var aggregationPiBn254 frBn254.Element
	aggregationPiBn254.SetBytes(aggregationPIBytes)
	for k := range bw6Proofs {
		logrus.Infof("Generating the proof for the emulation circuit (BW6 Proof #%v)", k)
		_, err = emulation.MakeProof(&setupEmulation, k, bw6Proofs[k], aggregationPiBn254)
		assert.NoError(t, err)
	}

}

const (
	typeExec   = pi_interconnection.Execution
	typeDecomp = pi_interconnection.Decompression
)

func pow5(s []frBls.Element) []frBls.Element {
	res := make([]frBls.Element, len(s))
	for i := range s {
		res[i].
			Mul(&s[i], &s[i]).
			Mul(&res[i], &res[i]).
			Mul(&res[i], &s[i])

	}
	return res
}
