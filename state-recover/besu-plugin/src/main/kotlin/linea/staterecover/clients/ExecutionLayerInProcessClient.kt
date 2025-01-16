package linea.staterecover.clients

import linea.staterecover.BlockFromL1RecoveredData
import linea.staterecover.ExecutionLayerClient
import linea.staterecover.RecoveryStatusPersistence
import linea.staterecover.StateRecoveryStatus
import linea.staterecover.plugin.BlockImporter
import linea.staterecover.plugin.RecoveryModeManager
import net.consensys.linea.BlockNumberAndHash
import net.consensys.linea.BlockParameter
import org.apache.logging.log4j.LogManager
import org.hyperledger.besu.plugin.data.BlockHeader
import org.hyperledger.besu.plugin.services.BlockSimulationService
import org.hyperledger.besu.plugin.services.BlockchainService
import org.hyperledger.besu.plugin.services.sync.SynchronizationService
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.jvm.optionals.getOrNull

class ExecutionLayerInProcessClient(
  private val blockchainService: BlockchainService,
  private val stateRecoveryModeManager: RecoveryModeManager,
  private val stateRecoveryStatusPersistence: RecoveryStatusPersistence,
  private val blockImporter: BlockImporter
) : ExecutionLayerClient {
  companion object {
    fun create(
      blockchainService: BlockchainService,
      simulatorService: BlockSimulationService,
      synchronizationService: SynchronizationService,
      stateRecoveryModeManager: RecoveryModeManager,
      stateRecoveryStatusPersistence: RecoveryStatusPersistence
    ): ExecutionLayerInProcessClient {
      return ExecutionLayerInProcessClient(
        blockchainService = blockchainService,
        stateRecoveryModeManager = stateRecoveryModeManager,
        stateRecoveryStatusPersistence = stateRecoveryStatusPersistence,
        blockImporter = BlockImporter(
          blockchainService = blockchainService,
          simulatorService = simulatorService,
          synchronizationService = synchronizationService
        )
      )
    }
  }

  private val log = LogManager.getLogger(ExecutionLayerInProcessClient::class.java)

  override fun getBlockNumberAndHash(blockParameter: BlockParameter): SafeFuture<BlockNumberAndHash> {
    val blockHeader: BlockHeader? = when (blockParameter) {
      is BlockParameter.Tag -> when {
        blockParameter == BlockParameter.Tag.LATEST -> blockchainService.chainHeadHeader
        else -> throw IllegalArgumentException("Unsupported block parameter: $blockParameter")
      }

      is BlockParameter.BlockNumber ->
        blockchainService
          .getBlockByNumber(blockParameter.getNumber().toLong())
          .map { it.blockHeader }
          .getOrNull()
    }

    return blockHeader
      ?.let {
        SafeFuture.completedFuture(
          BlockNumberAndHash(
            it.number.toULong(),
            it.blockHash.toArray()
          )
        )
      }
      ?: SafeFuture.failedFuture(IllegalArgumentException("Block not found for parameter: $blockParameter"))
  }

  override fun lineaEngineImportBlocksFromBlob(blocks: List<BlockFromL1RecoveredData>): SafeFuture<Unit> {
    logBlockImport(blocks)
    return kotlin.runCatching {
      blocks.map { blockImporter.importBlock(it) }
      SafeFuture.completedFuture(Unit)
    }.getOrElse { th -> SafeFuture.failedFuture(th) }
  }

  override fun lineaGetStateRecoveryStatus(): SafeFuture<StateRecoveryStatus> {
    return SafeFuture
      .completedFuture(
        StateRecoveryStatus(
          headBlockNumber = stateRecoveryModeManager.headBlockNumber,
          stateRecoverStartBlockNumber = stateRecoveryModeManager.targetBlockNumber
        )
      )
  }

  override fun lineaEnableStateRecovery(stateRecoverStartBlockNumber: ULong): SafeFuture<StateRecoveryStatus> {
    stateRecoveryModeManager.setTargetBlockNumber(stateRecoverStartBlockNumber)

    return SafeFuture.completedFuture(
      StateRecoveryStatus(
        headBlockNumber = stateRecoveryModeManager.headBlockNumber,
        stateRecoverStartBlockNumber = stateRecoveryStatusPersistence.getRecoveryStartBlockNumber()
      )
    )
  }

  private fun logBlockImport(blocks: List<BlockFromL1RecoveredData>) {
    if (log.isTraceEnabled) {
      log.trace("importing blocks from blob: blocks={}", blocks)
    } else {
      log.debug("importing blocks from blob: blocks={}", blocks.map { it.header.blockNumber })
    }
  }
}
