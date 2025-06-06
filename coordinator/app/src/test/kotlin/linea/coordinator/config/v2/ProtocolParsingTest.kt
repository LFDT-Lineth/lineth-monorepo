package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.ProtocolToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.kotlin.decodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

class ProtocolParsingTest {
  companion object {
    val toml = """
    [protocol.genesis]
    genesis-state-root-hash = "0x0000000000000000000000000000000000000000000000000000000000000001"
    genesis-shnarf = "0x0000000000000000000000000000000000000000000000000000000000000002"
    [protocol.l1]
    contract-address = "0x0000000000000000000000000000000000000001"
    block-time = "PT2S"
    [protocol.l2]
    contract-address = "0x0000000000000000000000000000000000000002"
    contract-deployment-block-number = 1
    """.trimIndent()

    val config = ProtocolToml(
      genesis = ProtocolToml.Genesis(
        genesisStateRootHash = "0x0000000000000000000000000000000000000000000000000000000000000001".decodeHex(),
        genesisShnarf = "0x0000000000000000000000000000000000000000000000000000000000000002".decodeHex(),
      ),
      l1 = ProtocolToml.Layer1Config(
        contractAddress = "0x0000000000000000000000000000000000000001",
        blockTime = 2.seconds,
      ),
      l2 = ProtocolToml.Layer2Config(
        contractAddress = "0x0000000000000000000000000000000000000002",
        contractDeploymentBlockNumber = 1UL,
      ),
    )

    val tomlMinimal = """
    [protocol.genesis]
    genesis-state-root-hash = "0x0000000000000000000000000000000000000000000000000000000000000001"
    genesis-shnarf = "0x0000000000000000000000000000000000000000000000000000000000000002"
    [protocol.l1]
    contract-address = "0x0000000000000000000000000000000000000001"
    [protocol.l2]
    contract-address = "0x0000000000000000000000000000000000000002"
    """.trimIndent()

    val configMinimal = ProtocolToml(
      genesis = ProtocolToml.Genesis(
        genesisStateRootHash = "0x0000000000000000000000000000000000000000000000000000000000000001".decodeHex(),
        genesisShnarf = "0x0000000000000000000000000000000000000000000000000000000000000002".decodeHex(),
      ),
      l1 = ProtocolToml.Layer1Config(
        contractAddress = "0x0000000000000000000000000000000000000001",
        blockTime = 12.seconds,
      ),
      l2 = ProtocolToml.Layer2Config(
        contractAddress = "0x0000000000000000000000000000000000000002",
        contractDeploymentBlockNumber = null,
      ),
    )
  }

  internal data class WrapperConfig(val protocol: ProtocolToml)

  @Test
  fun `should parse protocol full configs`() {
    assertThat(parseConfig<WrapperConfig>(toml).protocol)
      .isEqualTo(config)
  }

  @Test
  fun `should parse protocol minimal configs`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).protocol)
      .isEqualTo(configMinimal)
  }
}
