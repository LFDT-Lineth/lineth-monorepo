package pi_interconnection

import (
	"errors"
	"math/big"
	"slices"

	gkrposeidon2compressor "github.com/consensys/gnark/std/permutation/poseidon2/gkr-poseidon2"
	"github.com/sirupsen/logrus"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/config"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/std/math/cmp"
	blobdecompression "github.com/consensys/linea-monorepo/prover/circuits/dataavailability/v2"
	"github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak"
	"github.com/consensys/linea-monorepo/prover/utils"
)

type Circuit struct {
	AggregationPublicInput      [2]frontend.Variable `gnark:",public"` // the public input of the aggregation circuit; divided big-endian into two 16-byte chunks
	ExecutionPublicInput        []frontend.Variable  `gnark:",public"`
	DataAvailabilityPublicInput []frontend.Variable  `gnark:",public"`

	DataAvailabilityFPIQ []blobdecompression.FunctionalPublicInputQSnark
	ExecutionFPIQ        []execution.FunctionalPublicInputQSnark

	public_input.AggregationFPIQSnark

	Keccak keccak.StrictHasherCircuit

	// config
	L2MessageMerkleDepth int
	L2MessageMaxNbMerkle int

	// IsAllowedCircuitID is a public input parroting up the value of
	// [AggregationFPIQSnark.IsAllowedCircuitID]. It is needed so that the
	// aggregation can "see" this value while it cannot access directly the
	// content of the dynamic chain configuration.
	//
	// Its bits encodes which circuit is being allowed in the dynamic chain
	// configuration. For instance, the bits of weight "3" indicates whether the
	// circuit ID "3" is allowed and so on.  The packing order of the bits is
	// LSb to MSb. For instance if
	//
	// Circuit ID 0 -> Disallowed
	// Circuit ID 1 -> Allowed
	// Circuit ID 2 -> Allowed
	// Circuit ID 3 -> Disallowed
	// Circuit ID 4 -> Allowed
	//
	// Then the IsAllowedCircuitID public input must be encoded as 0b10110
	IsAllowedCircuitID frontend.Variable `gnark:",public"`

	// FirstExecutionInitialStateRootHash is the executionPublicInput of the
	// first execution circuit. This value is useful because during the
	// koalabear migration, on the first block. The aggregation.InitialStateRootHash
	// will be set to a different value. Otherwise, the value won't be used
	// in the circuit.
	FirstExecutionInitialStateRootHash [2]frontend.Variable

	MaxNbCircuits int // possibly useless TODO consider removing
}

