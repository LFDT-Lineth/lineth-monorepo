package net.consensys.zkevm.ethereum.finalization

import io.vertx.core.Vertx
import linea.contract.l1.FinalizedStateDataProvider
import linea.domain.BlockParameter
import linea.ethapi.EthApiBlockClient
import linea.finalization.FinalizationHandler
import linea.finalization.FinalizationMonitor
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes32
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Collections
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

class FinalizationMonitorImpl(
  private val config: Config,
  private val finalizedStateDataProvider: FinalizedStateDataProvider,
  private val l2EthApiClient: EthApiBlockClient,
  private val vertx: Vertx,
  private val log: Logger = LogManager.getLogger(FinalizationMonitor::class.java),
) : FinalizationMonitor, VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log,
  name = "FinalizationMonitor",
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {
  data class Config(
    val pollingInterval: Duration = 500.milliseconds,
    val l1QueryBlockTag: BlockParameter.Tag = BlockParameter.Tag.FINALIZED,
  )

  private val finalizationHandlers:
    MutableMap<String, FinalizationHandler> =
    Collections.synchronizedMap(LinkedHashMap())
  private val lastFinalizationUpdate = AtomicReference<FinalizationMonitor.FinalizationUpdate>(null)

  override fun handleError(error: Throwable) {
    log.error("Error with finalization monitor: errorMessage={}", error.message, error)
  }

  override fun start(): SafeFuture<Unit> {
    return getFinalizationUpdate(config.l1QueryBlockTag).thenApply {
      lastFinalizationUpdate.set(it)
      super.start()
    }
  }

  override fun action(): SafeFuture<Unit> {
    log.trace("Checking finalization updates")
    return getFinalizationUpdate(config.l1QueryBlockTag).thenCompose { currentState ->
      if (lastFinalizationUpdate.get() != currentState) {
        log.info(
          "finalization update: previousFinalizedBlock={} newFinalizedBlock={}",
          lastFinalizationUpdate.get().blockNumber,
          currentState,
        )
        lastFinalizationUpdate.set(currentState)
        onUpdate(currentState)
      } else {
        SafeFuture.completedFuture(Unit)
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
          finalizationUpdate.blockNumber,
        )
        try {
          finalizationHandler.handleUpdate(finalizationUpdate)
            .thenApply { }
        } catch (th: Throwable) {
          log.error(
            "Finalization handler={} failed. errorMessage={}",
            handlerName,
            th.message,
            th,
          )
          SafeFuture.completedFuture(Unit)
        }
      }
    }.thenApply {}
  }

  private fun getFinalizationUpdate(
    blockParameter: BlockParameter,
  ): SafeFuture<FinalizationMonitor.FinalizationUpdate> {
    return finalizedStateDataProvider.getFinalizedStateData(blockParameter)
      .thenCompose { finalizedStateData ->
        l2EthApiClient
          .ethGetBlockByNumberTxHashes(BlockParameter.fromNumber(finalizedStateData.blockNumber))
          .thenApply { finalizedBlock ->
            FinalizationMonitor.FinalizationUpdate(
              blockNumber = finalizedStateData.blockNumber,
              blockHash = Bytes32.wrap(finalizedBlock.hash),
              forcedTransactionNumber = finalizedStateData.forcedTransactionNumber,
            )
          }
      }
  }

  override fun getLastFinalizationUpdate(): FinalizationMonitor.FinalizationUpdate {
    return lastFinalizationUpdate.get()
  }

  override fun addFinalizationHandler(handlerName: String, handler: FinalizationHandler) {
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
