package linea.staterecover

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import build.linea.contract.l1.LineaContractVersion
import build.linea.domain.BlockInterval
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.log4j.configureLoggers
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
class StateRecoveryManualReplayToLocalStackIntTest {
  private val log = LogManager.getLogger("test.case.StateRecoverAppWithLocalStackIntTest")
  private lateinit var stateManagerClient: StateManagerClientV1
  private val testDataDir = "testdata/coordinator/prover/v3"

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
    log.info(
      """
      Start state recovery besu and shomei with the following configuration:

       ./gradlew state-recover:besu-plugin:shadowJar \
       && docker compose -f docker/compose.yml down zkbesu-shomei-sr shomei-sr \
       && L1_ROLLUP_CONTRACT_ADDRESS=${rollupDeploymentResult.contractAddress} docker compose -f docker/compose.yml up zkbesu-shomei-sr shomei-sr

      """.trimIndent()
    )

    val web3jElClient = createWeb3jHttpClient("http://localhost:9145")

    // wait for statemanager to be up and running
    await()
      .pollInterval(1.seconds.toJavaDuration())
      .atMost(5.minutes.toJavaDuration())
      .untilAsserted {
        kotlin.runCatching {
          assertThat(web3jElClient.ethBlockNumber().send().blockNumber.toLong()).isGreaterThanOrEqualTo(0L)
        }.getOrElse {
          log.info("could not connect to stateManager $stateManagerUrl")
          throw AssertionError("could not connect to stateManager $stateManagerUrl", it)
        }
      }

    val lastAggregationAndBlobs = aggregationsAndBlobs.findLast { it.aggregation != null }!!
    val lastAggregation = lastAggregationAndBlobs.aggregation!!
    // wait until state recovery Besu and Shomei are up
    submitBlobsAndAggregationsAndWaitExecution(
      contractClient = rollupDeploymentResult.rollupOperatorClient,
      aggregationsAndBlobs = aggregationsAndBlobs,
      blobChunksSize = 6,
      l1Web3jClient = Web3jClientManager.l1Client
    )
    log.info("finalization={} executed on l1", lastAggregation.intervalString())

    await()
      .untilAsserted {
        assertThat(
          rollupDeploymentResult.rollupOperatorClient
            .finalizedL2BlockNumber(blockParameter = BlockParameter.Tag.LATEST).get()
        ).isGreaterThanOrEqualTo(lastAggregation.endBlockNumber)
      }

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
