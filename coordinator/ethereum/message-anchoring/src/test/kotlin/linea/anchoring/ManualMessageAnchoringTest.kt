package linea.anchoring

import io.vertx.core.Vertx
import linea.contract.l2.FakeL2MessageService
import linea.domain.BlockParameter
import linea.domain.RetryConfig
import linea.log4j.configureLoggers
import linea.web3j.ethapi.createEthApiClient
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import java.lang.IllegalStateException
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

/**
 * This is meant to run Manually with FakeL2 Contract to troubleshoot anchoring issues against a production environment.
 */
class ManualMessageAnchoringTest {

  // @Test
  fun `should anchor messages`() {
    val vertx = Vertx.vertx()
    val fakeL2MessageService = FakeL2MessageService(contractAddress = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
    val l1Client = createEthApiClient(
      System.getenv("URL_SEPOLIA") ?: throw IllegalStateException("URL_SEPOLIA not set"),
      log = LogManager.getLogger("clients.l1"),
      requestRetryConfig = null,
      vertx = null,
    )
    val anchoringApp = MessageAnchoringApp(
      vertx = vertx,
      config = MessageAnchoringApp.Config(
        l1RequestRetryConfig = RetryConfig.noRetries,
        l1ContractAddress = "0xB218f8A4Bc926cF1cA7b3423c154a0D627Bdb7E5", // Sepolia Contract address
        l1HighestBlockTag = BlockParameter.Tag.FINALIZED,
        l2HighestBlockTag = BlockParameter.Tag.LATEST,
        anchoringTickInterval = 5.seconds,
      ),
      l1EthApiClient = l1Client,
      l2MessageService = fakeL2MessageService,
    )
    configureLoggers(
      rootLevel = Level.INFO,
      "clients.l1" to Level.TRACE,
      "linea.anchoring" to Level.DEBUG,
    )
    fakeL2MessageService.setLastAnchoredL1Message(83002UL, ByteArray(32) { 0 }) // Set the last anchored message number
    anchoringApp.start().get()
    Thread.sleep(5.minutes.inWholeMilliseconds) // Keep the app running for 5 minutes to allow anchoring
  }
}
