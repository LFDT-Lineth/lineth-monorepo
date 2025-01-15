package net.consensys.zkevm.coordinator.app.config

import net.consensys.linea.traces.TracesCountersV1
import net.consensys.linea.traces.TracingModuleV1

val expectedTracesCountersV1 = TracesCountersV1(
  mapOf(
    // EVM Arithmetization Limits
    TracingModuleV1.ADD to 1U,
    TracingModuleV1.BIN to 2U,
    TracingModuleV1.BIN_RT to 3U,
    TracingModuleV1.EC_DATA to 4U,
    TracingModuleV1.EXT to 5U,
    TracingModuleV1.HUB to 6U,
    TracingModuleV1.INSTRUCTION_DECODER to 7U,
    TracingModuleV1.MMIO to 8U,
    TracingModuleV1.MMU to 9U,
    TracingModuleV1.MMU_ID to 10U,
    TracingModuleV1.MOD to 11U,
    TracingModuleV1.MUL to 12U,
    TracingModuleV1.MXP to 13U,
    TracingModuleV1.PHONEY_RLP to 14U,
    TracingModuleV1.PUB_HASH to 15U,
    TracingModuleV1.PUB_HASH_INFO to 16U,
    TracingModuleV1.PUB_LOG to 17U,
    TracingModuleV1.PUB_LOG_INFO to 18U,
    TracingModuleV1.RLP to 19U,
    TracingModuleV1.ROM to 20U,
    TracingModuleV1.SHF to 21U,
    TracingModuleV1.SHF_RT to 22U,
    TracingModuleV1.TX_RLP to 23U,
    TracingModuleV1.WCP to 24U,
    // Block Limits
    TracingModuleV1.BLOCK_TX to 25U,
    TracingModuleV1.BLOCK_L2L1LOGS to 26U,
    TracingModuleV1.BLOCK_KECCAK to 27U,
    // Precompile Limits
    TracingModuleV1.PRECOMPILE_ECRECOVER to 28U,
    TracingModuleV1.PRECOMPILE_SHA2 to 29U,
    TracingModuleV1.PRECOMPILE_RIPEMD to 30U,
    TracingModuleV1.PRECOMPILE_IDENTITY to 31U,
    TracingModuleV1.PRECOMPILE_MODEXP to 32U,
    TracingModuleV1.PRECOMPILE_ECADD to 32U,
    TracingModuleV1.PRECOMPILE_ECMUL to 34U,
    TracingModuleV1.PRECOMPILE_ECPAIRING to 35U,
    TracingModuleV1.PRECOMPILE_BLAKE2F to 36U
  )
)