func (c *Circuit) Define(api frontend.API) error {

	maxNbDA, maxNbExecution := len(c.DataAvailabilityPublicInput), len(c.ExecutionPublicInput)
	if len(c.DataAvailabilityFPIQ) != maxNbDA || len(c.ExecutionFPIQ) != maxNbExecution {
		return errors.New("public / functional public input length mismatch")
	}

	c.AggregationFPIQSnark.RangeCheck(api)

	// implicit: CHECK_DECOMP_LIMIT
	rDA := internal.NewRange(api, c.NbDataAvailability, len(c.DataAvailabilityPublicInput))
	hshK := c.Keccak.NewHasher(api)

	// nbBatchesSums[i] is the index of the first execution circuit associated with the i+1-st data availability circuit.
	// Past the last DA circuit, this value remains constant.
	nbBatchesSums := rDA.PartialSumsF(func(i int) frontend.Variable { return api.Mul(rDA.InRange[i], c.DataAvailabilityFPIQ[i].NbBatches) })
	nbExecution := nbBatchesSums[len(nbBatchesSums)-1] // implicit: CHECK_NB_EXEC

	// These two checks prevents constructing a proof where no execution or no
	// compression proofs are provided. This is to prevent corner cases from
	// arising.
	api.AssertIsDifferent(c.NbDataAvailability, 0)
	api.AssertIsDifferent(nbExecution, 0)

	if c.MaxNbCircuits > 0 { // CHECK_CIRCUIT_LIMIT
		api.AssertIsLessOrEqual(api.Add(nbExecution, c.NbDataAvailability), c.MaxNbCircuits)
	}

	batchHashes := make([]frontend.Variable, len(c.ExecutionPublicInput))
	for i, pi := range c.ExecutionFPIQ {
		batchHashes[i] = pi.DataChecksum.Hash
	}

	finalStateRootHashesLo := logderivlookup.New(api)
	finalStateRootHashesHi := logderivlookup.New(api)
	finalStateRootHashesLo.Insert(c.InitialStateRootHash[1])
	finalStateRootHashesHi.Insert(c.InitialStateRootHash[0])

	for _, pi := range c.ExecutionFPIQ {
		finalStateRootHashesLo.Insert(pi.FinalStateRootHash[1])
		finalStateRootHashesHi.Insert(pi.FinalStateRootHash[0])
	}

	compressor, err := gkrposeidon2compressor.NewCompressor(api)
	if err != nil {
		return err
	}

	blobBatchHashes := make([]frontend.Variable, maxNbDA)
	for i := range blobBatchHashes {
		blobBatchHashes[i] = c.DataAvailabilityFPIQ[i].AllBatchesSum
	}
	if err = internal.MerkleDamgardChecksumSubSlices(api, compressor, 0, batchHashes, internal.VarSlice{Values: nbBatchesSums, Length: c.NbDataAvailability}, blobBatchHashes); err != nil {
		return err
	}

	shnarfParams := make([]ShnarfIteration, len(c.DataAvailabilityPublicInput))
	for i, piq := range c.DataAvailabilityFPIQ {
		piq.RangeCheck(api)

		var (
			newStateRootHi = gnarkutil.ToBytes16(api, finalStateRootHashesHi.Lookup(nbBatchesSums[i])[0])
			newStateRootLo = gnarkutil.ToBytes16(api, finalStateRootHashesLo.Lookup(nbBatchesSums[i])[0])
			newStateRoot   = [32]frontend.Variable(append(newStateRootHi[:], newStateRootLo[:]...))
		)

		shnarfParams[i] = ShnarfIteration{ // prepare shnarf verification data
			BlobDataSnarkHash:    gnarkutil.ToBytes32(api, piq.SnarkHash),
			NewStateRootHash:     newStateRoot,
			EvaluationPointBytes: piq.X,
			EvaluationClaimBytes: fr377EncodedFr381ToBytes(api, piq.Y),
		}

		// "open" decompression circuit public input
		api.AssertIsEqual(c.DataAvailabilityPublicInput[i], api.Mul(rDA.InRange[i], piq.Sum(api)))
	}

	var (
		// The circuit only has the last shnarf as input, therefore we do not
		// perform CHECK_SHNARF. However, since they are chained, the passing of
		// CHECK_FINAL_SHNARF implies that all shnarfs are correct.
		shnarfs = ComputeShnarfs(&hshK, c.ParentShnarf, shnarfParams)
		// implicit: CHECK_EXEC_LIMIT
		rExecution             = internal.NewRange(api, nbExecution, maxNbExecution)
		finalBlockNum          = c.LastFinalizedBlockNumber
		finalRollingHashMsgNum = c.LastFinalizedRollingHashNumber
		finalRollingHash       = c.LastFinalizedRollingHash
		finalBlockTime         = c.LastFinalizedBlockTimestamp
		l2MessagesByByte       [32][]internal.VarSlice
		execMaxNbL2Msg         = len(c.ExecutionFPIQ[0].L2MessageHashes.Values)
		merkleNbLeaves         = 1 << c.L2MessageMerkleDepth
		// This checks that the initial state-root hash represents a
		// koalabear octuplet. The only situation where this clause may apply
		// is when the value passed by the smart-contract can be on BLS12-377
		// (on the first proof after the migration to koalabear).
		isKoalaStateRootHash = isActuallyKoalaHash(api, c.InitialStateRootHash)
		finalStateRootHash   = [2]frontend.Variable{
			api.Select(isKoalaStateRootHash, c.InitialStateRootHash[0], c.FirstExecutionInitialStateRootHash[0]),
			api.Select(isKoalaStateRootHash, c.InitialStateRootHash[1], c.FirstExecutionInitialStateRootHash[1]),
		}
	)

	for j := range l2MessagesByByte {
		l2MessagesByByte[j] = make([]internal.VarSlice, maxNbExecution)
		for k := range l2MessagesByByte[j] {
			l2MessagesByByte[j][k] = internal.VarSlice{Values: make([]frontend.Variable, execMaxNbL2Msg)}
		}
	}

	// we can "allow non-deterministic behavior" because all compared values have been range-checked
	comparator := cmp.NewBoundedComparator(api, new(big.Int).Lsh(big.NewInt(1), 65), true)
	// TODO try using lookups or crumb decomposition to make comparisons more
	// efficient

	// Check that IsAllowedCircuitID public input matches the value in AggregationFPIQSnark
	api.AssertIsEqual(c.IsAllowedCircuitID, c.ChainConfigurationFPISnark.IsAllowedCircuitID)

	for i, piq := range c.ExecutionFPIQ {
		piq.RangeCheck(api) // CHECK_MSG_LIMIT

		// inRange is a binary value indicating that the current execution
		// being looked at in the current iteration is an actual execution and
		// not some padding.
		inRange := rExecution.InRange[i]

		pi := execution.FunctionalPublicInputSnark{
			FunctionalPublicInputQSnark: piq,
			InitialStateRootHash:        finalStateRootHash,                                   // implicit CHECK_STATE_CONSEC
			InitialBlockNumber:          api.Add(finalBlockNum, 1),                            // implicit CHECK_NUM_CONSEC
			ChainID:                     c.ChainConfigurationFPISnark.ChainID,                 // implicit CHECK_CHAIN_ID
			BaseFee:                     c.ChainConfigurationFPISnark.BaseFee,                 // implicit CHECK_BASE_FEE
			CoinBase:                    c.ChainConfigurationFPISnark.CoinBase,                // implicit CHECK_COINBASE
			L2MessageServiceAddr:        c.ChainConfigurationFPISnark.L2MessageServiceAddress, // implicit CHECK_SVC_ADDR
		}

		comparator.AssertIsLessEq(pi.InitialBlockTimestamp, pi.FinalBlockTimestamp)                // CHECK_TIME_NODECREASE
		comparator.AssertIsLessEq(pi.InitialBlockNumber, pi.FinalBlockNumber)                      // CHECK_NUM_NODECREASE
		comparator.AssertIsLess(finalBlockTime, pi.InitialBlockTimestamp)                          // CHECK_TIME_INCREASE
		comparator.AssertIsLessEq(pi.FirstRollingHashUpdateNumber, pi.LastRollingHashUpdateNumber) // CHECK_RHASH_NODECREASE

		finalRhMsgNumZero := api.IsZero(piq.LastRollingHashUpdateNumber)
		api.AssertIsEqual(finalRhMsgNumZero, api.IsZero(piq.FirstRollingHashUpdateNumber)) // CHECK_RHASH_FIRSTLAST
		rollingHashUpdated := api.Mul(inRange, api.Sub(1, finalRhMsgNumZero))

		// CHECK_RHASH_CONSEC
		internal.AssertIsLessIf(api, rollingHashUpdated, finalRollingHashMsgNum, pi.FirstRollingHashUpdateNumber)
		finalRollingHashMsgNum = api.Select(rollingHashUpdated, pi.LastRollingHashUpdateNumber, finalRollingHashMsgNum)
		copy(finalRollingHash[:], internal.SelectMany(api, rollingHashUpdated, pi.FinalRollingHashUpdate[:], finalRollingHash[:]))

		finalBlockTime = pi.FinalBlockTimestamp
		finalBlockNum = pi.FinalBlockNumber
		finalStateRootHash = pi.FinalStateRootHash

		api.AssertIsEqual(c.ExecutionPublicInput[i], api.Mul(rExecution.InRange[i], pi.Sum(api))) // "open" execution circuit public input

		if len(pi.L2MessageHashes.Values) != execMaxNbL2Msg {
			return errors.New("number of L2 messages must be the same for all executions")
		}

		// "transpose" the L2 messages by byte for Concat -> Merkle
		for j := range l2MessagesByByte { // perf-TODO probably better to change all 32bytes into four uint64s instead.
			for k := range pi.L2MessageHashes.Values {
				l2MessagesByByte[j][i].Values[k] = pi.L2MessageHashes.Values[k][j]
			}
			l2MessagesByByte[j][i].Length = pi.L2MessageHashes.Length
		}
	}

	merkleLeavesConcat := internal.Var32Slice{Values: make([][32]frontend.Variable, c.L2MessageMaxNbMerkle*merkleNbLeaves)}
	for i := 0; i < 32; i++ {
		ithBytes := internal.Concat(api, len(merkleLeavesConcat.Values), l2MessagesByByte[i]...)
		for j := range merkleLeavesConcat.Values {
			merkleLeavesConcat.Values[j][i] = ithBytes.Values[j]
		}
		merkleLeavesConcat.Length = ithBytes.Length // same value regardless of i
	}

	pi := public_input.AggregationFPISnark{
		AggregationFPIQSnark: c.AggregationFPIQSnark,
		NbL2Messages:         merkleLeavesConcat.Length,
		L2MsgMerkleTreeRoots: make([][32]frontend.Variable, c.L2MessageMaxNbMerkle), // implicit CHECK_MERKLE_CAP1
		// implicit CHECK_FINAL_NUM
		FinalBlockNumber: rExecution.LastF(func(i int) frontend.Variable { return c.ExecutionFPIQ[i].FinalBlockNumber }),
		// implicit CHECK_FINAL_TIME
		FinalBlockTimestamp:    rExecution.LastF(func(i int) frontend.Variable { return c.ExecutionFPIQ[i].FinalBlockTimestamp }),
		FinalShnarf:            rDA.LastArray32(shnarfs), // implicit CHECK_FINAL_SHNARF
		FinalRollingHash:       finalRollingHash,         // implicit CHECK_FINAL_RHASH
		FinalRollingHashNumber: finalRollingHashMsgNum,   // implicit CHECK_FINAL_RHASH_NUM
		L2MsgMerkleTreeDepth:   c.L2MessageMerkleDepth,
	}

	quotient, remainder := internal.DivEuclidean(api, merkleLeavesConcat.Length, merkleNbLeaves)
	pi.NbL2MsgMerkleTreeRoots = api.Add(quotient, api.Sub(1, api.IsZero(remainder)))
	comparator.AssertIsLessEq(pi.NbL2MsgMerkleTreeRoots, c.L2MessageMaxNbMerkle) // CHECK_MERKLE_CAP0
	// implicit CHECK_MERKLE
	for i := range pi.L2MsgMerkleTreeRoots {
		pi.L2MsgMerkleTreeRoots[i] = MerkleRootSnark(&hshK, merkleLeavesConcat.Values[i*merkleNbLeaves:(i+1)*merkleNbLeaves])
	}

	twoPow8 := big.NewInt(256)
	// "open" aggregation public input
	aggregationPIBytes := pi.Sum(api, &hshK)
	api.AssertIsEqual(c.AggregationPublicInput[0], compress.ReadNum(api, aggregationPIBytes[:16], twoPow8))
	api.AssertIsEqual(c.AggregationPublicInput[1], compress.ReadNum(api, aggregationPIBytes[16:], twoPow8))

	return hshK.Finalize()
}

