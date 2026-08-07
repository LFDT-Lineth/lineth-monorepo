package lineth.ethereum.gaspricing.dynamiccap

import lineth.domain.FeeHistory
import lineth.kotlin.toGWei
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.time.DayOfWeek
import kotlin.time.Duration

data class PercentileGasFees(
  val percentileBaseFeePerGas: ULong,
  val percentileBaseFeePerBlobGas: ULong,
  val percentileAvgReward: ULong,
) {
  override fun toString(): String {
    return "percentileBaseFeePerGas=${percentileBaseFeePerGas.toGWei()} GWei," +
      " percentileBaseFeePerBlobGas=${percentileBaseFeePerBlobGas.toGWei()} GWei" +
      " percentileAvgReward=${percentileAvgReward.toGWei()} GWei"
  }
}

typealias TimeOfDayMultipliers = Map<String, Double>

fun getTimeOfDayKey(dayOfWeek: DayOfWeek, hour: Int): String {
  return "${dayOfWeek}_$hour"
}

interface GasPriceCapFeeHistoryFetcher {
  fun getEthFeeHistoryData(startBlockNumberInclusive: Long, endBlockNumberInclusive: Long): SafeFuture<FeeHistory>
}

interface GasPriceCapCalculator {
  fun calculateGasPriceCap(
    adjustmentConstant: UInt,
    finalizationTargetMaxDelay: Duration,
    historicGasPriceCap: ULong,
    elapsedTimeSinceBlockTimestamp: Duration,
    timeOfDayMultiplier: Double = 1.0,
  ): ULong
}

interface GasPriceCapFeeHistoryCache {
  fun getCachedNumOfFeeHistoriesFromBlockNumber(): Int

  fun cacheNumOfFeeHistoriesFromBlockNumber(rewardPercentile: Double, fromBlockNumber: Long): SafeFuture<Int>

  fun getCachedPercentileGasFees(): PercentileGasFees

  fun cachePercentileGasFees(percentile: Double, fromBlockNumber: Long): SafeFuture<Unit>
}

interface FeeHistoriesRepositoryWithCache : FeeHistoriesRepository, GasPriceCapFeeHistoryCache
