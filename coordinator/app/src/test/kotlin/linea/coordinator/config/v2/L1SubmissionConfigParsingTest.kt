package linea.coordinator.config.v2

import com.sksamuel.hoplite.Masked
import linea.coordinator.config.v2.toml.L1SubmissionConfigToml
import linea.coordinator.config.v2.toml.SignerConfigToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.kotlin.decodeHex
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.nio.file.Path
import kotlin.time.Duration.Companion.days
import kotlin.time.Duration.Companion.hours
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class L1SubmissionConfigParsingTest {
  companion object {
    val toml =
      """
      [l1-submission]
      disabled = true
      data-availability = "VALIDIUM"
      [l1-submission.dynamic-gas-price-cap]
      disabled = true
      [l1-submission.dynamic-gas-price-cap.gas-price-cap-calculation]
      adjustment-constant = 25
      blob-adjustment-constant = 25
      finalization-target-max-delay = "PT32H"
      base-fee-per-gas-percentile-window = "P7D"
      base-fee-per-gas-percentile-window-leeway = "PT10M"
      base-fee-per-gas-percentile = 10
      gas-price-caps-check-coefficient = 0.9
      historic-base-fee-per-blob-gas-lower-bound=100000011 # 0.1 GWEI
      historic-avg-reward-constant=100000012 # 0.1 GWEI
      [l1-submission.dynamic-gas-price-cap.fee-history-fetcher]
      l1-endpoint = "http://l1-node:8545"
      fetch-interval = "PT1S"
      max-block-count = 1000
      reward-percentiles = [10, 20, 30, 40, 50, 60, 70, 80, 90, 100]
      num-of-blocks-before-latest = 2
      storage-period = "P10D"

      [l1-submission.fallback-gas-price]
      fee-history-block-count = 10
      fee-history-reward-percentile = 15

      [l1-submission.blob]
      disabled = false
      l1-endpoint = "http://l1-el-node:8545"
      submission-delay = "PT1S"
      submission-tick-interval = "PT10S"
      max-submission-transactions-per-tick = 10
      target-blobs-per-transaction=9
      db-max-blobs-to-return = 100
      [l1-submission.blob.gas]
      gas-limit = 10_000_000
      max-fee-per-gas-cap = 100_000_000_000
      max-fee-per-blob-gas-cap = 100_000_000
      max-priority-fee-per-gas-cap=10_000_000_000
      # Note: prefixed with "fallback-", used when dynamic gas price is disabled or DB is not populated yet
      [l1-submission.blob.gas.fallback]
      priority-fee-per-gas-upper-bound = 20_000_000_000 # 20 GWEI
      priority-fee-per-gas-lower-bound = 2_000_000_000 # 2 GWEI

      [l1-submission.blob.signer]
      # Web3j/Web3signer
      type = "Web3signer"

      # The account with this private key is in genesis file
      [l1-submission.blob.signer.web3j]
      private-key = "0x0000000000000000000000000000000000000000000000000000000000000001"

      [l1-submission.blob.signer.web3signer]
      endpoint = "https://web3signer:9000"
      max-pool-size = 10
      keep-alive = true
      public-key = "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"
      [l1-submission.blob.signer.web3signer.tls]
      key-store-path = "coordinator-client-keystore.p12"
      key-store-password = "xxxxx"
      trust-store-path = "web3signer-truststore.p12"
      trust-store-password = "xxxxx"

      [l1-submission.aggregation]
      disabled = false
      l1-endpoint = "http://l1-el-node:8545"
      submission-delay = "PT2S"
      submission-tick-interval = "PT12S"
      max-submissions-per-tick = 10
      [l1-submission.aggregation.gas]
      gas-limit = 10_000_001
      max-fee-per-gas-cap = 100_000_000_001
      max-priority-fee-per-gas-cap = 10_000_000_001

      [l1-submission.aggregation.gas.fallback]
      # Note: prefixed with "fallback-", used when dynamic gas price is disabled or DB is not populated yet
      priority-fee-per-gas-upper-bound = 20_000_000_001 # 20 GWEI
      priority-fee-per-gas-lower-bound = 2_000_000_001 # 2 GWEI

      [l1-submission.aggregation.signer]
      # Web3j/Web3signer
      type = "Web3signer"

      [l1-submission.aggregation.signer.web3j]
      private-key = "0x0000000000000000000000000000000000000000000000000000000000000002"

      [l1-submission.aggregation.signer.web3signer]
      endpoint = "https://web3signer:9000"
      max-pool-size = 10
      keep-alive = true
      public-key = "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002"
      [l1-submission.aggregation.signer.web3signer.tls]
      key-store-path = "coordinator-client-keystore.p12"
      key-store-password = "xxxxx"
      trust-store-path = "web3signer-truststore.p12"
      trust-store-password = "xxxxx"
      """.trimIndent()

    val config =
      L1SubmissionConfigToml(
        disabled = true,
        dataAvailability = L1SubmissionConfigToml.DataAvailability.VALIDIUM,
        dynamicGasPriceCap =
        L1SubmissionConfigToml.DynamicGasPriceCapToml(
          disabled = true,
          gasPriceCapCalculation =
          L1SubmissionConfigToml.DynamicGasPriceCapToml.GasPriceCapCalculationToml(
            adjustmentConstant = 25u,
            blobAdjustmentConstant = 25u,
            finalizationTargetMaxDelay = 32.hours,
            baseFeePerGasPercentileWindow = 7.days,
            baseFeePerGasPercentileWindowLeeway = 10.minutes,
            baseFeePerGasPercentile = 10u,
            gasPriceCapsCheckCoefficient = 0.9,
            historicBaseFeePerBlobGasLowerBound = 100000011UL,
            historicAvgRewardConstant = 100000012UL,
          ),
          feeHistoryFetcher =
          L1SubmissionConfigToml.DynamicGasPriceCapToml.FeeHistoryFetcherConfig(
            l1Endpoint = "http://l1-node:8545".toURL(),
            fetchInterval = 1.seconds,
            maxBlockCount = 1000u,
            rewardPercentiles = listOf(10, 20, 30, 40, 50, 60, 70, 80, 90, 100).map { it.toUInt() },
            numOfBlocksBeforeLatest = 2u,
            storagePeriod = 10.days,
          ),
        ),
        fallbackGasPrice =
        L1SubmissionConfigToml.FallbackGasPriceToml(
          feeHistoryBlockCount = 10u,
          feeHistoryRewardPercentile = 15u,
        ),
        blob =
        L1SubmissionConfigToml.BlobSubmissionConfigToml(
          disabled = false,
          l1Endpoint = "http://l1-el-node:8545".toURL(),
          submissionDelay = 1.seconds,
          submissionTickInterval = 10.seconds,
          maxSubmissionTransactionsPerTick = 10u,
          targetBlobsPerTransaction = 9u,
          dbMaxBlobsToReturn = 100u,
          gas =
          L1SubmissionConfigToml.GasConfigToml(
            gasLimit = 10_000_000u,
            maxFeePerGasCap = 100_000_000_000u,
            maxFeePerBlobGasCap = 100_000_000u,
            maxPriorityFeePerGasCap = 10_000_000_000u,
            fallback =
            L1SubmissionConfigToml.GasConfigToml.FallbackGasConfig(
              priorityFeePerGasUpperBound = 20_000_000_000u,
              priorityFeePerGasLowerBound = 2_000_000_000u,
            ),
          ),
          signer =
          SignerConfigToml(
            type = SignerConfigToml.SignerType.WEB3SIGNER,
            web3j =
            SignerConfigToml.Web3jConfig(
              privateKey = Masked("0x0000000000000000000000000000000000000000000000000000000000000001"),
            ),
            web3signer =
            SignerConfigToml.Web3SignerConfig(
              endpoint = "https://web3signer:9000".toURL(),
              publicKey =
              (
                "0000000000000000000000000000000000000000000000000000000000000000" +
                  "0000000000000000000000000000000000000000000000000000000000000001"
                ).decodeHex(),
              maxPoolSize = 10,
              keepAlive = true,
              tls =
              SignerConfigToml.Web3SignerConfig.TlsConfig(
                keyStorePath = Path.of("coordinator-client-keystore.p12"),
                keyStorePassword = Masked("xxxxx"),
                trustStorePath = Path.of("web3signer-truststore.p12"),
                trustStorePassword = Masked("xxxxx"),
              ),
            ),
          ),
        ),
        aggregation =
        L1SubmissionConfigToml.AggregationSubmissionToml(
          disabled = false,
          l1Endpoint = "http://l1-el-node:8545".toURL(),
          submissionDelay = 2.seconds,
          submissionTickInterval = 12.seconds,
          maxSubmissionsPerTick = 10u,
          gas =
          L1SubmissionConfigToml.GasConfigToml(
            gasLimit = 10_000_001u,
            maxFeePerGasCap = 100_000_000_001u,
            maxPriorityFeePerGasCap = 10_000_000_001u,
            fallback =
            L1SubmissionConfigToml.GasConfigToml.FallbackGasConfig(
              priorityFeePerGasUpperBound = 20_000_000_001u,
              priorityFeePerGasLowerBound = 2_000_000_001u,
            ),
          ),
          signer =
          SignerConfigToml(
            type = SignerConfigToml.SignerType.WEB3SIGNER,
            web3j =
            SignerConfigToml.Web3jConfig(
              privateKey = Masked("0x0000000000000000000000000000000000000000000000000000000000000002"),
            ),
            web3signer =
            SignerConfigToml.Web3SignerConfig(
              endpoint = "https://web3signer:9000".toURL(),
              publicKey =
              (
                "0000000000000000000000000000000000000000000000000000000000000000" +
                  "0000000000000000000000000000000000000000000000000000000000000002"
                ).decodeHex(),
              maxPoolSize = 10,
              keepAlive = true,
              tls =
              SignerConfigToml.Web3SignerConfig.TlsConfig(
                keyStorePath = Path.of("coordinator-client-keystore.p12"),
                keyStorePassword = Masked("xxxxx"),
                trustStorePath = Path.of("web3signer-truststore.p12"),
                trustStorePassword = Masked("xxxxx"),
              ),
            ),
          ),
        ),
      )

    val tomlMinimal =
      """
      [l1-submission]
      [l1-submission.dynamic-gas-price-cap]
      [l1-submission.dynamic-gas-price-cap.gas-price-cap-calculation]
      adjustment-constant = 25
      blob-adjustment-constant = 25
      finalization-target-max-delay = "PT32H"
      base-fee-per-gas-percentile-window = "P7D"
      base-fee-per-gas-percentile-window-leeway = "PT10M"
      base-fee-per-gas-percentile = 10
      gas-price-caps-check-coefficient = 0.9
      historic-base-fee-per-blob-gas-lower-bound=100000011 # 0.1 GWEI
      historic-avg-reward-constant=100000012 # 0.1 GWEI
      [l1-submission.dynamic-gas-price-cap.gas-price-cap-calculation]

      [l1-submission.fallback-gas-price]
      fee-history-block-count = 10
      fee-history-reward-percentile = 15

      [l1-submission.blob]
      [l1-submission.blob.gas]
      gas-limit = 10_000_000
      max-fee-per-gas-cap = 100_000_000_001
      max-fee-per-blob-gas-cap = 100_000_000
      max-priority-fee-per-gas-cap=10_000_000_000
      [l1-submission.blob.gas.fallback]
      priority-fee-per-gas-upper-bound = 20_000_000_000 # 20 GWEI
      priority-fee-per-gas-lower-bound = 2_000_000_000 # 2 GWEI

      [l1-submission.blob.signer]
      type = "Web3j"
      [l1-submission.blob.signer.web3j]
      private-key = "0x0000000000000000000000000000000000000000000000000000000000000001"

      [l1-submission.aggregation]
      [l1-submission.aggregation.gas]
      gas-limit = 10_000_001
      max-fee-per-gas-cap = 100_000_000_011
      max-priority-fee-per-gas-cap = 10_000_000_010

      [l1-submission.aggregation.gas.fallback]
      priority-fee-per-gas-upper-bound = 20_000_000_001 # 20 GWEI
      priority-fee-per-gas-lower-bound = 2_000_000_001 # 2 GWEI

      [l1-submission.aggregation.signer]
      type = "Web3signer"
      [l1-submission.aggregation.signer.web3signer]
      endpoint = "http://web3signer:9000"
      max-pool-size = 10
      keep-alive = true
      public-key = "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002"
      """.trimIndent()

    val configMinimal =
      L1SubmissionConfigToml(
        disabled = false,
        dynamicGasPriceCap =
        L1SubmissionConfigToml.DynamicGasPriceCapToml(
          disabled = false,
          gasPriceCapCalculation =
          L1SubmissionConfigToml.DynamicGasPriceCapToml.GasPriceCapCalculationToml(
            adjustmentConstant = 25u,
            blobAdjustmentConstant = 25u,
            finalizationTargetMaxDelay = 32.hours,
            baseFeePerGasPercentileWindow = 7.days,
            baseFeePerGasPercentileWindowLeeway = 10.minutes,
            baseFeePerGasPercentile = 10u,
            gasPriceCapsCheckCoefficient = 0.9,
            historicBaseFeePerBlobGasLowerBound = 100000011UL,
            historicAvgRewardConstant = 100000012UL,
          ),
          feeHistoryFetcher =
          L1SubmissionConfigToml.DynamicGasPriceCapToml.FeeHistoryFetcherConfig(
            fetchInterval = 3.seconds,
            maxBlockCount = 1000u,
            rewardPercentiles = listOf(10, 20, 30, 40, 50, 60, 70, 80, 90, 100).map { it.toUInt() },
            numOfBlocksBeforeLatest = 4u,
            storagePeriod = 10.days,
          ),
        ),
        fallbackGasPrice =
        L1SubmissionConfigToml.FallbackGasPriceToml(
          feeHistoryBlockCount = 10u,
          feeHistoryRewardPercentile = 15u,
        ),
        blob =
        L1SubmissionConfigToml.BlobSubmissionConfigToml(
          disabled = false,
          l1Endpoint = null,
          submissionDelay = 0.seconds,
          submissionTickInterval = 12.seconds,
          maxSubmissionTransactionsPerTick = 2u,
          targetBlobsPerTransaction = 7u,
          dbMaxBlobsToReturn = 100u,
          gas =
          L1SubmissionConfigToml.GasConfigToml(
            gasLimit = 10_000_000u,
            maxFeePerGasCap = 100_000_000_001u,
            maxFeePerBlobGasCap = 100_000_000u,
            maxPriorityFeePerGasCap = 10_000_000_000u,
            fallback =
            L1SubmissionConfigToml.GasConfigToml.FallbackGasConfig(
              priorityFeePerGasUpperBound = 20_000_000_000u,
              priorityFeePerGasLowerBound = 2_000_000_000u,
            ),
          ),
          signer =
          SignerConfigToml(
            type = SignerConfigToml.SignerType.WEB3J,
            web3j =
            SignerConfigToml.Web3jConfig(
              privateKey = Masked("0x0000000000000000000000000000000000000000000000000000000000000001"),
            ),
            web3signer = null,
          ),
        ),
        aggregation =
        L1SubmissionConfigToml.AggregationSubmissionToml(
          disabled = false,
          l1Endpoint = null,
          submissionDelay = 0.seconds,
          submissionTickInterval = 24.seconds,
          maxSubmissionsPerTick = 1u,
          gas =
          L1SubmissionConfigToml.GasConfigToml(
            gasLimit = 10_000_001u,
            maxFeePerGasCap = 100_000_000_011u,
            maxPriorityFeePerGasCap = 10_000_000_010u,
            fallback =
            L1SubmissionConfigToml.GasConfigToml.FallbackGasConfig(
              priorityFeePerGasUpperBound = 20_000_000_001u,
              priorityFeePerGasLowerBound = 2_000_000_001u,
            ),
          ),
          signer =
          SignerConfigToml(
            type = SignerConfigToml.SignerType.WEB3SIGNER,
            web3j = null,
            web3signer =
            SignerConfigToml.Web3SignerConfig(
              endpoint = "http://web3signer:9000".toURL(),
              publicKey =
              (
                "0000000000000000000000000000000000000000000000000000000000000000" +
                  "0000000000000000000000000000000000000000000000000000000000000002"
                ).decodeHex(),
              maxPoolSize = 10,
              keepAlive = true,
              tls = null,
            ),
          ),
        ),
      )
  }

  data class WrapperConfig(
    val l1Submission: L1SubmissionConfigToml,
  )

  @Test
  fun `should parse l1 submission full config`() {
    assertThat(parseConfig<WrapperConfig>(toml).l1Submission)
      .isEqualTo(config)
  }

  @Test
  fun `should parse l1 submission minimal config`() {
    val config = parseConfig<WrapperConfig>(tomlMinimal)
    assertThat(config.l1Submission).isEqualTo(configMinimal)
  }
}
