package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.L2NetworkGasPricingConfigToml
import linea.coordinator.config.v2.toml.RequestRetriesToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

class L2NetWorkingGasPricingConfigParsingTest {
  companion object {
    val toml = """
    [l2-network-gas-pricing]
    disabled = false
    price-update-interval = "PT12S"
    fee-history-block-count = 50
    fee-history-reward-percentile = 15
    gas-price-fixed-cost = 3000000
    l1-endpoint="http://l1-el-node:8545/"
    extra-data-update-endpoint = "http://sequencer:8545/"
    [l2-network-gas-pricing.extra-data-update-request-retries]
    max-retries = 3
    timeout = "PT6S"
    backoff-delay = "PT1S"
    failures-warning-threshold = 2

    [l2-network-gas-pricing.flat-rate-gas-pricing]
    gas-price-upper-bound = 10000000000 # 10 GWEI
    gas-price-lower-bound = 90000000 # 0.09 GWEI
    compressed-tx-size = 125
    expected-gas = 21000

    [l2-network-gas-pricing.dynamic-gas-pricing]
    l1-blob-gas = 131072 # 2^17
    blob-submission-expected-execution-gas = 213000 # Lower to 120k as we improve efficiency
    variable-cost-upper-bound = 10000000001 # ~10 GWEI
    variable-cost-lower-bound = 90000001  # ~0.09 GWEI
    margin = 4.0
    [l2-network-gas-pricing.dynamic-gas-pricing.calldata-based-pricing]
    calldata-sum-size-block-count = 5
    fee-change-denominator = 32
    calldata-sum-size-target = 109000
    block-size-non-calldata-overhead = 540
    """.trimIndent()

    val config = L2NetworkGasPricingConfigToml(
      disabled = false,
      priceUpdateInterval = 12.seconds,
      feeHistoryBlockCount = 50u,
      feeHistoryRewardPercentile = 15u,
      gasPriceFixedCost = 3_000_000UL,
      l1Endpoint = "http://l1-el-node:8545/".toURL(),
      extraDataUpdateEndpoint = "http://sequencer:8545/".toURL(),
      extraDataUpdateRequestRetries = RequestRetriesToml(
        maxRetries = 3u,
        timeout = 6.seconds,
        backoffDelay = 1.seconds,
        failuresWarningThreshold = 2u,
      ),
      dynamicGasPricing = L2NetworkGasPricingConfigToml.DynamicGasPricingToml(
        l1BlobGas = 131072UL, // 2^17
        blobSubmissionExpectedExecutionGas = 213000UL, // Lower to 120k as we improve efficiency
        variableCostUpperBound = 10_000_000_001UL, // ~10 GWEI
        variableCostLowerBound = 90_000_001UL, // ~0.09 GWEI
        margin = 4.0,
        calldataBasedPricing = L2NetworkGasPricingConfigToml.CalldataBasedPricingToml(
          calldataSumSizeBlockCount = 5u,
          feeChangeDenominator = 32u,
          calldataSumSizeTarget = 109000uL,
          blockSizeNonCalldataOverhead = 540u,
        ),
      ),
      flatRateGasPricing = L2NetworkGasPricingConfigToml.FlatRateGasPricingToml(
        gasPriceUpperBound = 10_000_000_000UL, // 10 GWEI
        gasPriceLowerBound = 90_000_000UL, // 0.09 GWEI
        compressedTxSize = 125u,
        expectedGas = 21000u,
      ),
    )

    val tomlMinimal = """
    [l2-network-gas-pricing]
    gas-price-fixed-cost = 3000000
    extra-data-update-endpoint = "http://sequencer:8545/"

    [l2-network-gas-pricing.flat-rate-gas-pricing]
    gas-price-upper-bound = 10000000000 # 10 GWEI
    gas-price-lower-bound = 90000000 # 0.09 GWEI
    compressed-tx-size = 125
    expected-gas = 21000

    [l2-network-gas-pricing.dynamic-gas-pricing]
    l1-blob-gas = 131072 # 2^17
    blob-submission-expected-execution-gas = 213000 # Lower to 120k as we improve efficiency
    variable-cost-upper-bound = 10000000001 # ~10 GWEI
    variable-cost-lower-bound = 90000001  # ~0.09 GWEI
    margin = 4.0
    """.trimIndent()

    val configMinimal = L2NetworkGasPricingConfigToml(
      disabled = false,
      priceUpdateInterval = 12.seconds,
      feeHistoryBlockCount = 1000u,
      feeHistoryRewardPercentile = 15u,
      gasPriceFixedCost = 3_000_000UL,
      l1Endpoint = null,
      extraDataUpdateEndpoint = "http://sequencer:8545/".toURL(),
      extraDataUpdateRequestRetries = RequestRetriesToml(
        maxRetries = null,
        timeout = 8.seconds,
        backoffDelay = 1.seconds,
        failuresWarningThreshold = 3u,
      ),
      dynamicGasPricing = L2NetworkGasPricingConfigToml.DynamicGasPricingToml(
        l1BlobGas = 131072UL, // 2^17
        blobSubmissionExpectedExecutionGas = 213000UL, // Lower to 120k as we improve efficiency
        variableCostUpperBound = 10_000_000_001UL, // ~10 GWEI
        variableCostLowerBound = 90_000_001UL, // ~0.09 GWEI
        margin = 4.0,
      ),
      flatRateGasPricing = L2NetworkGasPricingConfigToml.FlatRateGasPricingToml(
        gasPriceUpperBound = 10_000_000_000UL, // 10 GWEI
        gasPriceLowerBound = 90_000_000UL, // 0.09 GWEI
        compressedTxSize = 125u,
        expectedGas = 21000u,
      ),
    )
  }

  data class WrapperConfig(
    val l2NetworkGasPricing: L2NetworkGasPricingConfigToml,
  )

  @Test
  fun `should parse l2 network gaspricing full config`() {
    assertThat(parseConfig<WrapperConfig>(toml).l2NetworkGasPricing)
      .isEqualTo(config)
  }

  @Test
  fun `should parse l2 network gaspricing minimal config`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).l2NetworkGasPricing)
      .isEqualTo(configMinimal)
  }
}
