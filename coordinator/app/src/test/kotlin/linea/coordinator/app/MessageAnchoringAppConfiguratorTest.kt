package linea.coordinator.app

import com.sun.net.httpserver.HttpServer
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.anchoring.MessageAnchoringApp
import linea.coordinator.config.v2.MessageAnchoringConfig
import linea.coordinator.config.v2.ProtocolConfig
import linea.coordinator.config.v2.SignerConfig
import linea.coordinator.config.v2.toml.loadConfigs
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.net.InetSocketAddress
import java.net.URI
import java.nio.file.Path
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
      val config = loadConfigs(
        coordinatorConfigFiles = listOf(
          Path.of("../../docker/config/coordinator/coordinator-config-v2.toml"),
          Path.of("../../docker/config/coordinator/coordinator-config-v2-override-local-dev.toml"),
        ),
        tracesLimitsFileV4 = Path.of("../../docker/config/common/traces-limits-v4.4.toml"),
        tracesLimitsFileV5 = Path.of("../../docker/config/common/traces-limits-v5.toml"),
        gasPriceCapTimeOfDayMultipliersFile = Path.of(
          "../../docker/config/common/gas-price-cap-time-of-day-multipliers.toml",
        ),
        smartContractErrorsFile = Path.of("../../docker/config/common/smart-contract-errors.toml"),
        enforceStrict = true,
      ).copy(
        messageAnchoring = messageAnchoring,
        protocol = protocol,
      )

      assertThat(MessageAnchoringAppConfigurator.create(vertx, config))
        .isInstanceOf(MessageAnchoringApp::class.java)
    } finally {
      rpcServer.stop(0)
    }
  }
}
