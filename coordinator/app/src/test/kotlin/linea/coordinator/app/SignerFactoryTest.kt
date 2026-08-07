package linea.coordinator.app

import com.sksamuel.hoplite.Masked
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.coordinator.config.v2.SignerConfig
import lineth.crypto.Web3SignerRestClient
import lineth.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import org.web3j.crypto.ECDSASignature
import org.web3j.crypto.Hash
import org.web3j.crypto.Sign
import java.math.BigInteger
import java.nio.file.Path
import java.security.KeyStore
import kotlin.io.path.outputStream

@ExtendWith(VertxExtension::class)
class SignerFactoryTest {
  @Test
  fun `local signer signs a digest through the shared signer contract`(vertx: Vertx) {
    val signer = DefaultSignerFactory.create(vertx, web3jConfig())
    val digest = Hash.sha3("transaction".toByteArray())

    val signature = signer.sign(digest).get()
    val signatureData =
      Sign.createSignatureData(
        ECDSASignature(signature.r, signature.s),
        BigInteger(1, signer.publicKey()),
        digest,
      )

    assertThat(Sign.signedMessageHashToKey(digest, signatureData)).isEqualTo(BigInteger(1, signer.publicKey()))
  }

  @Test
  fun `local signer protects its public key and requires a digest`(vertx: Vertx) {
    val signer = DefaultSignerFactory.create(vertx, web3jConfig())
    val expectedPublicKey = signer.publicKey()

    signer.publicKey().fill(0)

    assertThat(signer.publicKey()).isEqualTo(expectedPublicKey)
    assertThatThrownBy { signer.sign(ByteArray(31)) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("32-byte digest")
  }

  @Test
  fun `web3signer backend is exposed as a shared signer`(vertx: Vertx) {
    val signer =
      DefaultSignerFactory.create(
        vertx,
        SignerConfig(
          type = SignerConfig.SignerType.WEB3SIGNER,
          web3j = null,
          web3signer =
          SignerConfig.Web3SignerConfig(
            endpoint = "http://localhost:9000".toURL(),
            publicKey = ByteArray(64) { 1 },
            tls = null,
          ),
        ),
      )

    assertThat(signer).isInstanceOf(Web3SignerRestClient::class.java)
  }

  @Test
  fun `web3signer backend loads TLS stores`(
    vertx: Vertx,
    @TempDir tempDir: Path,
  ) {
    val password = "changeit"
    val keyStorePath = tempDir.resolve("client-keystore.p12")
    val trustStorePath = tempDir.resolve("truststore.p12")
    createEmptyPkcs12(keyStorePath, password)
    createEmptyPkcs12(trustStorePath, password)

    val signer =
      DefaultSignerFactory.create(
        vertx,
        SignerConfig(
          type = SignerConfig.SignerType.WEB3SIGNER,
          web3j = null,
          web3signer =
          SignerConfig.Web3SignerConfig(
            endpoint = "https://localhost:9000".toURL(),
            publicKey = ByteArray(64) { 1 },
            tls =
            SignerConfig.Web3SignerConfig.TlsConfig(
              keyStorePath = keyStorePath,
              keyStorePassword = Masked(password),
              trustStorePath = trustStorePath,
              trustStorePassword = Masked(password),
            ),
          ),
        ),
      )

    assertThat(signer).isInstanceOf(Web3SignerRestClient::class.java)
  }

  @Test
  fun `default factory rejects custom signer with its configured name`(vertx: Vertx) {
    assertThatThrownBy {
      DefaultSignerFactory.create(
        vertx,
        SignerConfig(
          type = SignerConfig.SignerType.CUSTOM,
          web3j = null,
          web3signer = null,
          custom = SignerConfig.CustomConfig("l1-submitter"),
        ),
      )
    }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("l1-submitter")
  }

  private fun web3jConfig() =
    SignerConfig(
      type = SignerConfig.SignerType.WEB3J,
      web3j = SignerConfig.Web3jConfig(ByteArray(31) + 1),
      web3signer = null,
    )

  private fun createEmptyPkcs12(
    path: Path,
    password: String,
  ) {
    val keyStore = KeyStore.getInstance("PKCS12")
    keyStore.load(null, password.toCharArray())
    path.outputStream().use { keyStore.store(it, password.toCharArray()) }
  }
}
