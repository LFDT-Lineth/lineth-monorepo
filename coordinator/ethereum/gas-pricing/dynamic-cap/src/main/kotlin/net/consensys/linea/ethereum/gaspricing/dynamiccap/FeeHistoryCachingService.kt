package net.consensys.linea.ethereum.gaspricing.dynamiccap

import io.vertx.core.Vertx
import linea.ethapi.EthApiBlockClient
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

class FeeHistoryCachingService(
  private val config: Config,
  vertx: Vertx,
  private val ethApiBlockClient: EthApiBlockClient,
  private val feeHistoryFetcher: GasPriceCapFeeHistoryFetcher,
  private val feeHistoriesRepository: FeeHistoriesRepositoryWithCache,
  private val log: Logger = LogManager.getLogger(FeeHistoryCachingService::class.java),
) : VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log,
  name = "FeeHistoryCachingService",
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {
  data class Config(
    val pollingInterval: Duration,
    val feeHistoryMaxBlockCount: UInt,
    val gasFeePercentile: Double,
    val feeHistoryWindowInBlocks: UInt,
    val feeHistoryStoragePeriodInBlocks: UInt,
    val numOfBlocksBeforeLatest: UInt,
  )

  private fun fetchAndSaveFeeHistories(maxL1BlockNumberToFetch: Long): SafeFuture<Unit> {
    val minFromBlockNumberInclusive = maxL1BlockNumberToFetch.minus(config.feeHistoryWindowInBlocks.toLong())
      .coerceAtLeast(0L).inc()

    return feeHistoriesRepository.findHighestBlockNumberWithPercentile(
      config.gasFeePercentile,
    )
      .thenCompose {
        val highestStoredL1BlockNumber = it ?: 0L

        val startBlockNumberInclusive = highestStoredL1BlockNumber.inc()
          .coerceAtLeast(minFromBlockNumberInclusive)

        val endBlockNumberInclusive = startBlockNumberInclusive
          .plus(config.feeHistoryMaxBlockCount.toLong()).dec()
          .coerceAtMost(maxL1BlockNumberToFetch)

        if (endBlockNumberInclusive < startBlockNumberInclusive) {
          SafeFuture.completedFuture(Unit)
        } else {
          feeHistoryFetcher.getEthFeeHistoryData(
            startBlockNumberInclusive = startBlockNumberInclusive,
            endBlockNumberInclusive = endBlockNumberInclusive,
          ).thenCompose { feeHistory ->
            feeHistoriesRepository.saveNewFeeHistory(feeHistory)
          }.thenCompose {
            if (endBlockNumberInclusive == maxL1BlockNumberToFetch) {
              feeHistoriesRepository.cachePercentileGasFees(
                percentile = config.gasFeePercentile,
                fromBlockNumber = minFromBlockNumberInclusive,
              )
            } else {
              SafeFuture.completedFuture(Unit)
            }
          }
        }
      }.whenException { th ->
        log.warn(
          "failure occurred when fetch and save fee histories but will be ignored: errorMessage={}",
          th.message,
          th,
        )
      }.alwaysRun {
        feeHistoriesRepository.deleteFeeHistoriesUpToBlockNumber(
          maxL1BlockNumberToFetch.minus(config.feeHistoryStoragePeriodInBlocks.toLong())
            .coerceAtLeast(0L),
        ).thenCompose {
          feeHistoriesRepository.cacheNumOfFeeHistoriesFromBlockNumber(
            rewardPercentile = config.gasFeePercentile,
            fromBlockNumber = minFromBlockNumberInclusive,
          )
        }
      }
  }

  override fun action(): SafeFuture<Unit> {
    return ethApiBlockClient.ethBlockNumber()
      .thenCompose { latestL1BlockNumber ->
        fetchAndSaveFeeHistories(
          // subtracting the latest L1 block number with a predefined number
          // (default as 4) to avoid requesting fee history of the head block
          // from nodes that were not catching up with the head yet
          maxL1BlockNumberToFetch = latestL1BlockNumber.toLong()
            .minus(config.numOfBlocksBeforeLatest.toLong())
            .coerceAtLeast(1L),
        )
      }
  }

  override fun handleError(error: Throwable) {
    log.error(
      "Error with fee history caching service: errorMessage={}",
      error.message,
      error,
    )
  }
}
