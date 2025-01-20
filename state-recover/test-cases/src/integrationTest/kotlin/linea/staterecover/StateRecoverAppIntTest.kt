package linea.staterecover

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import build.linea.contract.l1.LineaContractVersion
import build.linea.contract.l1.LineaRollupSmartContractClientReadOnly
import build.linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly
import build.linea.staterecover.clients.el.ExecutionLayerJsonRpcClient
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.build.staterecover.clients.VertxTransactionDetailsClient
import linea.domain.RetryConfig
import linea.log4j.configureLoggers
import linea.staterecover.clients.blobscan.BlobScanClient
import linea.web3j.Web3JLogsSearcher
import net.consensys.linea.BlockParameter
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.testing.submission.AggregationAndBlobs
import net.consensys.linea.testing.submission.loadBlobsAndAggregationsSortedAndGrouped
import net.consensys.linea.testing.submission.submitBlobsAndAggregationsAndWaitExecution
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.ethereum.ContractsManager
import net.consensys.zkevm.ethereum.Web3jClientManager
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class StateRecoverAppIntTest {
  private val log = LogManager.getLogger("test.case.StateRecoverAppIntTest")
  private lateinit var stateRecoverApp: StateRecoverApp
  private lateinit var aggregationsAndBlobs: List<AggregationAndBlobs>
  private lateinit var executionLayerClient: ExecutionLayerClient
  private lateinit var stateManagerClient: StateManagerClientV1
  private lateinit var transactionDetailsClient: TransactionDetailsClient
  private lateinit var lineaContractClient: LineaRollupSmartContractClientReadOnly

  private lateinit var contractClientForSubmissions: LineaRollupSmartContractClient

  private val testDataDir = "testdata/coordinator/prover/v3/"

  private val l1RpcUrl = "http://localhost:8445"
  private val blobScanUrl = "http://localhost:4001"
  private val executionClientUrl = "http://localhost:9145"
  private val stateManagerUrl = "http://localhost:8890"

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    val jsonRpcFactory = VertxHttpJsonRpcClientFactory(
      vertx = vertx,
      metricsFacade = MicrometerMetricsFacade(SimpleMeterRegistry())
    )
    aggregationsAndBlobs = loadBlobsAndAggregationsSortedAndGrouped(
      blobsResponsesDir = "$testDataDir/compression/responses",
      aggregationsResponsesDir = "$testDataDir/aggregation/responses"
    )
    executionLayerClient = ExecutionLayerJsonRpcClient.create(
      rpcClientFactory = jsonRpcFactory,
      endpoint = URI(executionClientUrl),
      requestRetryConfig = RequestRetryConfig(
        backoffDelay = 10.milliseconds,
        timeout = 2.seconds,
        maxRetries = 4u,
        failuresWarningThreshold = 1U
      ),
      logger = LogManager.getLogger("test.clients.l1.executionlayer")
    )
    stateManagerClient = StateManagerV1JsonRpcClient.create(
      rpcClientFactory = jsonRpcFactory,
      endpoints = listOf(URI(stateManagerUrl)),
      maxInflightRequestsPerClient = 1U,
      requestRetry = RequestRetryConfig(
        backoffDelay = 10.milliseconds,
        timeout = 2.seconds
      ),
      zkStateManagerVersion = "2.3.0",
      logger = LogManager.getLogger("test.clients.l1.state-manager")
    )
    transactionDetailsClient = VertxTransactionDetailsClient.create(
      jsonRpcClientFactory = jsonRpcFactory,
      endpoint = URI(l1RpcUrl),
      retryConfig = RequestRetryConfig(
        backoffDelay = 10.milliseconds,
        timeout = 2.seconds
      ),
      logger = LogManager.getLogger("test.clients.l1.transaction-details")
    )

    val rollupDeploymentResult = ContractsManager.get()
      .deployLineaRollup(numberOfOperators = 2, contractVersion = LineaContractVersion.V6).get()

    lineaContractClient = Web3JLineaRollupSmartContractClientReadOnly(
      web3j = Web3jClientManager.buildL1Client(
        log = LogManager.getLogger("test.clients.l1.linea-contract"),
        requestResponseLogLevel = Level.INFO,
        failuresLogLevel = Level.WARN
      ),
      contractAddress = rollupDeploymentResult.contractAddress
    )
    val logsSearcher = run {
      val log = LogManager.getLogger("test.clients.l1.events-fetcher")
      Web3JLogsSearcher(
        vertx = vertx,
        web3jClient = Web3jClientManager.buildL1Client(
          log = log,
          requestResponseLogLevel = Level.TRACE,
          failuresLogLevel = Level.WARN
        ),
        Web3JLogsSearcher.Config(
          backoffDelay = 1.milliseconds,
          requestRetryConfig = RetryConfig.noRetries
        ),
        log = log
      )
    }

    contractClientForSubmissions = rollupDeploymentResult.rollupOperatorClient
    val blobScanClient = BlobScanClient.create(
      vertx = vertx,
      endpoint = URI(blobScanUrl),
      requestRetryConfig = RequestRetryConfig(
        backoffDelay = 10.milliseconds,
        timeout = 2.seconds
      ),
      logger = LogManager.getLogger("test.clients.l1.blobscan")
    )

    configureLoggers(
      rootLevel = Level.INFO,
      "test.clients.l1.executionlayer" to Level.INFO,
      "test.clients.l1.web3j-default" to Level.INFO,
      "test.clients.l1.state-manager" to Level.INFO,
      "test.clients.l1.transaction-details" to Level.INFO,
      "test.clients.l1.linea-contract" to Level.INFO,
      "test.clients.l1.events-fetcher" to Level.INFO,
      "test.clients.l1.blobscan" to Level.INFO,
      "net.consensys.linea.contract.l1" to Level.INFO
    )

    stateRecoverApp = StateRecoverApp(
      vertx = vertx,
      elClient = executionLayerClient,
      blobFetcher = blobScanClient,
      ethLogsSearcher = logsSearcher,
      stateManagerClient = stateManagerClient,
      transactionDetailsClient = transactionDetailsClient,
      blockHeaderStaticFields = BlockHeaderStaticFields.localDev,
      lineaContractClient = lineaContractClient,
      config = StateRecoverApp.Config(
        l1LatestSearchBlock = BlockParameter.Tag.LATEST,
        l1PollingInterval = 5.seconds,
        executionClientPollingInterval = 1.seconds,
        smartContractAddress = lineaContractClient.getAddress(),
        logsBlockChunkSize = 100_000u
      )
    )
  }

  @Test
  fun `state recovery from genesis`() {
    stateRecoverApp.start().get()

    submitBlobsAndAggregationsAndWaitExecution(
      contractClient = contractClientForSubmissions,
      aggregationsAndBlobs = aggregationsAndBlobs,
      l1Web3jClient = Web3jClientManager.l1Client
    )

    val lastAggregation = aggregationsAndBlobs.findLast { it.aggregation != null }!!.aggregation!!
    await()
      .atMost(4.minutes.toJavaDuration())
      .untilAsserted {
        assertThat(stateRecoverApp.lastSuccessfullyRecoveredFinalization?.event?.endBlockNumber)
          .isEqualTo(lastAggregation.endBlockNumber)
      }

    assertThat(executionLayerClient.lineaGetStateRecoveryStatus().get())
      .isEqualTo(
        StateRecoveryStatus(
          headBlockNumber = lastAggregation.endBlockNumber,
          stateRecoverStartBlockNumber = 1UL
        )
      )
  }
}
