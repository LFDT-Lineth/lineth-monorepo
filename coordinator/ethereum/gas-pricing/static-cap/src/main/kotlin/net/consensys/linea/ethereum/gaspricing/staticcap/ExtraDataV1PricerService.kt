package net.consensys.linea.ethereum.gaspricing.staticcap

import io.vertx.core.Vertx
import linea.OneKWei
import linea.kotlin.toIntervalString
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import net.consensys.linea.ethereum.gaspricing.ExtraDataUpdater
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.MinerExtraDataCalculator
import net.consensys.linea.ethereum.gaspricing.MinerExtraDataV1
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration

class ExtraDataV1PricerService(
  pollingInterval: Duration,
  vertx: Vertx,
  private val feesFetcher: FeesFetcher,
  private val minerExtraDataCalculator: MinerExtraDataCalculator,
  private val extraDataUpdater: ExtraDataUpdater,
  private val metricsFacade: MetricsFacade,
  private val log: Logger = LogManager.getLogger(ExtraDataV1PricerService::class.java),
) : VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = pollingInterval.inWholeMilliseconds,
  log = log,
  name = "ExtraDataV1PricerService",
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {
  private var lastExtraData: AtomicReference<MinerExtraDataV1?> = AtomicReference(null)

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.L2_PRICING,
      name = "variablecost",
      description = "VariableCost in wei from the miner extra data",
      measurementSupplier = { lastExtraData.get()?.variableCostInKWei?.toLong()?.times(OneKWei) ?: 0 },
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.L2_PRICING,
      name = "ethgasprice",
      description = "EthGasPrice in wei from the miner extra data",
      measurementSupplier = { lastExtraData.get()?.ethGasPriceInKWei?.toLong()?.times(OneKWei) ?: 0 },
    )
  }

  override fun action(): SafeFuture<Unit> {
    return feesFetcher
      .getL1EthGasPriceData()
      .thenCompose { feeHistory ->
        val blockRange = feeHistory.blocksRange()
        val newExtraData = minerExtraDataCalculator.calculateMinerExtraData(feeHistory)
        if (lastExtraData.get() != newExtraData) {
          // this is just to avoid log noise.
          lastExtraData.set(newExtraData)
          log.info(
            "L2 extra data update: extraData={} l1Blocks={}",
            newExtraData,
            blockRange.toIntervalString(),
          )
        }
        // even if extraData value is old, still call sequencer node, they may be restarted
        // and need extraData to be set
        extraDataUpdater.updateMinerExtraData(newExtraData)
      }
      .thenPeek { log.trace("Fetch, calculate, update new miner extra data are all done.") }
  }

  override fun handleError(error: Throwable) {
    log.error("Error with dynamic gas price service: errorMessage={}", error.message, error)
  }
}
