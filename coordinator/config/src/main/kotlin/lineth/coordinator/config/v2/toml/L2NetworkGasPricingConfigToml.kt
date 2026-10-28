package lineth.coordinator.config.v2.toml

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import lineth.coordinator.config.v2.L2NetworkGasPricingConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class L2NetworkGasPricingConfigToml(
  @param:ConfigDoc(description = "Whether L2 network gas pricing is disabled.", default = "false")
  val disabled: Boolean = false,
  @param:ConfigDoc(
    description = "L1 endpoint used for gas pricing. Falls back to defaults.l1-endpoint.",
    example = "http://l1-el-node:8545",
  )
  val l1Endpoint: URL? = null,
  @param:ConfigSection("Retry policy for L1 gas-pricing requests; falls back to defaults.l1-request-retries.")
  val l1RequestRetries: RequestRetriesToml? = null,
  @param:ConfigDoc(
    description = "L2 endpoint used for gas pricing. Falls back to defaults.l2-endpoint.",
    example = "http://sequencer:8545",
  )
  val l2Endpoint: URL? = null,
  @param:ConfigSection("Retry policy for L2 gas-pricing requests; falls back to defaults.l2-request-retries.")
  val l2RequestRetries: RequestRetriesToml? = null,
  @param:ConfigDoc(description = "Interval between L2 gas price recalculations/updates.", default = "PT12S")
  val priceUpdateInterval: Duration = 12.seconds,
  @param:ConfigDoc(description = "Number of blocks of fee history used in gas pricing.", default = "1000")
  val feeHistoryBlockCount: UInt = 1000u,
  @param:ConfigDoc(
    description = "Reward percentile (1-100) of fee history used in gas pricing.",
    default = "15",
  )
  val feeHistoryRewardPercentile: UInt = 15u,
  @param:ConfigDoc("Fixed cost component (in wei) added to the computed L2 gas price.")
  val gasPriceFixedCost: ULong,
  @param:ConfigDoc(
    description = "L2 endpoint that receives the computed gas pricing via the extra-data update call.",
    example = "http://sequencer:8545",
  )
  val extraDataUpdateEndpoint: URL,
  @param:ConfigSection("Retry policy for extra-data update requests.")
  val extraDataUpdateRequestRetries: RequestRetriesToml =
    RequestRetriesToml(
      timeout = 8.seconds,
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
  @param:ConfigSection("Dynamic (usage-based) gas pricing parameters.")
  val dynamicGasPricing: DynamicGasPricingToml,
  @param:ConfigSection("Flat-rate gas pricing parameters.")
  val flatRateGasPricing: FlatRateGasPricingToml,
) {
  init {
    require(feeHistoryBlockCount > 0u) { "feeHistoryBlockCount=$feeHistoryBlockCount must be greater than 0" }
    require(feeHistoryRewardPercentile in 1u..100u) {
      "feeHistoryRewardPercentile=$feeHistoryRewardPercentile must be between 1..100"
    }
  }

  data class DynamicGasPricingToml(
    @param:ConfigDoc("Expected L1 blob gas used per blob when computing the variable cost.")
    val l1BlobGas: ULong,
    @param:ConfigDoc("Expected L1 execution gas consumed by a blob submission transaction.")
    val blobSubmissionExpectedExecutionGas: ULong,
    @param:ConfigDoc("Upper bound for the computed variable cost component.")
    val variableCostUpperBound: ULong,
    @param:ConfigDoc("Lower bound for the computed variable cost component.")
    val variableCostLowerBound: ULong,
    @param:ConfigDoc("Multiplier applied to the computed variable cost (safety/profit margin).")
    val margin: Double,
    @param:ConfigSection("Optional calldata-based pricing overrides; omit to disable.")
    val calldataBasedPricing: CalldataBasedPricingToml? = null,
  ) {
    fun reified(): L2NetworkGasPricingConfig.DynamicGasPricing {
      return L2NetworkGasPricingConfig.DynamicGasPricing(
        l1BlobGas = this.l1BlobGas,
        blobSubmissionExpectedExecutionGas = this.blobSubmissionExpectedExecutionGas,
        variableCostUpperBound = this.variableCostUpperBound,
        variableCostLowerBound = this.variableCostLowerBound,
        margin = this.margin,
        calldataBasedPricing = this.calldataBasedPricing?.reified(),
      )
    }
  }

  data class CalldataBasedPricingToml(
    @param:ConfigDoc(description = "Number of recent blocks summed when measuring calldata usage.", default = "5")
    val calldataSumSizeBlockCount: UInt = 5U,
    @param:ConfigDoc(
      description = "Denominator controlling how quickly the fee reacts to calldata-usage deviations.",
      default = "32",
    )
    val feeChangeDenominator: UInt = 32U,
    @param:ConfigDoc(description = "Target total calldata size (bytes) across the measured blocks.", default = "109000")
    val calldataSumSizeTarget: ULong = 109000UL,
    @param:ConfigDoc(description = "Per-block non-calldata size overhead (bytes) in the calculation.", default = "540")
    val blockSizeNonCalldataOverhead: UInt = 540U,
  ) {
    fun reified(): L2NetworkGasPricingConfig.CalldataBasedPricing {
      return L2NetworkGasPricingConfig.CalldataBasedPricing(
        calldataSumSizeBlockCount = this.calldataSumSizeBlockCount,
        feeChangeDenominator = this.feeChangeDenominator,
        calldataSumSizeTarget = this.calldataSumSizeTarget,
        blockSizeNonCalldataOverhead = this.blockSizeNonCalldataOverhead,
      )
    }
  }

  data class FlatRateGasPricingToml(
    @param:ConfigDoc("Lower bound (in wei) for the flat-rate L2 gas price.")
    val gasPriceLowerBound: ULong,
    @param:ConfigDoc("Upper bound (in wei) for the flat-rate L2 gas price.")
    val gasPriceUpperBound: ULong,
    @param:ConfigDoc(
      description = "Multiplier applied to the plain-transfer cost when computing the price.",
      default = "1.0",
    )
    val plainTransferCostMultiplier: Double = 1.0,
    @param:ConfigDoc(description = "Assumed compressed size (bytes) of a transaction.", default = "125")
    val compressedTxSize: UInt = 125u,
    @param:ConfigDoc(description = "Assumed gas used by a plain transfer transaction.", default = "21000")
    val expectedGas: UInt = 21000u,
  ) {
    fun reified(): L2NetworkGasPricingConfig.FlatRateGasPricing {
      return L2NetworkGasPricingConfig.FlatRateGasPricing(
        gasPriceLowerBound = this.gasPriceLowerBound,
        gasPriceUpperBound = this.gasPriceUpperBound,
        plainTransferCostMultiplier = this.plainTransferCostMultiplier,
        compressedTxSize = this.compressedTxSize,
        expectedGas = this.expectedGas,
      )
    }
  }

  fun reified(
    defaults: DefaultsToml,
  ): L2NetworkGasPricingConfig {
    return L2NetworkGasPricingConfig(
      disabled = disabled,
      priceUpdateInterval = this.priceUpdateInterval,
      feeHistoryBlockCount = this.feeHistoryBlockCount,
      feeHistoryRewardPercentile = this.feeHistoryRewardPercentile,
      gasPriceFixedCost = this.gasPriceFixedCost,
      dynamicGasPricing = this.dynamicGasPricing.reified(),
      flatRateGasPricing = this.flatRateGasPricing.reified(),
      extraDataUpdateEndpoint = this.extraDataUpdateEndpoint,
      extraDataUpdateRequestRetries = this.extraDataUpdateRequestRetries.asDomain,
      l1Endpoint = this.l1Endpoint ?: defaults.l1Endpoint ?: throw AssertionError("l1Endpoint must be set"),
      l2Endpoint = this.l2Endpoint ?: defaults.l2Endpoint ?: throw AssertionError("l2Endpoint must be set"),
      l1RequestRetries = this.l1RequestRetries?.asDomain ?: defaults.l1RequestRetries.asDomain,
      l2RequestRetries = this.l2RequestRetries?.asDomain ?: defaults.l2RequestRetries.asDomain,
    )
  }
}
