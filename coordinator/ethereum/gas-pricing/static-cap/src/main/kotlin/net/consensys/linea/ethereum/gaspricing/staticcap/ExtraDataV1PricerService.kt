package net.consensys.linea.ethereum.gaspricing.staticcap

import io.vertx.core.Vertx
import linea.kotlin.toIntervalString
import net.consensys.linea.ethereum.gaspricing.ExtraDataUpdater
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.MinerExtraDataCalculator
import net.consensys.linea.ethereum.gaspricing.MinerExtraDataV1
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

class ExtraDataV1PricerService(
  pollingInterval: Duration,
  vertx: Vertx,
  private val feesFetcher: FeesFetcher,
  private val minerExtraDataCalculator: MinerExtraDataCalculator,
  private val extraDataUpdater: ExtraDataUpdater,
  private val log: Logger = LogManager.getLogger(ExtraDataV1PricerService::class.java),
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = pollingInterval.inWholeMilliseconds,
  log = log,
) {
  private var lastExtraData: MinerExtraDataV1? = null

  override fun action(): SafeFuture<Unit> {
    return feesFetcher
      .getL1EthGasPriceData()
      .thenCompose { feeHistory ->
        val blockRange = feeHistory.blocksRange()
        val newExtraData = minerExtraDataCalculator.calculateMinerExtraData(feeHistory)
        if (lastExtraData != newExtraData) {
          // this is just to avoid log noise.
          lastExtraData = newExtraData
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
      .thenApply { log.debug("Fetch, calculate, update new miner extra data are all done.") }
  }

  override fun handleError(error: Throwable) {
    log.error("Error with dynamic gas price service: errorMessage={}", error.message, error)
  }
}
