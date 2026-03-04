package aggregation_test

import (
	"context"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	frBls "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	frBw6 "github.com/consensys/gnark-crypto/ecc/bw6-761/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/aggregation"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	snarkTestUtils "github.com/consensys/linea-monorepo/prover/circuits/internal/test_utils"
	"github.com/consensys/linea-monorepo/prover/utils/test_utils"

	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak"
	"github.com/consensys/linea-monorepo/prover/config"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicInput(t *testing.T) {

	// test case taken from backend/aggregation
	testCases := []public_input.Aggregation{
		{
			FinalShnarf:                             "0x3f01b1a726e6317eb05d8fe8b370b1712dc16a7fde51dd38420d9a474401291c",
			ParentAggregationFinalShnarf:            "0x0f20c85d35a21767e81d5d2396169137a3ef03f58391767a17c7016cc82edf2e",
			ParentAggregationLastBlockTimestamp:     1711742796,
			FinalTimestamp:                          1711745271,
			LastFinalizedBlockNumber:                3237969,
			FinalBlockNumber:                        3238794,
			LastFinalizedL1RollingHash:              "0xe578e270cc6ee7164d4348ac7ca9a7cfc0c8c19b94954fc85669e75c1db46178",
			L1RollingHash:                           "0x0578f8009189d67ce0378619313b946f096ca20dde9cad0af12a245500054908",
			LastFinalizedL1RollingHashMessageNumber: 549238,
			L1RollingHashMessageNumber:              549263,
			L2MsgRootHashes:                         []string{"0xfb7ce9c89be905d39bfa2f6ecdf312f127f8984cf313cbea91bca882fca340cd"},
			L2MsgMerkleTreeDepth:                    5,
		},
	}

	for i := range testCases {

		fpi, err := public_input.NewAggregationFPI(&testCases[i])
		assert.NoError(t, err)

		sfpi := fpi.ToSnarkType()
		// TODO incorporate into public input hash or decide not to
		sfpi.NbDecompression = -1
		sfpi.InitialStateRootHash = -2
		sfpi.ChainID = -3
		sfpi.L2MessageServiceAddr = -4
		sfpi.NbL2Messages = -5

		var res [32]frontend.Variable
		assert.NoError(t, internal.CopyHexEncodedBytes(res[:], testCases[i].GetPublicInputHex()))

		snarkTestUtils.SnarkFunctionTest(func(api frontend.API) []frontend.Variable {
			sum := sfpi.Sum(api, keccak.NewHasher(api, 500))
			return sum[:]
		}, res[:]...)(t)
	}
}

func TestAggregationOneInner(t *testing.T) {
	t.Skipf("Skipped. DEBT: See TestFewDifferentOnes.")
	testAggregation(t, 2, 1)
}

func TestAggregationFewDifferentInners(t *testing.T) {
	t.Skipf("Skipped. CRITICAL TODO: this test is failing due to non-matching SRS for circuits of different sizes (most notably the PI circuit). Must come up with a solution: probably using circuits.NewSRSStore instead of circuits.NewUnsafeSRSProvider, but doing that correctly also requires a dummy public input circuit.")
	testAggregation(t, 1, 5)
	testAggregation(t, 2, 5)
	testAggregation(t, 3, 2, 6, 10)
}

func testAggregation(t *testing.T, nCircuits int, ncs ...int) {

	// Mock circuits to aggregate
	var innerSetups []circuits.Setup
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

	aggregationPIBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	var aggregationPI frBw6.Element
	aggregationPI.SetBytes(aggregationPIBytes)

	logrus.Infof("Compiling interconnection circuit")
	maxNC := utils.Max(ncs...)

	piConfig := config.PublicInput{
		MaxNbDecompression: maxNC,
		MaxNbExecution:     maxNC,
	}

	piCircuit := pi_interconnection.DummyCircuit{
		ExecutionPublicInput:     make([]frontend.Variable, piConfig.MaxNbExecution),
		ExecutionFPI:             make([]frontend.Variable, piConfig.MaxNbExecution),
		DecompressionPublicInput: make([]frontend.Variable, piConfig.MaxNbDecompression),
		DecompressionFPI:         make([]frontend.Variable, piConfig.MaxNbDecompression),
	}

	piCs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &piCircuit)
	assert.NoError(t, err)

	piSetup, err := circuits.MakeSetup(context.TODO(), circuits.PublicInputInterconnectionCircuitID, piCs, srsProvider, nil)
	assert.NoError(t, err)

	for _, nc := range ncs {

		// Building aggregation circuit for max `nc` proofs
		logrus.Infof("Building aggregation circuit for size of %v\n", nc)
		aggrCircuit, err := aggregation.AllocateCircuit(nc, piSetup, vkeys)
		require.NoError(t, err)

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

		aggrAssignment, err := aggregation.AssignAggregationCircuit(nc, innerProofClaims, piInfo, aggregationPI)
		assert.NoError(t, err)

		assert.NoError(t, test.IsSolved(aggrCircuit, aggrAssignment, ecc.BW6_761.ScalarField()))
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
