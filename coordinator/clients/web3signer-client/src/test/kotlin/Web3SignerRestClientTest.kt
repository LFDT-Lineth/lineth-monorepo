import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.vertx.core.Vertx
import io.vertx.core.http.HttpVersion
import io.vertx.core.http.PoolOptions
import io.vertx.ext.web.client.WebClientOptions
import io.vertx.junit5.VertxExtension
import lineth.crypto.Web3SignerRestClient
import lineth.httprest.client.VertxHttpRestClient
import lineth.kotlin.encodeHex
import org.assertj.core.api.Assertions.assertThat
import org.bouncycastle.util.encoders.Hex
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import org.web3j.crypto.ECDSASignature
import org.web3j.crypto.ECKeyPair
import org.web3j.crypto.Hash
import org.web3j.crypto.Keys
import org.web3j.crypto.Sign
import org.web3j.utils.Numeric
import java.math.BigInteger

@ExtendWith(VertxExtension::class)
class Web3SignerRestClientTest {
  private lateinit var wiremock: WireMockServer
  private lateinit var web3SignerClient: Web3SignerRestClient
  private val path = Web3SignerRestClient.WEB3SIGNER_SIGN_ENDPOINT
  private val privateKey = Keys.createEcKeyPair().privateKey
  private val publicKey: BigInteger = Sign.publicKeyFromPrivate(privateKey)
  private val publicKeyBytes: ByteArray = Numeric.toBytesPadded(publicKey, 64)

  @BeforeEach
  fun setup(vertx: Vertx) {
    wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()

    val webClientOptions: WebClientOptions =
      WebClientOptions()
        .setKeepAlive(true)
        .setProtocolVersion(HttpVersion.HTTP_1_1)
        .setDefaultHost("localhost")
        .setDefaultPort(wiremock.port())

    val vertxHttpRestClient = VertxHttpRestClient(webClientOptions, PoolOptions().setHttp1MaxSize(10), vertx)

    web3SignerClient = Web3SignerRestClient(vertxHttpRestClient, publicKeyBytes)
  }

  @AfterEach
  fun tearDown() {
    wiremock.stop()
  }

  @Test
  fun web3Signer_Sign() {
    val keyPair = ECKeyPair(privateKey, publicKey)

    val digest: ByteArray = Hash.sha3("Message for signing".toByteArray())
    val signature = Sign.signMessage(digest, keyPair, false)

    val returnSignature = Hex.toHexString(signature.r + signature.s + signature.v)
    wiremock.stubFor(
      WireMock.post("$path${publicKeyBytes.encodeHex()}")
        .withHeader("Content-Type", WireMock.containing("application/json"))
        .withRequestBody(
          WireMock.equalToJson(
            """{"data":"${digest.encodeHex()}","applyHash":false}""",
          ),
        )
        .willReturn(
          WireMock.ok()
            .withHeader("Content-type", "text/plain; charset=utf-8\n")
            .withBody(returnSignature),
        ),
    )

    assertThat(web3SignerClient.publicKey()).isEqualTo(publicKeyBytes)

    val signed = web3SignerClient.sign(digest).get()
    assertThat(signed.toRSBytes()).isEqualTo(signature.r + signature.s)

    val (r, s) = signed
    assertThat(r).isEqualTo(BigInteger(Hex.toHexString(signature.r), 16))
    assertThat(s).isEqualTo(BigInteger(Hex.toHexString(signature.s), 16))

    val eCDSASignature = ECDSASignature(r, s)
    val derivedSignatureData = Sign.createSignatureData(eCDSASignature, publicKey, digest)
    assertThat(derivedSignatureData).isEqualTo(signature)
  }

  @Test
  fun `rejects input that is not a 32-byte digest`() {
    val error = assertThrows<IllegalArgumentException> {
      web3SignerClient.sign(ByteArray(31))
    }

    assertThat(error).hasMessageContaining("32-byte digest")
    wiremock.verify(0, WireMock.postRequestedFor(WireMock.urlPathMatching("$path.*")))
  }

  @Test
  fun `rejects a malformed signature response`() {
    val digest = Hash.sha3("Message for signing".toByteArray())
    wiremock.stubFor(
      WireMock.post("$path${publicKeyBytes.encodeHex()}")
        .willReturn(WireMock.ok("0x01")),
    )

    val error = assertThrows<Exception> {
      web3SignerClient.sign(digest).get()
    }

    assertThat(error.cause ?: error).hasMessageContaining("expected 65 bytes")
  }

  @Test
  fun errorSign() {
    wiremock.stubFor(
      WireMock.post("$path${publicKeyBytes.encodeHex()}")
        .withHeader("Content-Type", WireMock.containing("application/json"))
        .willReturn(
          WireMock.notFound()
            .withHeader("Content-type", "text/plain; charset=utf-8\n")
            .withStatusMessage("Public Key not found"),
        ),
    )
    assertThrows<Exception> { web3SignerClient.sign(Hash.sha3("Message".toByteArray())).get() }
  }
}
