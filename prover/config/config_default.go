package config

import "github.com/spf13/viper"

var (
	DefaultDeferToOtherLargeCodes     = []int{137}     // List of exit codes for which the job will put back the job to be reexecuted in large mode.
	DefaultRetryLocallyWithLargeCodes = []int{77, 333} // List of exit codes for which the job will retry in large mode
)

func setDefaultValues() {
	setDefaultTracesLimit()
	setDefaultPaths()

	viper.SetDefault("debug.profiling", false)
	viper.SetDefault("debug.tracing", false)

	viper.SetDefault("controller.enable_execution", true)
	viper.SetDefault("controller.enable_blob_decompression", true)
	viper.SetDefault("controller.enable_aggregation", true)

	// Set the default values for the retry delays
	viper.SetDefault("controller.retry_delays", []int{0, 1, 2, 3, 5, 8, 13, 21, 44, 85})
	viper.SetDefault("controller.defer_to_other_large_codes", DefaultDeferToOtherLargeCodes)
	viper.SetDefault("controller.retry_locally_with_large_codes", DefaultRetryLocallyWithLargeCodes)

	// Set default for cmdTmpl and cmdLargeTmpl
	// TODO @gbotrel binary to run prover is hardcoded here.
	viper.SetDefault("controller.worker_cmd_tmpl", "prover prove --config {{.ConfFile}} --in {{.InFile}} --out {{.OutFile}}")
	viper.SetDefault("controller.worker_cmd_large_tmpl", "prover prove --config {{.ConfFile}} --in {{.InFile}} --out {{.OutFile}} --large")

	viper.SetDefault("execution.ignore_compatibility_check", false)

}

func setDefaultPaths() {
	viper.SetDefault("execution.conflated_traces_dir", "/shared/traces/conflated")
	viper.SetDefault("execution.requests_root_dir", "/shared/prover-execution")
	viper.SetDefault("blob_decompression.requests_root_dir", "/shared/prover-compression")
	viper.SetDefault("aggregation.requests_root_dir", "/shared/prover-aggregation")
}

