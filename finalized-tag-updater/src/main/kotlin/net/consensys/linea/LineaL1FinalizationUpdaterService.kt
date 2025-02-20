package net.consensys.linea

import build.linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly
import io.vertx.core.Vertx
import linea.consensus.EngineBlockTagUpdater
import linea.domain.BlockParameter
import net.consensys.linea.web3j.okHttpClientBuilder
import net.consensys.zkevm.LongRunningService
import net.consensys.zkevm.ethereum.finalization.FinalizationUpdatePoller
import net.consensys.zkevm.ethereum.finalization.FinalizationUpdatePollerConfig
import org.apache.logging.log4j.LogManager
import org.hyperledger.besu.plugin.services.BlockchainService
import org.slf4j.Logger
import org.slf4j.LoggerFactory
import org.web3j.protocol.Web3j
import org.web3j.protocol.http.HttpService
import java.util.concurrent.CompletableFuture

class LineaBesuEngineBlockTagUpdater(private val blockchainService: BlockchainService) :
  EngineBlockTagUpdater {
  private val log: Logger = LoggerFactory.getLogger(this::class.java)
  private fun setFinalizedAndSafeBlock(finalizedBlockNumber: Long): Boolean {
    // lookup finalized block by number in local chain
    val finalizedBlock = blockchainService.getBlockByNumber(finalizedBlockNumber)
    if (!finalizedBlock.isEmpty) {
      try {
        val blockHash = finalizedBlock.get().blockHeader.blockHash
        log.info(
          "Linea safe/finalized block update: blockNumber={} blockHash={}",
          finalizedBlockNumber,
          blockHash
        )
        blockchainService.setSafeBlock(blockHash)
        blockchainService.setFinalizedBlock(blockHash)
        return true
      } catch (e: IllegalArgumentException) {
        log.error("Linea safe/finalized block=$finalizedBlockNumber not found in the local chain", e)
        throw e
      } catch (e: UnsupportedOperationException) {
        log.error(
          "Linea safe/finalized block update failure: Method not supported or not enabled for PoS network: " +
            "setFinalizedBlock and setSafeBlock",
          e
        )
        throw e
      } catch (e: Exception) {
        log.error("Linea safe/finalized block update failure: Failed to set finalized block=$finalizedBlockNumber", e)
        throw e
      }
    } else {
      log.warn("Linea safe/finalized block update: Skipped as getBlockByNumber returns empty result")
    }
    return false
  }

  override fun lineaUpdateFinalizedBlockV1(
    finalizedBlockNumber: Long
  ) {
    val updateSuccess = setFinalizedAndSafeBlock(finalizedBlockNumber)
    log.debug("Linea safe/finalized block update: blockNumber={} success={}", finalizedBlockNumber, updateSuccess)
  }
}

class LineaL1FinalizationUpdater(
  private val engineBlockTagUpdater: EngineBlockTagUpdater
) {
  fun handleL1Finalization(
    finalizedBlockNumber: ULong
  ): CompletableFuture<Unit> {
    runCatching {
      engineBlockTagUpdater
        .lineaUpdateFinalizedBlockV1(finalizedBlockNumber.toLong())
    }.onFailure { e ->
      return CompletableFuture.failedFuture(e)
    }
    return CompletableFuture.completedFuture(Unit)
  }
}

class LineaL1FinalizationUpdaterService(
  vertx: Vertx,
  config: PluginConfig,
  engineBlockTagUpdater: EngineBlockTagUpdater
) : LongRunningService {
  private val web3j = Web3j.build(
    HttpService(
      config.l1RpcEndpoint.toString(),
      okHttpClientBuilder(LogManager.getLogger("clients.l1")).build()
    )
  )
  private val lineaRollup = Web3JLineaRollupSmartContractClientReadOnly(
    contractAddress = config.l1SmartContractAddress.toHexString(),
    web3j = web3j
  )
  private val updater = LineaL1FinalizationUpdater(engineBlockTagUpdater)
  private val poller = FinalizationUpdatePoller(
    vertx,
    FinalizationUpdatePollerConfig(
      pollingInterval = config.l1PollingInterval,
      blockTag = BlockParameter.Tag.FINALIZED
    ),
    lineaRollup,
    updater::handleL1Finalization,
    LogManager.getLogger(FinalizationUpdatePoller::class.java)
  )

  override fun start(): CompletableFuture<Unit> {
    return poller.start()
  }

  override fun stop(): CompletableFuture<Unit> {
    return poller.stop()
  }
}
