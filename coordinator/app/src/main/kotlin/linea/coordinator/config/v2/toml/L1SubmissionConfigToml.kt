package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.L1SubmissionConfig
import linea.coordinator.config.v2.L1SubmissionConfig.DynamicGasPriceCapConfig.GasPriceCapCalculationConfig
import lineth.config.docs.ConfigDoc
import lineth.config.docs.ConfigSection
import net.consensys.linea.ethereum.gaspricing.dynamiccap.TimeOfDayMultipliers
import java.net.URL
import kotlin.ULong
import kotlin.time.Duration
import kotlin.time.Duration.Companion.days
import kotlin.time.Duration.Companion.seconds

data class L1SubmissionConfigToml(
  @param:ConfigDoc(description = "Whether L1 submission is disabled.", default = "false")
  val disabled: Boolean = false,
  @param:ConfigSection("Dynamic L1 gas price cap calculation settings.")
  val dynamicGasPriceCap: DynamicGasPriceCapToml,
  @param:ConfigSection("Fallback gas price settings used when dynamic pricing is unavailable.")
  val fallbackGasPrice: FallbackGasPriceToml,
  @param:ConfigSection("Blob (data availability) submission settings.")
  val blob: BlobSubmissionConfigToml,
  @param:ConfigSection("Aggregation (finalization) submission settings.")
  val aggregation: AggregationSubmissionToml,
  @param:ConfigDoc(
    description = "Data availability mode: ROLLUP submits blobs to L1, VALIDIUM submits off-chain.",
    default = "ROLLUP",
  )
  val dataAvailability: DataAvailability = DataAvailability.ROLLUP,
) {
  enum class DataAvailability() {
    ROLLUP,
    VALIDIUM,
    ;

    fun reified(): L1SubmissionConfig.DataAvailability {
      return when (this) {
        ROLLUP -> L1SubmissionConfig.DataAvailability.ROLLUP
        VALIDIUM -> L1SubmissionConfig.DataAvailability.VALIDIUM
      }
    }
  }

  data class DynamicGasPriceCapToml(
    @param:ConfigDoc(description = "Whether dynamic gas price cap calculation is disabled.", default = "false")
    val disabled: Boolean = false,
    @param:ConfigSection("Parameters for computing the dynamic gas price cap.")
    val gasPriceCapCalculation: GasPriceCapCalculationToml,
    @param:ConfigSection("L1 fee-history fetcher settings feeding the gas price cap calculation.")
    val feeHistoryFetcher: FeeHistoryFetcherConfig = FeeHistoryFetcherConfig(),
  ) {
    data class GasPriceCapCalculationToml(
      @param:ConfigDoc("Additive constant applied when adjusting the (execution) gas price cap.")
      val adjustmentConstant: UInt,
      @param:ConfigDoc("Additive constant applied when adjusting the blob gas price cap.")
      val blobAdjustmentConstant: UInt,
      @param:ConfigDoc("Maximum expected delay to finalization, used to scale the gas price cap.")
      val finalizationTargetMaxDelay: Duration,
      @param:ConfigDoc("Time window over which the base-fee-per-gas percentile is computed.")
      val baseFeePerGasPercentileWindow: Duration,
      @param:ConfigDoc("Leeway subtracted from the base-fee-per-gas percentile window.")
      val baseFeePerGasPercentileWindowLeeway: Duration,
      @param:ConfigDoc("Percentile (1-100) of historical base fee per gas used in the calculation.")
      val baseFeePerGasPercentile: UInt,
      @param:ConfigDoc("Coefficient applied when checking computed gas price caps.")
      val gasPriceCapsCheckCoefficient: Double,
      @param:ConfigDoc("Lower bound for the historical base fee per blob gas.")
      val historicBaseFeePerBlobGasLowerBound: ULong,
      @param:ConfigDoc("Constant used for the historical average reward calculation.")
      val historicAvgRewardConstant: ULong,
    ) {
      fun reified(timeOfTheDayMultipliers: TimeOfDayMultipliers): GasPriceCapCalculationConfig {
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
      @param:ConfigDoc(
        description = "L1 endpoint for fetching fee history. Falls back to defaults.l1-endpoint.",
        example = "http://l1-el-node:8545",
      )
      val l1Endpoint: URL? = null,
      @param:ConfigSection("Retry policy for fee-history requests; falls back to defaults.l1-request-retries.")
      val l1RequestRetries: RequestRetriesToml? = null,
      @param:ConfigDoc(description = "Interval between fee-history fetches.", default = "PT3S")
      val fetchInterval: Duration = 3.seconds,
      @param:ConfigDoc(description = "Maximum number of blocks requested per fee-history call.", default = "1000")
      val maxBlockCount: UInt = 1000u,
      @param:ConfigDoc(
        description = "Reward percentiles requested from eth_feeHistory.",
        default = "[10, 20, 30, 40, 50, 60, 70, 80, 90, 100]",
      )
      val rewardPercentiles: List<UInt> = listOf(10u, 20u, 30u, 40u, 50u, 60u, 70u, 80u, 90u, 100u),
      @param:ConfigDoc(
        description = "Number of blocks before the latest to stop at when fetching fee history.",
        default = "4",
      )
      val numOfBlocksBeforeLatest: UInt = 4u,
      @param:ConfigDoc(description = "How long fetched fee-history data is retained.", default = "PT240H")
      val storagePeriod: Duration = 10.days,
    ) {
      fun reified(
        defaultL1Endpoint: URL?,
        defaultL1RequestRetries: RequestRetriesToml,
      ): L1SubmissionConfig.DynamicGasPriceCapConfig.FeeHistoryFetcherConfig {
        return L1SubmissionConfig.DynamicGasPriceCapConfig.FeeHistoryFetcherConfig(
          l1Endpoint =
          this.l1Endpoint ?: defaultL1Endpoint
            ?: throw AssertionError("l1Endpoint config missing"),
          l1RequestRetries = this.l1RequestRetries?.asDomain ?: defaultL1RequestRetries.asDomain,
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
    @param:ConfigDoc("Number of blocks of fee history used to compute the fallback gas price.")
    val feeHistoryBlockCount: UInt,
    @param:ConfigDoc("Reward percentile (1-100) used to compute the fallback gas price.")
    val feeHistoryRewardPercentile: UInt,
  )

  data class GasConfigToml(
    @param:ConfigDoc("Gas limit for L1 submission transactions.")
    val gasLimit: ULong,
    @param:ConfigDoc("Cap on the EIP-1559 max fee per gas for L1 submission transactions.")
    val maxFeePerGasCap: ULong,
    @param:ConfigDoc(
      description = "Cap on the max fee per blob gas for blob transactions. 0 disables the cap.",
      default = "0",
    )
    val maxFeePerBlobGasCap: ULong = 0UL,
    @param:ConfigDoc("Cap on the EIP-1559 max priority fee per gas for L1 submission transactions.")
    val maxPriorityFeePerGasCap: ULong,
    @param:ConfigSection("Fallback priority-fee bounds used when dynamic pricing is unavailable.")
    val fallback: FallbackGasConfig,
  ) {
    data class FallbackGasConfig(
      @param:ConfigDoc("Upper bound for the fallback priority fee per gas.")
      val priorityFeePerGasUpperBound: ULong,
      @param:ConfigDoc("Lower bound for the fallback priority fee per gas.")
      val priorityFeePerGasLowerBound: ULong,
    )

    fun reified(): L1SubmissionConfig.GasConfig {
      return L1SubmissionConfig.GasConfig(
        gasLimit = this.gasLimit,
        maxFeePerGasCap = this.maxFeePerGasCap,
        maxPriorityFeePerGasCap = this.maxPriorityFeePerGasCap,
        maxFeePerBlobGasCap = this.maxFeePerBlobGasCap,
        fallback =
        L1SubmissionConfig.GasConfig.FallbackGasConfig(
          priorityFeePerGasLowerBound = this.fallback.priorityFeePerGasLowerBound,
          priorityFeePerGasUpperBound = this.fallback.priorityFeePerGasUpperBound,
        ),
      )
    }
  }

  data class BlobSubmissionConfigToml(
    @param:ConfigDoc(description = "Whether blob submission is disabled.", default = "false")
    val disabled: Boolean = false,
    @param:ConfigDoc(
      description = "L1 endpoint for blob submission. Falls back to defaults.l1-endpoint.",
      example = "http://l1-el-node:8545",
    )
    val l1Endpoint: URL? = null,
    @param:ConfigSection("Retry policy for blob submission requests; falls back to defaults.l1-request-retries.")
    val l1RequestRetries: RequestRetriesToml? = null,
    @param:ConfigDoc(description = "Delay before submitting a blob after it becomes eligible.", default = "PT0S")
    val submissionDelay: Duration = 0.seconds,
    @param:ConfigDoc(description = "Interval between blob submission ticks.", default = "PT12S")
    val submissionTickInterval: Duration = 12.seconds,
    @param:ConfigDoc(description = "Maximum blob submission transactions sent per tick.", default = "2")
    val maxSubmissionTransactionsPerTick: UInt = 2u,
    // 6 is the current EIP-7594 (Fusaka/PeerDAS) network wrapper cap on blobs per transaction
    // (org.web3j.crypto.transaction.type.Transaction4844.MAX_BLOBS_PER_TRANSACTION). Not enforced
    // here so a future EIP raising the cap doesn't require a config-validation change; a value
    // above the library's cap fails fast when the blob transaction is built.
    @param:ConfigDoc(
      description = "Target number of blobs per submission transaction (EIP-4844/7594 network cap is 6).",
      default = "6",
    )
    val targetBlobsPerTransaction: UInt = 6u,
    @param:ConfigDoc(description = "Maximum number of blobs read from the database per query.", default = "100")
    val dbMaxBlobsToReturn: UInt = 100u,
    @param:ConfigSection("Gas settings for blob submission transactions.")
    val gas: GasConfigToml,
    @param:ConfigSection("Signer used to sign blob submission transactions.")
    val signer: SignerConfigToml,
  )

  data class AggregationSubmissionToml(
    @param:ConfigDoc(description = "Whether aggregation submission is disabled.", default = "false")
    val disabled: Boolean = false,
    @param:ConfigDoc(
      description = "L1 endpoint for aggregation submission. Falls back to defaults.l1-endpoint.",
      example = "http://l1-el-node:8545",
    )
    val l1Endpoint: URL? = null,
    @param:ConfigSection("Retry policy for aggregation requests; falls back to defaults.l1-request-retries.")
    val l1RequestRetries: RequestRetriesToml? = null,
    @param:ConfigDoc(
      description = "Delay before submitting an aggregation after it becomes eligible.",
      default = "PT0S",
    )
    val submissionDelay: Duration = 0.seconds,
    @param:ConfigDoc(description = "Interval between aggregation submission ticks.", default = "PT24S")
    val submissionTickInterval: Duration = 24.seconds,
    @param:ConfigDoc(description = "Maximum aggregation submissions sent per tick.", default = "1")
    val maxSubmissionsPerTick: UInt = 1u,
    @param:ConfigSection("Gas settings for aggregation submission transactions.")
    val gas: GasConfigToml,
    @param:ConfigSection("Signer used to sign aggregation submission transactions.")
    val signer: SignerConfigToml,
  )

  fun reified(
    l1DefaultEndpoint: URL?,
    l1DefaultRequestRetries: RequestRetriesToml,
    timeOfDayMultipliers: TimeOfDayMultipliers,
  ): L1SubmissionConfig {
    return L1SubmissionConfig(
      dynamicGasPriceCap =
      L1SubmissionConfig.DynamicGasPriceCapConfig(
        disabled = this.dynamicGasPriceCap.disabled,
        gasPriceCapCalculation = this.dynamicGasPriceCap.gasPriceCapCalculation.reified(timeOfDayMultipliers),
        feeHistoryFetcher = this.dynamicGasPriceCap.feeHistoryFetcher.reified(
          defaultL1Endpoint = l1DefaultEndpoint,
          defaultL1RequestRetries = l1DefaultRequestRetries,
        ),
        timeOfDayMultipliers = timeOfDayMultipliers,
      ),
      fallbackGasPrice =
      L1SubmissionConfig.FallbackGasPriceConfig(
        feeHistoryBlockCount = this.fallbackGasPrice.feeHistoryBlockCount,
        feeHistoryRewardPercentile = this.fallbackGasPrice.feeHistoryRewardPercentile,
      ),
      blob =
      L1SubmissionConfig.BlobSubmissionConfig(
        disabled = this.blob.disabled,
        l1Endpoint =
        this.blob.l1Endpoint ?: l1DefaultEndpoint
          ?: throw AssertionError("l1Endpoint config missing"),
        l1RequestRetries = this.blob.l1RequestRetries?.asDomain ?: l1DefaultRequestRetries.asDomain,
        submissionDelay = this.blob.submissionDelay,
        submissionTickInterval = this.blob.submissionTickInterval,
        maxSubmissionTransactionsPerTick = this.blob.maxSubmissionTransactionsPerTick,
        targetBlobsPerTransaction = this.blob.targetBlobsPerTransaction,
        dbMaxBlobsToReturn = this.blob.dbMaxBlobsToReturn,
        gas = this.blob.gas.reified(),
        signer = this.blob.signer.reified(),
      ),
      aggregation =
      L1SubmissionConfig.AggregationSubmissionConfig(
        disabled = this.aggregation.disabled,
        l1Endpoint =
        this.aggregation.l1Endpoint ?: l1DefaultEndpoint
          ?: throw AssertionError("l1Endpoint config missing"),
        l1RequestRetries = this.aggregation.l1RequestRetries?.asDomain ?: l1DefaultRequestRetries.asDomain,
        submissionDelay = this.aggregation.submissionDelay,
        submissionTickInterval = this.aggregation.submissionTickInterval,
        maxSubmissionsPerTick = this.aggregation.maxSubmissionsPerTick,
        gas = this.aggregation.gas.reified(),
        signer = this.aggregation.signer.reified(),
      ),
      dataAvailability = this.dataAvailability.reified(),
    )
  }
}
