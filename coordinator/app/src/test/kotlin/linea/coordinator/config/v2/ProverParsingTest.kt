package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.ProverToml
import linea.coordinator.config.v2.toml.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class ProverParsingTest {
  companion object {
    val toml =
      """
      [prover]
      version = "v2.0.0"
      fs-inprogress-request-writing-suffix = ".coordinator_writing_request"
      fs-inprogress-proving-suffix-pattern = "\\.inprogress\\.prover_is_proving.*"
      fs-polling-interval = "PT1S"
      fs-polling-timeout = "PT10M"
      [prover.execution]
      fs-requests-directory = "/data/prover/v2/execution/requests"
      fs-responses-directory = "/data/prover/v2/execution/responses"
      [prover.blob-compression]
      fs-requests-directory = "/data/prover/v2/compression/requests"
      fs-responses-directory = "/data/prover/v2/compression/responses"
      [prover.proof-aggregation]
      fs-requests-directory = "/data/prover/v2/aggregation/requests"
      fs-responses-directory = "/data/prover/v2/aggregation/responses"

      [prover.new]
      version = "v3.0.0"
      switch-block-number-inclusive=1000
      [prover.new.execution]
      fs-requests-directory = "/data/prover/v3/execution/requests"
      fs-responses-directory = "/data/prover/v3/execution/responses"
      [prover.new.blob-compression]
      fs-requests-directory = "/data/prover/v3/compression/requests"
      fs-responses-directory = "/data/prover/v3/compression/responses"
      [prover.new.proof-aggregation]
      fs-requests-directory = "/data/prover/v3/aggregation/requests"
      fs-responses-directory = "/data/prover/v3/aggregation/responses"
      """.trimIndent()

    val tomlWithCleanupEnabled =
      """
      [prover]
      version = "v2.0.0"
      enable-request-files-cleanup = true
      [prover.execution]
      fs-requests-directory = "/data/prover/v2/execution/requests"
      fs-responses-directory = "/data/prover/v2/execution/responses"
      [prover.blob-compression]
      fs-requests-directory = "/data/prover/v2/compression/requests"
      fs-responses-directory = "/data/prover/v2/compression/responses"
      [prover.proof-aggregation]
      fs-requests-directory = "/data/prover/v2/aggregation/requests"
      fs-responses-directory = "/data/prover/v2/aggregation/responses"
      """.trimIndent()

    val config =
      ProverToml(
        version = "v2.0.0",
        fsInprogressRequestWritingSuffix = ".coordinator_writing_request",
        fsInprogressProvingSuffixPattern = "\\.inprogress\\.prover_is_proving.*",
        fsPollingInterval = 1.seconds,
        fsPollingTimeout = 10.minutes,
        execution =
        ProverToml.ProverDirectoriesToml(
          fsRequestsDirectory = "/data/prover/v2/execution/requests",
          fsResponsesDirectory = "/data/prover/v2/execution/responses",
        ),
        blobCompression =
        ProverToml.ProverDirectoriesToml(
          fsRequestsDirectory = "/data/prover/v2/compression/requests",
          fsResponsesDirectory = "/data/prover/v2/compression/responses",
        ),
        proofAggregation =
        ProverToml.ProverDirectoriesToml(
          fsRequestsDirectory = "/data/prover/v2/aggregation/requests",
          fsResponsesDirectory = "/data/prover/v2/aggregation/responses",
        ),
        new =
        ProverToml(
          switchBlockNumberInclusive = 1_000u,
          version = "v3.0.0",
          execution =
          ProverToml.ProverDirectoriesToml(
            fsRequestsDirectory = "/data/prover/v3/execution/requests",
            fsResponsesDirectory = "/data/prover/v3/execution/responses",
          ),
          blobCompression =
          ProverToml.ProverDirectoriesToml(
            fsRequestsDirectory = "/data/prover/v3/compression/requests",
            fsResponsesDirectory = "/data/prover/v3/compression/responses",
          ),
          proofAggregation =
          ProverToml.ProverDirectoriesToml(
            fsRequestsDirectory = "/data/prover/v3/aggregation/requests",
            fsResponsesDirectory = "/data/prover/v3/aggregation/responses",
          ),
        ),
      )

    val tomlMinimal =
      """
      [prover]
      version = "v2.0.0"
      [prover.execution]
      fs-requests-directory = "/data/prover/v2/execution/requests"
      fs-responses-directory = "/data/prover/v2/execution/responses"
      [prover.blob-compression]
      fs-requests-directory = "/data/prover/v2/compression/requests"
      fs-responses-directory = "/data/prover/v2/compression/responses"
      [prover.proof-aggregation]
      fs-requests-directory = "/data/prover/v2/aggregation/requests"
      fs-responses-directory = "/data/prover/v2/aggregation/responses"
      """.trimIndent()

    val configMinimal =
      ProverToml(
        version = "v2.0.0",
        fsInprogressRequestWritingSuffix = ".inprogress_coordinator_writing",
        fsInprogressProvingSuffixPattern = "\\.inprogress\\.prover.*",
        execution =
        ProverToml.ProverDirectoriesToml(
          fsRequestsDirectory = "/data/prover/v2/execution/requests",
          fsResponsesDirectory = "/data/prover/v2/execution/responses",
        ),
        blobCompression =
        ProverToml.ProverDirectoriesToml(
          fsRequestsDirectory = "/data/prover/v2/compression/requests",
          fsResponsesDirectory = "/data/prover/v2/compression/responses",
        ),
        proofAggregation =
        ProverToml.ProverDirectoriesToml(
          fsRequestsDirectory = "/data/prover/v2/aggregation/requests",
          fsResponsesDirectory = "/data/prover/v2/aggregation/responses",
        ),
        switchBlockNumberInclusive = null,
        new = null,
      )

    val configWithCleanupEnabled = configMinimal.copy(enableRequestFilesCleanup = true)
  }

  data class WrapperConfig(
    val prover: ProverToml,
  )

  @Test
  fun `should parse prover toml configs - full`() {
    assertThat(
      parseConfig<WrapperConfig>(toml).prover,
    ).isEqualTo(config)
  }

  @Test
  fun `should parse conflation toml configs and provide defaults`() {
    assertThat(
      parseConfig<WrapperConfig>(tomlMinimal).prover,
    ).isEqualTo(configMinimal)
  }

  @Test
  fun `should parse prover toml configs with cleanup enabled`() {
    assertThat(
      parseConfig<WrapperConfig>(tomlWithCleanupEnabled).prover,
    ).isEqualTo(configWithCleanupEnabled)
  }

  @Test
  fun `should default cleanup to false when not specified`() {
    val parsed = parseConfig<WrapperConfig>(tomlMinimal).prover
    assertThat(parsed.enableRequestFilesCleanup).isFalse()
  }

  @Test
  fun `should parse cleanup setting when explicitly set to false`() {
    val tomlWithCleanupDisabled =
      """
      [prover]
      version = "v2.0.0"
      enable-request-files-cleanup = false
      [prover.execution]
      fs-requests-directory = "/data/prover/v2/execution/requests"
      fs-responses-directory = "/data/prover/v2/execution/responses"
      [prover.blob-compression]
      fs-requests-directory = "/data/prover/v2/compression/requests"
      fs-responses-directory = "/data/prover/v2/compression/responses"
      [prover.proof-aggregation]
      fs-requests-directory = "/data/prover/v2/aggregation/requests"
      fs-responses-directory = "/data/prover/v2/aggregation/responses"
      """.trimIndent()

    val parsed = parseConfig<WrapperConfig>(tomlWithCleanupDisabled).prover
    assertThat(parsed.enableRequestFilesCleanup).isFalse()
  }
}
