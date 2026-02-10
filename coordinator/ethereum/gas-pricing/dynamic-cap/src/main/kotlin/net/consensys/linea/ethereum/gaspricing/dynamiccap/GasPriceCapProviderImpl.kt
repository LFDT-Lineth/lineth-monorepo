package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.gas.GasPriceCaps
import linea.ethapi.EthApiBlockClient
import linea.kotlin.toGWei
import linea.kotlin.toULong
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCapProvider
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.time.LocalDateTime
import java.time.ZoneOffset
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Instant

class GasPriceCapProviderImpl(
  private val config: Config,
  private val l2EthApiBlockClient: EthApiBlockClient,
  private val feeHistoriesRepository: FeeHistoriesRepositoryWithCache,
  private val gasPriceCapCalculator: GasPriceCapCalculator,
  private val clock: Clock = Clock.System,
) : GasPriceCapProvider {
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

  private val log: Logger = LogManager.getLogger(this::class.java)

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

  private fun isEnoughDataForGasPriceCapCalculation(): Boolean {
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

  private fun calculateGasPriceCapsHelper(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> {
    return if (isEnoughDataForGasPriceCapCalculation()) {
      l2EthApiBlockClient.ethGetBlockByNumberTxHashes(targetL2BlockNumber.toBlockParameter())
        .thenApply { it.timestamp }
        .thenApply {
          val targetL2BlockTimestamp = Instant.fromEpochSeconds(it.toLong())
          val elapsedTimeSinceBlockTimestamp = getElapsedTimeSinceBlockTimestamp(targetL2BlockTimestamp)
          val percentileGasFees = feeHistoriesRepository.getCachedPercentileGasFees()
          val maxPriorityFeePerGasCap = gasPriceCapCalculator.calculateGasPriceCap(
            adjustmentConstant = config.adjustmentConstant,
            finalizationTargetMaxDelay = config.finalizationTargetMaxDelay,
            historicGasPriceCap = percentileGasFees.percentileAvgReward,
            elapsedTimeSinceBlockTimestamp = elapsedTimeSinceBlockTimestamp,
            timeOfDayMultiplier = getTimeOfDayMultiplierForNow(config.timeOfDayMultipliers),
          )
          val maxBaseFeePerGasCap = gasPriceCapCalculator.calculateGasPriceCap(
            adjustmentConstant = config.adjustmentConstant,
            finalizationTargetMaxDelay = config.finalizationTargetMaxDelay,
            historicGasPriceCap = percentileGasFees.percentileBaseFeePerGas,
            elapsedTimeSinceBlockTimestamp = elapsedTimeSinceBlockTimestamp,
            timeOfDayMultiplier = getTimeOfDayMultiplierForNow(config.timeOfDayMultipliers),
          )
          val maxFeePerBlobGasCap = gasPriceCapCalculator.calculateGasPriceCap(
            adjustmentConstant = config.blobAdjustmentConstant,
            finalizationTargetMaxDelay = config.finalizationTargetMaxDelay,
            historicGasPriceCap = percentileGasFees.percentileBaseFeePerBlobGas,
            elapsedTimeSinceBlockTimestamp = elapsedTimeSinceBlockTimestamp,
          )
          GasPriceCaps(
            maxBaseFeePerGasCap = maxBaseFeePerGasCap,
            maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
            maxFeePerGasCap = maxPriorityFeePerGasCap + maxBaseFeePerGasCap,
            maxFeePerBlobGasCap = maxFeePerBlobGasCap,
          )
        }.thenPeek { gasPriceCaps ->
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
        }.exceptionallyCompose { th ->
          log.error(
            "Gas price caps returned as null due to failure occurred: " +
              "errorMessage={}",
            th.message,
            th,
          )
          SafeFuture.completedFuture(null)
        }
    } else {
      SafeFuture.completedFuture(null)
    }
  }

  private fun calculateGasPriceCaps(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> {
    return if (config.enabled) {
      calculateGasPriceCapsHelper(
        targetL2BlockNumber = targetL2BlockNumber,
      )
    } else {
      SafeFuture.completedFuture(null)
    }
  }

  override fun getGasPriceCaps(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> {
    return calculateGasPriceCaps(targetL2BlockNumber)
  }

  override fun getGasPriceCapsWithCoefficient(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> {
    return calculateGasPriceCaps(targetL2BlockNumber).thenApply {
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