func setDefaultTracesLimit() {

	// Arithmetization modules
	viper.SetDefault("traces_limits.ADD", 524288)
	viper.SetDefault("traces_limits.BIN", 262144)
	viper.SetDefault("traces_limits.BLAKE_MODEXP_DATA", 16384)
	viper.SetDefault("traces_limits.BLOCK_DATA", 1024)
	viper.SetDefault("traces_limits.BLOCK_HASH", 512)
	viper.SetDefault("traces_limits.EC_DATA", 262144)
	viper.SetDefault("traces_limits.EUC", 65536)
	viper.SetDefault("traces_limits.EXP", 8192)
	viper.SetDefault("traces_limits.EXT", 1048576)
	viper.SetDefault("traces_limits.GAS", 65536)
	viper.SetDefault("traces_limits.HUB", 2097152)
	viper.SetDefault("traces_limits.LOG_DATA", 65536)
	viper.SetDefault("traces_limits.LOG_INFO", 4096)
	viper.SetDefault("traces_limits.MMIO", 4194304)
	viper.SetDefault("traces_limits.MMU", 4194304)
	viper.SetDefault("traces_limits.MOD", 131072)
	viper.SetDefault("traces_limits.MUL", 65536)
	viper.SetDefault("traces_limits.MXP", 524288)
	viper.SetDefault("traces_limits.OOB", 262144)
	viper.SetDefault("traces_limits.RLP_ADDR", 4096)
	viper.SetDefault("traces_limits.RLP_TXN", 131072)
	viper.SetDefault("traces_limits.RLP_TXN_RCPT", 65536)
	viper.SetDefault("traces_limits.ROM", 4194304)
	viper.SetDefault("traces_limits.ROM_LEX", 1024)
	viper.SetDefault("traces_limits.SHAKIRA_DATA", 32768)
	viper.SetDefault("traces_limits.SHF", 65536)
	viper.SetDefault("traces_limits.STP", 16384)
	viper.SetDefault("traces_limits.TRM", 32768)
	viper.SetDefault("traces_limits.TXN_DATA", 8192)
	viper.SetDefault("traces_limits.WCP", 262144)

	// Precompile limits
	viper.SetDefault("traces_limits.PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS", 128)
	viper.SetDefault("traces_limits.PRECOMPILE_SHA2_BLOCKS", 671)
	viper.SetDefault("traces_limits.PRECOMPILE_RIPEMD_BLOCKS", 671)
	viper.SetDefault("traces_limits.PRECOMPILE_MODEXP_EFFECTIVE_CALLS", 4)
	viper.SetDefault("traces_limits.PRECOMPILE_ECADD_EFFECTIVE_CALLS", 16384)
	viper.SetDefault("traces_limits.PRECOMPILE_ECMUL_EFFECTIVE_CALLS", 32)
	viper.SetDefault("traces_limits.PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS", 16)
	viper.SetDefault("traces_limits.PRECOMPILE_ECPAIRING_MILLER_LOOPS", 64)
	viper.SetDefault("traces_limits.PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS", 64)
	viper.SetDefault("traces_limits.PRECOMPILE_BLAKE_EFFECTIVE_CALLS", 600)
	viper.SetDefault("traces_limits.PRECOMPILE_BLAKE_ROUNDS", 600)

	// Block limits
	viper.SetDefault("traces_limits.BLOCK_KECCAK", 8192)
	viper.SetDefault("traces_limits.BLOCK_L1_SIZE", 1000000)
	viper.SetDefault("traces_limits.BLOCK_L2_L1_LOGS", 16)
	viper.SetDefault("traces_limits.BLOCK_TRANSACTIONS", 200)

	// Reference tables
	viper.SetDefault("traces_limits.BIN_REFERENCE_TABLE", 262144)
	viper.SetDefault("traces_limits.SHF_REFERENCE_TABLE", 4096)
	viper.SetDefault("traces_limits.INSTRUCTION_DECODER", 512)

	// Shomei limits
	viper.SetDefault("traces_limits.SHOMEI_MERKLE_PROOFS", 16384)

	// Large Limits

	// Arithmetization modules
	viper.SetDefault("traces_limits_large.ADD", 1048576)
	viper.SetDefault("traces_limits_large.BIN", 524288)
	viper.SetDefault("traces_limits_large.BLAKE_MODEXP_DATA", 32768)
	viper.SetDefault("traces_limits_large.BLOCK_DATA", 2048)
	viper.SetDefault("traces_limits_large.BLOCK_HASH", 1024)
	viper.SetDefault("traces_limits_large.EC_DATA", 524288)
	viper.SetDefault("traces_limits_large.EUC", 131072)
	viper.SetDefault("traces_limits_large.EXP", 16384)
	viper.SetDefault("traces_limits_large.EXT", 2097152)
	viper.SetDefault("traces_limits_large.GAS", 131072)
	viper.SetDefault("traces_limits_large.HUB", 4194304)
	viper.SetDefault("traces_limits_large.LOG_DATA", 131072)
	viper.SetDefault("traces_limits_large.LOG_INFO", 8192)
	viper.SetDefault("traces_limits_large.MMIO", 8388608)
	viper.SetDefault("traces_limits_large.MMU", 8388608)
	viper.SetDefault("traces_limits_large.MOD", 262144)
	viper.SetDefault("traces_limits_large.MUL", 131072)
	viper.SetDefault("traces_limits_large.MXP", 1048576)
	viper.SetDefault("traces_limits_large.OOB", 524288)
	viper.SetDefault("traces_limits_large.RLP_ADDR", 8192)
	viper.SetDefault("traces_limits_large.RLP_TXN", 262144)
	viper.SetDefault("traces_limits_large.RLP_TXN_RCPT", 131072)
	viper.SetDefault("traces_limits_large.ROM", 8388608)
	viper.SetDefault("traces_limits_large.ROM_LEX", 2048)
	viper.SetDefault("traces_limits_large.SHAKIRA_DATA", 65536)
	viper.SetDefault("traces_limits_large.SHF", 131072)
	viper.SetDefault("traces_limits_large.STP", 32768)
	viper.SetDefault("traces_limits_large.TRM", 65536)
	viper.SetDefault("traces_limits_large.TXN_DATA", 16384)
	viper.SetDefault("traces_limits_large.WCP", 524288)

	// Precompile limits
	viper.SetDefault("traces_limits_large.PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS", 256)
	viper.SetDefault("traces_limits_large.PRECOMPILE_SHA2_BLOCKS", 671)
	viper.SetDefault("traces_limits_large.PRECOMPILE_RIPEMD_BLOCKS", 671)
	viper.SetDefault("traces_limits_large.PRECOMPILE_MODEXP_EFFECTIVE_CALLS", 8)
	viper.SetDefault("traces_limits_large.PRECOMPILE_ECADD_EFFECTIVE_CALLS", 32768)
	viper.SetDefault("traces_limits_large.PRECOMPILE_ECMUL_EFFECTIVE_CALLS", 64)
	viper.SetDefault("traces_limits_large.PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS", 32)
	viper.SetDefault("traces_limits_large.PRECOMPILE_ECPAIRING_MILLER_LOOPS", 128)
	viper.SetDefault("traces_limits_large.PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS", 128)
	viper.SetDefault("traces_limits_large.PRECOMPILE_BLAKE_EFFECTIVE_CALLS", 600)
	viper.SetDefault("traces_limits_large.PRECOMPILE_BLAKE_ROUNDS", 600)

	// Block limits
	viper.SetDefault("traces_limits_large.BLOCK_KECCAK", 8192)
	viper.SetDefault("traces_limits_large.BLOCK_L1_SIZE", 1000000)
	viper.SetDefault("traces_limits_large.BLOCK_L2_L1_LOGS", 16)
	viper.SetDefault("traces_limits_large.BLOCK_TRANSACTIONS", 200)

	// Reference tables
	viper.SetDefault("traces_limits_large.BIN_REFERENCE_TABLE", 262144)
	viper.SetDefault("traces_limits_large.SHF_REFERENCE_TABLE", 4096)
	viper.SetDefault("traces_limits_large.INSTRUCTION_DECODER", 512)

	// Shomei limits
	viper.SetDefault("traces_limits_large.SHOMEI_MERKLE_PROOFS", 32768)

}
