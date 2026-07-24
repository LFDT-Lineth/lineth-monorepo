package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.domain.gas.GasPriceCaps
import linea.gaspricing.GasPriceCapProviderV2
import linea.kotlin.toGWei
import linea.kotlin.toULong
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.time.LocalDateTime
import java.time.ZoneOffset
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Instant

class GasPriceCapProviderImplV2(
  private val config: Config,
  private val feeHistoriesRepository: FeeHistoriesRepositoryWithCache,
  private val gasPriceCapCalculator: GasPriceCapCalculator,
  private val clock: Clock = Clock.System,
  private val log: Logger = LogManager.getLogger(GasPriceCapProviderImplV2::class.java),
) : GasPriceCapProviderV2 {
  data class Config(
    val enabled: Boolean,
    val gasFeePercentile: Double,
    val gasFeePercentileWindowInBlocks: UInt,
    val gasFeePercentileWindowLeewayInBlocks: UInt,
    val timeOfDayMultipliers: TimeOfDayMultipliers,
    val adjustmentConstant: UInt,
    val blobAdjustmentConstant: UInt,
    val finalizationTargetMaxDelay: Duration,
    val gasPriceCapsCoefficient: Double,
  )

  init {
    require(config.gasFeePercentile >= 0.0) {
      "gasFeePercentile must be no less than 0.0." +
        " Value=${config.gasFeePercentile}"
    }

    require(config.finalizationTargetMaxDelay > Duration.ZERO) {
      "finalizationTargetMaxDelay duration must be longer than zero second." +
        " Value=${config.finalizationTargetMaxDelay}"
    }

    require(config.gasPriceCapsCoefficient > 0.0) {
      "gasPriceCapsCoefficient must be greater than 0.0." +
        " Value=${config.gasPriceCapsCoefficient}"
    }
  }

  internal fun hasEnoughDataForGasPriceCapCalculation(): Boolean {
    val minNumOfFeeHistoriesNeeded = BigInteger.valueOf(config.gasFeePercentileWindowInBlocks.toLong())
      .minus(BigInteger.valueOf(config.gasFeePercentileWindowLeewayInBlocks.toLong()))
      .coerceAtLeast(BigInteger.ZERO).toULong()

    val numOfValidFeeHistories = feeHistoriesRepository.getCachedNumOfFeeHistoriesFromBlockNumber()
    val isEnoughData = numOfValidFeeHistories.toULong() >= minNumOfFeeHistoriesNeeded
    if (!isEnoughData) {
      log.warn(
        "Not enough fee history data for gas price cap update: numOfValidFeeHistoriesInDb={}, " +
          "minNumOfFeeHistoriesNeeded={}",
        numOfValidFeeHistories,
        minNumOfFeeHistoriesNeeded,
      )
    }
    return isEnoughData
  }

  private fun getElapsedTimeSinceBlockTimestamp(blockTimestamp: Instant): Duration {
    return (clock.now() - blockTimestamp).coerceAtLeast(Duration.ZERO)
  }

  private fun getTimeOfDayMultiplierForNow(timeOfDayMultipliers: TimeOfDayMultipliers): Double {
    val dateTime = LocalDateTime.ofEpochSecond(clock.now().epochSeconds, 0, ZoneOffset.UTC)
    val tdmKey = getTimeOfDayKey(dateTime.dayOfWeek, dateTime.hour)
    return timeOfDayMultipliers[tdmKey]!!
  }

  private fun calculateGasPriceCaps(timestamp: Instant): GasPriceCaps {
    val elapsedTimeSinceBlockTimestamp = getElapsedTimeSinceBlockTimestamp(timestamp)
    val percentileGasFees = feeHistoriesRepository.getCachedPercentileGasFees()
    val timeOfDayMultiplier = getTimeOfDayMultiplierForNow(config.timeOfDayMultipliers)
    val maxPriorityFeePerGasCap = gasPriceCapCalculator.calculateGasPriceCap(
      adjustmentConstant = config.adjustmentConstant,
      finalizationTargetMaxDelay = config.finalizationTargetMaxDelay,
      historicGasPriceCap = percentileGasFees.percentileAvgReward,
      elapsedTimeSinceBlockTimestamp = elapsedTimeSinceBlockTimestamp,
      timeOfDayMultiplier = timeOfDayMultiplier,
    )
    val maxBaseFeePerGasCap = gasPriceCapCalculator.calculateGasPriceCap(
      adjustmentConstant = config.adjustmentConstant,
      finalizationTargetMaxDelay = config.finalizationTargetMaxDelay,
      historicGasPriceCap = percentileGasFees.percentileBaseFeePerGas,
      elapsedTimeSinceBlockTimestamp = elapsedTimeSinceBlockTimestamp,
      timeOfDayMultiplier = timeOfDayMultiplier,
    )
    val maxFeePerBlobGasCap = gasPriceCapCalculator.calculateGasPriceCap(
      adjustmentConstant = config.blobAdjustmentConstant,
      finalizationTargetMaxDelay = config.finalizationTargetMaxDelay,
      historicGasPriceCap = percentileGasFees.percentileBaseFeePerBlobGas,
      elapsedTimeSinceBlockTimestamp = elapsedTimeSinceBlockTimestamp,
    )
    val gasPriceCaps = GasPriceCaps(
      maxBaseFeePerGasCap = maxBaseFeePerGasCap,
      maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
      maxFeePerGasCap = maxPriorityFeePerGasCap + maxBaseFeePerGasCap,
      maxFeePerBlobGasCap = maxFeePerBlobGasCap,
    )

    log.debug(
      "Calculated raw gas price caps: " +
        "maxBaseFeePerGasCap={} GWei, maxPriorityFeePerGasCap={} GWei, " +
        "maxFeePerGasCap={} GWei, maxFeePerBlobGasCap={} GWei, percentile={}",
      gasPriceCaps.maxBaseFeePerGasCap?.toGWei(),
      gasPriceCaps.maxPriorityFeePerGasCap.toGWei(),
      gasPriceCaps.maxFeePerGasCap.toGWei(),
      gasPriceCaps.maxFeePerBlobGasCap.toGWei(),
      config.gasFeePercentile,
    )

    return gasPriceCaps
  }

  override fun getGasPriceCaps(timestamp: Instant): SafeFuture<GasPriceCaps?> {
    return if (config.enabled && hasEnoughDataForGasPriceCapCalculation()) {
      try {
        calculateGasPriceCaps(timestamp)
      } catch (e: Exception) {
        log.warn("Gas price caps will default to null due to calculation error: errorMessage={}", e.message, e)
        null
      }
    } else {
      null
    }.let { SafeFuture.completedFuture(it) }
  }

  override fun getGasPriceCapsWithCoefficient(timestamp: Instant): SafeFuture<GasPriceCaps?> {
    return getGasPriceCaps(timestamp).thenApply {
      it?.run {
        val multipliedMaxBaseFeePerGasCap = it.maxBaseFeePerGasCap!!.toDouble() * config.gasPriceCapsCoefficient
        val multipliedMaxPriorityFeePerGas = it.maxPriorityFeePerGasCap.toDouble() * config.gasPriceCapsCoefficient
        val multipliedMaxFeePerBlobGasCap = (it.maxFeePerBlobGasCap.toDouble() * config.gasPriceCapsCoefficient)
          .coerceAtLeast(1.0)
        GasPriceCaps(
          maxBaseFeePerGasCap = multipliedMaxBaseFeePerGasCap.toULong(),
          maxPriorityFeePerGasCap = multipliedMaxPriorityFeePerGas.toULong(),
          maxFeePerGasCap = (multipliedMaxBaseFeePerGasCap + multipliedMaxPriorityFeePerGas).toULong(),
          maxFeePerBlobGasCap = multipliedMaxFeePerBlobGasCap.toULong(),
        )
      }
    }
  }
}
