package lineth.ethereum.gaspricing.dynamiccap

import lineth.domain.gas.GasPriceCaps
import lineth.domain.toBlockParameter
import lineth.ethapi.EthApiBlockClient
import lineth.gaspricing.GasPriceCapProvider
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Instant

class GasPriceCapProviderImpl(
  private val config: Config,
  private val l2EthApiBlockClient: EthApiBlockClient,
  feeHistoriesRepository: FeeHistoriesRepositoryWithCache,
  gasPriceCapCalculator: GasPriceCapCalculator,
  clock: Clock = Clock.System,
  private val log: Logger = LogManager.getLogger(GasPriceCapProviderImpl::class.java),
  private val delegate: GasPriceCapProviderImplV2 = GasPriceCapProviderImplV2(
    gasPriceCapCalculator = gasPriceCapCalculator,
    feeHistoriesRepository = feeHistoriesRepository,
    clock = clock,
    log = log,
    config = GasPriceCapProviderImplV2.Config(
      enabled = config.enabled,
      gasFeePercentile = config.gasFeePercentile,
      gasFeePercentileWindowInBlocks = config.gasFeePercentileWindowInBlocks,
      gasFeePercentileWindowLeewayInBlocks = config.gasFeePercentileWindowLeewayInBlocks,
      timeOfDayMultipliers = config.timeOfDayMultipliers,
      adjustmentConstant = config.adjustmentConstant,
      blobAdjustmentConstant = config.blobAdjustmentConstant,
      finalizationTargetMaxDelay = config.finalizationTargetMaxDelay,
      gasPriceCapsCoefficient = config.gasPriceCapsCoefficient,
    ),
  ),
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

  private fun getGasPriceCaps(
    targetL2BlockNumber: Long,
    capsProvider: (Instant) -> SafeFuture<GasPriceCaps?>,
  ): SafeFuture<GasPriceCaps?> {
    if (!config.enabled || !delegate.hasEnoughDataForGasPriceCapCalculation()) {
      return SafeFuture.completedFuture(null)
    }

    return l2EthApiBlockClient
      .ethGetBlockByNumberTxHashes(targetL2BlockNumber.toBlockParameter())
      .thenCompose { block -> capsProvider(Instant.fromEpochSeconds(block.timestamp.toLong())) }
      .exceptionally { th ->
        log.warn(
          "Gas price caps will default to null because it could not fetch block={}, errorMessage={}",
          targetL2BlockNumber,
          th.message,
          th,
        )
        null
      }
  }

  override fun getGasPriceCaps(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> {
    return getGasPriceCaps(targetL2BlockNumber, delegate::getGasPriceCaps)
  }

  override fun getGasPriceCapsWithCoefficient(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> {
    return getGasPriceCaps(targetL2BlockNumber, delegate::getGasPriceCapsWithCoefficient)
  }
}
