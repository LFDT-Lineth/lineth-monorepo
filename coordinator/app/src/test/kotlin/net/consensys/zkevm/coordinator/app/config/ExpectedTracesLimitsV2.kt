package net.consensys.zkevm.coordinator.app.config

import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.TracingModuleV2

val expectedTracesLimitsV2 = TracesCountersV2(
  mapOf(
    TracingModuleV2.ADD to 1u,
    TracingModuleV2.BIN to 2u,
    TracingModuleV2.BLAKE_MODEXP_DATA to 3u,
    TracingModuleV2.BLOCK_DATA to 4u,
    TracingModuleV2.BLOCK_HASH to 5u,
    TracingModuleV2.EC_DATA to 6u,
    TracingModuleV2.EUC to 7u,
    TracingModuleV2.EXP to 8u,
    TracingModuleV2.EXT to 9u,
    TracingModuleV2.GAS to 10u,
    TracingModuleV2.HUB to 11u,
    TracingModuleV2.LOG_DATA to 12u,
    TracingModuleV2.LOG_INFO to 13u,
    TracingModuleV2.MMIO to 14u,
    TracingModuleV2.MMU to 15u,
    TracingModuleV2.MOD to 16u,
    TracingModuleV2.MUL to 18u,
    TracingModuleV2.MXP to 19u,
    TracingModuleV2.OOB to 20u,
    TracingModuleV2.RLP_ADDR to 21u,
    TracingModuleV2.RLP_TXN to 22u,
    TracingModuleV2.RLP_TXN_RCPT to 23u,
    TracingModuleV2.ROM to 24u,
    TracingModuleV2.ROM_LEX to 25u,
    TracingModuleV2.SHAKIRA_DATA to 26u,
    TracingModuleV2.SHF to 27u,
    TracingModuleV2.STP to 28u,
    TracingModuleV2.TRM to 29u,
    TracingModuleV2.TXN_DATA to 30u,
    TracingModuleV2.WCP to 31u,
    // Reference table limits, set to UInt.MAX_VALUE
    TracingModuleV2.BIN_REFERENCE_TABLE to 32u,
    TracingModuleV2.INSTRUCTION_DECODER to 33u,
    TracingModuleV2.SHF_REFERENCE_TABLE to 34u,
    // Precompiles limits
    TracingModuleV2.PRECOMPILE_BLAKE_EFFECTIVE_CALLS to 35u,
    TracingModuleV2.PRECOMPILE_BLAKE_ROUNDS to 36u,
    TracingModuleV2.PRECOMPILE_ECADD_EFFECTIVE_CALLS to 37u,
    TracingModuleV2.PRECOMPILE_ECMUL_EFFECTIVE_CALLS to 38u,
    TracingModuleV2.PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS to 39u,
    TracingModuleV2.PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS to 40u,
    TracingModuleV2.PRECOMPILE_ECPAIRING_MILLER_LOOPS to 41u,
    TracingModuleV2.PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS to 42u,
    TracingModuleV2.PRECOMPILE_MODEXP_EFFECTIVE_CALLS to 43u,
    TracingModuleV2.PRECOMPILE_RIPEMD_BLOCKS to 44u,
    TracingModuleV2.PRECOMPILE_SHA2_BLOCKS to 45u,
    // Block limits
    TracingModuleV2.BLOCK_KECCAK to 46u,
    TracingModuleV2.BLOCK_L1_SIZE to 47u,
    TracingModuleV2.BLOCK_L2_L1_LOGS to 48u,
    TracingModuleV2.BLOCK_TRANSACTIONS to 49u
  )
)