func MerkleRootSnark(hshK keccak.BlockHasher, leaves [][32]frontend.Variable) [32]frontend.Variable {

	values := slices.Clone(leaves)
	if !utils.IsPowerOfTwo(len(values)) {
		panic("number of leaves must be a perfect power of two")
	}
	for len(values) > 1 {
		for i := 0; i < len(values)/2; i++ {
			values[i] = hshK.Sum(nil, values[i*2], values[i*2+1])
		}
		values = values[:len(values)/2]
	}

	return values[0]
}

type Compiled struct {
	Circuit *Circuit
	Keccak  keccak.CompiledStrictHasher
}

func Compile(c config.PublicInput, wizardCompilationOpts keccak.CompilationParams) (*Compiled, error) {

	if c.L2MsgMaxNbMerkle <= 0 {
		merkleNbLeaves := 1 << c.L2MsgMerkleDepth
		c.L2MsgMaxNbMerkle = (c.MaxNbExecution*c.ExecutionMaxNbMsg + merkleNbLeaves - 1) / merkleNbLeaves
	}

	if c.MockKeccakWizard {
		wizardCompilationOpts = nil
		logrus.Warn("KECCAK HASH RESULTS WILL NOT BE CHECKED. THIS SHOULD ONLY OCCUR IN A UNIT TEST.")
	}
	sh := newKeccakCompiler(c).Compile(wizardCompilationOpts)
	shc, err := sh.GetCircuit()
	if err != nil {
		return nil, err
	}

	circuit := allocateCircuit(c)
	circuit.Keccak = shc

	return &Compiled{
		Circuit: &circuit,
		Keccak:  sh,
	}, nil
}

