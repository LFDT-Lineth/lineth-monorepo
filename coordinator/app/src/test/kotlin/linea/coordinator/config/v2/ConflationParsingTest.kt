package linea.coordinator.config.v2

import com.sksamuel.hoplite.ConfigException
import kotlinx.datetime.Instant
import linea.coordinator.config.v2.toml.ConflationToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import kotlin.time.Duration.Companion.seconds

class ConflationParsingTest {
  companion object {
    val toml =
      """
      [conflation]
      disabled = true
      blocks-limit = 2
      conflation-deadline = "PT6S"
      conflation-deadline-check-interval = "PT3S"
      conflation-deadline-last-block-confirmation-delay = "PT2S" # recommended: at least 2 * blockInterval
      l2-fetch-blocks-limit = 4_000
      l2-endpoint = "http://l2-node-1:8545"
      l2-logs-endpoint = "http://l2-node-2:8545"
      consistent-number-of-blocks-on-l1-to-wait = 1
      force-stop-conflation-at-block-inclusive = 5000
      force-stop-conflation-at-block-timestamp-inclusive= 1758083130


      [conflation.blob-compression]
      blob-size-limit = 102_400 # 100KB
      handler-polling-interval = "PT1S"
      # default batches limit is  aggregation-proofs-limit -1
      # batches-limit must be less than or equal to aggregation-proofs-limit-1
      batches-limit = 1

      [conflation.proof-aggregation]
      proofs-limit = 4
      blobs-limit = 2
      deadline = "PT1M"
      coordinator-polling-interval = "PT2S"
      deadline-check-interval = "PT8S"
      target-end-blocks = [10, 20, 30_000]
      timestamp-based-hard-forks = ["2024-01-15T12:00:00Z", "2024-06-01T16:00:00Z", 1758083127]
      wait-for-no-l2-activity-to-trigger-aggregation = true,
      """.trimIndent()
    val config =
      ConflationToml(
        disabled = true,
        blocksLimit = 2u,
        conflationDeadline = 6.seconds,
        conflationDeadlineCheckInterval = 3.seconds,
        conflationDeadlineLastBlockConfirmationDelay = 2.seconds,
        l2FetchBlocksLimit = 4000u,
        l2Endpoint = "http://l2-node-1:8545".toURL(),
        l2LogsEndpoint = "http://l2-node-2:8545".toURL(),
        consistentNumberOfBlocksOnL1ToWait = 1u,
        forceStopConflationAtBlockInclusive = 5000u,
        forceStopConflationAtBlockTimestampInclusive = Instant.fromEpochSeconds(1758083130),
        blobCompression =
        ConflationToml.BlobCompressionToml(
          blobSizeLimit = 102_400U,
          handlerPollingInterval = 1.seconds,
          batchesLimit = 1u,
        ),
        proofAggregation =
        ConflationToml.ProofAggregationToml(
          proofsLimit = 4u,
          blobsLimit = 2u,
          deadline = 60.seconds,
          coordinatorPollingInterval = 2.seconds,
          deadlineCheckInterval = 8.seconds,
          targetEndBlocks = listOf(10uL, 20uL, 30_000uL),
          timestampBasedHardForks =
          listOf(
            Instant.parse("2024-01-15T12:00:00Z"),
            Instant.parse("2024-06-01T16:00:00Z"),
            Instant.fromEpochSeconds(1758083127L),
          ),
          waitForNoL2ActivityToTriggerAggregation = true,
        ),
      )

    val tomlMinimal =
      """
      # all fields are optional, defaults will be used if not specified
      """.trimIndent()
    val configMinimal =
      ConflationToml(
        disabled = false,
        blocksLimit = null,
        conflationDeadline = null,
        l2FetchBlocksLimit = null,
        l2Endpoint = null,
        l2LogsEndpoint = null,
        consistentNumberOfBlocksOnL1ToWait = 32u,
        blobCompression =
        ConflationToml.BlobCompressionToml(
          blobSizeLimit = 102_400U,
          handlerPollingInterval = 1.seconds,
          batchesLimit = null,
        ),
        proofAggregation =
        ConflationToml.ProofAggregationToml(
          proofsLimit = 300u,
          deadline = null,
          deadlineCheckInterval = 30.seconds,
          coordinatorPollingInterval = 3.seconds,
          targetEndBlocks = null,
          timestampBasedHardForks = emptyList(),
        ),
      )
  }

  data class WrapperConfig(
    val conflation: ConflationToml = ConflationToml(),
  )

  @Test
  fun `should parse conflation toml configs - full`() {
    assertThat(
      parseConfig<WrapperConfig>(toml).conflation,
    )
      .isEqualTo(config)
  }

  @Test
  fun `should parse conflation toml configs - minimal`() {
    assertThat(
      parseConfig<WrapperConfig>(tomlMinimal).conflation,
    )
      .isEqualTo(configMinimal)
  }

  @Test
  fun `should throw when proofsLimit and batchesLimit constraint is violated `() {
    val toml1 =
      """
      [conflation.proof-aggregation]
      proofs-limit = 0
      """.trimIndent()
    val toml2 =
      """
      [conflation.blob-compression]
      batches-limit = 3
      [conflation.proof-aggregation]
      proofs-limit = 3
      """.trimIndent()
    val toml3 =
      """
      [conflation.blob-compression]
      batches-limit = 4
      [conflation.proof-aggregation]
      proofs-limit = 3
      """.trimIndent()
    for (toml in listOf(toml1, toml2, toml3)) {
      assertThrows<ConfigException>(
        "Aggregation proofsLimit must be greater than or equal to Blobs batchesLimit + 1",
      ) {
        parseConfig<WrapperConfig>(toml)
      }
    }
  }
}
