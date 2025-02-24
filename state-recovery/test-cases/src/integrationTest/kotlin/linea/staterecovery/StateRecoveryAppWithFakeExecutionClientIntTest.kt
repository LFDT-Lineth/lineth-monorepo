package linea.staterecovery

import build.linea.contract.l1.LineaContractVersion
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.domain.BlockNumberAndHash
import linea.domain.BlockParameter
import linea.domain.RetryConfig
import linea.log4j.configureLoggers
import linea.staterecovery.plugin.AppClients
import linea.staterecovery.plugin.createAppClients
import linea.staterecovery.test.FakeExecutionLayerClient
import linea.staterecovery.test.FakeStateManagerClient
import linea.staterecovery.test.FakeStateManagerClientBasedOnBlobsRecords
import linea.web3j.createWeb3jHttpClient
import net.consensys.linea.testing.submission.AggregationAndBlobs
import net.consensys.linea.testing.submission.loadBlobsAndAggregationsSortedAndGrouped
import net.consensys.linea.testing.submission.submitBlobsAndAggregationsAndWaitExecution
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.ethereum.ContractsManager
import net.consensys.zkevm.ethereum.MakeFileDelegatedContractsManager.connectToLineaRollupContract
import net.consensys.zkevm.ethereum.MakeFileDelegatedContractsManager.lineaRollupContractErrors
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class StateRecoveryAppWithFakeExecutionClientIntTest {
  private val log = LogManager.getLogger("test.case.StateRecoverAppWithFakeExecutionClientIntTest")
  private lateinit var appConfigs: StateRecoveryApp.Config
  private lateinit var aggregationsAndBlobs: List<AggregationAndBlobs>
  private lateinit var fakeExecutionLayerClient: FakeExecutionLayerClient
  private lateinit var fakeStateManagerClient: FakeStateManagerClient
  private lateinit var contractClientForBlobSubmissions: LineaRollupSmartContractClient
  private lateinit var contractClientForAggregationSubmissions: LineaRollupSmartContractClient
  private lateinit var vertx: Vertx
  private lateinit var appClients: AppClients

  private val testDataDir = run {
    "testdata/coordinator/prover/v3"
  }

  private val l1RpcUrl = "http://localhost:8445"
  private val blobScanUrl = "http://localhost:4001"

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    this.vertx = vertx
    vertx.exceptionHandler {
      log.error("unhandled exception", it)
    }
    aggregationsAndBlobs = loadBlobsAndAggregationsSortedAndGrouped(
      blobsResponsesDir = "$testDataDir/compression/responses",
      aggregationsResponsesDir = "$testDataDir/aggregation/responses",
      numberOfAggregations = 4,
      extraBlobsWithoutAggregation = 0
    )
    fakeExecutionLayerClient = FakeExecutionLayerClient(
      headBlock = BlockNumberAndHash(number = 0uL, hash = ByteArray(32) { 0 }),
      initialStateRecoverStartBlockNumber = null,
      loggerName = "test.fake.clients.l1.fake-execution-layer"
    )
    fakeStateManagerClient =
      FakeStateManagerClientBasedOnBlobsRecords(blobRecords = aggregationsAndBlobs.flatMap { it.blobs })

    val rollupDeploymentResult = ContractsManager.get()
      .deployLineaRollup(numberOfOperators = 2, contractVersion = LineaContractVersion.V6).get()

    this.appConfigs = StateRecoveryApp.Config(
      l1EarliestSearchBlock = BlockParameter.Tag.EARLIEST,
      l1LatestSearchBlock = BlockParameter.Tag.LATEST,
      l1PollingInterval = 100.milliseconds,
      l1getLogsChunkSize = 1000u,
      executionClientPollingInterval = 1.seconds,
      smartContractAddress = rollupDeploymentResult.contractAddress
    )

    appClients = createAppClients(
      vertx = vertx,
      smartContractAddress = appConfigs.smartContractAddress,
      l1RpcEndpoint = URI(l1RpcUrl),
      l1RequestRetryConfig = RetryConfig(backoffDelay = 2.seconds),
      blobScanEndpoint = URI(blobScanUrl),
      stateManagerClientEndpoint = URI("http://it-does-not-matter:5432")
    )

    contractClientForBlobSubmissions = rollupDeploymentResult.rollupOperatorClient
    contractClientForAggregationSubmissions = connectToLineaRollupContract(
      rollupDeploymentResult.contractAddress,
      rollupDeploymentResult.rollupOperators[1].txManager,
      smartContractErrors = lineaRollupContractErrors
    )

    configureLoggers(
      rootLevel = Level.INFO,
      log.name to Level.DEBUG,
      "linea.testing.submission" to Level.DEBUG,
      "net.consensys.linea.contract.Web3JContractAsyncHelper" to Level.WARN, // silence noisy gasPrice Caps logs
      "linea.staterecovery.BlobDecompressorToDomainV1" to Level.DEBUG,
      "linea.plugin.staterecovery.clients" to Level.INFO,
      "test.fake.clients.l1.fake-execution-layer" to Level.DEBUG,
      "test.clients.l1.web3j-default" to Level.DEBUG,
      "test.clients.l1.web3j.receipt-poller" to Level.TRACE,
      "linea.staterecovery.datafetching" to Level.TRACE
    )
  }

  fun instantiateStateRecoveryApp(
    debugForceSyncStopBlockNumber: ULong? = null
  ): StateRecoveryApp {
    return StateRecoveryApp(
      vertx = vertx,
      elClient = fakeExecutionLayerClient,
      blobFetcher = appClients.blobScanClient,
      ethLogsSearcher = appClients.ethLogsSearcher,
      stateManagerClient = fakeStateManagerClient,
      transactionDetailsClient = appClients.transactionDetailsClient,
      blockHeaderStaticFields = BlockHeaderStaticFields.localDev,
      lineaContractClient = appClients.lineaContractClient,
      config = appConfigs.copy(
        debugForceSyncStopBlockNumber = debugForceSyncStopBlockNumber
      )
    )
  }

  private fun submitDataToL1ContactAndWaitExecution(
    aggregationsAndBlobs: List<AggregationAndBlobs> = this.aggregationsAndBlobs,
    blobChunksSize: Int = 6,
    waitTimeout: Duration = 4.minutes
  ) {
    submitBlobsAndAggregationsAndWaitExecution(
      contractClientForBlobSubmission = contractClientForBlobSubmissions,
      contractClientForAggregationSubmission = contractClientForAggregationSubmissions,
      aggregationsAndBlobs = aggregationsAndBlobs,
      blobChunksMaxSize = blobChunksSize,
      waitTimeout = waitTimeout,
      l1Web3jClient = createWeb3jHttpClient(
        rpcUrl = l1RpcUrl,
        log = LogManager.getLogger("test.clients.l1.web3j.receipt-poller")
      )
    )
  }

  /*
  1. when state recovery disabled: enables with lastFinalizedBlock + 1
  1.1 no finalizations and start from genesis
  1.2 headBlockNumber > lastFinalizedBlock -> enable with headBlockNumber + 1
  1.3 headBlockNumber < lastFinalizedBlock -> enable with lastFinalizedBlock + 1

  2. when state recovery enabled:
  2.1 recoveryStartBlockNumber > headBlockNumber: pull for head block number until is reached and start recovery there
  2.2 recoveryStartBlockNumber <= headBlockNumber: resume recovery from headBlockNumber
  */
  @Test
  fun `when state recovery disabled and is starting from genesis`() {
    instantiateStateRecoveryApp().start().get()
    submitDataToL1ContactAndWaitExecution()

    val lastAggregation = aggregationsAndBlobs.findLast { it.aggregation != null }!!.aggregation
    await()
      .atMost(2.minutes.toJavaDuration())
      .untilAsserted {
        assertThat(fakeExecutionLayerClient.importedBlockNumbersInRecoveryMode.lastOrNull())
          .isEqualTo(lastAggregation!!.endBlockNumber)
      }

    assertThat(fakeExecutionLayerClient.lineaGetStateRecoveryStatus().get())
      .isEqualTo(
        StateRecoveryStatus(
          headBlockNumber = lastAggregation!!.endBlockNumber,
          stateRecoverStartBlockNumber = 1UL
        )
      )
  }

  @Test
  fun `when recovery is disabled and headBlock is before lastFinalizedBlock resumes from lastFinalizedBlock+1`() {
    val finalizationToResumeFrom = aggregationsAndBlobs.get(1).aggregation!!
    // assert that the finalization event to resume from has at least 1 middle block
    assertThat(finalizationToResumeFrom.endBlockNumber)
      .isGreaterThan(finalizationToResumeFrom.startBlockNumber + 1UL)
      .withFailMessage("finalizationEventToResumeFrom must at least 3 blocks for this test")

    val finalizationsBeforeCutOff = aggregationsAndBlobs
      .filter { it.aggregation != null }
      .filter { it.aggregation!!.endBlockNumber < finalizationToResumeFrom.startBlockNumber }

    val finalizationsAfterCutOff = aggregationsAndBlobs
      .filter { it.aggregation != null }
      .filter { it.aggregation!!.startBlockNumber >= finalizationToResumeFrom.startBlockNumber }

    log.debug(
      "finalizations={} finalizationToStartRecoveryFrom={}",
      aggregationsAndBlobs.map { it.aggregation?.intervalString() },
      finalizationToResumeFrom.intervalString()
    )

    submitDataToL1ContactAndWaitExecution(
      aggregationsAndBlobs = finalizationsBeforeCutOff
    )

    fakeExecutionLayerClient.headBlock = BlockNumberAndHash(
      number = 1UL,
      hash = ByteArray(32) { 0 }
    )

    val lastFinalizedBlockNumber = finalizationsBeforeCutOff.last().aggregation!!.endBlockNumber
    val expectedStateRecoverStartBlockNumber = lastFinalizedBlockNumber + 1UL
    instantiateStateRecoveryApp().start().get()

    await()
      .atMost(4.minutes.toJavaDuration())
      .pollInterval(1.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(fakeExecutionLayerClient.stateRecoverStatus).isEqualTo(
          StateRecoveryStatus(
            headBlockNumber = 1UL,
            stateRecoverStartBlockNumber = expectedStateRecoverStartBlockNumber
          )
        )
        log.info("stateRecoverStatus={}", fakeExecutionLayerClient.stateRecoverStatus)
      }

    // simulate that execution client has synced up to the last finalized block through P2P network
    fakeExecutionLayerClient.headBlock = BlockNumberAndHash(
      number = lastFinalizedBlockNumber,
      hash = ByteArray(32) { 0 }
    )

    // continue finalizing the rest of the aggregations
    submitDataToL1ContactAndWaitExecution(
      aggregationsAndBlobs = finalizationsAfterCutOff
    )

    val lastAggregation = aggregationsAndBlobs.findLast { it.aggregation != null }!!.aggregation
    await()
      .atMost(1.minutes.toJavaDuration())
      .pollInterval(1.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(fakeExecutionLayerClient.headBlock.number).isEqualTo(lastAggregation!!.endBlockNumber)
      }

    // assert it imports correct blocks
    val importedBlocks = fakeExecutionLayerClient.importedBlockNumbersInRecoveryMode
    assertThat(importedBlocks.first()).isEqualTo(expectedStateRecoverStartBlockNumber)
    assertThat(importedBlocks.last()).isEqualTo(lastAggregation!!.endBlockNumber)
  }

  @Test
  fun `when node starts with headblock greater lastFinalizedBlock`() {
    val finalizationToResumeFrom = aggregationsAndBlobs.get(1).aggregation!!
    // assert that the finalization event to resume from has at least 1 middle block
    assertThat(finalizationToResumeFrom.endBlockNumber)
      .isGreaterThan(finalizationToResumeFrom.startBlockNumber + 1UL)
      .withFailMessage("finalizationEventToResumeFrom must at least 3 blocks for this test")

    val finalizationsBeforeCutOff = aggregationsAndBlobs
      .filter { it.aggregation != null }
      .filter { it.aggregation!!.endBlockNumber < finalizationToResumeFrom.startBlockNumber }

    val finalizationsAfterCutOff = aggregationsAndBlobs
      .filter { it.aggregation != null }
      .filter { it.aggregation!!.startBlockNumber >= finalizationToResumeFrom.startBlockNumber }

    log.debug(
      "finalizations={} finalizationToStartRecoveryFrom={}",
      aggregationsAndBlobs.map { it.aggregation?.intervalString() },
      finalizationToResumeFrom.intervalString()
    )

    submitDataToL1ContactAndWaitExecution(
      aggregationsAndBlobs = finalizationsBeforeCutOff
    )

    // set execution layer head block after latest finalization
    val headBlockNumberAtStart = finalizationsBeforeCutOff.last().aggregation!!.endBlockNumber + 1UL
    fakeExecutionLayerClient.headBlock = BlockNumberAndHash(
      number = headBlockNumberAtStart,
      hash = ByteArray(32) { 0 }
    )
    instantiateStateRecoveryApp().start().get()
    await()
      .atMost(2.minutes.toJavaDuration())
      .pollInterval(1.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(fakeExecutionLayerClient.stateRecoverStatus).isEqualTo(
          StateRecoveryStatus(
            headBlockNumber = headBlockNumberAtStart,
            stateRecoverStartBlockNumber = headBlockNumberAtStart + 1UL
          )
        )
        log.debug("stateRecoverStatus={}", fakeExecutionLayerClient.stateRecoverStatus)
      }

    // continue finalizing the rest of the aggregations
    submitDataToL1ContactAndWaitExecution(
      aggregationsAndBlobs = finalizationsAfterCutOff
    )

    val lastAggregation = aggregationsAndBlobs.findLast { it.aggregation != null }!!.aggregation
    await()
      .atMost(2.minutes.toJavaDuration())
      .pollInterval(1.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(fakeExecutionLayerClient.stateRecoverStatus)
          .isEqualTo(
            StateRecoveryStatus(
              headBlockNumber = lastAggregation!!.endBlockNumber,
              stateRecoverStartBlockNumber = headBlockNumberAtStart + 1UL
            )
          )
      }
    // assert it does not try to import blocks behind the head block
    assertThat(fakeExecutionLayerClient.importedBlockNumbersInRecoveryMode.minOrNull())
      .isEqualTo(headBlockNumberAtStart + 1UL)
  }

  @Test
  fun `should stop recovery as soon as stateroot mismatches`() {
    fakeStateManagerClient.setBlockStateRootHash(
      aggregationsAndBlobs[1].aggregation!!.endBlockNumber,
      ByteArray(32) { 1 }
    )
    log.debug(
      "aggregations={} forcedMismatchAggregation={}",
      aggregationsAndBlobs.map { it.aggregation?.intervalString() },
      aggregationsAndBlobs[1].aggregation!!.intervalString()
    )

    val stateRecoverApp = instantiateStateRecoveryApp()
    stateRecoverApp.start().get()
    submitDataToL1ContactAndWaitExecution()

    await()
      .atMost(1.minutes.toJavaDuration())
      .untilAsserted {
        assertThat(stateRecoverApp.stateRootMismatchFound).isTrue()
      }

    assertThat(fakeExecutionLayerClient.headBlock.number)
      .isEqualTo(aggregationsAndBlobs[1].aggregation!!.endBlockNumber)
  }

  @Test
  fun `should stop synch at forceSyncStopBlockNumber`() {
    val debugForceSyncStopBlockNumber = aggregationsAndBlobs[2].aggregation!!.startBlockNumber
    log.debug("forceSyncStopBlockNumber={}", fakeStateManagerClient)
    instantiateStateRecoveryApp(debugForceSyncStopBlockNumber = debugForceSyncStopBlockNumber)
      .start().get()
    submitDataToL1ContactAndWaitExecution(waitTimeout = 3.minutes)

    await()
      .atMost(1.minutes.toJavaDuration())
      .pollInterval(2.seconds.toJavaDuration())
      .untilAsserted {
        log.debug(
          "headBlockNumber={} forceSyncStopBlockNumber={}",
          fakeExecutionLayerClient.headBlock.number,
          debugForceSyncStopBlockNumber
        )
        assertThat(fakeExecutionLayerClient.headBlock.number).isGreaterThanOrEqualTo(debugForceSyncStopBlockNumber)
      }

    assertThat(fakeExecutionLayerClient.headBlock.number).isEqualTo(debugForceSyncStopBlockNumber)
  }
}