func (c *Compiled) getConfig() (config.PublicInput, error) {
	executionNbMsg := 0
	execs := c.Circuit.ExecutionFPIQ
	if len(c.Circuit.ExecutionFPIQ) != 0 {
		executionNbMsg = len(execs[0].L2MessageHashes.Values)
		for i := range execs {
			if len(execs[i].L2MessageHashes.Values) != executionNbMsg {
				return config.PublicInput{}, errors.New("inconsistent max number of L2 message hashes")
			}
		}
	}
	return config.PublicInput{
		MaxNbDataAvailability: len(c.Circuit.DataAvailabilityFPIQ),
		MaxNbExecution:        len(c.Circuit.ExecutionFPIQ),
		ExecutionMaxNbMsg:     executionNbMsg,
		L2MsgMerkleDepth:      c.Circuit.L2MessageMerkleDepth,
		L2MsgMaxNbMerkle:      c.Circuit.L2MessageMaxNbMerkle,
		MaxNbCircuits:         c.Circuit.MaxNbCircuits,
	}, nil
}

func allocateCircuit(cfg config.PublicInput) Circuit {

	res := Circuit{
		DataAvailabilityPublicInput: make([]frontend.Variable, cfg.MaxNbDataAvailability),
		ExecutionPublicInput:        make([]frontend.Variable, cfg.MaxNbExecution),
		DataAvailabilityFPIQ:        make([]blobdecompression.FunctionalPublicInputQSnark, cfg.MaxNbDataAvailability),
		ExecutionFPIQ:               make([]execution.FunctionalPublicInputQSnark, cfg.MaxNbExecution),
		L2MessageMerkleDepth:        cfg.L2MsgMerkleDepth,
		L2MessageMaxNbMerkle:        cfg.L2MsgMaxNbMerkle,
		MaxNbCircuits:               cfg.MaxNbCircuits,
	}

	for i := range res.ExecutionFPIQ {
		res.ExecutionFPIQ[i].L2MessageHashes.Values = make([][32]frontend.Variable, cfg.ExecutionMaxNbMsg)
	}

	return res
}

