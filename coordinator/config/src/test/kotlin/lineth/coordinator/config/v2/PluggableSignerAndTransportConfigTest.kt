package lineth.coordinator.config.v2

import lineth.coordinator.config.v2.toml.ProverToml
import lineth.coordinator.config.v2.toml.loadConfigs
import lineth.coordinator.config.v2.toml.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path

class PluggableSignerAndTransportConfigTest {
  @TempDir
  lateinit var tempDir: Path

  private fun override(toml: String): Path = Files.writeString(Files.createTempFile(tempDir, "override", ".toml"), toml)

  private fun load(vararg overrides: Path): CoordinatorConfig =
    loadConfigs(
      coordinatorConfigFiles =
      listOf(
        Path.of("../../docker/config/coordinator/coordinator-config-v2.toml"),
        Path.of("../../docker/config/coordinator/coordinator-config-v2-override-local-dev.toml"),
      ) + overrides,
      tracesLimitsFileV4 = Path.of("../../docker/config/common/traces-limits-v4.4.toml"),
      tracesLimitsFileV5 = Path.of("../../docker/config/common/traces-limits-v5.toml"),
      gasPriceCapTimeOfDayMultipliersFile = Path.of(
        "../../docker/config/common/gas-price-cap-time-of-day-multipliers.toml",
      ),
      enforceStrict = true,
    )

  @Test
  fun `registered signer types are configured inline in every signer table`() {
    val configs =
      load(
        override(
          """
          [l1-submission.blob.signer]
          type = "kms"
          [l1-submission.blob.signer.kms]
          key-id = "abc"
          region = "eu-west-1"
          [l1-submission.aggregation.signer]
          type = "Hsm"
          [l1-submission.aggregation.signer.hsm]
          slot = 3
          [message-anchoring.signer]
          type = "kms"
          [message-anchoring.signer.kms]
          key-id = "def"
          """.trimIndent(),
        ),
      )

    val blob = configs.l1Submission.blob.signer
    assertThat(blob.type).isEqualTo(SignerConfig.SignerType("kms"))
    assertThat(blob.registered!!.settings).isEqualTo(mapOf("key-id" to "abc", "region" to "eu-west-1"))
    assertThat(configs.l1Submission.aggregation.signer.registered!!.settings).isEqualTo(mapOf("slot" to "3"))
    assertThat(configs.messageAnchoring!!.signer.registered!!.settings).isEqualTo(mapOf("key-id" to "def"))
  }

  @Test
  fun `registered signer settings can be overridden by a higher priority file`() {
    val declaring =
      override("[message-anchoring.signer]\ntype = \"kms\"\n[message-anchoring.signer.kms]\nkey-id = \"a\"\n")
    val overriding = override("[message-anchoring.signer.kms]\nkey-id = \"b\"\n")

    assertThat(load(declaring, overriding).messageAnchoring!!.signer.registered!!.settings)
      .isEqualTo(mapOf("key-id" to "b"))
  }

  @Test
  fun `registered signer type requires its table`() {
    assertThatThrownBy { load(override("[message-anchoring.signer]\ntype = \"kms\"\n")) }
      .hasMessageContaining("requires a [signer.kms] table")
  }

  @Test
  fun `custom signer type still works`() {
    val signer =
      load(
        override("[message-anchoring.signer]\ntype = \"custom\"\n[message-anchoring.signer.custom]\nname = \"x\"\n"),
      ).messageAnchoring!!.signer

    assertThat(signer.type).isEqualTo(SignerConfig.SignerType.CUSTOM)
    assertThat(signer.custom).isEqualTo(SignerConfig.CustomConfig("x"))
    assertThat(signer.registered).isNull()
  }

  private val riscvToml =
    """
    type = "riscv"
    fork-name = "amsterdam"
    proving-system-version = "0x1"
    [rollup]
    program-id = "0x2"
    fs-requests-directory = "/r/req"
    fs-responses-directory = "/r/resp"
    [rollup-aggregation]
    program-id = "0x3"
    fs-requests-directory = "/a/req"
    fs-responses-directory = "/a/resp"
    """.trimIndent()

  @Test
  fun `riscv proof types can use different transports and skip fs directories`() {
    val toml = riscvToml + "\n[l2-execution]\nprogram-id = \"0x1\"\ntransport = \"http\"\n"

    val config = parseConfig<ProverToml>(toml).reified().currentProver.riscvConfig!!

    assertThat(config.l2Execution.transport).isEqualTo("http")
    assertThat(config.l2Execution.fileBased).isNull()
    assertThat(config.l2Execution.isFileTransport).isFalse()
    assertThatThrownBy { config.l2Execution.requireFileBased() }.hasMessageContaining("transport 'http'")
    assertThat(config.rollup.isFileTransport).isTrue()
    assertThat(config.rollup.fileBased!!.requestsDirectory).isEqualTo(Path.of("/r/req"))
  }

  @Test
  fun `file transport still requires the fs directories`() {
    val toml = riscvToml + "\n[l2-execution]\nprogram-id = \"0x1\"\n"

    assertThatThrownBy { parseConfig<ProverToml>(toml) }
      .hasMessageContaining("fs-requests-directory and fs-responses-directory are required")
  }

  @Test
  fun `pre riscv provers only support the file transport`() {
    val toml =
      """
      [execution]
      transport = "http"
      [blob-compression]
      fs-requests-directory = "/b/req"
      fs-responses-directory = "/b/resp"
      [proof-aggregation]
      fs-requests-directory = "/p/req"
      fs-responses-directory = "/p/resp"
      """.trimIndent()

    assertThatThrownBy { parseConfig<ProverToml>(toml).reified() }
      .hasMessageContaining("PRE-RISCV only supports transport=file")
  }
}
