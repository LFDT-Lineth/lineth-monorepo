package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.ConflationToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

class ConflationParsingTest {
  companion object {
    val toml = """
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

      [conflation.blob-compression]
      blob-size-limit = 102_400 # 100KB
      handler-polling-interval = "PT1S"
      # default batches limit is  aggregation-proofs-limit -1
      # batches-limit must be less than or equal to aggregation-proofs-limit-1
      batches-limit = 1

      [conflation.proof-aggregation]
      proofs-limit = 3
      deadline = "PT1M"
      coordinator-polling-interval = "PT2S"
      deadline-check-interval = "PT8S"
      target-end-blocks = [10, 20, 30_000]
    """.trimIndent()
    val config = ConflationToml(
      disabled = true,
      blocksLimit = 2u,
      conflationDeadline = 6.seconds,
      conflationDeadlineCheckInterval = 3.seconds,
      conflationDeadlineLastBlockConfirmationDelay = 2.seconds,
      l2FetchBlocksLimit = 4000u,
      l2Endpoint = "http://l2-node-1:8545".toURL(),
      l2LogsEndpoint = "http://l2-node-2:8545".toURL(),
      consistentNumberOfBlocksOnL1ToWait = 1u,
      blobCompression = ConflationToml.BlobCompressionToml(
        blobSizeLimit = 102_400U,
        handlerPollingInterval = 1.seconds,
        batchesLimit = 1u,
      ),
      proofAggregation = ConflationToml.ProofAggregationToml(
        proofsLimit = 3u,
        deadline = 60.seconds,
        coordinatorPollingInterval = 2.seconds,
        deadlineCheckInterval = 8.seconds,
        targetEndBlocks = listOf(10uL, 20uL, 30_000uL),
      ),
    )

    val tomlMinimal = """
      # all fields are optional, defaults will be used if not specified
    """.trimIndent()
    val configMinimal = ConflationToml(
      disabled = false,
      blocksLimit = null,
      conflationDeadline = null,
      l2FetchBlocksLimit = null,
      l2Endpoint = null,
      l2LogsEndpoint = null,
      consistentNumberOfBlocksOnL1ToWait = 32u,
      blobCompression = ConflationToml.BlobCompressionToml(
        blobSizeLimit = 102_400U,
        handlerPollingInterval = 1.seconds,
        batchesLimit = null,
      ),
      proofAggregation = ConflationToml.ProofAggregationToml(
        proofsLimit = 300u,
        deadline = null,
        deadlineCheckInterval = 30.seconds,
        coordinatorPollingInterval = 3.seconds,
        targetEndBlocks = null,
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
}
