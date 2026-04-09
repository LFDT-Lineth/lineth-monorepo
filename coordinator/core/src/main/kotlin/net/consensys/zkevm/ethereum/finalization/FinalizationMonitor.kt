package net.consensys.zkevm.ethereum.finalization

import org.apache.tuweni.bytes.Bytes32
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface FinalizationMonitor {
  data class FinalizationUpdate(
    val blockNumber: ULong,
    val blockHash: Bytes32,
    val forcedTransactionNumber: ULong? = null, // null means ftx number is not available as contract is still before V8
  )

  fun getLastFinalizationUpdate(): FinalizationUpdate

  fun addFinalizationHandler(handlerName: String, handler: FinalizationHandler)

  fun removeFinalizationHandler(handlerName: String)
}

fun interface FinalizationHandler {
  fun handleUpdate(update: FinalizationMonitor.FinalizationUpdate): SafeFuture<*>
}
