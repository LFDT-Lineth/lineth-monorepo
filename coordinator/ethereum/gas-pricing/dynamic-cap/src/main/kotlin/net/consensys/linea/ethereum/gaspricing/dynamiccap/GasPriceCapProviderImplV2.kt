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

  private fun getElapsedTimeSinceBlockTimestamp(blockTimestamp: Instant, referenceTime: Instant): Duration {
    return (referenceTime - blockTimestamp).coerceAtLeast(Duration.ZERO)
  }

  private fun getTimeOfDayMultiplierForNow(referenceTime: Instant): Double {
    val dateTime = LocalDateTime.ofEpochSecond(referenceTime.epochSeconds, 0, ZoneOffset.UTC)
    val tdmKey = getTimeOfDayKey(dateTime.dayOfWeek, dateTime.hour)
    return config.timeOfDayMultipliers[tdmKey] ?: run {
      log.info("No multiplier found for key={} date={} defaulting to 1.0", tdmKey, dateTime)
      1.0
    }
  }

  private fun calculateGasPriceCaps(blockTimestamp: Instant, referenceTime: Instant): GasPriceCaps {
    val elapsedTimeSinceBlockTimestamp = getElapsedTimeSinceBlockTimestamp(blockTimestamp, referenceTime)
    val percentileGasFees = feeHistoriesRepository.getCachedPercentileGasFees()
    val timeOfDayMultiplier = getTimeOfDayMultiplierForNow(referenceTime)
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
    if (!config.enabled || !hasEnoughDataForGasPriceCapCalculation()) {
      return SafeFuture.completedFuture(null)
    }

    val gasPriceCaps = runCatching { calculateGasPriceCaps(timestamp, clock.now()) }
      .onFailure { th ->
        log.warn("Gas price caps returned as null due to calculation failure: errorMessage={}", th.message, th)
      }
      .getOrNull()

    return SafeFuture.completedFuture(gasPriceCaps)
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
