package lineth.coordinator.config.v2

import com.sksamuel.hoplite.Secret
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
          type = "someOtherType"
          [l1-submission.blob.signer.someOtherType]
          someOtherTypeParam = "someOtherTypeValue"
          someOtherTypeParam2 = "someOtherTypeValue2"
          [l1-submission.aggregation.signer]
          type = "someOtherType2"
          [l1-submission.aggregation.signer.someOtherType2]
          someOtherType2Param = "someOtherType2Value"
          [message-anchoring.signer]
          type = "someOtherType"
          [message-anchoring.signer.someOtherType]
          someOtherTypeParam = "someOtherTypeValue3"
          """.trimIndent(),
        ),
      )

    configs.l1Submission.blob.signer.also { signerConfig ->
      assertThat(signerConfig.type).isEqualTo(SignerConfig.SignerType("someOtherType"))
      assertThat(signerConfig.registered!!.settings).isEqualTo(
        mapOf(
          "someOtherTypeParam" to Secret("someOtherTypeValue"),
          "someOtherTypeParam2" to Secret("someOtherTypeValue2"),
        ),
      )
    }
    configs.l1Submission.aggregation.signer.also { signerConfig ->
      assertThat(signerConfig.type).isEqualTo(SignerConfig.SignerType("someOtherType2"))
      assertThat(signerConfig.registered!!.settings).isEqualTo(
        mapOf("someOtherType2Param" to Secret("someOtherType2Value")),
      )
    }
    assertThat(
      configs.messageAnchoring!!.signer.registered!!.settings,
    ).isEqualTo(mapOf("someOtherTypeParam" to Secret("someOtherTypeValue3")))
  }

  @Test
  fun `registered signer settings can be overridden by a higher priority file`() {
    val declaring =
      override(
        "[message-anchoring.signer]\ntype = \"someOtherType\"\n" +
          "[message-anchoring.signer.someOtherType]\nsomeOtherTypeParam = \"a\"\n",
      )
    val overriding = override("[message-anchoring.signer.someOtherType]\nsomeOtherTypeParam = \"b\"\n")

    assertThat(load(declaring, overriding).messageAnchoring!!.signer.registered!!.settings)
      .isEqualTo(mapOf("someOtherTypeParam" to Secret("b")))
  }

  @Test
  fun `registered signer type requires its table`() {
    assertThatThrownBy { load(override("[message-anchoring.signer]\ntype = \"someOtherType\"\n")) }
      .hasMessageContaining("requires a [signer.someothertype] table")
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
  fun `riscv provers can use a non-file transport and skip fs directories`() {
    val toml = "transport = \"http\"\n" + riscvToml + "\n[l2-execution]\nprogram-id = \"0x1\"\n"

    val config = parseConfig<ProverToml>(toml).reified().currentProver.riscvConfig!!

    assertThat(config.transport).isEqualTo("http")
    assertThat(config.isFileTransport).isFalse()
    assertThat(config.l2Execution.fileBased).isNull()
    assertThat(config.rollup.fileBased).isNull()
    assertThatThrownBy { config.l2Execution.requireFileBased() }
      .hasMessageContaining("not supported by the file-based prover client factory")
  }

  @Test
  fun `riscv provers default to the file transport`() {
    val toml = riscvToml + "\n[l2-execution]\nprogram-id = \"0x1\"\nfs-requests-directory = \"/e/req\"\n" +
      "fs-responses-directory = \"/e/resp\"\n"

    val config = parseConfig<ProverToml>(toml).reified().currentProver.riscvConfig!!

    assertThat(config.isFileTransport).isTrue()
    assertThat(config.rollup.fileBased!!.requestsDirectory).isEqualTo(Path.of("/r/req"))
  }

  @Test
  fun `file transport still requires the fs directories`() {
    val toml = riscvToml + "\n[l2-execution]\nprogram-id = \"0x1\"\n"

    assertThatThrownBy { parseConfig<ProverToml>(toml).reified() }
      .hasMessageContaining("fs-requests-directory and fs-responses-directory are required")
  }

  @Test
  fun `pre riscv provers only support the file transport`() {
    val toml =
      """
      transport = "http"
      [execution]
      fs-requests-directory = "/e/req"
      fs-responses-directory = "/e/resp"
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
