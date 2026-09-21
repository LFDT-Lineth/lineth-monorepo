package lineth.coordinator.config.v2

import lineth.coordinator.config.v2.toml.FileBasedProverConfigToml
import lineth.coordinator.config.v2.toml.PreRiscvProverToml
import lineth.coordinator.config.v2.toml.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class PreRiscvProverParsingTest {
  companion object {
    val toml =
      """
      [pre-riscv-prover]
      fs-inprogress-request-writing-suffix = ".coordinator_writing_request"
      fs-inprogress-proving-suffix-pattern = "\\.inprogress\\.prover_is_proving.*"
      fs-polling-interval = "PT1S"
      fs-polling-timeout = "PT10M"
      [pre-riscv-prover.execution]
      fs-requests-directory = "/data/prover/v2/execution/requests"
      fs-responses-directory = "/data/prover/v2/execution/responses"
      [pre-riscv-prover.blob-compression]
      fs-requests-directory = "/data/prover/v2/compression/requests"
      fs-responses-directory = "/data/prover/v2/compression/responses"
      [pre-riscv-prover.invalidity]
      fs-requests-directory = "/data/prover/v2/invalidity/requests"
      fs-responses-directory = "/data/prover/v2/invalidity/responses"
      [pre-riscv-prover.proof-aggregation]
      fs-requests-directory = "/data/prover/v2/aggregation/requests"
      fs-responses-directory = "/data/prover/v2/aggregation/responses"

      [pre-riscv-prover.new]
      switch-block-number-inclusive=1000
      [pre-riscv-prover.new.execution]
      fs-requests-directory = "/data/prover/v3/execution/requests"
      fs-responses-directory = "/data/prover/v3/execution/responses"
      [pre-riscv-prover.new.blob-compression]
      fs-requests-directory = "/data/prover/v3/compression/requests"
      fs-responses-directory = "/data/prover/v3/compression/responses"
      [pre-riscv-prover.new.invalidity]
      fs-requests-directory = "/data/prover/v3/invalidity/requests"
      fs-responses-directory = "/data/prover/v3/invalidity/responses"
      [pre-riscv-prover.new.proof-aggregation]
      fs-requests-directory = "/data/prover/v3/aggregation/requests"
      fs-responses-directory = "/data/prover/v3/aggregation/responses"
      """.trimIndent()

    val tomlWithCleanupEnabled =
      """
      [pre-riscv-prover]
      enable-request-files-cleanup = true
      [pre-riscv-prover.execution]
      fs-requests-directory = "/data/prover/v2/execution/requests"
      fs-responses-directory = "/data/prover/v2/execution/responses"
      [pre-riscv-prover.blob-compression]
      fs-requests-directory = "/data/prover/v2/compression/requests"
      fs-responses-directory = "/data/prover/v2/compression/responses"
      [pre-riscv-prover.proof-aggregation]
      fs-requests-directory = "/data/prover/v2/aggregation/requests"
      fs-responses-directory = "/data/prover/v2/aggregation/responses"
      """.trimIndent()

    val config =
      PreRiscvProverToml(
        fsInprogressRequestWritingSuffix = ".coordinator_writing_request",
        fsInprogressProvingSuffixPattern = "\\.inprogress\\.prover_is_proving.*",
        fsPollingInterval = 1.seconds,
        fsPollingTimeout = 10.minutes,
        execution =
        FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/execution/requests",
          fsResponsesDirectory = "/data/prover/v2/execution/responses",
        ),
        blobCompression =
        FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/compression/requests",
          fsResponsesDirectory = "/data/prover/v2/compression/responses",
        ),
        invalidity =
        FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/invalidity/requests",
          fsResponsesDirectory = "/data/prover/v2/invalidity/responses",
        ),
        proofAggregation =
        FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/aggregation/requests",
          fsResponsesDirectory = "/data/prover/v2/aggregation/responses",
        ),
        new =
        PreRiscvProverToml(
          switchBlockNumberInclusive = 1_000u,
          execution =
          FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/prover/v3/execution/requests",
            fsResponsesDirectory = "/data/prover/v3/execution/responses",
          ),
          blobCompression =
          FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/prover/v3/compression/requests",
            fsResponsesDirectory = "/data/prover/v3/compression/responses",
          ),
          invalidity =
          FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/prover/v3/invalidity/requests",
            fsResponsesDirectory = "/data/prover/v3/invalidity/responses",
          ),
          proofAggregation =
          FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/prover/v3/aggregation/requests",
            fsResponsesDirectory = "/data/prover/v3/aggregation/responses",
          ),
        ),
      )

    val tomlMinimal =
      """
      [pre-riscv-prover]
      [pre-riscv-prover.execution]
      fs-requests-directory = "/data/prover/v2/execution/requests"
      fs-responses-directory = "/data/prover/v2/execution/responses"
      [pre-riscv-prover.blob-compression]
      fs-requests-directory = "/data/prover/v2/compression/requests"
      fs-responses-directory = "/data/prover/v2/compression/responses"
      [pre-riscv-prover.proof-aggregation]
      fs-requests-directory = "/data/prover/v2/aggregation/requests"
      fs-responses-directory = "/data/prover/v2/aggregation/responses"
      """.trimIndent()

    val configMinimal =
      PreRiscvProverToml(
        fsInprogressRequestWritingSuffix = ".inprogress_coordinator_writing",
        fsInprogressProvingSuffixPattern = "\\.inprogress\\.prover.*",
        execution = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/execution/requests",
          fsResponsesDirectory = "/data/prover/v2/execution/responses",
        ),
        blobCompression = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/compression/requests",
          fsResponsesDirectory = "/data/prover/v2/compression/responses",
        ),
        invalidity = null,
        proofAggregation = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/aggregation/requests",
          fsResponsesDirectory = "/data/prover/v2/aggregation/responses",
        ),
        switchBlockNumberInclusive = null,
        new = null,
      )

    val configWithCleanupEnabled = configMinimal.copy(enableRequestFilesCleanup = true)
  }

  data class WrapperConfig(
    val preRiscvProver: PreRiscvProverToml,
  )

  @Test
  fun `should parse prover toml configs - full`() {
    assertThat(
      parseConfig<WrapperConfig>(toml).preRiscvProver,
    ).isEqualTo(config)
  }

  @Test
  fun `should parse conflation toml configs and provide defaults`() {
    assertThat(
      parseConfig<WrapperConfig>(tomlMinimal).preRiscvProver,
    ).isEqualTo(configMinimal)
  }

  @Test
  fun `should parse prover toml configs with cleanup enabled`() {
    assertThat(
      parseConfig<WrapperConfig>(tomlWithCleanupEnabled).preRiscvProver,
    ).isEqualTo(configWithCleanupEnabled)
  }

  @Test
  fun `should default cleanup to false when not specified`() {
    val parsed = parseConfig<WrapperConfig>(tomlMinimal).preRiscvProver
    assertThat(parsed.enableRequestFilesCleanup).isFalse()
  }

  @Test
  fun `should parse cleanup setting when explicitly set to false`() {
    val tomlWithCleanupDisabled =
      """
      [pre-riscv-prover]
      enable-request-files-cleanup = false
      [pre-riscv-prover.execution]
      fs-requests-directory = "/data/prover/v2/execution/requests"
      fs-responses-directory = "/data/prover/v2/execution/responses"
      [pre-riscv-prover.blob-compression]
      fs-requests-directory = "/data/prover/v2/compression/requests"
      fs-responses-directory = "/data/prover/v2/compression/responses"
      [pre-riscv-prover.proof-aggregation]
      fs-requests-directory = "/data/prover/v2/aggregation/requests"
      fs-responses-directory = "/data/prover/v2/aggregation/responses"
      """.trimIndent()

    val parsed = parseConfig<WrapperConfig>(tomlWithCleanupDisabled).preRiscvProver
    assertThat(parsed.enableRequestFilesCleanup).isFalse()
  }
}
