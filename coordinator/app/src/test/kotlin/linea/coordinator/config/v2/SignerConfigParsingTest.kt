package linea.coordinator.config.v2

import com.sksamuel.hoplite.Masked
import linea.coordinator.config.v2.toml.SignerConfigToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.kotlin.decodeHex
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class SignerConfigParsingTest {
  companion object {
    val toml = """
    [web3jExample]
    type = "wEb3j" # Shall be case insensitive
    [web3jExample.web3j]
    private-key = "0x0000000000000000000000000000000000000000000000000000000000000001"

    [web3signerExample]
    type = "Web3SiGner" # Shall be case insensitive
    [web3signerExample.web3signer]
    endpoint = "http://web3signer:9000"
    max-pool-size = 10
    keep-alive = true
    public-key = "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"
    """.trimIndent()

    val config = WrapperConfig(
      web3jExample = SignerConfigToml(
        type = SignerConfigToml.SignerType.WEB3J,
        web3j = SignerConfigToml.Web3jConfig(
          privateKey = Masked("0x0000000000000000000000000000000000000000000000000000000000000001"),
        ),
        web3signer = null,
      ),
      web3SignerExample = SignerConfigToml(
        type = SignerConfigToml.SignerType.WEB3SIGNER,
        web3j = null,
        web3signer = SignerConfigToml.Web3SignerConfig(
          endpoint = "http://web3signer:9000".toURL(),
          publicKey = (
            "0000000000000000000000000000000000000000000000000000000000000000" +
              "0000000000000000000000000000000000000000000000000000000000000001"
            ).decodeHex(),
          maxPoolSize = 10,
          keepAlive = true,
        ),
      ),
    )
  }

  data class WrapperConfig(
    val web3jExample: SignerConfigToml,
    val web3SignerExample: SignerConfigToml,
  )

  @Test
  fun `should parse full state manager config`() {
    assertThat(parseConfig<WrapperConfig>(toml)).isEqualTo(config)
  }
}
