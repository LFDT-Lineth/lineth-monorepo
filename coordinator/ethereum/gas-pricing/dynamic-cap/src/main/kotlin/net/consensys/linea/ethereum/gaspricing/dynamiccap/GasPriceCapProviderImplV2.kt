package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.gaspricing.GasPriceCapProviderV2
import lineth.domain.gas.GasPriceCaps
import lineth.kotlin.minusCoercingUnderflow
import lineth.kotlin.toBigDecimal
import lineth.kotlin.toGWei
import lineth.kotlin.toULong
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigDecimal
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
    val minNumOfFeeHistoriesNeeded =
      config.gasFeePercentileWindowInBlocks
        .minusCoercingUnderflow(config.gasFeePercentileWindowLeewayInBlocks)
        .toLong()

    val numOfValidFeeHistories = feeHistoriesRepository.getCachedNumOfFeeHistoriesFromBlockNumber()
    val hasEnoughData = numOfValidFeeHistories >= minNumOfFeeHistoriesNeeded
    if (!hasEnoughData) {
      log.warn(
        "Not enough fee history data for gas price cap update: numOfValidFeeHistoriesInDb={}, " +
          "minNumOfFeeHistoriesNeeded={}",
        numOfValidFeeHistories,
        minNumOfFeeHistoriesNeeded,
      )
    }
    return hasEnoughData
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

    val gasPriceCaps = runCatching {
      calculateGasPriceCaps(timestamp, clock.now())
    }
      .onFailure { th ->
        log.warn("Gas price caps returned as null due to calculation failure: errorMessage={}", th.message, th)
      }
      .getOrNull()

    return SafeFuture.completedFuture(gasPriceCaps)
  }

  override fun getGasPriceCapsWithCoefficient(timestamp: Instant): SafeFuture<GasPriceCaps?> {
    return getGasPriceCaps(timestamp)
      .thenApply { caps ->
        caps?.let {
          val coeff = config.gasPriceCapsCoefficient.toBigDecimal()
          // Coefficient application historically truncates fractional wei.
          val multipliedMaxBaseFeePerGasCap =
            // NOTE: at this stage maxBaseFeePerGasCap is always defined
            // on upper classes (e.g GasPriceCapProviderForDataSubmission) may be nullified
            (it.maxBaseFeePerGasCap!!.toBigDecimal() * coeff).toBigInteger().toULong()
          val multipliedMaxPriorityFeePerGas =
            (it.maxPriorityFeePerGasCap.toBigDecimal() * coeff).toBigInteger().toULong()
          val multipliedMaxFeePerBlobGasCap =
            (it.maxFeePerBlobGasCap.toBigDecimal() * coeff)
              .coerceAtLeast(BigDecimal.ONE).toBigInteger().toULong()
          GasPriceCaps(
            maxBaseFeePerGasCap = multipliedMaxBaseFeePerGasCap,
            maxPriorityFeePerGasCap = multipliedMaxPriorityFeePerGas,
            maxFeePerGasCap = multipliedMaxBaseFeePerGasCap + multipliedMaxPriorityFeePerGas,
            maxFeePerBlobGasCap = multipliedMaxFeePerBlobGasCap,
          )
        }
      }
  }
}
