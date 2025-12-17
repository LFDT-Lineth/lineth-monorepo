package net.consensys.zkevm.ethereum.finalization

import org.apache.tuweni.bytes.Bytes32
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface FinalizationMonitor {
  data class FinalizationUpdate(
    val blockNumber: ULong,
    val zkStateRootHash: Bytes32,
    val blockHash: Bytes32,
  )

  fun getLastFinalizationUpdate(): FinalizationUpdate

  fun addFinalizationHandler(handlerName: String, handler: FinalizationHandler)

  fun removeFinalizationHandler(handlerName: String)
}

fun interface FinalizationHandler {
  fun handleUpdate(update: FinalizationMonitor.FinalizationUpdate): SafeFuture<*>
}
