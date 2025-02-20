package net.consensys.linea.ethereum.gaspricing.dynamiccap

import kotlinx.datetime.DayOfWeek
import linea.domain.FeeHistory
import linea.kotlin.toGWei
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.days

const val MAX_REWARD_PERCENTILES_SIZE = 10
const val MAX_FEE_HISTORY_BLOCK_COUNT = 1024U
val MAX_FEE_HISTORIES_STORAGE_PERIOD = 30.days

data class PercentileGasFees(
  val percentileBaseFeePerGas: ULong,
  val percentileBaseFeePerBlobGas: ULong,
  val percentileAvgReward: ULong
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

fun getAllTimeOfDayKeys(): Set<String> {
  return DayOfWeek.values()
    .flatMap { dayOfWeek ->
      (0..23).map { hour -> getTimeOfDayKey(dayOfWeek, hour) }
    }.toSet()
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
    timeOfDayMultiplier: Double = 1.0
  ): ULong
}

interface GasPriceCapFeeHistoryCache {
  fun getCachedNumOfFeeHistoriesFromBlockNumber(): Int

  fun cacheNumOfFeeHistoriesFromBlockNumber(
    rewardPercentile: Double,
    fromBlockNumber: Long
  ): SafeFuture<Int>

  fun getCachedPercentileGasFees(): PercentileGasFees

  fun cachePercentileGasFees(
    percentile: Double,
    fromBlockNumber: Long
  ): SafeFuture<Unit>
}

interface FeeHistoriesRepositoryWithCache : FeeHistoriesRepository, GasPriceCapFeeHistoryCache
