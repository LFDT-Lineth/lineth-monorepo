package linea.staterecovery

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.contract.l1.LineaContractVersion
import linea.log4j.configureLoggers
import linea.staterecovery.test.assertBesuAndShomeiRecoveredAsExpected
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
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI
import kotlin.time.Duration
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
        backoffDelay = 1.seconds,
        timeout = 4.seconds
      ),
      zkStateManagerVersion = "2.3.0",
      logger = LogManager.getLogger("test.clients.l1.state-manager")
    )

    configureLoggers(
      rootLevel = Level.INFO,
      log.name to Level.DEBUG,
      "test.clients.l1.state-manager" to Level.DEBUG,
      "test.clients.l1.web3j-default" to Level.DEBUG
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
        "STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER=1",
      timeout = 1.minutes,
      log = log
    ).thenPeek {
      log.info("make staterecovery-replay-from-block executed")
    }

    val lastAggregationAndBlobs = aggregationsAndBlobs.findLast { it.aggregation != null }!!
    val lastAggregation = lastAggregationAndBlobs.aggregation!!

    val blobsSubmissionFuture = SafeFuture.supplyAsync {
      submitBlobsAndAggregationsAndWaitExecution(
        contractClientForBlobSubmission = contractClientForBlobSubmission,
        contractClientForAggregationSubmission = contractClientForAggregationSubmission,
        aggregationsAndBlobs = aggregationsAndBlobs,
        blobChunksMaxSize = 6,
        l1Web3jClient = Web3jClientManager.l1Client,
        waitTimeout = 4.minutes,
        log = log
      )
      log.info("finalization={} executed on l1", lastAggregation.intervalString())
    }
    SafeFuture.allOf(staterecoveryNodesStartFuture, blobsSubmissionFuture).get()

    // wait for Besu to be up and running
    waitExecutionLayerToBeUpAndRunning(executionLayerUrl, log = log, timeout = 30.seconds)

    assertBesuAndShomeiRecoveredAsExpected(
      lastAggregationAndBlobs,
      timeout = 5.minutes
    )
  }

  private fun assertBesuAndShomeiRecoveredAsExpected(
    targetAggregationAndBlobs: AggregationAndBlobs,
    timeout: Duration
  ) {
    val targetAggregation = targetAggregationAndBlobs.aggregation!!
    val expectedZkEndStateRootHash = targetAggregationAndBlobs.blobs.last().blobCompressionProof!!.finalStateRootHash

    assertBesuAndShomeiRecoveredAsExpected(
      createWeb3jHttpClient(executionLayerUrl),
      stateManagerClient = stateManagerClient,
      targetAggregation.endBlockNumber,
      expectedZkEndStateRootHash,
      timeout = timeout
    )
  }
}
