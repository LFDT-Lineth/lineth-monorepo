package lineth.coordinator.config.v2

import lineth.coordinator.config.v2.toml.ProverToml
import lineth.coordinator.config.v2.toml.ProverToml.FileBasedProverConfigToml
import lineth.coordinator.config.v2.toml.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class ProverParsingTest {
  companion object {
    val preRiscvToml =
      """
      [prover]
      type = "pre_riscv"
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
      [prover.invalidity]
      fs-requests-directory = "/data/prover/v2/invalidity/requests"
      fs-responses-directory = "/data/prover/v2/invalidity/responses"
      [prover.proof-aggregation]
      fs-requests-directory = "/data/prover/v2/aggregation/requests"
      fs-responses-directory = "/data/prover/v2/aggregation/responses"

      [prover.new]
      switch-block-number-inclusive=1000
      [prover.new.execution]
      fs-requests-directory = "/data/prover/v3/execution/requests"
      fs-responses-directory = "/data/prover/v3/execution/responses"
      [prover.new.blob-compression]
      fs-requests-directory = "/data/prover/v3/compression/requests"
      fs-responses-directory = "/data/prover/v3/compression/responses"
      [prover.new.invalidity]
      fs-requests-directory = "/data/prover/v3/invalidity/requests"
      fs-responses-directory = "/data/prover/v3/invalidity/responses"
      [prover.new.proof-aggregation]
      fs-requests-directory = "/data/prover/v3/aggregation/requests"
      fs-responses-directory = "/data/prover/v3/aggregation/responses"
      """.trimIndent()

    val preRiscvTomlWithCleanupEnabled =
      """
      [prover]
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

    val preRiscvConfig =
      ProverToml(
        fsInprogressRequestWritingSuffix = ".coordinator_writing_request",
        fsInprogressProvingSuffixPattern = "\\.inprogress\\.prover_is_proving.*",
        fsPollingInterval = 1.seconds,
        fsPollingTimeout = 10.minutes,
        execution = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/execution/requests",
          fsResponsesDirectory = "/data/prover/v2/execution/responses",
        ),
        blobCompression = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/compression/requests",
          fsResponsesDirectory = "/data/prover/v2/compression/responses",
        ),
        invalidity = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/invalidity/requests",
          fsResponsesDirectory = "/data/prover/v2/invalidity/responses",
        ),
        proofAggregation = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/aggregation/requests",
          fsResponsesDirectory = "/data/prover/v2/aggregation/responses",
        ),
        new =
        ProverToml(
          switchBlockNumberInclusive = 1_000u,
          execution = FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/prover/v3/execution/requests",
            fsResponsesDirectory = "/data/prover/v3/execution/responses",
          ),
          blobCompression = FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/prover/v3/compression/requests",
            fsResponsesDirectory = "/data/prover/v3/compression/responses",
          ),
          invalidity = FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/prover/v3/invalidity/requests",
            fsResponsesDirectory = "/data/prover/v3/invalidity/responses",
          ),
          proofAggregation = FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/prover/v3/aggregation/requests",
            fsResponsesDirectory = "/data/prover/v3/aggregation/responses",
          ),
        ),
      )

    val preRiscvTomlMinimal =
      """
      [prover]
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

    val preRiscvConfigMinimal =
      ProverToml(
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

    val preRiscvConfigWithCleanupEnabled = preRiscvConfigMinimal.copy(enableRequestFilesCleanup = true)

    val toml =
      """
      [prover]
      type = "riscv"
      proving-system-version = "0xabcdef123"
      fork-name = "amsterdam"
      fs-polling-interval = "PT1S"
      fs-polling-timeout = "PT10M"
      fs-inprogress-request-writing-suffix = ".inprogress_coordinator_riscv_writing"
      fs-inprogress-proving-suffix-pattern = "\\.inprogress\\.riscv-prover.*"
      enable-request-files-cleanup = true
      [prover.l2-execution]
      programId = "0xdeadbeef1"
      fs-requests-directory = "/data/riscv-prover/execution/requests"
      fs-responses-directory = "/data/riscv-prover/execution/responses"
      [prover.rollup]
      programId = "0xdeadbeef2"
      fs-requests-directory = "/data/riscv-prover/rollup/requests"
      fs-responses-directory = "/data/riscv-prover/rollup/responses"
      [prover.rollup-aggregation]
      programId = "0xdeadbeef3"
      fs-requests-directory = "/data/riscv-prover/aggregation/requests"
      fs-responses-directory = "/data/riscv-prover/aggregation/responses"
      """.trimIndent()

    val config =
      ProverToml(
        type = ProverToml.ProverType.RISCV,
        fsPollingInterval = 1.seconds,
        fsPollingTimeout = 10.minutes,
        forkName = "amsterdam",
        provingSystemVersion = "0xabcdef123",
        fsInprogressRequestWritingSuffix = ".inprogress_coordinator_riscv_writing",
        fsInprogressProvingSuffixPattern = "\\.inprogress\\.riscv-prover.*",
        enableRequestFilesCleanup = true,
        l2Execution = FileBasedProverConfigToml(
          programId = "0xdeadbeef1",
          fsRequestsDirectory = "/data/riscv-prover/execution/requests",
          fsResponsesDirectory = "/data/riscv-prover/execution/responses",
        ),
        rollup = FileBasedProverConfigToml(
          programId = "0xdeadbeef2",
          fsRequestsDirectory = "/data/riscv-prover/rollup/requests",
          fsResponsesDirectory = "/data/riscv-prover/rollup/responses",
        ),
        rollupAggregation = FileBasedProverConfigToml(
          programId = "0xdeadbeef3",
          fsRequestsDirectory = "/data/riscv-prover/aggregation/requests",
          fsResponsesDirectory = "/data/riscv-prover/aggregation/responses",
        ),
      )

    val tomlMinimal =
      """
      [prover]
      type = "riscv"
      proving-system-version = "0xabcdef123"
      fork-name = "amsterdam"
      [prover.l2-execution]
      programId = "0xdeadbeef1"
      fs-requests-directory = "/data/riscv-prover/execution/requests"
      fs-responses-directory = "/data/riscv-prover/execution/responses"
      [prover.rollup]
      programId = "0xdeadbeef2"
      fs-requests-directory = "/data/riscv-prover/rollup/requests"
      fs-responses-directory = "/data/riscv-prover/rollup/responses"
      [prover.rollup-aggregation]
      programId = "0xdeadbeef3"
      fs-requests-directory = "/data/riscv-prover/aggregation/requests"
      fs-responses-directory = "/data/riscv-prover/aggregation/responses"
      """.trimIndent()

    val configMinimal =
      ProverToml(
        type = ProverToml.ProverType.RISCV,
        forkName = "amsterdam",
        provingSystemVersion = "0xabcdef123",
        l2Execution = FileBasedProverConfigToml(
          programId = "0xdeadbeef1",
          fsRequestsDirectory = "/data/riscv-prover/execution/requests",
          fsResponsesDirectory = "/data/riscv-prover/execution/responses",
        ),
        rollup = FileBasedProverConfigToml(
          programId = "0xdeadbeef2",
          fsRequestsDirectory = "/data/riscv-prover/rollup/requests",
          fsResponsesDirectory = "/data/riscv-prover/rollup/responses",
        ),
        rollupAggregation = FileBasedProverConfigToml(
          programId = "0xdeadbeef3",
          fsRequestsDirectory = "/data/riscv-prover/aggregation/requests",
          fsResponsesDirectory = "/data/riscv-prover/aggregation/responses",
        ),
      )

    val switchPreRiscvToRiscvToml =
      """
      [prover]
      [prover.execution]
      fs-requests-directory = "/data/prover/v2/execution/requests"
      fs-responses-directory = "/data/prover/v2/execution/responses"
      [prover.blob-compression]
      fs-requests-directory = "/data/prover/v2/compression/requests"
      fs-responses-directory = "/data/prover/v2/compression/responses"
      [prover.invalidity]
      fs-requests-directory = "/data/prover/v2/invalidity/requests"
      fs-responses-directory = "/data/prover/v2/invalidity/responses"
      [prover.proof-aggregation]
      fs-requests-directory = "/data/prover/v2/aggregation/requests"
      fs-responses-directory = "/data/prover/v2/aggregation/responses"
      [prover.new]
      type = "riscv"
      switch-block-timestamp = 1000
      proving-system-version = "0xabcdef123"
      fork-name = "amsterdam"
      [prover.new.l2-execution]
      programId = "0xdeadbeef1"
      fs-requests-directory = "/data/riscv-prover/execution/requests"
      fs-responses-directory = "/data/riscv-prover/execution/responses"
      [prover.new.rollup]
      programId = "0xdeadbeef2"
      fs-requests-directory = "/data/riscv-prover/rollup/requests"
      fs-responses-directory = "/data/riscv-prover/rollup/responses"
      [prover.new.rollup-aggregation]
      programId = "0xdeadbeef3"
      fs-requests-directory = "/data/riscv-prover/aggregation/requests"
      fs-responses-directory = "/data/riscv-prover/aggregation/responses"
      """.trimIndent()

    val switchPreRiscvToRiscvTomlConfig =
      ProverToml(
        execution = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/execution/requests",
          fsResponsesDirectory = "/data/prover/v2/execution/responses",
        ),
        blobCompression = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/compression/requests",
          fsResponsesDirectory = "/data/prover/v2/compression/responses",
        ),
        invalidity = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/invalidity/requests",
          fsResponsesDirectory = "/data/prover/v2/invalidity/responses",
        ),
        proofAggregation = FileBasedProverConfigToml(
          fsRequestsDirectory = "/data/prover/v2/aggregation/requests",
          fsResponsesDirectory = "/data/prover/v2/aggregation/responses",
        ),
        new = ProverToml(
          switchBlockTimestamp = Instant.fromEpochSeconds(1000),
          type = ProverToml.ProverType.RISCV,
          forkName = "amsterdam",
          provingSystemVersion = "0xabcdef123",
          l2Execution = FileBasedProverConfigToml(
            programId = "0xdeadbeef1",
            fsRequestsDirectory = "/data/riscv-prover/execution/requests",
            fsResponsesDirectory = "/data/riscv-prover/execution/responses",
          ),
          rollup = FileBasedProverConfigToml(
            programId = "0xdeadbeef2",
            fsRequestsDirectory = "/data/riscv-prover/rollup/requests",
            fsResponsesDirectory = "/data/riscv-prover/rollup/responses",
          ),
          rollupAggregation = FileBasedProverConfigToml(
            programId = "0xdeadbeef3",
            fsRequestsDirectory = "/data/riscv-prover/aggregation/requests",
            fsResponsesDirectory = "/data/riscv-prover/aggregation/responses",
          ),
        ),
      )
  }

  data class PreRiscvWrapperConfig(
    val prover: ProverToml,
  )

  @Test
  fun `should parse pre-riscv prover toml configs - full`() {
    assertThat(
      parseConfig<PreRiscvWrapperConfig>(preRiscvToml).prover,
    ).isEqualTo(preRiscvConfig)
  }

  @Test
  fun `should parse pre-riscv prover toml configs and provide defaults`() {
    assertThat(
      parseConfig<PreRiscvWrapperConfig>(preRiscvTomlMinimal).prover,
    ).isEqualTo(preRiscvConfigMinimal)
  }

  @Test
  fun `should parse pre-riscv prover toml configs with cleanup enabled`() {
    assertThat(
      parseConfig<PreRiscvWrapperConfig>(preRiscvTomlWithCleanupEnabled).prover,
    ).isEqualTo(preRiscvConfigWithCleanupEnabled)
  }

  @Test
  fun `should default pre-riscv cleanup to false when not specified`() {
    val parsed = parseConfig<PreRiscvWrapperConfig>(preRiscvTomlMinimal).prover
    assertThat(parsed.enableRequestFilesCleanup).isFalse()
  }

  @Test
  fun `should parse pre-riscv cleanup setting when explicitly set to false`() {
    val tomlWithCleanupDisabled =
      """
      [prover]
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

    val parsed = parseConfig<PreRiscvWrapperConfig>(tomlWithCleanupDisabled).prover
    assertThat(parsed.enableRequestFilesCleanup).isFalse()
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
  fun `should parse prover toml configs and provide defaults`() {
    assertThat(
      parseConfig<WrapperConfig>(tomlMinimal).prover,
    ).isEqualTo(configMinimal)
  }

  // --- switching from pre-riscv to riscv provers ---

  @Test
  fun `should switch from pre-riscv to riscv prover with valid config`() {
    val proversToml = parseConfig<WrapperConfig>(switchPreRiscvToRiscvToml).prover
    assertThat(proversToml).isEqualTo(switchPreRiscvToRiscvTomlConfig)

    val proversConfig = proversToml.reified()
    assertThat(proversConfig.switchBlockTimestamp).isEqualTo(Instant.fromEpochSeconds(1000))
    assertThat(proversConfig.switchBlockNumberInclusive).isNull()

    val current = proversConfig.proverSwitch.current
    assertThat(current.preRiscvConfig).isNotNull()
    assertThat(current.riscvConfig).isNull()

    val next = proversConfig.proverSwitch.next
    assertThat(next).isNotNull()
    assertThat(next!!.preRiscvConfig).isNull()
    assertThat(next.riscvConfig).isNotNull()
    assertThat(next.riscvConfig!!.l2Execution.programId).isEqualTo("0xdeadbeef1")
    assertThat(next.riscvConfig!!.rollup.programId).isEqualTo("0xdeadbeef2")
    assertThat(next.riscvConfig!!.rollupAggregation.programId).isEqualTo("0xdeadbeef3")
  }

  @Test
  fun `should fail to switch from pre-riscv to riscv prover with invalid config`() {
    // [prover.new.rollup-aggregation] is intentionally omitted, making the riscv `new` config incomplete.
    val invalidSwitchToml =
      """
      [prover]
      type = "pre_riscv"
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
      type = "riscv"
      switch-block-number-inclusive = 1000
      proving-system-version = "0xabcdef123"
      fork-name = "amsterdam"
      [prover.new.l2-execution]
      programId = "0xdeadbeef1"
      fs-requests-directory = "/data/riscv-prover/execution/requests"
      fs-responses-directory = "/data/riscv-prover/execution/responses"
      [prover.new.rollup]
      programId = "0xdeadbeef2"
      fs-requests-directory = "/data/riscv-prover/rollup/requests"
      fs-responses-directory = "/data/riscv-prover/rollup/responses"
      """.trimIndent()

    val prover = parseConfig<WrapperConfig>(invalidSwitchToml).prover

    assertThatThrownBy { prover.reified() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("Prover type of RISCV must configure l2Execution, rollup, and rollupAggregation")
  }

  @Test
  fun `should fail to switch from riscv to pre-riscv prover`() {
    val invalidSwitchToml =
      """
      [prover]
      type = "riscv"
      switch-block-number-inclusive = 1000
      proving-system-version = "0xabcdef123"
      fork-name = "amsterdam"
      [prover.new.l2-execution]
      programId = "0xdeadbeef1"
      fs-requests-directory = "/data/riscv-prover/execution/requests"
      fs-responses-directory = "/data/riscv-prover/execution/responses"
      [prover.new.rollup]
      programId = "0xdeadbeef2"
      fs-requests-directory = "/data/riscv-prover/rollup/requests"
      fs-responses-directory = "/data/riscv-prover/rollup/responses"
      [prover.new.rollup-aggregation]
      programId = "0xdeadbeef3"
      fs-requests-directory = "/data/riscv-prover/aggregation/requests"
      fs-responses-directory = "/data/riscv-prover/aggregation/responses"
      [prover.new]
      type = "pre_riscv"
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

    val prover = parseConfig<WrapperConfig>(invalidSwitchToml).prover

    assertThatThrownBy { prover.reified() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("Prover type of new must be RISCV if the current prover type is RISCV")
  }
}
