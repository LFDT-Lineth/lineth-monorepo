package net.consensys.zkevm.coordinator.app.config

import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.toml.TomlPropertySource
import net.consensys.zkevm.coordinator.clients.prover.FileBasedProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProversConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.nio.file.Path
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class ProverConfigTest {
  data class Config(
    val prover: ProverConfigTomlDto,
  )

  private fun parseConfig(toml: String): ProversConfig {
    return ConfigLoaderBuilder
      .default()
      .addSource(TomlPropertySource(toml))
      .build()
      .loadConfigOrThrow<Config>()
      .let { it.prover.reified() }
  }

  val proverAConfigToml = """
    [prover]
    fs-inprogress-request-writing-suffix = ".inprogress_coordinator_writing"
    fs-inprogress-proving-suffix-pattern = "\\.inprogress\\.prover.*"
    fs-polling-interval = "PT10S"
    fs-polling-timeout = "PT10M"
    [prover.execution]
    fs-requests-directory = "/data/prover/execution/requests"
    fs-responses-directory = "/data/prover/execution/responses"
    fs-inprogress-request-writing-suffix = ".OVERRIDE_inprogress_coordinator_writing"
    fs-inprogress-proving-suffix-pattern = "OVERRIDE_\\.inprogress\\.prover.*"
    [prover.blob-compression]
    fs-requests-directory = "/data/prover/compression/requests"
    fs-responses-directory = "/data/prover/compression/responses"
    fs-polling-interval = "PT20S"
    fs-polling-timeout = "PT20M"
    [prover.proof-aggregation]
    fs-requests-directory = "/data/prover/aggregation/requests"
    fs-responses-directory = "/data/prover/aggregation/responses"
  """.trimIndent()

  @Test
  fun `should load configs with single prover and overrids`() {
    val config = parseConfig(proverAConfigToml)
    assertThat(config.switchBlockNumberInclusive).isNull()
    assertProverAConfig(config.proverA)
    assertThat(config.proverB).isNull()
  }

  fun assertProverAConfig(config: ProverConfig) {
    assertThat(config.execution).isEqualTo(
      FileBasedProverConfig(
        requestsDirectory = Path.of("/data/prover/execution/requests"),
        responsesDirectory = Path.of("/data/prover/execution/responses"),
        inprogressRequestWritingSuffix = ".OVERRIDE_inprogress_coordinator_writing",
        inprogressProvingSuffixPattern = "OVERRIDE_\\.inprogress\\.prover.*",
        pollingInterval = 10.seconds,
        pollingTimeout = 10.minutes,
      ),
    )
    assertThat(config.blobCompression).isEqualTo(
      FileBasedProverConfig(
        requestsDirectory = Path.of("/data/prover/compression/requests"),
        responsesDirectory = Path.of("/data/prover/compression/responses"),
        inprogressRequestWritingSuffix = ".inprogress_coordinator_writing",
        inprogressProvingSuffixPattern = "\\.inprogress\\.prover.*",
        pollingInterval = 20.seconds,
        pollingTimeout = 20.minutes,
      ),
    )
    assertThat(config.proofAggregation).isEqualTo(
      FileBasedProverConfig(
        requestsDirectory = Path.of("/data/prover/aggregation/requests"),
        responsesDirectory = Path.of("/data/prover/aggregation/responses"),
        inprogressRequestWritingSuffix = ".inprogress_coordinator_writing",
        inprogressProvingSuffixPattern = "\\.inprogress\\.prover.*",
        pollingInterval = 10.seconds,
        pollingTimeout = 10.minutes,
      ),
    )
  }

  @Test
  fun `should load configs with 2 provers and overrides`() {
    val toml = """
     $proverAConfigToml
    [prover.new]
    switch-block-number-inclusive=200
    fs-inprogress-request-writing-suffix = ".NEW_OVERRIDE_inprogress_coordinator_writing"
    fs-polling-timeout = "PT5M"
    [prover.new.execution]
    fs-requests-directory = "/data/prover-new/execution/requests"
    fs-responses-directory = "/data/prover-new/execution/responses"
    fs-inprogress-request-writing-suffix = ".NEW_OVERRIDE_2_inprogress_coordinator_writing"
    fs-inprogress-proving-suffix-pattern = "NEW_OVERRIDE_2\\.inprogress\\.prover.*"
    [prover.new.blob-compression]
    fs-requests-directory = "/data/prover-new/compression/requests"
    fs-responses-directory = "/data/prover-new/compression/responses"
    fs-polling-interval = "PT12S"
    fs-polling-timeout = "PT12M"
    [prover.new.proof-aggregation]
    fs-requests-directory = "/data/prover-new/aggregation/requests"
    fs-responses-directory = "/data/prover-new/aggregation/responses
    "
    """.trimIndent()
    val config = parseConfig(toml)

    assertProverAConfig(config.proverA)
    assertThat(config.switchBlockNumberInclusive).isEqualTo(200uL)
    assertThat(config.proverB).isNotNull
    assertThat(config.proverB!!.execution).isEqualTo(
      FileBasedProverConfig(
        requestsDirectory = Path.of("/data/prover-new/execution/requests"),
        responsesDirectory = Path.of("/data/prover-new/execution/responses"),
        inprogressRequestWritingSuffix = ".NEW_OVERRIDE_2_inprogress_coordinator_writing",
        inprogressProvingSuffixPattern = "NEW_OVERRIDE_2\\.inprogress\\.prover.*",
        pollingInterval = 10.seconds,
        pollingTimeout = 5.minutes,
      ),
    )
    assertThat(config.proverB!!.blobCompression).isEqualTo(
      FileBasedProverConfig(
        requestsDirectory = Path.of("/data/prover-new/compression/requests"),
        responsesDirectory = Path.of("/data/prover-new/compression/responses"),
        inprogressRequestWritingSuffix = ".NEW_OVERRIDE_inprogress_coordinator_writing",
        inprogressProvingSuffixPattern = "\\.inprogress\\.prover.*",
        pollingInterval = 12.seconds,
        pollingTimeout = 12.minutes,
      ),
    )
    assertThat(config.proverB!!.proofAggregation).isEqualTo(
      FileBasedProverConfig(
        requestsDirectory = Path.of("/data/prover-new/aggregation/requests"),
        responsesDirectory = Path.of("/data/prover-new/aggregation/responses"),
        inprogressRequestWritingSuffix = ".NEW_OVERRIDE_inprogress_coordinator_writing",
        inprogressProvingSuffixPattern = "\\.inprogress\\.prover.*",
        pollingInterval = 10.seconds,
        pollingTimeout = 5.minutes,
      ),
    )
  }
}
