package net.consensys.zkevm.ethereum.finalization

import io.vertx.core.Vertx
import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.domain.BlockParameter
import linea.kotlin.toBigInteger
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes32
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Collections
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

class FinalizationMonitorImpl(
  private val config: Config,
  private val contract: LineaRollupSmartContractClientReadOnly,
  private val l2Client: Web3j,
  private val vertx: Vertx,
  private val log: Logger = LogManager.getLogger(FinalizationMonitor::class.java)
) : FinalizationMonitor, PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log
) {
  data class Config(
    val pollingInterval: Duration = 500.milliseconds,
    val l1QueryBlockTag: BlockParameter.Tag = BlockParameter.Tag.FINALIZED
  )

  private val finalizationHandlers:
    MutableMap<String, FinalizationHandler> =
      Collections.synchronizedMap(LinkedHashMap())
  private val lastFinalizationUpdate = AtomicReference<FinalizationMonitor.FinalizationUpdate>(null)

  override fun handleError(error: Throwable) {
    log.error("Error with finalization monitor: errorMessage={}", error.message, error)
  }

  override fun start(): SafeFuture<Unit> {
    return getFinalizationState().thenApply {
      lastFinalizationUpdate.set(it)
      super.start()
    }
  }

  override fun action(): SafeFuture<Unit> {
    log.debug("Checking finalization updates")
    return getFinalizationState().thenCompose { currentState ->
      if (lastFinalizationUpdate.get() != currentState) {
        log.info(
          "finalization update: previousFinalizedBlock={} newFinalizedBlock={}",
          lastFinalizationUpdate.get().blockNumber,
          currentState
        )
        lastFinalizationUpdate.set(currentState)
        onUpdate(currentState)
      } else {
        SafeFuture.completedFuture(Unit)
      }
    }
  }

  private fun getFinalizationState(): SafeFuture<FinalizationMonitor.FinalizationUpdate> {
    return contract
      .finalizedL2BlockNumber(blockParameter = config.l1QueryBlockTag)
      .thenCompose { lineaFinalizedBlockNumber ->
        l2Client
          .ethGetBlockByNumber(DefaultBlockParameter.valueOf(lineaFinalizedBlockNumber.toBigInteger()), false)
          .sendAsync()
          .thenCombine(
            contract.blockStateRootHash(
              blockParameter = config.l1QueryBlockTag,
              lineaL2BlockNumber = lineaFinalizedBlockNumber
            )
          ) { finalizedBlock, stateRootHash ->
            FinalizationMonitor.FinalizationUpdate(
              lineaFinalizedBlockNumber,
              Bytes32.wrap(stateRootHash),
              Bytes32.fromHexString(finalizedBlock.block.hash)
            )
          }
      }
  }

  private fun onUpdate(finalizationUpdate: FinalizationMonitor.FinalizationUpdate): SafeFuture<Unit> {
    return finalizationHandlers.entries.fold(SafeFuture.completedFuture(Unit)) { agg, entry ->
      val handlerName = entry.key
      val finalizationHandler = entry.value
      agg.thenCompose {
        log.trace(
          "calling finalization handler: handler={} update={}",
          handlerName,
          finalizationUpdate.blockNumber
        )
        try {
          finalizationHandler.handleUpdate(finalizationUpdate)
            .thenApply { }
        } catch (th: Throwable) {
          log.error("Finalization handler={} failed. errorMessage={}", handlerName, th.message, th)
          SafeFuture.completedFuture(Unit)
        }
      }
    }.thenApply {}
  }

  override fun getLastFinalizationUpdate(): FinalizationMonitor.FinalizationUpdate {
    return lastFinalizationUpdate.get()
  }

  override fun addFinalizationHandler(
    handlerName: String,
    handler: FinalizationHandler
  ) {
    synchronized(finalizationHandlers) {
      finalizationHandlers[handlerName] = handler
    }
  }

  override fun removeFinalizationHandler(handlerName: String) {
    synchronized(finalizationHandlers) {
      finalizationHandlers.remove(handlerName)
    }
  }
}
