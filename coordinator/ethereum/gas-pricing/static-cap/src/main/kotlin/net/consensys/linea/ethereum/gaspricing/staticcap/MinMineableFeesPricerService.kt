package net.consensys.linea.ethereum.gaspricing.staticcap

import io.vertx.core.Vertx
import linea.kotlin.toGWei
import linea.kotlin.toIntervalString
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.FeesFetcher
import net.consensys.linea.ethereum.gaspricing.GasPriceUpdater
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

class MinMineableFeesPricerService(
  pollingInterval: Duration,
  vertx: Vertx,
  private val feesFetcher: FeesFetcher,
  private val feesCalculator: FeesCalculator,
  private val gasPriceUpdater: GasPriceUpdater,
  private val log: Logger = LogManager.getLogger(MinMineableFeesPricerService::class.java),
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = pollingInterval.inWholeMilliseconds,
  log = log,
) {
  private var lastPriceUpdate = 0uL

  override fun action(): SafeFuture<Unit> {
    return feesFetcher
      .getL1EthGasPriceData()
      .thenCompose { feeHistory ->
        val blockRange = feeHistory.blocksRange()
        val gasPrice = feesCalculator.calculateFees(feeHistory).toULong()
        if (lastPriceUpdate != gasPrice) {
          // this is just to avoid log noise.
          // very often price update is capped by limits
          lastPriceUpdate = gasPrice
          log.info(
            "L2 Gas price update: gasPrice={} GWei l1Blocks={}",
            gasPrice.toGWei(),
            blockRange.toIntervalString(),
          )
        }
        // even if price was not update, still call Besu/Geth nodes, they may be restarted
        // and need this info to be updated
        gasPriceUpdater.updateMinerGasPrice(gasPrice)
      }
      .thenApply { log.debug("Fetch, calculate, update new miner gas price are all done.") }
  }

  override fun handleError(error: Throwable) {
    log.error("Error with dynamic gas price service: errorMessage={}", error.message, error)
  }
}
