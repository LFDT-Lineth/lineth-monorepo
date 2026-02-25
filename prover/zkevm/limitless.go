package zkevm

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path"
	"strings"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/serialization"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var (
	bootstrapperFile              = "dw-bootstrapper.bin"
	discFile                      = "disc.bin"
	zkevmFile                     = "zkevm-wiop.bin"
	blueprintGLPrefix             = "dw-blueprint-gl"
	blueprintLppPrefix            = "dw-blueprint-lpp"
	blueprintGLTemplate           = blueprintGLPrefix + "-%d.bin"
	blueprintLppTemplate          = blueprintLppPrefix + "-%d.bin"
	compileLppTemplate            = "dw-compiled-lpp-%v.bin"
	compileGlTemplate             = "dw-compiled-gl-%v.bin"
	debugLppTemplate              = "dw-debug-lpp-%v.bin"
	debugGlTemplate               = "dw-debug-gl-%v.bin"
	conglomerationFile            = "dw-compiled-conglomeration.bin"
	executionLimitlessPath        = "execution-limitless"
	verificationKeyMerkleTreeFile = "verification-key-merkle-tree.bin"
)

var LimitlessCompilationParams = distributed.CompilationParams{
	FixedNbRowPlonkCircuit:       1 << 19,
	FixedNbRowExternalHasher:     1 << 15,
	FixedNbPublicInput:           1 << 10,
	InitialCompilerSize:          1 << 18,
	InitialCompilerSizeConglo:    1 << 13,
	ColumnProfileMPTS:            []int{17, 335, 37, 3, 5, 15, 0, 1},
	ColumnProfileMPTSPrecomputed: 22,
}

const (
	TinyStuffsModuleName  = "TINY-STUFFS"
	ArithOpsModuleName    = "ARITH-OPS"
	HubModuleName         = "HUB"
	KeccakModuleName      = "KECCAK"
	StaticModuleName      = "STATIC"
	Modexp256ModuleName   = "MODEXP-256"
	ModexpLargeModuleName = "MODEXP_LARGE"
	Sha2ModuleName        = "SHA2"
	EcdsaModuleName       = "ECDSA"
	P256ModuleName        = "P256"
	BlsG1ModuleName       = "BLS-G1"
	BlsG2ModuleName       = "BLS-G2"
	BlsPairingModuleName  = "BLS-PAIRING"
	BlsKzgModuleName      = "BLS-KZG"
	BnEcOpsModuleName     = "BN-EC-OPS"
	BnPairingModuleName   = "BN-PAIRING"
	BnG2CheckModuleName   = "BN-G2-CHECK"
)

// GetTestZkEVM returns a ZkEVM object configured for testing.
func GetTestZkEVM() *ZkEvm {
	return FullZKEVMWithSuite(
		config.GetTestTracesLimits(),
		CompilationSuite{},
		&config.Config{
			Execution: config.Execution{
				IgnoreCompatibilityCheck: true,
			},
		},
	)
}

// LimitlessZkEVM defines the wizard responsible for proving execution of the EVM
// and the associated wizard circuits for the limitless prover protocol.
type LimitlessZkEVM struct {
	Zkevm      *ZkEvm
	DistWizard *distributed.DistributedWizard
}

