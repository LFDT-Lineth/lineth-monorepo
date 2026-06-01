package linea.coordinator.clients.executionwitness

import linea.domain.BlockParameter
import linea.kotlin.encodeHex

internal fun BlockParameter.toDebugExecutionWitnessRpcParam(): String =
  when (this) {
    is BlockParameter.Tag -> getTag()
    is BlockParameter.BlockNumber -> getNumber().toString()
    is BlockParameter.BlockHash -> getHash().encodeHex(prefix = true)
  }