func newKeccakCompiler(c config.PublicInput) keccak.StrictHasherCompiler {
	nbShnarf := c.MaxNbDataAvailability
	nbMerkle := c.L2MsgMaxNbMerkle * ((1 << c.L2MsgMerkleDepth) - 1)
	res := keccak.NewStrictHasherCompiler(0)
	for range nbShnarf {
		res.WithStrictHashLengths(160) // 5 components in every shnarf
	}

	for range nbMerkle {
		res.WithStrictHashLengths(64) // 2 tree nodes
	}

	// aggregation PI opening
	res.WithFlexibleHashLengths(32 * c.L2MsgMaxNbMerkle)
	res.WithStrictHashLengths(416) // 416 (13 × 32 bytes)
	return res
}

type builder struct {
	*config.PublicInput
}

func NewBuilder(c config.PublicInput) circuits.Builder {
	return builder{&c}
}

func (b builder) Compile() (constraint.ConstraintSystem, error) {
	c, err := Compile(*b.PublicInput, keccak.WizardCompilationParameters())
	if err != nil {
		return nil, err
	}
	const estimatedNbConstraints = 1 << 27
	cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, c.Circuit, frontend.WithCapacity(estimatedNbConstraints))
	if err != nil {
		return nil, err
	}

	return cs, nil
}

// GetMaxNbCircuitsSum computes MaxNbDA + MaxNbExecution from the compiled constraint system
// TODO replace with something cleaner, using the config
func GetMaxNbCircuitsSum(cs constraint.ConstraintSystem) int {
	return cs.GetNbPublicVariables() - 3
}

type InnerCircuitType uint8

const (
	Execution     InnerCircuitType = 0
	Decompression InnerCircuitType = 1
)

func InnerCircuitTypesToIndexes(cfg *config.PublicInput, types []InnerCircuitType) []int {
	indexes := utils.RightPad(utils.Partition(utils.RangeSlice[int](len(types)), types), 2)
	return utils.RightPad(
		append(utils.RightPad(indexes[Execution], cfg.MaxNbExecution), indexes[Decompression]...), cfg.MaxNbExecution+cfg.MaxNbDataAvailability)

}

// isActuallyKoalaHash checks if the given pair of frontend.Variable represents
// an octuplet of field elements. This is done by splitting the variables in
// 4 limbs (of each 32 bits) and checking that these are smaller than the
// koalabear modulus.
func isActuallyKoalaHash(api frontend.API, hash [2]frontend.Variable) frontend.Variable {

	// The cmpRes is computed by adding the result of Cmp for each (allegedly)
	// koalabear element. If the limbs are koalabear, the result of cmp will
	// be -1. Thus, at the end of the function cmpRes would be equal to 0 and
	// to some positive value if any of the cmpRes is NOT -1. Thus, it is
	// an equivalent test.
	cmpRes := frontend.Variable(8)

	for i := range hash {
		// The decomposition is done by splitting in bits, and then recombining
		// them in 4 uint32s.
		bitsOfHalf := api.ToBinary(hash[i], 128)
		for k := range 4 {
			limbs := api.FromBinary(bitsOfHalf[k*32 : (k+1)*32]...)
			shouldBeNeg := api.Cmp(limbs, koalabear.Modulus())
			cmpRes = api.Add(cmpRes, shouldBeNeg)
		}
	}

	return api.IsZero(cmpRes)
}
