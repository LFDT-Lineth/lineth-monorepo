package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.L1SubmissionConfig
import linea.coordinator.config.v2.L1SubmissionConfig.DynamicGasPriceCapConfig.GasPriceCapCalculationConfig
import net.consensys.linea.ethereum.gaspricing.dynamiccap.TimeOfDayMultipliers
import java.net.URL
import kotlin.ULong
import kotlin.time.Duration
import kotlin.time.Duration.Companion.days
import kotlin.time.Duration.Companion.seconds

data class L1SubmissionConfigToml(
  val disabled: Boolean = false,
  val dynamicGasPriceCap: DynamicGasPriceCapToml,
  val fallbackGasPrice: FallbackGasPriceToml,
  val blob: BlobSubmissionConfigToml,
  val aggregation: AggregationSubmissionToml,
) {

  data class DynamicGasPriceCapToml(
    val disabled: Boolean = false,
    val gasPriceCapCalculation: GasPriceCapCalculationToml,
    val feeHistoryFetcher: FeeHistoryFetcherConfig = FeeHistoryFetcherConfig(),
  ) {
    data class GasPriceCapCalculationToml(
      val adjustmentConstant: UInt,
      val blobAdjustmentConstant: UInt,
      val finalizationTargetMaxDelay: Duration,
      val baseFeePerGasPercentileWindow: Duration,
      val baseFeePerGasPercentileWindowLeeway: Duration,
      val baseFeePerGasPercentile: UInt,
      val gasPriceCapsCheckCoefficient: Double,
      val historicBaseFeePerBlobGasLowerBound: ULong,
      val historicAvgRewardConstant: ULong,
    ) {
      fun reified(
        timeOfTheDayMultipliers: TimeOfDayMultipliers,
      ): GasPriceCapCalculationConfig {
        return GasPriceCapCalculationConfig(
          adjustmentConstant = this.adjustmentConstant,
          blobAdjustmentConstant = this.blobAdjustmentConstant,
          finalizationTargetMaxDelay = this.finalizationTargetMaxDelay,
          baseFeePerGasPercentileWindow = this.baseFeePerGasPercentileWindow,
          baseFeePerGasPercentileWindowLeeway = this.baseFeePerGasPercentileWindowLeeway,
          baseFeePerGasPercentile = this.baseFeePerGasPercentile,
          gasPriceCapsCheckCoefficient = this.gasPriceCapsCheckCoefficient,
          historicBaseFeePerBlobGasLowerBound = this.historicBaseFeePerBlobGasLowerBound,
          historicAvgRewardConstant = this.historicAvgRewardConstant,
          timeOfTheDayMultipliers = timeOfTheDayMultipliers,
        )
      }
    }

    data class FeeHistoryFetcherConfig(
      val l1Endpoint: URL? = null,
      val fetchInterval: Duration = 3.seconds,
      val maxBlockCount: UInt = 1000u,
      val rewardPercentiles: List<UInt> = listOf(10u, 20u, 30u, 40u, 50u, 60u, 70u, 80u, 90u, 100u),
      val numOfBlocksBeforeLatest: UInt = 4u,
      val storagePeriod: Duration = 10.days,
    ) {
      fun reified(
        defaultL1Endpoint: URL?,
      ): L1SubmissionConfig.DynamicGasPriceCapConfig.FeeHistoryFetcherConfig {
        return L1SubmissionConfig.DynamicGasPriceCapConfig.FeeHistoryFetcherConfig(
          l1Endpoint = this.l1Endpoint ?: defaultL1Endpoint
            ?: throw AssertionError("l1Endpoint config missing"),
          fetchInterval = this.fetchInterval,
          maxBlockCount = this.maxBlockCount,
          rewardPercentiles = this.rewardPercentiles,
          numOfBlocksBeforeLatest = this.numOfBlocksBeforeLatest,
          storagePeriod = this.storagePeriod,
        )
      }
    }
  }

  data class FallbackGasPriceToml(
    val feeHistoryBlockCount: UInt,
    val feeHistoryRewardPercentile: UInt,
  )

  data class GasConfigToml(
    val gasLimit: ULong,
    val maxFeePerGasCap: ULong,
    val maxFeePerBlobGasCap: ULong? = null,
    val maxPriorityFeePerGasCap: ULong,
    val fallback: FallbackGasConfig,
  ) {
    data class FallbackGasConfig(
      val priorityFeePerGasUpperBound: ULong,
      val priorityFeePerGasLowerBound: ULong,
    )

    fun reified(): L1SubmissionConfig.GasConfig {
      return L1SubmissionConfig.GasConfig(
        gasLimit = this.gasLimit,
        maxFeePerGasCap = this.maxFeePerGasCap,
        maxPriorityFeePerGasCap = this.maxPriorityFeePerGasCap,
        maxFeePerBlobGasCap = this.maxFeePerBlobGasCap,
        fallback = L1SubmissionConfig.GasConfig.FallbackGasConfig(
          priorityFeePerGasLowerBound = this.fallback.priorityFeePerGasLowerBound,
          priorityFeePerGasUpperBound = this.fallback.priorityFeePerGasUpperBound,
        ),
      )
    }
  }

  data class BlobSubmissionConfigToml(
    val disabled: Boolean = false,
    val l1Endpoint: URL? = null,
    val submissionDelay: Duration = 0.seconds,
    val submissionTickInterval: Duration = 12.seconds,
    val maxSubmissionTransactionsPerTick: UInt = 2u,
    // eip-7691 on Prague fork allows up to 9 blobs per transaction.
    // however, Geth nodes fail with "transaction too large" error. only 7 blobs are accepted
    val targetBlobsPerTransaction: UInt = 7u,
    val dbMaxBlobsToReturn: UInt = 100u,
    val gas: GasConfigToml,
    val signer: SignerConfigToml,
  )

  data class AggregationSubmissionToml(
    val disabled: Boolean = false,
    val l1Endpoint: URL? = null,
    val submissionDelay: Duration = 0.seconds,
    val submissionTickInterval: Duration = 24.seconds,
    val maxSubmissionsPerTick: UInt = 1u,
    val gas: GasConfigToml,
    val signer: SignerConfigToml,
  )

  fun reified(
    l1DefaultEndpoint: URL?,
    timeOfDayMultipliers: TimeOfDayMultipliers,
  ): L1SubmissionConfig {
    return L1SubmissionConfig(
      dynamicGasPriceCap = L1SubmissionConfig.DynamicGasPriceCapConfig(
        disabled = this.dynamicGasPriceCap.disabled,
        gasPriceCapCalculation = this.dynamicGasPriceCap.gasPriceCapCalculation.reified(timeOfDayMultipliers),
        feeHistoryFetcher = this.dynamicGasPriceCap.feeHistoryFetcher.reified(l1DefaultEndpoint),
        timeOfDayMultipliers = timeOfDayMultipliers,
      ),
      fallbackGasPrice = L1SubmissionConfig.FallbackGasPriceConfig(
        feeHistoryBlockCount = this.fallbackGasPrice.feeHistoryBlockCount,
        feeHistoryRewardPercentile = this.fallbackGasPrice.feeHistoryRewardPercentile,
      ),
      blob = L1SubmissionConfig.BlobSubmissionConfig(
        disabled = this.blob.disabled,
        l1Endpoint = this.blob.l1Endpoint ?: l1DefaultEndpoint
          ?: throw AssertionError("l1Endpoint config missing"),
        submissionDelay = this.blob.submissionDelay,
        submissionTickInterval = this.blob.submissionTickInterval,
        maxSubmissionTransactionsPerTick = this.blob.maxSubmissionTransactionsPerTick,
        targetBlobsPerTransaction = this.blob.targetBlobsPerTransaction,
        dbMaxBlobsToReturn = this.blob.dbMaxBlobsToReturn,
        gas = this.blob.gas.reified(),
        signer = this.blob.signer.reified(),
      ),
      aggregation = L1SubmissionConfig.AggregationSubmissionConfig(
        disabled = this.aggregation.disabled,
        l1Endpoint = this.aggregation.l1Endpoint ?: l1DefaultEndpoint
          ?: throw AssertionError("l1Endpoint config missing"),
        submissionDelay = this.aggregation.submissionDelay,
        submissionTickInterval = this.aggregation.submissionTickInterval,
        maxSubmissionsPerTick = this.aggregation.maxSubmissionsPerTick,
        gas = this.aggregation.gas.reified(),
        signer = this.aggregation.signer.reified(),
      ),
    )
  }
}
