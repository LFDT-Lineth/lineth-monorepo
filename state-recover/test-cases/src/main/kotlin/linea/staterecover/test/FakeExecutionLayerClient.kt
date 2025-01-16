package linea.staterecover.test

import linea.staterecover.BlockFromL1RecoveredData
import linea.staterecover.ExecutionLayerClient
import linea.staterecover.StateRecoveryStatus
import net.consensys.linea.BlockNumberAndHash
import net.consensys.linea.BlockParameter
import net.consensys.linea.CommonDomainFunctions
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture

class FakeExecutionLayerClient(
  headBlock: BlockNumberAndHash = BlockNumberAndHash(number = 0uL, hash = ByteArray(32) { 0 }),
  initialStateRecoverStartBlockNumber: ULong? = null,
  loggerName: String? = null
) : ExecutionLayerClient {
  private val log = loggerName
    ?.let { LogManager.getLogger(loggerName) }
    ?: LogManager.getLogger(FakeExecutionLayerClient::class.java)

  @get:Synchronized @set:Synchronized
  var lastImportedBlock: BlockNumberAndHash = headBlock

  @get:Synchronized @set:Synchronized
  var stateRecoverStartBlockNumber = initialStateRecoverStartBlockNumber

  @get:Synchronized
  val stateRecoverStatus: StateRecoveryStatus
    get() = StateRecoveryStatus(
      headBlockNumber = lastImportedBlock.number,
      stateRecoverStartBlockNumber = stateRecoverStartBlockNumber
    )

  @Synchronized
  override fun lineaEngineImportBlocksFromBlob(
    blocks: List<BlockFromL1RecoveredData>
  ): SafeFuture<Unit> {
    if (log.isTraceEnabled) {
      log.trace("lineaEngineImportBlocksFromBlob($blocks)")
    } else {
      val interval = CommonDomainFunctions.blockIntervalString(
        blocks.first().header.blockNumber,
        blocks.last().header.blockNumber
      )
      log.debug("lineaEngineImportBlocksFromBlob(interval=$interval)")
    }
    lastImportedBlock = blocks.last().let { BlockNumberAndHash(it.header.blockNumber, it.header.blockHash) }
    return SafeFuture.completedFuture(Unit)
  }

  @Synchronized
  override fun getBlockNumberAndHash(
    blockParameter: BlockParameter
  ): SafeFuture<BlockNumberAndHash> {
    log.debug("getBlockNumberAndHash($blockParameter)")
    return SafeFuture.completedFuture(lastImportedBlock)
  }

  @Synchronized
  override fun lineaGetStateRecoveryStatus(): SafeFuture<StateRecoveryStatus> {
    log.debug("lineaGetStateRecoveryStatus()= $stateRecoverStatus")
    return SafeFuture.completedFuture(stateRecoverStatus)
  }

  @Synchronized
  override fun lineaEnableStateRecovery(
    stateRecoverStartBlockNumber: ULong
  ): SafeFuture<StateRecoveryStatus> {
    this.stateRecoverStartBlockNumber = stateRecoverStartBlockNumber
    log.debug("lineaEnableStateRecovery($stateRecoverStartBlockNumber) = $stateRecoverStatus")
    return SafeFuture.completedFuture(stateRecoverStatus)
  }
}
