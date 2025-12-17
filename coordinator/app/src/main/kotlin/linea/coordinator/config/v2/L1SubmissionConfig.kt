package linea.coordinator.config.v2

import net.consensys.linea.ethereum.gaspricing.dynamiccap.TimeOfDayMultipliers
import java.net.URL
import kotlin.time.Duration

data class L1SubmissionConfig(
  val dynamicGasPriceCap: DynamicGasPriceCapConfig,
  val fallbackGasPrice: FallbackGasPriceConfig,
  val blob: BlobSubmissionConfig,
  val aggregation: AggregationSubmissionConfig,
  val dataAvailability: DataAvailability,
) : FeatureToggle {
  enum class DataAvailability {
    ROLLUP,
    VALIDIUM,
  }

  override val disabled: Boolean
    get() = blob.disabled && aggregation.disabled

  data class DynamicGasPriceCapConfig(
    override val disabled: Boolean,
    val gasPriceCapCalculation: GasPriceCapCalculationConfig,
    val feeHistoryFetcher: FeeHistoryFetcherConfig,
    val timeOfDayMultipliers: TimeOfDayMultipliers,
  ) : FeatureToggle {
    data class GasPriceCapCalculationConfig(
      val adjustmentConstant: UInt,
      val blobAdjustmentConstant: UInt,
      val finalizationTargetMaxDelay: Duration,
      val baseFeePerGasPercentileWindow: Duration,
      val baseFeePerGasPercentileWindowLeeway: Duration,
      val baseFeePerGasPercentile: UInt,
      val gasPriceCapsCheckCoefficient: Double,
      val historicBaseFeePerBlobGasLowerBound: ULong,
      val historicAvgRewardConstant: ULong,
      val timeOfTheDayMultipliers: Map<String, Double>,
    )

    data class FeeHistoryFetcherConfig(
      val l1Endpoint: URL,
      val fetchInterval: Duration,
      val maxBlockCount: UInt,
      val rewardPercentiles: List<UInt>,
      val numOfBlocksBeforeLatest: UInt,
      val storagePeriod: Duration,
    )
  }

  data class FallbackGasPriceConfig(
    val feeHistoryBlockCount: UInt,
    val feeHistoryRewardPercentile: UInt,
  )

  data class GasConfig(
    val gasLimit: ULong,
    val maxFeePerGasCap: ULong,
    val maxPriorityFeePerGasCap: ULong,
    val maxFeePerBlobGasCap: ULong = 0UL,
    val fallback: FallbackGasConfig,
  ) {
    data class FallbackGasConfig(
      val priorityFeePerGasUpperBound: ULong,
      val priorityFeePerGasLowerBound: ULong,
    )
  }

  data class BlobSubmissionConfig(
    override val disabled: Boolean,
    val l1Endpoint: URL,
    val submissionDelay: Duration,
    val submissionTickInterval: Duration,
    val maxSubmissionTransactionsPerTick: UInt,
    val targetBlobsPerTransaction: UInt,
    val dbMaxBlobsToReturn: UInt,
    val gas: GasConfig,
    val signer: SignerConfig,
  ) : FeatureToggle

  data class AggregationSubmissionConfig(
    override val disabled: Boolean,
    val l1Endpoint: URL,
    val submissionDelay: Duration,
    val submissionTickInterval: Duration,
    val maxSubmissionsPerTick: UInt,
    val gas: GasConfig,
    val signer: SignerConfig,
  ) : FeatureToggle
}
