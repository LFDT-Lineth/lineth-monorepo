import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import io.vertx.core.Vertx
import io.vertx.core.http.HttpVersion
import io.vertx.ext.web.client.WebClientOptions
import io.vertx.junit5.VertxExtension
import net.consensys.linea.httprest.client.VertxHttpRestClient
import net.consensys.zkevm.ethereum.crypto.Web3SignerRestClient
import org.apache.tuweni.bytes.Bytes
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
import java.math.BigInteger

@ExtendWith(VertxExtension::class)
class Web3SignerRestClientTest {
  private lateinit var wiremock: WireMockServer
  private lateinit var web3SignerClient: Web3SignerRestClient
  private val path = Web3SignerRestClient.WEB3SIGNER_SIGN_ENDPOINT
  private val privateKey = Keys.createEcKeyPair().privateKey
  private val publicKey: BigInteger = Sign.publicKeyFromPrivate(privateKey)

  @BeforeEach
  fun setup(vertx: Vertx) {
    wiremock = WireMockServer(WireMockConfiguration.options().dynamicPort())
    wiremock.start()

    val webClientOptions: WebClientOptions =
      WebClientOptions()
        .setKeepAlive(true)
        .setProtocolVersion(HttpVersion.HTTP_1_1)
        .setMaxPoolSize(10)
        .setDefaultHost("localhost")
        .setDefaultPort(wiremock.port())

    val vertxHttpRestClient = VertxHttpRestClient(webClientOptions, vertx)

    web3SignerClient = Web3SignerRestClient(vertxHttpRestClient, publicKey.toString())
  }

  @AfterEach
  fun tearDown() {
    wiremock.stop()
  }

  @Test
  fun web3Signer_Sign() {
    val keyPair = ECKeyPair(privateKey, publicKey)

    val msg = "Message for signing"
    val msgHash: ByteArray = Hash.sha3(msg.toByteArray())
    val signature = Sign.signMessage(msg.toByteArray(), keyPair, true)

    val returnSignature = Hex.toHexString(signature.r + signature.s + signature.v)
    wiremock.stubFor(
      WireMock.post("$path${this.publicKey}")
        .withHeader("Content-Type", WireMock.containing("application/json"))
        .willReturn(
          WireMock.ok()
            .withHeader("Content-type", "text/plain; charset=utf-8\n")
            .withBody(returnSignature),
        ),
    )

    val (r, s) = web3SignerClient.sign(Bytes.wrap(msg.toByteArray()))
    assertThat(r).isEqualTo(BigInteger(Hex.toHexString(signature.r), 16))
    assertThat(s).isEqualTo(BigInteger(Hex.toHexString(signature.s), 16))

    val eCDSASignature = ECDSASignature(r, s)
    val derivedSignatureData = Sign.createSignatureData(eCDSASignature, publicKey, msgHash)
    assertThat(derivedSignatureData).isEqualTo(signature)
  }

  @Test
  fun errorSign() {
    wiremock.stubFor(
      WireMock.post("$path${this.publicKey}")
        .withHeader("Content-Type", WireMock.containing("application/json"))
        .willReturn(
          WireMock.notFound()
            .withHeader("Content-type", "text/plain; charset=utf-8\n")
            .withStatusMessage("Public Key not found"),
        ),
    )
    assertThrows<Exception> { web3SignerClient.sign(Bytes.wrap("Message".toByteArray())) }
  }
}
