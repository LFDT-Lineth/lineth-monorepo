package lineth.coordinator.config.v2

import com.sksamuel.hoplite.Masked
import lineth.coordinator.config.v2.toml.SignerConfigToml
import lineth.coordinator.config.v2.toml.parseConfig
import lineth.kotlin.decodeHex
import lineth.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
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

      [customExample]
      type = "CuStOm" # Shall be case insensitive
      [customExample.custom]
      name = "l1-submitter"
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
        customExample =
        SignerConfigToml(
          type = SignerConfigToml.SignerType.CUSTOM,
          web3j = null,
          web3signer = null,
          custom = SignerConfigToml.CustomConfig("l1-submitter"),
        ),
      )
  }

  data class WrapperConfig(
    val web3jExample: SignerConfigToml,
    val web3SignerExample: SignerConfigToml,
    val web3signerWithTlsExample: SignerConfigToml,
    val customExample: SignerConfigToml,
  )

  @Test
  fun `should parse full state manager config`() {
    assertThat(parseConfig<WrapperConfig>(toml)).isEqualTo(config)
    assertThat(parseConfig<WrapperConfig>(toml).customExample.reified()).isEqualTo(
      SignerConfig(
        type = SignerConfig.SignerType.CUSTOM,
        web3j = null,
        web3signer = null,
        custom = SignerConfig.CustomConfig("l1-submitter"),
      ),
    )
  }

  @Test
  fun `custom signer requires non-blank custom config`() {
    assertThatThrownBy {
      SignerConfigToml(
        type = SignerConfigToml.SignerType.CUSTOM,
        web3j = null,
        web3signer = null,
      )
    }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("requires custom config")

    assertThatThrownBy {
      SignerConfig(
        type = SignerConfig.SignerType.CUSTOM,
        web3j = null,
        web3signer = null,
      )
    }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("requires custom config")

    assertThatThrownBy { SignerConfigToml.CustomConfig(" ") }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("must not be blank")
  }
}
