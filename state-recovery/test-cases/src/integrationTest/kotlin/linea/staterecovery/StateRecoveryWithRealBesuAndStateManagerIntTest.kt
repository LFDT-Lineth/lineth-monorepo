package linea.staterecovery

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import build.linea.contract.l1.LineaContractVersion
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.log4j.configureLoggers
import linea.staterecovery.test.assertBesuAndShomeiStateRootMatches
import linea.staterecovery.test.execCommandAndAssertSuccess
import linea.staterecovery.test.waitExecutionLayerToBeUpAndRunning
import linea.web3j.createWeb3jHttpClient
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.testing.submission.AggregationAndBlobs
import net.consensys.linea.testing.submission.loadBlobsAndAggregationsSortedAndGrouped
import net.consensys.linea.testing.submission.submitBlobsAndAggregationsAndWaitExecution
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.ethereum.ContractsManager
import net.consensys.zkevm.ethereum.LineaRollupDeploymentResult
import net.consensys.zkevm.ethereum.MakeFileDelegatedContractsManager.connectToLineaRollupContract
import net.consensys.zkevm.ethereum.MakeFileDelegatedContractsManager.lineaRollupContractErrors
import net.consensys.zkevm.ethereum.Web3jClientManager
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Order
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class StateRecoveryWithRealBesuAndStateManagerIntTest {
  private val log = LogManager.getLogger("test.case.StateRecoverAppWithLocalStackIntTest")
  private lateinit var stateManagerClient: StateManagerClientV1
  private val testDataDir = "testdata/coordinator/prover/v3"
  private val aggregationsAndBlobs: List<AggregationAndBlobs> = loadBlobsAndAggregationsSortedAndGrouped(
    blobsResponsesDir = "$testDataDir/compression/responses",
    aggregationsResponsesDir = "$testDataDir/aggregation/responses"
  )
  private lateinit var rollupDeploymentResult: LineaRollupDeploymentResult
  private lateinit var contractClientForBlobSubmission: LineaRollupSmartContractClient
  private lateinit var contractClientForAggregationSubmission: LineaRollupSmartContractClient
  private val executionLayerUrl = "http://localhost:9145"
  private val stateManagerUrl = "http://localhost:8890"

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    val jsonRpcFactory = VertxHttpJsonRpcClientFactory(
      vertx = vertx,
      metricsFacade = MicrometerMetricsFacade(SimpleMeterRegistry())
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

    configureLoggers(
      rootLevel = Level.INFO,
      log.name to Level.DEBUG,
      "net.consensys.linea.contract.Web3JContractAsyncHelper" to Level.WARN,
      "test.clients.l1.executionlayer" to Level.INFO,
      "test.clients.l1.web3j-default" to Level.INFO,
      "test.clients.l1.state-manager" to Level.DEBUG,
      "test.clients.l1.transaction-details" to Level.INFO,
      "test.clients.l1.linea-contract" to Level.INFO,
      "test.clients.l1.events-fetcher" to Level.INFO,
      "test.clients.l1.blobscan" to Level.INFO,
      "net.consensys.linea.contract.l1" to Level.INFO
    )
  }

  @Test
  @Order(1)
  fun `should recover status from genesis - seed data replay`() {
    this.rollupDeploymentResult = ContractsManager.get()
      .deployLineaRollup(numberOfOperators = 2, contractVersion = LineaContractVersion.V6)
      .get()
    log.info("LineaRollup address={}", rollupDeploymentResult.contractAddress)
    contractClientForBlobSubmission = rollupDeploymentResult.rollupOperatorClient
    contractClientForAggregationSubmission = connectToLineaRollupContract(
      rollupDeploymentResult.contractAddress,
      // index 0 is the first operator in rollupOperatorClient
      rollupDeploymentResult.rollupOperators[1].txManager,
      smartContractErrors = lineaRollupContractErrors
    )
    log.info("starting stack for recovery of state pushed to L1")
    val staterecoveryNodesStartFuture = execCommandAndAssertSuccess(
      "make staterecovery-replay-from-block " +
        "L1_ROLLUP_CONTRACT_ADDRESS=${rollupDeploymentResult.contractAddress} " +
        "PLUGIN_STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER=1",
      log = log
    )
    val lastAggregationAndBlobs = aggregationsAndBlobs.findLast { it.aggregation != null }!!
    val lastAggregation = lastAggregationAndBlobs.aggregation!!

    val blobsSubmissionFuture = SafeFuture.supplyAsync {
      submitBlobsAndAggregationsAndWaitExecution(
        contractClientForBlobSubmission = contractClientForBlobSubmission,
        contractClientForAggregationSubmission = contractClientForAggregationSubmission,
        aggregationsAndBlobs = aggregationsAndBlobs,
        blobChunksSize = 6,
        l1Web3jClient = Web3jClientManager.l1Client,
        waitTimeout = 4.minutes
      )
      log.info("finalization={} executed on l1", lastAggregation.intervalString())
    }
    SafeFuture.allOf(staterecoveryNodesStartFuture, blobsSubmissionFuture).get()

    val web3jElClient = createWeb3jHttpClient(executionLayerUrl)

    // wait for Besu to be up and running
    waitExecutionLayerToBeUpAndRunning(executionLayerUrl, log = log)

    assertBesuAndShomeiStateRootMatches(web3jElClient, stateManagerClient, lastAggregationAndBlobs)
  }

  private fun assertBesuAndShomeiStateRootMatches(
    web3jElClient: Web3j,
    stateManagerClient: StateManagerClientV1,
    targetAggregationAndBlobs: AggregationAndBlobs
  ) {
    val targetAggregation = targetAggregationAndBlobs.aggregation!!
    val expectedZkEndStateRootHash = targetAggregationAndBlobs.blobs.last().blobCompressionProof!!.finalStateRootHash

    assertBesuAndShomeiStateRootMatches(
      web3jElClient,
      stateManagerClient,
      targetAggregation.endBlockNumber,
      expectedZkEndStateRootHash
    )
  }
}
