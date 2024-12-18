package pi_interconnection

import (
	"errors"
	"math/big"
	"slices"

	"github.com/sirupsen/logrus"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/config"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/types"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/hash"
	"github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/std/math/cmp"
	decompression "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v1"
	"github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc/gkrmimc"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/cleanup"
	mimcComp "github.com/consensys/linea-monorepo/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

type Circuit struct {
	AggregationPublicInput   [2]frontend.Variable `gnark:",public"` // the public input of the aggregation circuit; divided big-endian into two 16-byte chunks
	ExecutionPublicInput     []frontend.Variable  `gnark:",public"`
	DecompressionPublicInput []frontend.Variable  `gnark:",public"`

	DecompressionFPIQ []decompression.FunctionalPublicInputQSnark
	ExecutionFPIQ     []execution.FunctionalPublicInputQSnark

	public_input.AggregationFPIQSnark

	Keccak keccak.StrictHasherCircuit

	// config
	L2MessageMerkleDepth int
	L2MessageMaxNbMerkle int

	// TODO @Tabaie @alexandre.belling remove hard coded values once these are included in aggregation PI sum
	L2MessageServiceAddr types.EthAddress
	ChainID              uint64

	MaxNbCircuits int // possibly useless TODO consider removing
	UseGkrMimc    bool
}

// type alias to denote a wizard-compilation suite. This is used when calling
// compile and provides internal parameters for the wizard package.
type compilationSuite = []func(*wizard.CompiledIOP)

func (c *Circuit) Define(api frontend.API) error {
	// TODO @Tabaie @alexandre.belling remove hard coded values once these are included in aggregation PI sum
	api.AssertIsEqual(c.ChainID, c.AggregationFPIQSnark.ChainID)
	api.AssertIsEqual(c.L2MessageServiceAddr[:], c.AggregationFPIQSnark.L2MessageServiceAddr)

	maxNbDecompression, maxNbExecution := len(c.DecompressionPublicInput), len(c.ExecutionPublicInput)
	if len(c.DecompressionFPIQ) != maxNbDecompression || len(c.ExecutionFPIQ) != maxNbExecution {
		return errors.New("public / functional public input length mismatch")
	}

	c.AggregationFPIQSnark.RangeCheck(api)

	// implicit: CHECK_DECOMP_LIMIT
	rDecompression := internal.NewRange(api, c.NbDecompression, len(c.DecompressionPublicInput))
	hshK := c.Keccak.NewHasher(api)

	// equivalently, nbBatchesSums[i] is the index of the first execution circuit associated with the i+1-st decompression circuit
	nbBatchesSums := rDecompression.PartialSumsF(func(i int) frontend.Variable { return c.DecompressionFPIQ[i].NbBatches })
	nbExecution := nbBatchesSums[len(nbBatchesSums)-1] // implicit: CHECK_NB_EXEC

	// These two checks prevents constructing a proof where no execution or no
	// compression proofs are provided. This is to prevent corner cases from
	// arising.
	api.AssertIsDifferent(c.NbDecompression, 0)
	api.AssertIsDifferent(nbExecution, 0)

	if c.MaxNbCircuits > 0 { // CHECK_CIRCUIT_LIMIT
		api.AssertIsLessOrEqual(api.Add(nbExecution, c.NbDecompression), c.MaxNbCircuits)
	}

	var (
		hshM hash.FieldHasher
	)
	if c.UseGkrMimc {
		hsh := gkrmimc.NewHasherFactory(api).NewHasher()
		hshM = &hsh
	} else {
		if hsh, err := mimc.NewMiMC(api); err != nil {
			return err
		} else {
			hshM = &hsh
		}
	}

	batchHashes := make([]frontend.Variable, len(c.ExecutionPublicInput))
	for i, pi := range c.ExecutionFPIQ {
		batchHashes[i] = pi.DataChecksum
	}

	finalStateRootHashes := logderivlookup.New(api)
	finalStateRootHashes.Insert(c.InitialStateRootHash)
	for _, pi := range c.ExecutionFPIQ {
		finalStateRootHashes.Insert(pi.FinalStateRootHash)
	}

	blobBatchHashes := internal.ChecksumSubSlices(api, hshM, batchHashes, internal.VarSlice{Values: nbBatchesSums, Length: c.NbDecompression})

	shnarfParams := make([]ShnarfIteration, len(c.DecompressionPublicInput))
	for i, piq := range c.DecompressionFPIQ {
		piq.RangeCheck(api)

		shnarfParams[i] = ShnarfIteration{ // prepare shnarf verification data
			BlobDataSnarkHash:    utils.ToBytes(api, piq.SnarkHash),
			NewStateRootHash:     utils.ToBytes(api, finalStateRootHashes.Lookup(nbBatchesSums[i])[0]),
			EvaluationPointBytes: piq.X,
			EvaluationClaimBytes: fr377EncodedFr381ToBytes(api, piq.Y),
		}

		// "open" decompression circuit public input
		api.AssertIsEqual(c.DecompressionPublicInput[i], api.Mul(rDecompression.InRange[i], piq.Sum(api, hshM, blobBatchHashes[i])))
	}

	shnarfs := ComputeShnarfs(&hshK, c.ParentShnarf, shnarfParams)
	// The circuit only has the last shnarf as input, therefore we do not perform
	// CHECK_SHNARF. However, since they are chained, the passing of CHECK_FINAL_SHNARF
	// implies that all shnarfs are correct.

	// implicit: CHECK_EXEC_LIMIT
	rExecution := internal.NewRange(api, nbExecution, maxNbExecution)

	finalBlockNum, finalRollingHashMsgNum, finalRollingHash := c.LastFinalizedBlockNumber, c.LastFinalizedRollingHashNumber, c.LastFinalizedRollingHash
	finalBlockTime, finalState := c.LastFinalizedBlockTimestamp, c.InitialStateRootHash
	var l2MessagesByByte [32][]internal.VarSlice

	execMaxNbL2Msg := len(c.ExecutionFPIQ[0].L2MessageHashes.Values)
	merkleNbLeaves := 1 << c.L2MessageMerkleDepth
	for j := range l2MessagesByByte {
		l2MessagesByByte[j] = make([]internal.VarSlice, maxNbExecution)
		for k := range l2MessagesByByte[j] {
			l2MessagesByByte[j][k] = internal.VarSlice{Values: make([]frontend.Variable, execMaxNbL2Msg)}
		}
	}

	// we can "allow non-deterministic behavior" because all compared values have been range-checked
	comparator := cmp.NewBoundedComparator(api, new(big.Int).Lsh(big.NewInt(1), 65), true)
	// TODO try using lookups or crumb decomposition to make comparisons more efficient
	for i, piq := range c.ExecutionFPIQ {
		piq.RangeCheck(api) // CHECK_MSG_LIMIT

		// inRange is a binary value indicating that the current execution
		// being looked at in the current iteration is an actual execution and
		// not some padding.
		inRange := rExecution.InRange[i]

		pi := execution.FunctionalPublicInputSnark{
			FunctionalPublicInputQSnark: piq,
			InitialStateRootHash:        finalState,                // implicit CHECK_STATE_CONSEC
			InitialBlockNumber:          api.Add(finalBlockNum, 1), // implicit CHECK_NUM_CONSEC
			ChainID:                     c.ChainID,                 // implicit CHECK_CHAIN_ID
			L2MessageServiceAddr:        c.L2MessageServiceAddr[:], // implicit CHECK_SVC_ADDR
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
		finalState = pi.FinalStateRootHash

		api.AssertIsEqual(c.ExecutionPublicInput[i], api.Mul(rExecution.InRange[i], pi.Sum(api, hshM))) // "open" execution circuit public input

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
		FinalShnarf:            rDecompression.LastArray32(shnarfs), // implicit CHECK_FINAL_SHNARF
		FinalRollingHash:       finalRollingHash,                    // implicit CHECK_FINAL_RHASH
		FinalRollingHashNumber: finalRollingHashMsgNum,              // implicit CHECK_FINAL_RHASH_NUM
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

func Compile(c config.PublicInput, wizardCompilationOpts ...func(iop *wizard.CompiledIOP)) (*Compiled, error) {

	if c.L2MsgMaxNbMerkle <= 0 {
		merkleNbLeaves := 1 << c.L2MsgMerkleDepth
		c.L2MsgMaxNbMerkle = (c.MaxNbExecution*c.ExecutionMaxNbMsg + merkleNbLeaves - 1) / merkleNbLeaves
	}

	if c.MockKeccakWizard {
		wizardCompilationOpts = nil
		logrus.Warn("KECCAK HASH RESULTS WILL NOT BE CHECKED. THIS SHOULD ONLY OCCUR IN A UNIT TEST.")
	}
	sh := newKeccakCompiler(c).Compile(wizardCompilationOpts...)
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
		MaxNbDecompression: len(c.Circuit.DecompressionFPIQ),
		MaxNbExecution:     len(c.Circuit.ExecutionFPIQ),
		ExecutionMaxNbMsg:  executionNbMsg,
		L2MsgMerkleDepth:   c.Circuit.L2MessageMerkleDepth,
		L2MsgMaxNbMerkle:   c.Circuit.L2MessageMaxNbMerkle,
		MaxNbCircuits:      c.Circuit.MaxNbCircuits,
	}, nil
}

func allocateCircuit(cfg config.PublicInput) Circuit {
	res := Circuit{
		DecompressionPublicInput: make([]frontend.Variable, cfg.MaxNbDecompression),
		ExecutionPublicInput:     make([]frontend.Variable, cfg.MaxNbExecution),
		DecompressionFPIQ:        make([]decompression.FunctionalPublicInputQSnark, cfg.MaxNbDecompression),
		ExecutionFPIQ:            make([]execution.FunctionalPublicInputQSnark, cfg.MaxNbExecution),
		L2MessageMerkleDepth:     cfg.L2MsgMerkleDepth,
		L2MessageMaxNbMerkle:     cfg.L2MsgMaxNbMerkle,
		MaxNbCircuits:            cfg.MaxNbCircuits,
		L2MessageServiceAddr:     types.EthAddress(cfg.L2MsgServiceAddr),
		ChainID:                  cfg.ChainID,
		UseGkrMimc:               true,
	}

	for i := range res.ExecutionFPIQ {
		res.ExecutionFPIQ[i].L2MessageHashes.Values = make([][32]frontend.Variable, cfg.ExecutionMaxNbMsg)
	}

	return res
}

func newKeccakCompiler(c config.PublicInput) *keccak.StrictHasherCompiler {
	nbShnarf := c.MaxNbDecompression
	nbMerkle := c.L2MsgMaxNbMerkle * ((1 << c.L2MsgMerkleDepth) - 1)
	res := keccak.NewStrictHasherCompiler(nbShnarf, nbMerkle, 2)
	for i := 0; i < nbShnarf; i++ {
		res.WithStrictHashLengths(160) // 5 components in every shnarf
	}

	for i := 0; i < nbMerkle; i++ {
		res.WithStrictHashLengths(64) // 2 tree nodes
	}

	// aggregation PI opening
	res.WithFlexibleHashLengths(32 * c.L2MsgMaxNbMerkle)
	res.WithStrictHashLengths(384)

	return &res
}

type builder struct {
	*config.PublicInput
}

func NewBuilder(c config.PublicInput) circuits.Builder {
	return builder{&c}
}

func (b builder) Compile() (constraint.ConstraintSystem, error) {
	c, err := Compile(*b.PublicInput, WizardCompilationParameters()...)
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

func WizardCompilationParameters() []func(iop *wizard.CompiledIOP) {
	var (
		sisInstance = ringsis.Params{LogTwoBound: 16, LogTwoDegree: 6}

		fullCompilationSuite = compilationSuite{

			compiler.Arcane(1<<10, 1<<18, false),
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(256),
				vortex.WithSISParams(&sisInstance),
			),

			selfrecursion.SelfRecurse,
			cleanup.CleanUp,
			mimcComp.CompileMiMC,
			compiler.Arcane(1<<10, 1<<16, false),
			vortex.Compile(
				8,
				vortex.ForceNumOpenedColumns(64),
				vortex.WithSISParams(&sisInstance),
			),

			selfrecursion.SelfRecurse,
			cleanup.CleanUp,
			mimcComp.CompileMiMC,
			compiler.Arcane(1<<10, 1<<13, false),
			vortex.Compile(
				8,
				vortex.ForceNumOpenedColumns(64),
				vortex.ReplaceSisByMimc(),
			),
		}
	)

	return fullCompilationSuite

}

// GetMaxNbCircuitsSum computes MaxNbDecompression + MaxNbExecution from the compiled constraint system
// TODO replace with something cleaner, using the config
func GetMaxNbCircuitsSum(cs constraint.ConstraintSystem) int {
	return cs.GetNbPublicVariables() - 2
}

type InnerCircuitType uint8

const (
	Execution     InnerCircuitType = 0
	Decompression InnerCircuitType = 1
)

func InnerCircuitTypesToIndexes(cfg *config.PublicInput, types []InnerCircuitType) []int {
	indexes := utils.RightPad(utils.Partition(utils.RangeSlice[int](len(types)), types), 2)
	return utils.RightPad(
		append(utils.RightPad(indexes[Execution], cfg.MaxNbExecution), indexes[Decompression]...), cfg.MaxNbExecution+cfg.MaxNbDecompression)

}
