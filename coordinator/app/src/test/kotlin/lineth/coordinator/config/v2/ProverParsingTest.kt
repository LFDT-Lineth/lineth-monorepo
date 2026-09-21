package lineth.coordinator.config.v2

import lineth.coordinator.clients.prover.RestfulBasedProverConfig
import lineth.coordinator.config.v2.toml.FileBasedProverConfigToml
import lineth.coordinator.config.v2.toml.ProverConfigToml
import lineth.coordinator.config.v2.toml.ProverToml
import lineth.coordinator.config.v2.toml.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.catchThrowable
import org.junit.jupiter.api.Test
import java.net.URI
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class ProverParsingTest {
  companion object {
    val toml =
      """
      [prover]
      proving-system-version = "0xabcdef123"
      fork-name = "amsterdam"
      polling-interval = "PT1S"
      polling-timeout = "PT10M"
      fs-inprogress-request-writing-suffix = ".inprogress_coordinator_riscv_writing"
      fs-inprogress-proving-suffix-pattern = "\\.inprogress\\.riscv-prover.*"
      fs-enable-request-files-cleanup = true
      [prover.l2-execution]
      programId = "0xdeadbeef1"
      [prover.l2-execution.file-based-folder-config]
      fs-requests-directory = "/data/riscv-prover/execution/requests"
      fs-responses-directory = "/data/riscv-prover/execution/responses"
      [prover.rollup]
      programId = "0xdeadbeef2"
      [prover.rollup.file-based-folder-config]
      fs-requests-directory = "/data/riscv-prover/rollup/requests"
      fs-responses-directory = "/data/riscv-prover/rollup/responses"
      [prover.rollup-aggregation]
      programId = "0xdeadbeef3"
      [prover.rollup-aggregation.file-based-folder-config]
      fs-requests-directory = "/data/riscv-prover/aggregation/requests"
      fs-responses-directory = "/data/riscv-prover/aggregation/responses"
      """.trimIndent()

    val config =
      ProverToml(
        pollingInterval = 1.seconds,
        pollingTimeout = 10.minutes,
        forkName = "amsterdam",
        provingSystemVersion = "0xabcdef123",
        fsInprogressRequestWritingSuffix = ".inprogress_coordinator_riscv_writing",
        fsInprogressProvingSuffixPattern = "\\.inprogress\\.riscv-prover.*",
        fsEnableRequestFilesCleanup = true,
        l2Execution = ProverConfigToml(
          programId = "0xdeadbeef1",
          fileBasedFolderConfig = FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/riscv-prover/execution/requests",
            fsResponsesDirectory = "/data/riscv-prover/execution/responses",
          ),
        ),
        rollup = ProverConfigToml(
          programId = "0xdeadbeef2",
          fileBasedFolderConfig = FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/riscv-prover/rollup/requests",
            fsResponsesDirectory = "/data/riscv-prover/rollup/responses",
          ),
        ),
        rollupAggregation = ProverConfigToml(
          programId = "0xdeadbeef3",
          fileBasedFolderConfig = FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/riscv-prover/aggregation/requests",
            fsResponsesDirectory = "/data/riscv-prover/aggregation/responses",
          ),
        ),
      )

    val tomlMinimal =
      """
      [prover]
      proving-system-version = "0xabcdef123"
      fork-name = "amsterdam"
      [prover.l2-execution]
      programId = "0xdeadbeef1"
      [prover.l2-execution.file-based-folder-config]
      fs-requests-directory = "/data/riscv-prover/execution/requests"
      fs-responses-directory = "/data/riscv-prover/execution/responses"
      [prover.rollup]
      programId = "0xdeadbeef2"
      [prover.rollup.file-based-folder-config]
      fs-requests-directory = "/data/riscv-prover/rollup/requests"
      fs-responses-directory = "/data/riscv-prover/rollup/responses"
      [prover.rollup-aggregation]
      programId = "0xdeadbeef3"
      [prover.rollup-aggregation.file-based-folder-config]
      fs-requests-directory = "/data/riscv-prover/aggregation/requests"
      fs-responses-directory = "/data/riscv-prover/aggregation/responses"
      """.trimIndent()

    val configMinimal =
      ProverToml(
        forkName = "amsterdam",
        provingSystemVersion = "0xabcdef123",
        l2Execution = ProverConfigToml(
          programId = "0xdeadbeef1",
          fileBasedFolderConfig = FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/riscv-prover/execution/requests",
            fsResponsesDirectory = "/data/riscv-prover/execution/responses",
          ),
        ),
        rollup = ProverConfigToml(
          programId = "0xdeadbeef2",
          fileBasedFolderConfig = FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/riscv-prover/rollup/requests",
            fsResponsesDirectory = "/data/riscv-prover/rollup/responses",
          ),
        ),
        rollupAggregation = ProverConfigToml(
          programId = "0xdeadbeef3",
          fileBasedFolderConfig = FileBasedProverConfigToml(
            fsRequestsDirectory = "/data/riscv-prover/aggregation/requests",
            fsResponsesDirectory = "/data/riscv-prover/aggregation/responses",
          ),
        ),
      )

    val tomlRestful =
      """
      [prover]
      proving-system-version = "0xabcdef123"
      fork-name = "amsterdam"
      polling-interval = "PT1S"
      polling-timeout = "PT10M"
      restful-endpoint = "http://127.0.0.1:8090/"
      restful-api-base-path = "/api"
      restful-api-version = "v1"
      [prover.l2-execution]
      programId = "0xdeadbeef1"
      [prover.rollup]
      programId = "0xdeadbeef2"
      [prover.rollup-aggregation]
      programId = "0xdeadbeef3"
      """.trimIndent()

    val configRestful =
      ProverToml(
        pollingInterval = 1.seconds,
        pollingTimeout = 10.minutes,
        forkName = "amsterdam",
        provingSystemVersion = "0xabcdef123",
        restfulEndpoint = URI("http://127.0.0.1:8090/").toURL(),
        restfulApiBasePath = "/api",
        restfulApiVersion = "v1",
        l2Execution = ProverConfigToml(programId = "0xdeadbeef1"),
        rollup = ProverConfigToml(programId = "0xdeadbeef2"),
        rollupAggregation = ProverConfigToml(programId = "0xdeadbeef3"),
      )
  }

  data class WrapperConfig(
    val prover: ProverToml,
  )

  @Test
  fun `should parse riscv prover toml config`() {
    assertThat(
      parseConfig<WrapperConfig>(toml).prover,
    ).isEqualTo(config)
  }

  @Test
  fun `should parse riscv prover toml config with defaults`() {
    assertThat(
      parseConfig<WrapperConfig>(tomlMinimal).prover,
    ).isEqualTo(configMinimal)
  }

  @Test
  fun `should parse riscv prover toml config with restful transport`() {
    assertThat(
      parseConfig<WrapperConfig>(tomlRestful).prover,
    ).isEqualTo(configRestful)
  }

  @Test
  fun `should reify restful prover config into RestfulBasedProverConfig for each prover client`() {
    val proversConfig = parseConfig<WrapperConfig>(tomlRestful).prover.reified()

    val expectedRestfulConfig = RestfulBasedProverConfig(
      endpoint = URI("http://127.0.0.1:8090/").toURL(),
      restfulApiBasePath = "/api",
      restfulApiVersion = "v1",
      pollingInterval = 1.seconds,
      pollingTimeout = 10.minutes,
    )

    assertThat(proversConfig.proverSwitch.current.l2Execution.restfulBased).isEqualTo(expectedRestfulConfig)
    assertThat(proversConfig.proverSwitch.current.l2Execution.fileBased).isNull()
    assertThat(proversConfig.proverSwitch.current.rollup.restfulBased).isEqualTo(expectedRestfulConfig)
    assertThat(proversConfig.proverSwitch.current.rollup.fileBased).isNull()
    assertThat(proversConfig.proverSwitch.current.rollupAggregation.restfulBased).isEqualTo(expectedRestfulConfig)
    assertThat(proversConfig.proverSwitch.current.rollupAggregation.fileBased).isNull()
  }

  @Test
  fun `should fail reify when neither fileBasedFolderConfig nor restfulEndpoint is configured`() {
    val tomlMissingTransport =
      """
      [prover]
      proving-system-version = "0xabcdef123"
      fork-name = "amsterdam"
      [prover.l2-execution]
      programId = "0xdeadbeef1"
      [prover.rollup]
      programId = "0xdeadbeef2"
      [prover.rollup-aggregation]
      programId = "0xdeadbeef3"
      """.trimIndent()

    assertThat(
      catchThrowable {
        parseConfig<WrapperConfig>(tomlMissingTransport).prover.reified()
      },
    ).isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("restfulEndpoint must be defined")
  }
}
