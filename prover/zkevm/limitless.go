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

// DiscoveryAdvices is a list of advice for the discovery of the modules. These
// values have been obtained thanks to a statistical analysis of the traces
// assignments involving correlation of the modules and hierarchical clustering.
// The advices are optimized to minimize the number of segments generated when
// producing an EVM proof.
var DiscoveryAdvices = []distributed.ModuleDiscoveryAdvice{

	// ARITH-OPS
	//
	{BaseSize: 8192, Cluster: "ARITH-OPS", Column: "ACCUMULATOR_COUNTER"},
	{BaseSize: 16384, Cluster: "ARITH-OPS", Column: "exp.INST"},
	//
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_sar256.n,bit_sar256.res'0,bit_sar256.res'1,bit_sar256.word'0,bit_sar256.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_sar256_u1.n,bit_sar256_u1.res'0,bit_sar256_u1.res'1,bit_sar256_u1.word'0,bit_sar256_u1.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_sar256_u2.n,bit_sar256_u2.res'0,bit_sar256_u2.res'1,bit_sar256_u2.word'0,bit_sar256_u2.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_sar256_u3.n,bit_sar256_u3.res'0,bit_sar256_u3.res'1,bit_sar256_u3.word'0,bit_sar256_u3.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_sar256_u4.n,bit_sar256_u4.res'0,bit_sar256_u4.res'1,bit_sar256_u4.word'0,bit_sar256_u4.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_sar256_u5.n,bit_sar256_u5.res'0,bit_sar256_u5.res'1,bit_sar256_u5.word'0,bit_sar256_u5.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_sar256_u6.n,bit_sar256_u6.res'0,bit_sar256_u6.res'1,bit_sar256_u6.word'0,bit_sar256_u6.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_sar256_u7.n,bit_sar256_u7.res'0,bit_sar256_u7.res'1,bit_sar256_u7.word'0,bit_sar256_u7.word'1_0_LOGDERIVATIVE_M"},
	//
	// ARITH-OPS: bit 256 main tables
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shl256.n,bit_shl256.res'0,bit_shl256.res'1,bit_shl256.word'0,bit_shl256.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shr256.n,bit_shr256.res'0,bit_shr256.res'1,bit_shr256.word'0,bit_shr256.word'1_0_LOGDERIVATIVE_M"},
	//
	// ARITH-OPS: bit 256 u1..u7 stages (shl/shr/sar)
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shl256_u1.n,bit_shl256_u1.res'0,bit_shl256_u1.res'1,bit_shl256_u1.word'0,bit_shl256_u1.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shl256_u2.n,bit_shl256_u2.res'0,bit_shl256_u2.res'1,bit_shl256_u2.word'0,bit_shl256_u2.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shl256_u3.n,bit_shl256_u3.res'0,bit_shl256_u3.res'1,bit_shl256_u3.word'0,bit_shl256_u3.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shl256_u4.n,bit_shl256_u4.res'0,bit_shl256_u4.res'1,bit_shl256_u4.word'0,bit_shl256_u4.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shl256_u5.n,bit_shl256_u5.res'0,bit_shl256_u5.res'1,bit_shl256_u5.word'0,bit_shl256_u5.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shl256_u6.n,bit_shl256_u6.res'0,bit_shl256_u6.res'1,bit_shl256_u6.word'0,bit_shl256_u6.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shl256_u7.n,bit_shl256_u7.res'0,bit_shl256_u7.res'1,bit_shl256_u7.word'0,bit_shl256_u7.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shr256_u1.n,bit_shr256_u1.res'0,bit_shr256_u1.res'1,bit_shr256_u1.word'0,bit_shr256_u1.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shr256_u2.n,bit_shr256_u2.res'0,bit_shr256_u2.res'1,bit_shr256_u2.word'0,bit_shr256_u2.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shr256_u3.n,bit_shr256_u3.res'0,bit_shr256_u3.res'1,bit_shr256_u3.word'0,bit_shr256_u3.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shr256_u4.n,bit_shr256_u4.res'0,bit_shr256_u4.res'1,bit_shr256_u4.word'0,bit_shr256_u4.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shr256_u5.n,bit_shr256_u5.res'0,bit_shr256_u5.res'1,bit_shr256_u5.word'0,bit_shr256_u5.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shr256_u6.n,bit_shr256_u6.res'0,bit_shr256_u6.res'1,bit_shr256_u6.word'0,bit_shr256_u6.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_shr256_u7.n,bit_shr256_u7.res'0,bit_shr256_u7.res'1,bit_shr256_u7.word'0,bit_shr256_u7.word'1_0_LOGDERIVATIVE_M"},
	//
	// ARITH-OPS: fill bytes
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_fill_bytes_between.end,fill_bytes_between.res'0,fill_bytes_between.res'1,fill_bytes_between.start,fill_bytes_between.value,fill_bytes_between.word'0,fill_bytes_between.word'1_0_LOGDERIVATIVE_M"},
	//
	// ARITH-OPS: uint columns
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u20.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u23.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u24.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u26.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u27.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u28.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u29.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u30.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u31.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u32.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u36.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u47.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u48.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u55.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u56.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u58.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u59.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u60.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u61.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u62.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u63.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u64.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u95.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u96.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u111.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u112.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u119.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u120.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u123.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u124.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u125.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u126.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u127.V"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "u128.V"},
	//
	// ARITH-OPS: log-256
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log256.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log256_u16.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log256_u32.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log256_u64.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log256_u128.hi"},
	//
	// ARITH-OPS: log-2
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log2.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log2_u2.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log2_u4.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log2_u8.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log2_u16.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log2_u32.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log2_u64.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "log2_u128.hi"},
	//
	// ARITH-OPS: set-byte
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "set_byte16.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "set_byte32.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "set_byte64.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "set_byte128.hi"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "set_byte256.hi"},
	//
	// ARITH-OPS: actual ops
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_mul.ARG_1_HI,mul.ARG_1_LO,mul.ARG_2_HI,mul.ARG_2_LO,mul.INSTRUCTION,mul.RES_HI,mul.RES_LO_0_LOGDERIVATIVE_M"},
	{BaseSize: 65536, Cluster: "ARITH-OPS", Column: "TABLE_add.ARG_1'0,add.ARG_1'1,add.ARG_2'0,add.ARG_2'1,add.INST,add.RES'0,add.RES'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 65536, Cluster: "ARITH-OPS", Column: "TABLE_mod.ARG_1_HI,mod.ARG_1_LO,mod.ARG_2_HI,mod.ARG_2_LO,mod.INST,mod.RES_HI,mod.RES_LO_0_LOGDERIVATIVE_M"},
	{BaseSize: 65536, Cluster: "ARITH-OPS", Column: "TABLE_min256_64.L_gas_diff,min256_64.gas'0,min256_64.gas'1,min256_64.res_0_LOGDERIVATIVE_M"},
	{BaseSize: 131072, Cluster: "ARITH-OPS", Column: "shf.ARG_1'0"},
	//
	// ARITH-OPS: MIMC
	{BaseSize: 1048576, Cluster: "ARITH-OPS", Column: "MIMC_COMPILER"},
	//
	// ARITH-OPS: OSAKA
	{BaseSize: 131072, Cluster: "ARITH-OPS", Column: "TABLE_bin.ARGUMENT_1'0,bin.ARGUMENT_1'1,bin.ARGUMENT_2'0,bin.ARGUMENT_2'1,bin.INST,bin.RES'0,bin.RES'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "bit_sar256_u1.lsw"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "bit_shr256_u1.lsw"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_xoan_u2.ARG_1,bit_xoan_u2.ARG_2,bit_xoan_u2.INST,bit_xoan_u2.RES_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "bit_xoan_u2.c0"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_xoan_u4.$ret,bit_xoan_u4.ARG_1,bit_xoan_u4.ARG_2,bit_xoan_u4.INST,bit_xoan_u4.RES_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_xoan_u8.$ret,bit_xoan_u8.ARG_1,bit_xoan_u8.ARG_2,bit_xoan_u8.INST,bit_xoan_u8.RES_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_xoan_u16.$ret,bit_xoan_u16.ARG_1,bit_xoan_u16.ARG_2,bit_xoan_u16.INST,bit_xoan_u16.RES_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_xoan_u32.$ret,bit_xoan_u32.ARG_1,bit_xoan_u32.ARG_2,bit_xoan_u32.INST,bit_xoan_u32.RES_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_xoan_u64.$ret,bit_xoan_u64.ARG_1,bit_xoan_u64.ARG_2,bit_xoan_u64.INST,bit_xoan_u64.RES_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_xoan_u128.$ret,bit_xoan_u128.ARG_1,bit_xoan_u128.ARG_2,bit_xoan_u128.INST,bit_xoan_u128.RES_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_bit_xoan_u256.$ret,bit_xoan_u256.ARG_1'0,bit_xoan_u256.ARG_1'1,bit_xoan_u256.ARG_2'0,bit_xoan_u256.ARG_2'1,bit_xoan_u256.INST,bit_xoan_u256.RES'0,bit_xoan_u256.RES'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_byte16.n,byte16.res,byte16.word_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_byte32.n,byte32.res,byte32.word_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_byte64.n,byte64.res,byte64.word_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_byte128.n,byte128.res,byte128.word_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_byte256.n,byte256.res,byte256.word'0,byte256.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_signextend.res'0,signextend.res'1,signextend.size,signextend.word'0,signextend.word'1_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_max3_u128.arg1,max3_u128.arg2,max3_u128.arg3,max3_u128.res_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "ARITH-OPS", Column: "TABLE_maxlog.inst,maxlog.res,maxlog.x,maxlog.y,maxlog.z_0_LOGDERIVATIVE_M"},
	{BaseSize: 262144, Cluster: "ARITH-OPS", Column: "TABLE_wcp.ARG_1'0,wcp.ARG_1'1,wcp.ARG_2'0,wcp.ARG_2'1,wcp.INST,wcp.RES_0_LOGDERIVATIVE_M"},

	// HUB-KECCAK
	//
	{BaseSize: 16384, Cluster: "HUB-KECCAK", Column: "GENERIC_ACCUMULATOR_Hash_Hi"},
	{BaseSize: 32768, Cluster: "HUB-KECCAK", Column: "rlptxn.txnVALUE"},
	{BaseSize: 32768, Cluster: "HUB-KECCAK", Column: "KECCAK_OVER_BLOCKS_TAGS_9"},
	{BaseSize: 32768, Cluster: "HUB-KECCAK", Column: "KECCAKF_OUTPUT_MODULE_HashOutPut_SlicesBaseB_3_9"},
	{BaseSize: 65536, Cluster: "HUB-KECCAK", Column: "TABLE_gas.GAS_ACTUAL,gas.GAS_COST,gas.OOGX,gas.XAHOY_0_LOGDERIVATIVE_M"},
	{BaseSize: 65536, Cluster: "HUB-KECCAK", Column: "TABLE_gas_out_of_pocket.gas_actual,gas_out_of_pocket.gas_upfront,gas_out_of_pocket.oogx,gas_out_of_pocket.oop_0_LOGDERIVATIVE_M"},
	{BaseSize: 65536, Cluster: "HUB-KECCAK", Column: "TABLE_shakiradata.(shift shakiradata:LIMB -1),shakiradata.ID,shakiradata.INDEX,shakiradata.LIMB,shakiradata.PHASE_0_LOGDERIVATIVE_M"},
	{BaseSize: 65536, Cluster: "HUB-KECCAK", Column: "stp.INST"},
	{BaseSize: 65536, Cluster: "HUB-KECCAK", Column: "CLEANING_KECCAK_CleanLimb"},
	{BaseSize: 65536, Cluster: "HUB-KECCAK", Column: "TABLE_call_gas_extra.exists,call_gas_extra.gas_extra,call_gas_extra.inst,call_gas_extra.stipend,call_gas_extra.value'0,call_gas_extra.value'1,call_gas_extra.warm_0_LOGDERIVATIVE_M"},
	{BaseSize: 131072, Cluster: "HUB-KECCAK", Column: "TABLE_mxp.CN,mxp.MACRO,mxp.MXP_STAMP,mxp.computationARG_1_HI_xor_macroOFFSET_1_HI,mxp.computationARG_1_LO_xor_macroOFFSET_1_LO,mxp.computationARG_2_HI_xor_macroOFFSET_2_HI,mxp.computationARG_2_LO_xor_macroOFFSET_2_LO,mxp.computationEUC_FLAG_xor_decoderIS_BYTE_PRICING_xor_macroDEPLOYING_xor_scenarioMSIZE,mxp.computationEXO_INST_xor_decoderG_BYTE_xor_macroINST,mxp.computationRES_A_xor_macroGAS_MXP_xor_scenarioC_MEM,mxp.computationWCP_FLAG_xor_decoderIS_DOUBLE_MAX_OFFSET_xor_macroMXPX_xor_scenarioMXPX,mxp.decoderIS_FIXED_SIZE_1_xor_macroS1NZNOMXPX_xor_scenarioSTATE_UPDATE_BYTE_PRICING,mxp.decoderIS_FIXED_SIZE_32_xor_macroS2NZNOMXPX_xor_scenarioSTATE_UPDATE_WORD_PRICING,mxp.macroRES,mxp.macroSIZE_1_HI,mxp.macroSIZE_1_LO,mxp.macroSIZE_2_HI,mxp.macroSIZE_2_LO_0_LOGDERIVATIVE_M"},
	{BaseSize: 131072, Cluster: "HUB-KECCAK", Column: "oob.WCP_FLAG"},
	{BaseSize: 131072, Cluster: "HUB-KECCAK", Column: "GENERIC_ACCUMULATOR_IsActive"},
	{BaseSize: 262144, Cluster: "HUB-KECCAK", Column: "MIMC_CODE_HASH_CODE_SIZE"},
	{BaseSize: 262144, Cluster: "HUB-KECCAK", Column: "LANE_KECCAK_Lane"},
	{BaseSize: 262144, Cluster: "HUB-KECCAK", Column: "hub.transactionSYST_TXN_DATA_1"},
	{BaseSize: 262144, Cluster: "HUB-KECCAK", Column: "mmio.VAL_C_NEW"},
	{BaseSize: 262144, Cluster: "HUB-KECCAK", Column: "mmu.prprcWCP_RES"},
	{BaseSize: 524288, Cluster: "HUB-KECCAK", Column: "rom.IS_PUSH"},
	{BaseSize: 524288, Cluster: "HUB-KECCAK", Column: "KECCAK_TAGS_SPAGHETTI"},
	{BaseSize: 1048576, Cluster: "HUB-KECCAK", Column: "hub×4.stkcp_VALUE_LO_1234"},
	{BaseSize: 1048576, Cluster: "HUB-KECCAK", Column: "mmio×3.VAL_ABC_SORTED"},
	{BaseSize: 65536, Cluster: "HUB-KECCAK", Column: "TABLE_euc.DIVIDEND,euc.DIVISOR,euc.QUOTIENT,euc.REMAINDER_0_LOGDERIVATIVE_M"},

	// MODEXP 256
	//
	{BaseSize: 65536, Cluster: "MODEXP_256", Column: "blake2fmodexpdata.STAMP"},
	{BaseSize: 8192, Cluster: "MODEXP_256", Column: "MODEXP_IS_ACTIVE"},
	{BaseSize: 512, Cluster: "MODEXP_256", Column: "MODEXP_256_BITS"},

	// MODEXP 8192
	//
	{BaseSize: 256, Cluster: "MODEXP_LARGE", Column: "MODEXP_LARGE"},

	// SHA2
	//
	{BaseSize: 512, Cluster: "SHA2", Column: "SHA2_OVER_BLOCK_SHA2_COMPRESSION_CIRCUIT"},
	{BaseSize: 16384, Cluster: "SHA2", Column: "CLEANING_SHA2_CleanLimb"},
	{BaseSize: 16384, Cluster: "SHA2", Column: "SHA2_TAGS_SPAGHETTI"},
	{BaseSize: 16384, Cluster: "SHA2", Column: "BLOCK_SHA2_AccNumLane"},
	{BaseSize: 16384, Cluster: "SHA2", Column: "SHA2_OVER_BLOCK_HASH_HI"},

	// TINY-STUFFS
	//
	{BaseSize: 512, Cluster: "TINY-STUFFS", Column: "romlex.ADDRESS_HI"},
	{BaseSize: 512, Cluster: "TINY-STUFFS", Column: "STATE_SUMMARY_CODEHASHCONSISTENCY_CODEHASH_CONSISTENCY_ROM_KECCAK_HI"},
	{BaseSize: 2048, Cluster: "TINY-STUFFS", Column: "loginfo.TXN_EMITS_LOGS"},
	{BaseSize: 2048, Cluster: "TINY-STUFFS", Column: "trm.tmp"},
	{BaseSize: 2048, Cluster: "TINY-STUFFS", Column: "blockhash.IOMF"},
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "logdata.ABS_LOG_NUM"},
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "rlpaddr.ADDR_HI"},
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "blockdata.COINBASE_HI"},
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "PUBLIC_INPUT_TIMESTAMP_FETCHER_DATA"},
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "PUBLIC_INPUT_L2L1LOGS_EXTRACTED_HI"},
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "PUBLIC_INPUT_ROLLING_MSG_EXTRACTED_HI"},
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "PUBLIC_INPUT_ROLLING_HASH_EXTRACTED_HI"},
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "PUBLIC_INPUT_ROLLING_SEL_EXISTS_MSG"},
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "BLOCK_TX_METADATA_BLOCK_ID"},
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "PUBLIC_INPUT_TXN_DATA_FETCHER_ABS_TX_NUM"},
	{BaseSize: 16384, Cluster: "TINY-STUFFS", Column: "STATE_SUMMARY_WORLD_STATE_ROOT"},
	{BaseSize: 32768, Cluster: "TINY-STUFFS", Column: "TABLE_rlptxrcpt.ABS_LOG_NUM,rlptxrcpt.ABS_LOG_NUM_MAX,rlptxrcpt.ABS_TX_NUM,rlptxrcpt.ABS_TX_NUM_MAX,rlptxrcpt.INPUT_1,rlptxrcpt.INPUT_2,rlptxrcpt.PHASE_ID_0_LOGDERIVATIVE_M"},
	{BaseSize: 32768, Cluster: "TINY-STUFFS", Column: "TABLE_rlputils.MACRO,rlputils.comptACC_xor_macroDATA_1,rlputils.comptARG_1_HI_xor_macroDATA_2,rlputils.comptARG_1_LO_xor_macroDATA_6,rlputils.comptARG_2_LO_xor_macroDATA_7,rlputils.comptINST_xor_macroDATA_8,rlputils.comptRES_xor_macroDATA_3,rlputils.comptSHF_ARG_xor_macroINST,rlputils.comptSHF_FLAG_xor_macroDATA_4,rlputils.macroDATA_5_0_LOGDERIVATIVE_M"},
	{BaseSize: 65536, Cluster: "TINY-STUFFS", Column: "txndata.rlpTX_TYPE"},
	{BaseSize: 131072, Cluster: "TINY-STUFFS", Column: "PUBLIC_INPUT_RLP_TXN_FETCHER_NBYTES"},
	{BaseSize: 262144, Cluster: "TINY-STUFFS", Column: "EXECUTION_DATA_COLLECTOR_ABS_TX_ID"},
	{BaseSize: 262144, Cluster: "TINY-STUFFS", Column: "CLEANING_EXECUTION_DATA_MIMC_CleanLimb"},
	{BaseSize: 262144, Cluster: "TINY-STUFFS", Column: "EXECUTION_DATA_MIMC_TAGS_SPAGHETTI"},
	{BaseSize: 262144, Cluster: "TINY-STUFFS", Column: "MIMC_HASHER_STATE"},
	{BaseSize: 262144, Cluster: "TINY-STUFFS", Column: "BLOCK_EXECUTION_DATA_MIMC_AccNumLane"},

	// ECDSA
	//
	{BaseSize: 65536, Cluster: "ECDSA", Column: "TABLE_ext.ARG_1_HI,ext.ARG_1_LO,ext.ARG_2_HI,ext.ARG_2_LO,ext.ARG_3_HI,ext.ARG_3_LO,ext.INST,ext.RES_HI,ext.RES_LO_0_LOGDERIVATIVE_M"},
	{BaseSize: 4096, Cluster: "ECDSA", Column: "ECDSA_ANTICHAMBER_ADDRESSES_ADDRESS_HI"},
	{BaseSize: 4096, Cluster: "ECDSA", Column: "ECDSA_ANTICHAMBER_GNARK_DATA"},

	// P256
	//
	{BaseSize: 4096, Cluster: "P256", Column: "P256_VERIFY_ALIGNMENT"},

	// ELLIPTIC CURVES
	//
	{BaseSize: 512, Cluster: "ELLIPTIC_CURVES", Column: "TABLE_blsdata.ID,blsdata.INDEX,blsdata.LIMB,blsdata.PHASE,blsdata.SUCCESS_BIT,blsdata.TOTAL_SIZE_0_LOGDERIVATIVE_M"},
	{BaseSize: 4096, Cluster: "ELLIPTIC_CURVES", Column: "TABLE_ecdata.ID,ecdata.INDEX,ecdata.LIMB,ecdata.PHASE,ecdata.SUCCESS_BIT,ecdata.TOTAL_SIZE_0_LOGDERIVATIVE_M"},
	{BaseSize: 2048, Cluster: "ELLIPTIC_CURVES", Column: "ECADD_INTEGRATION_ALIGNMENT"},
	{BaseSize: 256, Cluster: "ELLIPTIC_CURVES", Column: "ECMUL_INTEGRATION_ALIGNMENT"},

	// ECPAIRING
	//
	{BaseSize: 256, Cluster: "ECPAIRING", Column: "ECPAIR_IS_ACTIVE"},
	{BaseSize: 256, Cluster: "ECPAIRING", Column: "ECPAIR_ALIGNMENT_ML"},
	{BaseSize: 256, Cluster: "ECPAIRING", Column: "ECPAIR_ALIGNMENT_FINALEXP"},

	// G2_CHECK
	//
	{BaseSize: 1024, Cluster: "G2_CHECK", Column: "ECPAIR_ALIGNMENT_G2"},

	// BLS_G1
	//
	{BaseSize: 4096, Cluster: "BLS_G1", Column: "UNALIGNED_G1_BLS_MSM_CURRENT_ACCUMULATOR_0"},
	{BaseSize: 4096, Cluster: "BLS_G1", Column: "UNALIGNED_G1_BLS_MSM_GNARK_DATA_MSM"},
	{BaseSize: 256, Cluster: "BLS_G1", Column: "BLS_MSM_G1_MSM"},
	{BaseSize: 256, Cluster: "BLS_G1", Column: "BLS_MAP_G1_ALIGNMENT"},
	{BaseSize: 512, Cluster: "BLS_G1", Column: "BLS_ADD_G1_ALIGNMENT"},
	{BaseSize: 128, Cluster: "BLS_G1", Column: "BLS_ADD_C1_CURVE_MEMBERSHIP_ALIGNMENT"},
	{BaseSize: 64, Cluster: "BLS_G1", Column: "BLS_MSM_G1_GROUP_MEMBERSHIP"},
	{BaseSize: 64, Cluster: "BLS_G1", Column: "BLS_PAIR_G1_MEMBERSHIP"},

	// BLS_G2
	//
	{BaseSize: 4096, Cluster: "BLS_G2", Column: "UNALIGNED_G2_BLS_MSM_CURRENT_ACCUMULATOR_0"},
	{BaseSize: 256, Cluster: "BLS_G2", Column: "BLS_ADD_C2_CURVE_MEMBERSHIP_ALIGNMENT"},
	{BaseSize: 4096, Cluster: "BLS_G2", Column: "UNALIGNED_G2_BLS_MSM_GNARK_DATA_MSM"},
	{BaseSize: 256, Cluster: "BLS_G2", Column: "BLS_MSM_G2_GROUP_MEMBERSHIP"},
	{BaseSize: 128, Cluster: "BLS_G2", Column: "BLS_MAP_G2_ALIGNMENT"},
	{BaseSize: 1024, Cluster: "BLS_G2", Column: "BLS_PAIR_ML"},
	{BaseSize: 512, Cluster: "BLS_G2", Column: "BLS_PAIR_FE"},
	{BaseSize: 1024, Cluster: "BLS_G2", Column: "BLS_ADD_G2_ALIGNMENT"},
	{BaseSize: 512, Cluster: "BLS_G2", Column: "BLS_MSM_G2_MSM"},
	{BaseSize: 256, Cluster: "BLS_G2", Column: "BLS_PAIR_G2_MEMBERSHIP"},

	// BLS POINT EVAL
	{BaseSize: 32, Cluster: "BLS_KZG", Column: "BLS_POINTEVAL"},
	{BaseSize: 32, Cluster: "BLS_KZG", Column: "BLS_POINTEVAL_FAILURE"},

	// BLS PAIR
	{BaseSize: 4096, Cluster: "BLS_PAIR", Column: "UNALIGNED_BLS_PAIR_IS_ACTIVE"},
	{BaseSize: 1024, Cluster: "BLS_PAIR", Column: "UNALIGNED_BLS_PAIR_GNARK_DATA_ML"},
	{BaseSize: 1024, Cluster: "BLS_PAIR", Column: "UNALIGNED_BLS_PAIR_GNARK_DATA_FE"},

	// STATIC
	//
	{BaseSize: 16, Cluster: "STATIC", Column: "LOOKUP_TABLE_RANGE_1_16"},
	{BaseSize: 32, Cluster: "STATIC", Column: "TABLE_power.EXPONENT,power.IOMF,power.POWER_0_LOGDERIVATIVE_M"},
	{BaseSize: 32, Cluster: "STATIC", Column: "LookUp_Num"},
	{BaseSize: 32, Cluster: "STATIC", Column: "LOOKUP_TABLE_RANGE_1_30"},
	{BaseSize: 256, Cluster: "STATIC", Column: "LOOKUP_TABLE_RANGE_1_136"},
	{BaseSize: 256, Cluster: "STATIC", Column: "LOOKUP_TABLE_RANGE_1_144"},
	{BaseSize: 512, Cluster: "STATIC", Column: "instdecoder.TWO_LINE_INSTRUCTION"},
	{BaseSize: 512, Cluster: "STATIC", Column: "blsreftable.DISCOUNT"},
	{BaseSize: 16384, Cluster: "STATIC", Column: "LOOKUP_BaseBDirty"},
	{BaseSize: 16384, Cluster: "STATIC", Column: "KECCAKF_BASE1_CLEAN_"},
	{BaseSize: 32768, Cluster: "STATIC", Column: "KECCAKF_BASE1_DIRTY_"},
	{BaseSize: 65536, Cluster: "STATIC", Column: "LOOKUP_BaseA"},
	//
	{BaseSize: 64, Column: "REPEATED_PATTERN_REPEATED_PATTERN_ECDSA_ANTICHAMBER_GNARK_DATA", Cluster: "STATIC"},
	{BaseSize: 32, Column: "REPEATED_PATTERN_KECCAK_RC_PATTERN", Cluster: "STATIC"},
	{BaseSize: 128, Column: "REPEATED_PATTERN_REPEATED_PATTERN_MODEXP_256_BITS", Cluster: "STATIC"},
	{BaseSize: 256, Column: "REPEATED_PATTERN_REPEATED_PATTERN_MODEXP_LARGE", Cluster: "STATIC"},
	{BaseSize: 512, Column: "REPEATED_PATTERN_REPEATED_PATTERN_ECADD_INTEGRATION_ALIGNMENT", Cluster: "STATIC"},
	{BaseSize: 64, Column: "REPEATED_PATTERN_REPEATED_PATTERN_ECMUL_INTEGRATION_ALIGNMENT", Cluster: "STATIC"},
	{BaseSize: 16, Column: "REPEATED_PATTERN_REPEATED_PATTERN_ECPAIR_ALIGNMENT_G2", Cluster: "STATIC"},
	{BaseSize: 64, Column: "REPEATED_PATTERN_REPEATED_PATTERN_ECPAIR_ALIGNMENT_ML", Cluster: "STATIC"},
	{BaseSize: 64, Column: "REPEATED_PATTERN_REPEATED_PATTERN_ECPAIR_ALIGNMENT_FINALEXP", Cluster: "STATIC"},
	{BaseSize: 32, Column: "REPEATED_PATTERN_SHA2_BLOCK_OF_INSTANCE_SELECTION", Cluster: "STATIC"},
	{BaseSize: 64, Column: "REPEATED_PATTERN_REPEATED_PATTERN_SHA2_OVER_BLOCK_SHA2_COMPRESSION_CIRCUIT", Cluster: "STATIC"},
	{BaseSize: 512, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_ADD_G1_ALIGNMENT", Cluster: "STATIC"},
	{BaseSize: 128, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_ADD_C1_CURVE_MEMBERSHIP_ALIGNMENT", Cluster: "STATIC"},
	{BaseSize: 256, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_MSM_G1_MSM", Cluster: "STATIC"},
	{BaseSize: 64, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_MSM_G1_GROUP_MEMBERSHIP", Cluster: "STATIC"},
	{BaseSize: 256, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_MAP_G1_ALIGNMENT", Cluster: "STATIC"},
	{BaseSize: 1024, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_ADD_G2_ALIGNMENT", Cluster: "STATIC"},
	{BaseSize: 256, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_ADD_C2_CURVE_MEMBERSHIP_ALIGNMENT", Cluster: "STATIC"},
	{BaseSize: 512, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_MSM_G2_MSM", Cluster: "STATIC"},
	{BaseSize: 128, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_MSM_G2_GROUP_MEMBERSHIP", Cluster: "STATIC"},
	{BaseSize: 128, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_MAP_G2_ALIGNMENT", Cluster: "STATIC"},
	{BaseSize: 512, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_PAIR_ML", Cluster: "STATIC"},
	{BaseSize: 512, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_PAIR_FE", Cluster: "STATIC"},
	{BaseSize: 64, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_PAIR_G1_MEMBERSHIP", Cluster: "STATIC"},
	{BaseSize: 256, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_PAIR_G2_MEMBERSHIP", Cluster: "STATIC"},
	{BaseSize: 32, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_POINTEVAL", Cluster: "STATIC"},
	{BaseSize: 32, Column: "REPEATED_PATTERN_REPEATED_PATTERN_BLS_POINTEVAL_FAILURE", Cluster: "STATIC"},
	{BaseSize: 128, Column: "REPEATED_PATTERN_REPEATED_PATTERN_P256_VERIFY_ALIGNMENT", Cluster: "STATIC"},

	// TINY-STUFFS
	//
	{BaseSize: 4096, Cluster: "TINY-STUFFS", Column: "PUBLIC_INPUT_CHAIN_ID_FETCHER_N_BYTES_CHAIN_ID"},

	// End of new discovery advices for Osaka
}

// NewLimitlessZkEVM returns a new LimitlessZkEVM object.
func NewLimitlessZkEVM(cfg *config.Config) *LimitlessZkEVM {
	var (
		traceLimits = cfg.TracesLimits
		zkevm       = FullZKEVMWithSuite(&traceLimits, CompilationSuite{}, cfg)
		disc        = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 28,
			Predivision:  1,
			Advices:      DiscoveryAdvices,
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
			Advices:      DiscoveryAdvices,
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
			Advices:      DiscoveryAdvices,
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