// DiscoveryAdvices returns a list of advice for the discovery of the modules. These
// values have been obtained thanks to a statistical analysis of the traces
// assignments involving correlation of the modules and hierarchical clustering.
// The advices are optimized to minimize the number of segments generated when
// producing an EVM proof.
func DiscoveryAdvices(zkevm *ZkEvm) []*distributed.ModuleDiscoveryAdvice {

	return []*distributed.ModuleDiscoveryAdvice{

		// ARITH-OPS
		//
		{BaseSize: 16384, Cluster: ArithOpsModuleName, Regexp: `^exp\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^bit_(sar|shr|ror|shl|xoan)[0-9]+(_u[0-9]+)?\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^fill_bytes_between\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^log[0-9]+(_u[0-9]+)?\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^set_byte[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^bit_xoan_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^mul\.`},
		{BaseSize: 65536, Cluster: ArithOpsModuleName, Regexp: `^add\.`},
		{BaseSize: 65536, Cluster: ArithOpsModuleName, Regexp: `^mod\.`},
		{BaseSize: 65536, Cluster: ArithOpsModuleName, Regexp: `^min256_64\.`},
		{BaseSize: 131072, Cluster: ArithOpsModuleName, Regexp: `^shf\.`},
		{BaseSize: 131072, Cluster: ArithOpsModuleName, Regexp: `^bin\.`},
		{BaseSize: 1048576, Cluster: ArithOpsModuleName, ModuleRef: "MIMC_COMPILER"},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^byte[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^signextend\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^max3_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^maxlog\.`},
		{BaseSize: 262144, Cluster: ArithOpsModuleName, Regexp: `^wcp\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^counts_nz_[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^divide\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^max_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^negate\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^pow\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^signed_divide\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^xor_on_xor\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^zero_check\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^abs\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^byte_size\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^rpad_[0-9]+_[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^switch_endian_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^switch_endian_8_args\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^cap32\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^ceil_div\.`},

		// Hub
		//
		{BaseSize: 32768, Cluster: HubModuleName, Regexp: `^rlptxn\.`},
		{BaseSize: 65536, Cluster: HubModuleName, Regexp: `^gas\.`},
		{BaseSize: 65536, Cluster: HubModuleName, Regexp: `^gas_out_of_pocket\.`},
		{BaseSize: 65536, Cluster: HubModuleName, Regexp: `^shakiradata\.`},
		{BaseSize: 65536, Cluster: HubModuleName, Regexp: `^stp\.`},
		{BaseSize: 65536, Cluster: HubModuleName, Regexp: `^call_gas_extra\.`},
		{BaseSize: 131072, Cluster: HubModuleName, Regexp: `^mxp\.`},
		{BaseSize: 131072, Cluster: HubModuleName, Regexp: `^oob\.`},
		{BaseSize: 262144, Cluster: HubModuleName, Regexp: `^hub\.`},
		{BaseSize: 262144, Cluster: HubModuleName, Regexp: `^mmio\.`},
		{BaseSize: 262144, Cluster: HubModuleName, Regexp: `^mmu\.`},
		{BaseSize: 524288, Cluster: HubModuleName, Regexp: `^rom\.`},
		{BaseSize: 1048576, Cluster: HubModuleName, Regexp: `^hub×4\.`},
		{BaseSize: 1048576, Cluster: HubModuleName, Regexp: `^mmio×3\.`},
		{BaseSize: 65536, Cluster: HubModuleName, Regexp: `^euc\.`},
		{BaseSize: 16384, Cluster: HubModuleName, Regexp: `^oob_prc_pricing\.`},
		{BaseSize: 16384, Cluster: HubModuleName, Regexp: `^oob_prc\.`},
		{BaseSize: 16384, Cluster: HubModuleName, Regexp: `^jump_target_check\.`},
		{BaseSize: 16384, Cluster: HubModuleName, Regexp: `^oob_gas_cost\.`},
		{BaseSize: 16384, Cluster: HubModuleName, Regexp: `^oob_cds_valid\.`},
		{BaseSize: 16384, Cluster: HubModuleName, Regexp: `^out_of_bounds_check\.`},
		{BaseSize: 16384, Cluster: HubModuleName, Regexp: `^rpad_[0-9]+\.`},
		{BaseSize: 16384, Cluster: HubModuleName, Regexp: `^abort_check\.`},
		{BaseSize: 16384, Cluster: HubModuleName, Regexp: `^get_ms\.`},

		// Keccak
		//
		{BaseSize: 32768, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Pa_cKeccak.KeccakF.IO.IsBlock},
		{BaseSize: 16384, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_accInfo.Provider.IsHashHi},
		{BaseSize: 32768, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Pa_cKeccak.HashHi},
		{BaseSize: 131072, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_accData.IsActive},
		{BaseSize: 262144, Cluster: KeccakModuleName, Column: zkevm.StateManager.MimcCodeHash.CodeSize},
		{BaseSize: 262144, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Pa_packing.Repacked.Lanes},
		{BaseSize: 262144, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Pa_packing.Block.AccNumLane},
		{BaseSize: 32768, Cluster: KeccakModuleName, Column: zkevm.StateManager.Accumulator.Cols.IsActiveAccumulator},
		{BaseSize: 32768, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Pa_importPad.IsPadded},
		{BaseSize: 131072, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Pa_packing.Repacked.Inputs.Spaghetti.FilterSpaghetti},
		{BaseSize: 131072, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Pa_packing.Repacked.Inputs.Spaghetti.PA.ContentSpaghetti[0]},

		// MODEXP 256
		//
		{BaseSize: 65536, Cluster: Modexp256ModuleName, Regexp: `^blake2fmodexpdata\.`},
		{BaseSize: 8192, Cluster: Modexp256ModuleName, Column: zkevm.Modexp.IsActive},
		{BaseSize: 8192, Cluster: Modexp256ModuleName, Column: zkevm.Modexp.GnarkCircuitConnector256Bits.IsActive},
		{BaseSize: 8192, Cluster: Modexp256ModuleName, Column: zkevm.Modexp.GnarkCircuitConnector256Bits.ActualCircuitInputMask.PatternPosPrecomp},
		{BaseSize: 8192, Cluster: Modexp256ModuleName, Regexp: `^oob_modexp`},
		{BaseSize: 8192, Cluster: Modexp256ModuleName, Regexp: `^oob_prc_blake`},
		{BaseSize: 8192, Cluster: Modexp256ModuleName, Regexp: `^blake2f`},

		// MODEXP 8192
		//
		{BaseSize: 256, Cluster: ModexpLargeModuleName, Column: zkevm.Modexp.GnarkCircuitConnectorLarge.ActualCircuitInputMask.PatternPosPrecomp},
		{BaseSize: 256, Cluster: ModexpLargeModuleName, Column: zkevm.Modexp.GnarkCircuitConnectorLarge.IsActive},

		// SHA2
		//
		{BaseSize: 512, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_cSha2.GnarkCircuitConnector.IsActive},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_packing.Repacked.Inputs.Spaghetti.NewHashSp},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_packing.Repacked.Inputs.Spaghetti.PA.TagSpaghetti},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_packing.Block.AccNumLane},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_cSha2.HashHi},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_importPad.Index},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_packing.Repacked.IsLaneActive},

		// TINY-STUFFS
		//
		{BaseSize: 512, Cluster: TinyStuffsModuleName, Regexp: `^romlex\.`},
		{BaseSize: 512, Cluster: TinyStuffsModuleName, Column: zkevm.StateManager.CodeHashConsistency.RomKeccak.Hi},
		{BaseSize: 2048, Cluster: TinyStuffsModuleName, Regexp: `^loginfo\.`},
		{BaseSize: 2048, Cluster: TinyStuffsModuleName, Regexp: `^trm\.`},
		{BaseSize: 2048, Cluster: TinyStuffsModuleName, Regexp: `^blockhash\.`},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Regexp: `^logdata\.`},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Regexp: `^rlpaddr\.`},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Regexp: `^blockdata\.`},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.TimestampFetcher.First},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.FetchedL2L1.Hi},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.FetchedRollingHash.Hi},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.FetchedRollingMsg.Hi},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.RollingHashFetcher.ExistsMsg},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.BlockTxnMetadata.BlockID},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.TxnDataFetcher.AbsTxNum},
		{BaseSize: 16384, Cluster: TinyStuffsModuleName, Column: zkevm.StateManager.StateSummary.WorldStateRoot},
		{BaseSize: 32768, Cluster: TinyStuffsModuleName, Regexp: `^rlptxrcpt\.`},
		{BaseSize: 32768, Cluster: TinyStuffsModuleName, Regexp: `^rlputils\.`},
		{BaseSize: 65536, Cluster: TinyStuffsModuleName, Regexp: `^txndata\.`},
		{BaseSize: 131072, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.RlpTxnFetcher.NBytes},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.ExecDataCollector.AbsTxID},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.ExecDataCollectorPacking.Cleaning.CleanLimb},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.ExecDataCollectorPacking.Decomposed.Filter[0]},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.ExecDataCollectorPacking.Block.Inputs.Lanes.Inputs.Spaghetti.PA.ContentSpaghetti[0]},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.ExecDataCollectorPacking.Block.Inputs.Lanes.IsFirstLaneOfNewHash},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.ExecMiMCHasher.Hash},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.ChainID},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.ChainIDNBytes},

		// ECDSA
		//
		{BaseSize: 65536, Cluster: EcdsaModuleName, Regexp: `^ext\.`},
		{BaseSize: 4096, Cluster: EcdsaModuleName, Column: zkevm.Ecdsa.Ant.AlignedGnarkData.CircuitInput},
		{BaseSize: 4096, Cluster: EcdsaModuleName, Column: zkevm.Ecdsa.Ant.Addresses.IsAddress},

		// P256
		//
		{BaseSize: 4096, Cluster: P256ModuleName, Column: zkevm.P256Verify.P256VerifyGnarkData.CircuitInput},

		// ELLIPTIC CURVES
		//
		{BaseSize: 512, Cluster: BnEcOpsModuleName, Regexp: `^blsdata\.`},
		{BaseSize: 4096, Cluster: BnEcOpsModuleName, Regexp: `^ecdata\.`},
		{BaseSize: 4096, Cluster: BnEcOpsModuleName, Column: zkevm.Ecadd.AlignedGnarkData.IsActive},
		{BaseSize: 512, Cluster: BnEcOpsModuleName, Column: zkevm.Ecmul.AlignedGnarkData.IsActive},
		{BaseSize: 1024, Cluster: BnEcOpsModuleName, Regexp: `^g1\.`},
		{BaseSize: 1024, Cluster: BnEcOpsModuleName, Regexp: `^g1_discount\.`},
		{BaseSize: 1024, Cluster: BnEcOpsModuleName, Regexp: `^g1g2\.`},
		{BaseSize: 1024, Cluster: BnEcOpsModuleName, Regexp: `^g2\.`},
		{BaseSize: 1024, Cluster: BnEcOpsModuleName, Regexp: `^g2_discount\.`},

		// ECPAIRING
		//
		{BaseSize: 1024, Cluster: BnPairingModuleName, Column: zkevm.Ecpair.IsActive},
		{BaseSize: 1024, Cluster: BnPairingModuleName, Column: zkevm.Ecpair.AlignedMillerLoopCircuit.IsActive},
		{BaseSize: 1024, Cluster: BnPairingModuleName, Column: zkevm.Ecpair.AlignedFinalExpCircuit.IsActive},

		// G2_CHECK
		//
		{BaseSize: 1024, Cluster: BnG2CheckModuleName, Column: zkevm.Ecpair.AlignedG2MembershipData.IsActive},

		// BLS_G1
		//
		{BaseSize: 4096, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Msm.UnalignedMsmData.CurrentAccumulator[0]},
		{BaseSize: 4096, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Msm.GnarkDataMsm},
		{BaseSize: 1024, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Msm.AlignedGnarkMsmData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Map.AlignedGnarkData.CircuitInput},
		{BaseSize: 4096, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Add.AlignedAddGnarkData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Add.AlignedCurveMembershipGnarkData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Msm.AlignedGnarkGroupMembershipData.CircuitInput},

		// BLS_G2
		//
		{BaseSize: 4096, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Msm.UnalignedMsmData.CurrentAccumulator[0]},
		{BaseSize: 2048, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Add.AlignedCurveMembershipGnarkData.CircuitInput},
		{BaseSize: 4096, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Msm.AlignedGnarkMsmData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Msm.AlignedGnarkGroupMembershipData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Map.AlignedGnarkData.CircuitInput},
		{BaseSize: 8192, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Add.AlignedAddGnarkData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Msm.GnarkDataMsm},

		// BLS POINT EVAL
		//
		{BaseSize: 128, Cluster: BlsKzgModuleName, Column: zkevm.PointEval.AlignedGnarkData.CircuitInput},
		{BaseSize: 128, Cluster: BlsKzgModuleName, Column: zkevm.PointEval.AlignedFailureGnarkData.CircuitInput},

		// BLS PAIR
		//
		{BaseSize: 1024, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.CsG1Membership},
		{BaseSize: 1024, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.AlignedG1MembershipGnarkData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.AlignedG2MembershipGnarkData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.AlignedMillerLoopData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.AlignedFinalExpData.CircuitInput},
		{BaseSize: 4096, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.UnalignedPairData.IsActive},
		{BaseSize: 1024, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.UnalignedPairData.GnarkDataMillerLoop},
		{BaseSize: 1024, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.UnalignedPairData.GnarkIsActiveFinalExp},

		// STATIC
		//
		{BaseSize: 16, Cluster: StaticModuleName, Regexp: `^LOOKUP_TABLE_RANGE_1_16$`},
		{BaseSize: 32, Cluster: StaticModuleName, Regexp: `^LOOKUP_TABLE_RANGE_1_30$`},
		{BaseSize: 128, Cluster: StaticModuleName, Regexp: `^LOOKUP_TABLE_RANGE_1_72$`},
		{BaseSize: 256, Cluster: StaticModuleName, Regexp: `^LOOKUP_TABLE_RANGE_1_136`},
		{BaseSize: 256, Cluster: StaticModuleName, Regexp: `^LOOKUP_TABLE_RANGE_1_144`},
		{BaseSize: 32, Cluster: StaticModuleName, Regexp: `^power\.`},
		{BaseSize: 512, Cluster: StaticModuleName, Regexp: `^instdecoder\.`},
		{BaseSize: 512, Cluster: StaticModuleName, Regexp: `^blsreftable\.`},
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_packing.LookUps.ColNumber),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_cKeccak.Pa_blockBaseConversion.Inputs.Lookup.ColUint4),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_cKeccak.Pa_blockBaseConversion.Inputs.Lookup.ColUint16),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_cKeccak.Pa_blockBaseConversion.Inputs.Lookup.ColBaseA),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_cKeccak.Pa_blockBaseConversion.Inputs.Lookup.ColBaseB),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_cKeccak.Pa_blockBaseConversion.Inputs.Lookup.ColBaseBDirty),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecdsa.Ant.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_cKeccak.KeccakF.Lookups.BaseAClean),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_cKeccak.KeccakF.Lookups.BaseADirty),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_cKeccak.KeccakF.Lookups.BaseBClean),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_cKeccak.KeccakF.Lookups.BaseBDirty),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Pa_cKeccak.KeccakF.Lookups.RC.PatternPosPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecadd.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecmul.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecpair.AlignedG2MembershipData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecpair.AlignedFinalExpCircuit.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecpair.AlignedMillerLoopCircuit.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Sha2.Pa_cSha2.GnarkCircuitConnector.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Sha2.Pa_cSha2.CanBeBlockOfInstance.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG1Add.AlignedAddGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG1Add.AlignedCurveMembershipGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG1Msm.AlignedGnarkGroupMembershipData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG1Msm.AlignedGnarkMsmData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG1Map.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG2Add.AlignedAddGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG2Add.AlignedCurveMembershipGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG2Msm.AlignedGnarkGroupMembershipData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG2Msm.AlignedGnarkMsmData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG2Map.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsPairingCheck.AlignedG2MembershipGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsPairingCheck.AlignedMillerLoopData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsPairingCheck.AlignedFinalExpData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsPairingCheck.AlignedG1MembershipGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.PointEval.AlignedFailureGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.PointEval.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.P256Verify.P256VerifyGnarkData.ActualCircuitInputMask.PatternPrecomp),
	}
}

// NewLimitlessZkEVM returns a new LimitlessZkEVM object.
func NewLimitlessZkEVM(cfg *config.Config) *LimitlessZkEVM {
	var (
		traceLimits = cfg.TracesLimits
		zkevm       = FullZKEVMWithSuite(&traceLimits, CompilationSuite{}, cfg)
		disc        = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 28,
			Predivision:  1,
			Advices:      DiscoveryAdvices(zkevm),
		}
		dw = distributed.DistributeWizard(zkevm.WizardIOP, disc)
	)

	// These are the slow and expensive operations.
	dw.CompileSegments(LimitlessCompilationParams).Conglomerate(LimitlessCompilationParams)

	return &LimitlessZkEVM{
		Zkevm:      zkevm,
		DistWizard: dw,
	}
}

// NewLimitlessRawZkEVM returns a new LimitlessZkEVM object without any
// compilation.
func NewLimitlessRawZkEVM(cfg *config.Config) *LimitlessZkEVM {

	var (
		traceLimits = cfg.TracesLimits
		zkevm       = FullZKEVMWithSuite(&traceLimits, CompilationSuite{}, cfg)
		disc        = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 29,
			Predivision:  1,
			Advices:      DiscoveryAdvices(zkevm),
		}
		dw = distributed.DistributeWizard(zkevm.WizardIOP, disc)
	)

	return &LimitlessZkEVM{
		Zkevm:      zkevm,
		DistWizard: dw,
	}
}

// NewLimitlessDebugZkEVM returns a new LimitlessZkEVM with only the debugging
// components. The resulting object is not meant to be stored on disk and should
// be used right away to debug the prover. The return object can run the
// bootstrapper (with added) sanity-checks, the segmentation and then sanity-
// checking all the segments.
func NewLimitlessDebugZkEVM(cfg *config.Config) *LimitlessZkEVM {

	var (
		traceLimits = cfg.TracesLimits
		zkevm       = FullZKEVMWithSuite(&traceLimits, CompilationSuite{}, cfg)
		disc        = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 29,
			Predivision:  1,
			Advices:      DiscoveryAdvices(zkevm),
		}
		dw             = distributed.DistributeWizard(zkevm.WizardIOP, disc)
		limitlessZkEVM = &LimitlessZkEVM{
			Zkevm:      zkevm,
			DistWizard: dw,
		}
	)

	// This adds debugging to the bootstrapper which are normally not present by
	// default.
	wizard.ContinueCompilation(
		limitlessZkEVM.DistWizard.Bootstrapper,
		dummy.CompileAtProverLvl(dummy.WithMsg("bootstrapper")),
	)

	return limitlessZkEVM
}

// GetScaledUpBootstrapper returns a bootstrapper where all the limits have
// been increased.
func GetScaledUpBootstrapper(cfg *config.Config, disc *distributed.StandardModuleDiscoverer, scalingFactor int) (*wizard.CompiledIOP, *ZkEvm) {

	traceLimits := cfg.TracesLimits
	traceLimits.ScaleUp(scalingFactor)
	zkevm := FullZKEVMWithSuite(&traceLimits, CompilationSuite{}, cfg)
	return distributed.PrecompileInitialWizard(zkevm.WizardIOP, disc), zkevm
}

// RunStatRecords runs only the bootstrapper and returns a list of stat records
func (lz *LimitlessZkEVM) RunStatRecords(cfg *config.Config, witness *Witness) []distributed.QueryBasedAssignmentStatsRecord {

	var (
		runtimeBoot = runBootstrapperWithRescaling(
			cfg,
			lz.DistWizard.Bootstrapper,
			lz.Zkevm,
			lz.DistWizard.Disc,
			witness,
			true,
		)

		res  = []distributed.QueryBasedAssignmentStatsRecord{}
		disc = lz.DistWizard.Disc
	)

	for _, mod := range disc.Modules {
		res = append(res, mod.RecordAssignmentStats(runtimeBoot)...)
	}

	return res
}

// RunDebug runs the LimitlessZkEVM on debug mode. It will run the boostrapper,
// the segmentation and then the sanity checks for all the segments. The
// check of the LPP module is done using a deterministic pseudo-random number
// generator and will yield the same result every time.
func (lz *LimitlessZkEVM) RunDebug(cfg *config.Config, witness *Witness) {

	runtimeBoot := runBootstrapperWithRescaling(
		cfg,
		lz.DistWizard.Bootstrapper,
		lz.Zkevm,
		lz.DistWizard.Disc,
		witness,
		true,
	)

	witnessGLs, witnessLPPs := distributed.SegmentRuntime(
		runtimeBoot,
		lz.DistWizard.Disc,
		lz.DistWizard.BlueprintGLs,
		lz.DistWizard.BlueprintLPPs,
		// The verification key merkle tree does not exists in debug mode. So
		// we can get the value here. It is not needed anyway.
		field.Element{},
	)

	logrus.Infof("Segmented %v GL segments and %v LPP segments", len(witnessGLs), len(witnessLPPs))

	runtimes := []*wizard.ProverRuntime{}

	for i, witness := range witnessGLs {

		logrus.Infof("Checking GL witness %v, module=%v", i, witness.ModuleName)

		var (
			debugGL        = lz.DistWizard.DebugGLs[witness.ModuleIndex]
			mainProverStep = debugGL.GetMainProverStep(witness)
			compiledIOP    = debugGL.Wiop
		)

		// The debugGLs is compiled with the CompileAtProverLevel routine so we
		// don't need the proof to complete the sanity checks: everything is
		// done at the prover level.
		rt := wizard.RunProver(compiledIOP, mainProverStep)
		runtimes = append(runtimes, rt)
	}

	// Here, we can't we can't just use 0 or a dummy small value because there
	// is a risk of creating false-positives with the grand-products and the
	// horner (as if one of the term of the product cancels, the product is
	// zero and we want to prevent that) or false negative due to inverting
	// zeroes in the log-derivative sums.
	// #nosec G404 --we don't need a cryptographic RNG for debugging purpose
	rng := rand.New(utils.NewRandSource(42))
	sharedRandomness := field.PseudoRand(rng)

	for i, witness := range witnessLPPs {

		logrus.Infof("Checking LPP witness %v, module=%v", i, witness.ModuleName)

		var (
			// moduleToFind = witness.ModuleName
			debugLPP *distributed.ModuleLPP
		)

		for range lz.DistWizard.DebugLPPs {
			panic("uncomment me")
			// if reflect.DeepEqual(lz.DistWizard.DebugLPPs[i].ModuleNames(), moduleToFind) {
			// 	debugLPP = lz.DistWizard.DebugLPPs[i]
			// 	break
			// }
		}

		if debugLPP == nil {
			utils.Panic("debugLPP not found")
		}

		witness.InitialFiatShamirState = sharedRandomness

		var (
			mainProverStep = debugLPP.GetMainProverStep(witness)
			compiledIOP    = debugLPP.Wiop
		)

		// The debugLPP is compiled with the CompileAtProverLevel routine so we
		// don't need the proof to complete the sanity checks: everything is
		// done at the prover level.
		rt := wizard.RunProver(compiledIOP, mainProverStep)

		runtimes = append(runtimes, rt)
	}
}

// runBootstrapperWithRescaling runs the bootstrapper and returns the resulting
// prover runtime.
func runBootstrapperWithRescaling(
	cfg *config.Config,
	bootstrapper *wizard.CompiledIOP,
	zkevm *ZkEvm,
	disc *distributed.StandardModuleDiscoverer,
	zkevmWitness *Witness,
	withDebug bool,
) *wizard.ProverRuntime {

	var (
		scalingFactor = 1
		runtimeBoot   *wizard.ProverRuntime
	)

	for runtimeBoot == nil {

		logrus.Infof("Trying to bootstrap with a scaling of %v\n", scalingFactor)

		func() {

			// Since the [exit] package is configured to only send panic messages
			// on overflow. The overflows are catchable.
			defer func() {
				if err := recover(); err != nil {
					oFReport, isOF := err.(exit.LimitOverflowReport)
					if isOF {
						extra := utils.DivCeil(oFReport.RequestedSize, oFReport.Limit)
						scalingFactor *= utils.NextPowerOfTwo(extra)
						return
					}

					panic(err)
				}
			}()

			if scalingFactor == 1 {
				logrus.Infof("Running bootstrapper")
				runtimeBoot = wizard.RunProver(
					bootstrapper,
					zkevm.GetMainProverStep(zkevmWitness),
				)
				return
			}

			scaledUpBootstrapper, scaledUpZkEVM := GetScaledUpBootstrapper(
				cfg, disc, scalingFactor,
			)

			if withDebug {
				// This adds debugging to the bootstrapper which are normally
				// not present by default.
				wizard.ContinueCompilation(
					scaledUpBootstrapper,
					dummy.CompileAtProverLvl(dummy.WithMsg("bootstrapper")),
				)
			}

			runtimeBoot = wizard.RunProver(
				scaledUpBootstrapper,
				scaledUpZkEVM.GetMainProverStep(zkevmWitness),
			)
		}()
	}

	return runtimeBoot
}

// Store writes the limitless prover zkevm into disk in the folder given by
// [cfg.PathforLimitlessProverAssets].
func (lz *LimitlessZkEVM) Store(cfg *config.Config) error {

	// asset is a utility struct used to list the object and the file name
	type asset struct {
		Name   string
		Object any
	}

	if cfg == nil {
		utils.Panic("config is nil")
	}

	// Create directory for assets
	assetDir := cfg.PathForSetup(executionLimitlessPath)
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", assetDir, err)
	}

	assets := []asset{
		{
			Name:   zkevmFile,
			Object: lz.Zkevm,
		},
		{
			Name:   discFile,
			Object: *lz.DistWizard.Disc,
		},
		{
			Name:   bootstrapperFile,
			Object: lz.DistWizard.Bootstrapper,
		},
		{
			Name:   conglomerationFile,
			Object: *lz.DistWizard.CompiledConglomeration,
		},
		{
			Name:   verificationKeyMerkleTreeFile,
			Object: lz.DistWizard.VerificationKeyMerkleTree,
		},
	}

	for _, modGl := range lz.DistWizard.CompiledGLs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(compileGlTemplate, modGl.ModuleGL.DefinitionInput.ModuleName),
			Object: *modGl,
		})
	}

	for i, blueprintGL := range lz.DistWizard.BlueprintGLs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(blueprintGLTemplate, i),
			Object: blueprintGL,
		})
	}

	for _, debugGL := range lz.DistWizard.DebugGLs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(debugGlTemplate, debugGL.DefinitionInput.ModuleName),
			Object: debugGL,
		})
	}

	for _, modLpp := range lz.DistWizard.CompiledLPPs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(compileLppTemplate, modLpp.ModuleLPP.ModuleName()),
			Object: *modLpp,
		})
	}

	for i, blueprintLPP := range lz.DistWizard.BlueprintLPPs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(blueprintLppTemplate, i),
			Object: blueprintLPP,
		})
	}

	for _, debugLPP := range lz.DistWizard.DebugLPPs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(debugLppTemplate, debugLPP.ModuleName()),
			Object: debugLPP,
		})
	}

	for _, asset := range assets {
		logrus.Infof("writing %s to disk", asset.Name)
		if err := serialization.StoreToDisk(assetDir+"/"+asset.Name, asset.Object, true); err != nil {
			return err
		}
	}

	logrus.Info("limitless prover assets written to disk")
	return nil
}

// LoadBootstrapperAsync loads the bootstrapper from disk.
func (lz *LimitlessZkEVM) LoadBootstrapper(cfg *config.Config) error {
	if lz.DistWizard == nil {
		lz.DistWizard = &distributed.DistributedWizard{}
	}
	return serialization.LoadFromDisk(
		cfg.PathForSetup(executionLimitlessPath)+"/"+bootstrapperFile,
		&lz.DistWizard.Bootstrapper,
		true,
	)
}

// LoadZkEVM loads the zkevm from disk
func (lz *LimitlessZkEVM) LoadZkEVM(cfg *config.Config) error {
	return serialization.LoadFromDisk(cfg.PathForSetup(executionLimitlessPath)+"/"+zkevmFile, &lz.Zkevm, true)
}

// LoadDisc loads the discoverer from disk
func (lz *LimitlessZkEVM) LoadDisc(cfg *config.Config) error {
	if lz.DistWizard == nil {
		lz.DistWizard = &distributed.DistributedWizard{}
	}

	// The discoverer is not directly deserialized as an interface object as we
	// figured that it does not work very well and the reason is unclear. This
	// conversion step is a workaround for the problem.
	res := &distributed.StandardModuleDiscoverer{}

	err := serialization.LoadFromDisk(cfg.PathForSetup(executionLimitlessPath)+"/"+discFile, res, true)
	if err != nil {
		return err
	}

	lz.DistWizard.Disc = res
	return nil
}

// LoadBlueprints loads the segmentation blueprints from disk for all the modules
// LPP and GL.
func (lz *LimitlessZkEVM) LoadBlueprints(cfg *config.Config) error {

	var (
		assetDir        = cfg.PathForSetup(executionLimitlessPath)
		cntLpps, cntGLs int
	)

	if lz.DistWizard == nil {
		lz.DistWizard = &distributed.DistributedWizard{}
	}

	files, err := os.ReadDir(assetDir)
	if err != nil {
		return fmt.Errorf("could not read directory %s: %w", assetDir, err)
	}

	for _, file := range files {

		if strings.HasPrefix(file.Name(), blueprintGLPrefix) {
			cntGLs++
		}

		if strings.HasPrefix(file.Name(), blueprintLppPrefix) {
			cntLpps++
		}
	}

	lz.DistWizard.BlueprintGLs = make([]distributed.ModuleSegmentationBlueprint, cntGLs)
	lz.DistWizard.BlueprintLPPs = make([]distributed.ModuleSegmentationBlueprint, cntLpps)

	eg := &errgroup.Group{}

	for i := 0; i < cntGLs; i++ {
		eg.Go(func() error {
			filePath := path.Join(assetDir, fmt.Sprintf(blueprintGLTemplate, i))
			if err := serialization.LoadFromDisk(filePath, &lz.DistWizard.BlueprintGLs[i], true); err != nil {
				return err
			}
			return nil
		})
	}

	for i := 0; i < cntLpps; i++ {
		eg.Go(func() error {
			filePath := path.Join(assetDir, fmt.Sprintf(blueprintLppTemplate, i))
			if err := serialization.LoadFromDisk(filePath, &lz.DistWizard.BlueprintLPPs[i], true); err != nil {
				return err
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// LoadCompiledGL loads the compiled GL from disk
func LoadCompiledGL(cfg *config.Config, moduleName distributed.ModuleName) (*distributed.RecursedSegmentCompilation, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, fmt.Sprintf(compileGlTemplate, moduleName))
		res      = &distributed.RecursedSegmentCompilation{}
	)

	if err := serialization.LoadFromDisk(filePath, res, true); err != nil {
		return nil, err
	}

	return res, nil
}

// LoadCompiledLPP loads the compiled LPP from disk
func LoadCompiledLPP(cfg *config.Config, moduleNames distributed.ModuleName) (*distributed.RecursedSegmentCompilation, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, fmt.Sprintf(compileLppTemplate, moduleNames))
		res      = &distributed.RecursedSegmentCompilation{}
	)

	if err := serialization.LoadFromDisk(filePath, res, true); err != nil {
		return nil, err
	}

	return res, nil
}

// LoadDebugGL loads the debug GL from disk
func LoadDebugGL(cfg *config.Config, moduleName distributed.ModuleName) (*distributed.ModuleGL, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, fmt.Sprintf(debugGlTemplate, moduleName))
		res      = &distributed.ModuleGL{}
	)

	if err := serialization.LoadFromDisk(filePath, res, true); err != nil {
		return nil, err
	}

	return res, nil
}

// LoadDebugLPP loads the debug LPP from disk
func LoadDebugLPP(cfg *config.Config, moduleName []distributed.ModuleName) (*distributed.ModuleLPP, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, fmt.Sprintf(debugLppTemplate, moduleName))
		res      = &distributed.ModuleLPP{}
	)

	if err := serialization.LoadFromDisk(filePath, res, true); err != nil {
		return nil, err
	}

	return res, nil
}

// LoadCompiledConglomeration loads the conglomeration assets from disk
func LoadCompiledConglomeration(cfg *config.Config) (*distributed.RecursedSegmentCompilation, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, conglomerationFile)
		conglo   = &distributed.RecursedSegmentCompilation{}
	)

	if err := serialization.LoadFromDisk(filePath, conglo, true); err != nil {
		return nil, err
	}

	return conglo, nil
}

func LoadVerificationKeyMerkleTree(cfg *config.Config) (*distributed.VerificationKeyMerkleTree, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, verificationKeyMerkleTreeFile)
		mt       = &distributed.VerificationKeyMerkleTree{}
	)

	if err := serialization.LoadFromDisk(filePath, mt, true); err != nil {
		return nil, err
	}

	return mt, nil
}

// GetAffinities returns a list of affinities for the following modules. This
// affinities regroup how the modules are grouped.
//
//	ecadd / ecmul / ecpairing
//	hub / hub.scp / hub.acp
//	everything related to keccak
func GetAffinities(z *ZkEvm) [][]column.Natural {

	return [][]column.Natural{
		{
			z.Ecmul.AlignedGnarkData.IsActive.(column.Natural),
			z.Ecadd.AlignedGnarkData.IsActive.(column.Natural),
			z.Ecpair.AlignedFinalExpCircuit.IsActive.(column.Natural),
			z.Ecpair.AlignedG2MembershipData.IsActive.(column.Natural),
			z.Ecpair.AlignedMillerLoopCircuit.IsActive.(column.Natural),
		},
		{
			z.WizardIOP.Columns.GetHandle("hub.HUB_STAMP").(column.Natural),
			z.WizardIOP.Columns.GetHandle("hub.scp_ADDRESS_HI").(column.Natural),
			z.WizardIOP.Columns.GetHandle("hub.acp_ADDRESS_HI").(column.Natural),
			z.WizardIOP.Columns.GetHandle("hub.ccp_HUB_STAMP").(column.Natural),
			z.WizardIOP.Columns.GetHandle("hub.envcp_HUB_STAMP").(column.Natural),
		},
		{
			z.WizardIOP.Columns.GetHandle("KECCAK_IMPORT_PAD_HASH_NUM").(column.Natural),
			z.WizardIOP.Columns.GetHandle("CLEANING_KECCAK_CleanLimb").(column.Natural),
			z.WizardIOP.Columns.GetHandle("DECOMPOSITION_KECCAK_Decomposed_Len_0").(column.Natural),
			z.WizardIOP.Columns.GetHandle("KECCAK_FILTERS_SPAGHETTI").(column.Natural),
			z.WizardIOP.Columns.GetHandle("LANE_KECCAK_Lane").(column.Natural),
			z.WizardIOP.Columns.GetHandle("KECCAKF_IS_ACTIVE_").(column.Natural),
			z.WizardIOP.Columns.GetHandle("KECCAKF_BLOCK_BASE_2_0").(column.Natural),
			z.WizardIOP.Columns.GetHandle("KECCAK_OVER_BLOCKS_TAGS_0").(column.Natural),
			z.WizardIOP.Columns.GetHandle("HASH_OUTPUT_Hash_Lo").(column.Natural),
		},
		{
			z.WizardIOP.Columns.GetHandle("SHA2_IMPORT_PAD_HASH_NUM").(column.Natural),
			z.WizardIOP.Columns.GetHandle("DECOMPOSITION_SHA2_Decomposed_Len_0").(column.Natural),
			z.WizardIOP.Columns.GetHandle("LENGTH_CONSISTENCY_SHA2_BYTE_LEN_0_0").(column.Natural),
			z.WizardIOP.Columns.GetHandle("SHA2_FILTERS_SPAGHETTI").(column.Natural),
			z.WizardIOP.Columns.GetHandle("LANE_SHA2_Lane").(column.Natural),
			z.WizardIOP.Columns.GetHandle("Coefficient_SHA2").(column.Natural),
			z.WizardIOP.Columns.GetHandle("SHA2_OVER_BLOCK_IS_ACTIVE").(column.Natural),
			z.WizardIOP.Columns.GetHandle("SHA2_OVER_BLOCK_SHA2_COMPRESSION_CIRCUIT_IS_ACTIVE").(column.Natural),
		},
		{
			z.WizardIOP.Columns.GetHandle("mmio.MMIO_STAMP").(column.Natural),
			z.WizardIOP.Columns.GetHandle("mmu.STAMP").(column.Natural),
		},
	}
}

var publicInputNames = []string{
	publicInput.DataNbBytes,
	publicInput.DataChecksum,
	publicInput.L2MessageHash,
	publicInput.InitialStateRootHash,
	publicInput.FinalStateRootHash,
	publicInput.InitialBlockNumber,
	publicInput.FinalBlockNumber,
	publicInput.InitialBlockTimestamp,
	publicInput.FinalBlockTimestamp,
	publicInput.FirstRollingHashUpdate_0,
	publicInput.FirstRollingHashUpdate_1,
	publicInput.LastRollingHashUpdate_0,
	publicInput.LastRollingHashUpdate_1,
	publicInput.FirstRollingHashUpdateNumber,
	publicInput.LastRollingHashNumberUpdate,
	publicInput.ChainID,
	publicInput.NBytesChainID,
	publicInput.L2MessageServiceAddrHi,
	publicInput.L2MessageServiceAddrLo,
}

// LogPublicInputs logs the list of the public inputs for the module
func LogPublicInputs(vr wizard.Runtime) {
	for _, name := range publicInputNames {
		x := vr.GetPublicInput(name)
		fmt.Printf("[public input] %s: %v\n", name, x)
	}
}
