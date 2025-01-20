package linea.staterecovery

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import build.linea.contract.l1.LineaContractVersion
import build.linea.domain.BlockInterval
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.log4j.configureLoggers
import linea.testing.Runner
import linea.web3j.createWeb3jHttpClient
import net.consensys.linea.BlockParameter
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.testing.submission.AggregationAndBlobs
import net.consensys.linea.testing.submission.loadBlobsAndAggregationsSortedAndGrouped
import net.consensys.linea.testing.submission.submitBlobsAndAggregationsAndWaitExecution
import net.consensys.toULong
import net.consensys.zkevm.ethereum.ContractsManager
import net.consensys.zkevm.ethereum.LineaRollupDeploymentResult
import net.consensys.zkevm.ethereum.MakeFileDelegatedContractsManager.connectToLineaRollupContract
import net.consensys.zkevm.ethereum.MakeFileDelegatedContractsManager.lineaRollupContractErrors
import net.consensys.zkevm.ethereum.Web3jClientManager
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class StateRecoveryWithRealBesuAndStateManagerIntTest {
  private val log = LogManager.getLogger("test.case.StateRecoverAppWithLocalStackIntTest")
  private lateinit var stateManagerClient: StateManagerClientV1
  private val testDataDir = "testdata/coordinator/prover/v3"

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
  }

  private lateinit var rollupDeploymentResult: LineaRollupDeploymentResult

  @Test
  fun setupDeployContractForL2L1StateReplay() {
    configureLoggers(
      rootLevel = Level.INFO,
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
    val aggregationsAndBlobs: List<AggregationAndBlobs> = loadBlobsAndAggregationsSortedAndGrouped(
      blobsResponsesDir = "$testDataDir/compression/responses",
      aggregationsResponsesDir = "$testDataDir/aggregation/responses"
    )

    this.rollupDeploymentResult = ContractsManager.get()
      .deployLineaRollup(numberOfOperators = 2, contractVersion = LineaContractVersion.V6).get()
    log.info("""LineaRollup address=${rollupDeploymentResult.contractAddress}""")
    log.info("starting stack for recovery of state pushed to L1")
    val staterecoveryNodesStartFuture = SafeFuture.supplyAsync {
      Runner.executeCommand(
        "make staterecovery-replay-from-genesis L1_ROLLUP_CONTRACT_ADDRESS=${rollupDeploymentResult.contractAddress}"
      )
    }
    val blobsSubmissionFuture = SafeFuture.supplyAsync {
      submitBlobsAndAggregationsAndWaitExecution(
        contractClientForBlobSubmission = rollupDeploymentResult.rollupOperatorClient,
        contractClientForAggregationSubmission = connectToLineaRollupContract(
          rollupDeploymentResult.contractAddress,
          // index 0 is the first operator in rollupOperatorClient
          rollupDeploymentResult.rollupOperators[1].txManager,
          smartContractErrors = lineaRollupContractErrors
        ),
        aggregationsAndBlobs = aggregationsAndBlobs,
        blobChunksSize = 6,
        l1Web3jClient = Web3jClientManager.l1Client,
        waitTimeout = 4.minutes
      )
    }
    SafeFuture.allOf(staterecoveryNodesStartFuture, blobsSubmissionFuture).get()

    val web3jElClient = createWeb3jHttpClient(executionLayerUrl)

    // wait for state-manager to be up and running
    await()
      .pollInterval(1.seconds.toJavaDuration())
      .atMost(5.minutes.toJavaDuration())
      .untilAsserted {
        kotlin.runCatching {
          assertThat(web3jElClient.ethBlockNumber().send().blockNumber.toLong()).isGreaterThanOrEqualTo(0L)
        }.getOrElse {
          log.info("waiting for Besu to start, trying to connect to $executionLayerUrl")
          throw AssertionError("could not connect to $executionLayerUrl", it)
        }
      }

    val lastAggregationAndBlobs = aggregationsAndBlobs.findLast { it.aggregation != null }!!
    val lastAggregation = lastAggregationAndBlobs.aggregation!!
    await()
      .untilAsserted {
        assertThat(
          rollupDeploymentResult.rollupOperatorClient
            .finalizedL2BlockNumber(blockParameter = BlockParameter.Tag.LATEST).get()
        ).isGreaterThanOrEqualTo(lastAggregation.endBlockNumber)
      }
    log.info("finalization={} executed on l1", lastAggregation.intervalString())

    val expectedZkEndStateRootHash = lastAggregationAndBlobs.blobs.last().blobCompressionProof!!.finalStateRootHash
    await()
      .atMost(5.minutes.toJavaDuration())
      .untilAsserted {
        assertThat(web3jElClient.ethBlockNumber().send().blockNumber.toULong())
          .isGreaterThanOrEqualTo(lastAggregation.endBlockNumber)
        val blockInterval = BlockInterval(lastAggregation.endBlockNumber, lastAggregation.endBlockNumber)
        assertThat(stateManagerClient.rollupGetStateMerkleProof(blockInterval).get().zkEndStateRootHash)
          .isEqualTo(expectedZkEndStateRootHash)
      }
  }
}
