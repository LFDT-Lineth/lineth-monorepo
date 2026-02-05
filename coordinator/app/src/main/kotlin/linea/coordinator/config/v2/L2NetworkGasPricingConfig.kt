package linea.coordinator.config.v2

import linea.domain.RetryConfig
import java.net.URL
import kotlin.time.Duration

data class L2NetworkGasPricingConfig(
  override val disabled: Boolean,
  val priceUpdateInterval: Duration,
  val feeHistoryBlockCount: UInt,
  val feeHistoryRewardPercentile: UInt,
  val gasPriceFixedCost: ULong,
  val dynamicGasPricing: DynamicGasPricing,
  val flatRateGasPricing: FlatRateGasPricing,
  val extraDataUpdateEndpoint: URL,
  val extraDataUpdateRequestRetries: RetryConfig,
  val l1Endpoint: URL,
  val l1RequestRetries: RetryConfig,
  val l2Endpoint: URL,
  val l2RequestRetries: RetryConfig,
) : FeatureToggle {
  data class DynamicGasPricing(
    val l1BlobGas: ULong,
    val blobSubmissionExpectedExecutionGas: ULong,
    val variableCostUpperBound: ULong,
    val variableCostLowerBound: ULong,
    val margin: Double,
    val calldataBasedPricing: CalldataBasedPricing?,
  )

  data class CalldataBasedPricing(
    val calldataSumSizeBlockCount: UInt = 5u,
    val feeChangeDenominator: UInt = 32u,
    val calldataSumSizeTarget: ULong = 109000uL,
    val blockSizeNonCalldataOverhead: UInt = 540u,
  )

  data class FlatRateGasPricing(
    val gasPriceLowerBound: ULong,
    val gasPriceUpperBound: ULong,
    val plainTransferCostMultiplier: Double = 1.0,
    val compressedTxSize: UInt = 125u,
    val expectedGas: UInt = 21000u,
  )
}
