package linea.staterecovery

import build.linea.clients.StateManagerClientV1
import build.linea.contract.l1.LineaRollupSmartContractClientReadOnly
import build.linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.EthLogsSearcher
import linea.domain.RetryConfig
import linea.log4j.configureLoggers
import linea.staterecovery.clients.VertxTransactionDetailsClient
import linea.staterecovery.clients.blobscan.BlobScanClient
import linea.staterecovery.test.FakeExecutionLayerClient
import linea.staterecovery.test.FakeStateManagerClientReadFromL1
import linea.web3j.Web3JLogsSearcher
import net.consensys.linea.BlockNumberAndHash
import net.consensys.linea.BlockParameter
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.zkevm.ethereum.Web3jClientManager.buildWeb3Client
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class StateRecoverSepoliaWithFakeExecutionClientIntTest {
  private val log = LogManager.getLogger("test.case.StateRecoverSepoliaWithFakeExecutionClientIntTest")
  private lateinit var stateRecoverApp: StateRecoverApp
  private lateinit var logsSearcher: EthLogsSearcher
  private lateinit var executionLayerClient: FakeExecutionLayerClient
  private lateinit var blobFetcher: BlobFetcher
  private lateinit var fakeStateManagerClient: StateManagerClientV1
  private lateinit var transactionDetailsClient: TransactionDetailsClient
  private lateinit var lineaContractClient: LineaRollupSmartContractClientReadOnly
  private val infuraAppKey = System.getenv("INFURA_APP_KEY")
    .also {
      assertThat(it)
        .withFailMessage("Please define INFURA_APP_KEY environment variable")
        .isNotEmpty()
    }
  private val l1RpcUrl = "https://sepolia.infura.io/v3/$infuraAppKey"
  private val blobScanUrl = "https://api.sepolia.blobscan.com/"

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    val jsonRpcFactory = VertxHttpJsonRpcClientFactory(
      vertx = vertx,
      metricsFacade = MicrometerMetricsFacade(SimpleMeterRegistry())
    )
    executionLayerClient = FakeExecutionLayerClient(
      headBlock = BlockNumberAndHash(number = 0uL, hash = ByteArray(32) { 0 }),
      initialStateRecoverStartBlockNumber = null,
      loggerName = "test.fake.clients.execution-layer"
    )
    blobFetcher = BlobScanClient.create(
      vertx = vertx,
      endpoint = URI(blobScanUrl),
      requestRetryConfig = RequestRetryConfig(
        backoffDelay = 10.milliseconds,
        timeout = 5.seconds
      ),
      logger = LogManager.getLogger("test.clients.l1.blobscan"),
      responseLogMaxSize = 100u
    )
    logsSearcher = Web3JLogsSearcher(
      vertx = vertx,
      web3jClient = buildWeb3Client(
        rpcUrl = l1RpcUrl,
        log = LogManager.getLogger("test.clients.l1.events-fetcher"),
        requestResponseLogLevel = Level.TRACE,
        failuresLogLevel = Level.DEBUG
      ),
      Web3JLogsSearcher.Config(
        backoffDelay = 400.milliseconds,
        requestRetryConfig = RetryConfig(
          backoffDelay = 1.seconds
        )
      ),
      log = LogManager.getLogger("test.clients.l1.events-fetcher")
    )
    fakeStateManagerClient = FakeStateManagerClientReadFromL1(
      headBlockNumber = ULong.MAX_VALUE,
      logsSearcher = logsSearcher,
      contractAddress = StateRecoverApp.Config.lineaSepolia.smartContractAddress
    )
    transactionDetailsClient = VertxTransactionDetailsClient.create(
      jsonRpcClientFactory = jsonRpcFactory,
      endpoint = URI(l1RpcUrl),
      retryConfig = RequestRetryConfig(
        backoffDelay = 1.seconds
      ),
      logger = LogManager.getLogger("test.clients.l1.transaction-details")
    )

    lineaContractClient = Web3JLineaRollupSmartContractClientReadOnly(
      web3j = buildWeb3Client(
        rpcUrl = l1RpcUrl,
        log = LogManager.getLogger("test.clients.l1.linea-contract"),
        requestResponseLogLevel = Level.INFO,
        failuresLogLevel = Level.DEBUG
      ),
      contractAddress = StateRecoverApp.Config.lineaSepolia.smartContractAddress
    )

    stateRecoverApp = StateRecoverApp(
      vertx = vertx,
      elClient = executionLayerClient,
      blobFetcher = blobFetcher,
      ethLogsSearcher = logsSearcher,
      stateManagerClient = fakeStateManagerClient,
      transactionDetailsClient = transactionDetailsClient,
      blockHeaderStaticFields = BlockHeaderStaticFields.localDev,
      lineaContractClient = lineaContractClient,
      config = StateRecoverApp.Config(
        l1LatestSearchBlock = BlockParameter.Tag.LATEST,
        l1PollingInterval = 5.seconds,
        executionClientPollingInterval = 1.seconds,
        smartContractAddress = lineaContractClient.getAddress(),
        logsBlockChunkSize = 5000u
      )
    )
    configureLoggers(
      rootLevel = Level.INFO,
      "test.clients.l1.execution-layer" to Level.INFO,
      "test.clients.l1.web3j-default" to Level.DEBUG,
      "test.clients.l1.transaction-details" to Level.INFO,
      "test.clients.l1.linea-contract" to Level.INFO,
      "test.clients.l1.events-fetcher" to Level.DEBUG,
      "test.clients.l1.blobscan" to Level.INFO,
      "net.consensys.linea.contract.l1" to Level.DEBUG
    )
  }

  // "Disabled because it is for local testing and debug purposes"
//  @Test
  fun `simulate recovery from given point`() {
    val finalizationEvents = logsSearcher
      .getLogs(
        fromBlock = BlockParameter.Tag.EARLIEST,
        toBlock = BlockParameter.Tag.LATEST,
        address = lineaContractClient.getAddress(),
        topics = listOf(DataFinalizedV3.topic)
      )
      .get()
    val firstFinalizationEvent = DataFinalizedV3.fromEthLog(finalizationEvents.first())
    val lastFinalizationEvent = DataFinalizedV3.fromEthLog(finalizationEvents.last())
    log.info("First finalization event: $firstFinalizationEvent")
    log.info("Latest finalization event: $lastFinalizationEvent")

    executionLayerClient.headBlock = BlockNumberAndHash(
      number = firstFinalizationEvent.event.startBlockNumber - 1UL,
      hash = ByteArray(32) { 0 }
    )
    stateRecoverApp.trySetRecoveryModeAtBlockHeight(firstFinalizationEvent.event.startBlockNumber).get()
    stateRecoverApp.start().get()

    await()
      .atMost(10.minutes.toJavaDuration())
      .pollInterval(10.seconds.toJavaDuration())
      .untilAsserted {
        val updatedStatus = executionLayerClient.lineaGetStateRecoveryStatus().get()
        assertThat(updatedStatus.headBlockNumber).isGreaterThan(lastFinalizationEvent.event.endBlockNumber)
      }
  }
}
