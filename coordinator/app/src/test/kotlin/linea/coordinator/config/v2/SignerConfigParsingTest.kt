package linea.coordinator.config.v2

import com.sksamuel.hoplite.Masked
import linea.coordinator.config.v2.toml.SignerConfigToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.kotlin.decodeHex
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.nio.file.Path

class SignerConfigParsingTest {
  companion object {
    val toml =
      """
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

      [web3signerWithTlsExample]
      type = "Web3SiGner" # Shall be case insensitive
      [web3signerWithTlsExample.web3signer]
      endpoint = "https://web3signer:9000"
      max-pool-size = 10
      keep-alive = true
      public-key = "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"
      [web3signerWithTlsExample.web3signer.tls]
      key-store-path = "coordinator-client-keystore.p12"
      key-store-password = "xxxxx"
      trust-store-path = "web3signer-truststore.p12"
      trust-store-password = "xxxxx"
      """.trimIndent()

    val config =
      WrapperConfig(
        web3jExample =
        SignerConfigToml(
          type = SignerConfigToml.SignerType.WEB3J,
          web3j =
          SignerConfigToml.Web3jConfig(
            privateKey = Masked("0x0000000000000000000000000000000000000000000000000000000000000001"),
          ),
          web3signer = null,
        ),
        web3SignerExample =
        SignerConfigToml(
          type = SignerConfigToml.SignerType.WEB3SIGNER,
          web3j = null,
          web3signer =
          SignerConfigToml.Web3SignerConfig(
            endpoint = "http://web3signer:9000".toURL(),
            publicKey =
            (
              "0000000000000000000000000000000000000000000000000000000000000000" +
                "0000000000000000000000000000000000000000000000000000000000000001"
              ).decodeHex(),
            maxPoolSize = 10,
            keepAlive = true,
            tls = null,
          ),
        ),
        web3signerWithTlsExample =
        SignerConfigToml(
          type = SignerConfigToml.SignerType.WEB3SIGNER,
          web3j = null,
          web3signer =
          SignerConfigToml.Web3SignerConfig(
            endpoint = "https://web3signer:9000".toURL(),
            publicKey =
            (
              "0000000000000000000000000000000000000000000000000000000000000000" +
                "0000000000000000000000000000000000000000000000000000000000000001"
              ).decodeHex(),
            maxPoolSize = 10,
            keepAlive = true,
            tls =
            SignerConfigToml.Web3SignerConfig.TlsConfig(
              keyStorePath = Path.of("coordinator-client-keystore.p12"),
              keyStorePassword = Masked("xxxxx"),
              trustStorePath = Path.of("web3signer-truststore.p12"),
              trustStorePassword = Masked("xxxxx"),
            ),
          ),
        ),
      )
  }

  data class WrapperConfig(
    val web3jExample: SignerConfigToml,
    val web3SignerExample: SignerConfigToml,
    val web3signerWithTlsExample: SignerConfigToml,
  )

  @Test
  fun `should parse full state manager config`() {
    assertThat(parseConfig<WrapperConfig>(toml)).isEqualTo(config)
  }
}
