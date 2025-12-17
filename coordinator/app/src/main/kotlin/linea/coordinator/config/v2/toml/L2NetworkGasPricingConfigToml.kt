package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.L2NetworkGasPricingConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class L2NetworkGasPricingConfigToml(
  val disabled: Boolean = false,
  val l1Endpoint: URL? = null,
  val l2Endpoint: URL? = null,
  val priceUpdateInterval: Duration = 12.seconds,
  val feeHistoryBlockCount: UInt = 1000u,
  val feeHistoryRewardPercentile: UInt = 15u,
  val gasPriceFixedCost: ULong,
  val extraDataUpdateEndpoint: URL,
  val extraDataUpdateRequestRetries: RequestRetriesToml =
    RequestRetriesToml(
      timeout = 8.seconds,
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
  val dynamicGasPricing: DynamicGasPricingToml,
  val flatRateGasPricing: FlatRateGasPricingToml,
) {
  init {
    require(feeHistoryBlockCount > 0u) { "feeHistoryBlockCount=$feeHistoryBlockCount must be greater than 0" }
    require(feeHistoryRewardPercentile in 1u..100u) {
      "feeHistoryRewardPercentile=$feeHistoryRewardPercentile must be between 1..100"
    }
  }

  data class DynamicGasPricingToml(
    val l1BlobGas: ULong,
    val blobSubmissionExpectedExecutionGas: ULong,
    val variableCostUpperBound: ULong,
    val variableCostLowerBound: ULong,
    val margin: Double,
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
    val calldataSumSizeBlockCount: UInt = 5U,
    val feeChangeDenominator: UInt = 32U,
    val calldataSumSizeTarget: ULong = 109000UL,
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
    val gasPriceLowerBound: ULong,
    val gasPriceUpperBound: ULong,
    val plainTransferCostMultiplier: Double = 1.0,
    val compressedTxSize: UInt = 125u,
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

  fun reified(l1DefaultEndpoint: URL?, l2DefaultEndpoint: URL?): L2NetworkGasPricingConfig {
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
      l1Endpoint = this.l1Endpoint ?: l1DefaultEndpoint ?: throw AssertionError("l1Endpoint must be set"),
      l2Endpoint = this.l2Endpoint ?: l2DefaultEndpoint ?: throw AssertionError("l2Endpoint must be set"),
    )
  }
}
