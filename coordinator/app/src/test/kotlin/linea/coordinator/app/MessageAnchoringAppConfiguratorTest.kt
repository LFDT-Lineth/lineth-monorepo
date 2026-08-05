package linea.coordinator.app

import com.sun.net.httpserver.HttpServer
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.anchoring.MessageAnchoringApp
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.MessageAnchoringConfig
import linea.coordinator.config.v2.ProtocolConfig
import linea.coordinator.config.v2.SignerConfig
import linea.web3j.SmartContractErrors
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.mock
import java.net.InetSocketAddress
import java.net.URI
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class MessageAnchoringAppConfiguratorTest {
  @Test
  fun `creates message anchoring with configured L1 retries`(vertx: Vertx) {
    val rpcServer = HttpServer.create(InetSocketAddress(0), 0).apply {
      createContext("/") { exchange ->
        val response = """{"jsonrpc":"2.0","id":1,"result":"0x0"}""".toByteArray()
        exchange.sendResponseHeaders(200, response.size.toLong())
        exchange.responseBody.use { it.write(response) }
      }
      start()
    }

    try {
      val endpoint = URI("http://127.0.0.1:${rpcServer.address.port}").toURL()
      val messageAnchoring = MessageAnchoringConfig(
        l1Endpoint = endpoint,
        l2Endpoint = endpoint,
        signer = SignerConfig(
          type = SignerConfig.SignerType.WEB3J,
          web3j = SignerConfig.Web3jConfig(ByteArray(32) { 1 }),
          web3signer = null,
        ),
      )
      val protocol = ProtocolConfig(
        genesis = ProtocolConfig.Genesis(ByteArray(32), ByteArray(32)),
        l1 = ProtocolConfig.Layer1Config(
          contractAddress = "0x0000000000000000000000000000000000000001",
          blockTime = 12.seconds,
          contractDeploymentBlockNumber = null,
        ),
        l2 = ProtocolConfig.Layer2Config(
          contractAddress = "0x0000000000000000000000000000000000000002",
          contractDeploymentBlockNumber = null,
        ),
      )
      val config = mock<CoordinatorConfig> {
        on { this.messageAnchoring } doReturn messageAnchoring
        on { this.protocol } doReturn protocol
        on { smartContractErrors } doReturn mock<SmartContractErrors>()
      }

      assertThat(MessageAnchoringAppConfigurator.create(vertx, config))
        .isInstanceOf(MessageAnchoringApp::class.java)
    } finally {
      rpcServer.stop(0)
    }
  }
}
