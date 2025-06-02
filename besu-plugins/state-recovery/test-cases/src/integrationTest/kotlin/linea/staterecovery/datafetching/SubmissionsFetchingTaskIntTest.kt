package linea.staterecovery.datafetching

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.contract.l1.LineaContractVersion
import linea.domain.BlockParameter
import linea.domain.RetryConfig
import linea.log4j.configureLoggers
import linea.staterecovery.BlobDecompressorAndDeserializer
import linea.staterecovery.BlobDecompressorToDomainV1
import linea.staterecovery.BlockFromL1RecoveredData
import linea.staterecovery.BlockHeaderStaticFields
import linea.staterecovery.DataFinalizedV3
import linea.staterecovery.LineaSubmissionEventsClientImpl
import linea.staterecovery.plugin.AppClients
import linea.staterecovery.plugin.createAppClients
import linea.web3j.createWeb3jHttpClient
import net.consensys.linea.blob.BlobDecompressorVersion
import net.consensys.linea.blob.GoNativeBlobDecompressorFactory
import net.consensys.linea.testing.submission.AggregationAndBlobs
import net.consensys.linea.testing.submission.loadBlobsAndAggregationsSortedAndGrouped
import net.consensys.linea.testing.submission.submitBlobsAndAggregationsAndWaitExecution
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.ethereum.ContractsManager
import net.consensys.zkevm.ethereum.MakeFileDelegatedContractsManager.connectToLineaRollupContract
import net.consensys.zkevm.ethereum.MakeFileDelegatedContractsManager.lineaRollupContractErrors
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class SubmissionsFetchingTaskIntTest {
  private val log = LogManager.getLogger("test.case.L1SubmissionsFetchingTaskIntTest")
  private lateinit var aggregationsAndBlobs: List<AggregationAndBlobs>
  private lateinit var contractClientForBlobSubmissions: LineaRollupSmartContractClient
  private lateinit var contractClientForAggregationSubmissions: LineaRollupSmartContractClient
  private lateinit var vertx: Vertx
  private lateinit var appClients: AppClients

  private val testDataDir = run {
    "testdata/coordinator/prover/v3/submissionAndFinalization"
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
      numberOfAggregations = 7,
      extraBlobsWithoutAggregation = 0,
    )
    val rollupDeploymentResult = ContractsManager.get()
      .deployLineaRollup(numberOfOperators = 2, contractVersion = LineaContractVersion.V6).get()

    appClients = createAppClients(
      vertx = vertx,
      smartContractAddress = rollupDeploymentResult.contractAddress,
      l1RpcEndpoint = URI(l1RpcUrl),
      l1RequestRetryConfig = RetryConfig(backoffDelay = 2.seconds),
      blobScanEndpoint = URI(blobScanUrl),
      stateManagerClientEndpoint = URI("http://it-does-not-matter:5432"),
    )

    contractClientForBlobSubmissions = rollupDeploymentResult.rollupOperatorClient
    contractClientForAggregationSubmissions = connectToLineaRollupContract(
      rollupDeploymentResult.contractAddress,
      rollupDeploymentResult.rollupOperators[1].txManager,
      smartContractErrors = lineaRollupContractErrors,
    )
    configureLoggers(
      rootLevel = Level.INFO,
      log.name to Level.DEBUG,
      "linea.testing.submission" to Level.INFO,
      "net.consensys.linea.contract.Web3JContractAsyncHelper" to Level.WARN, // silence noisy gasPrice Caps logs
      "linea.staterecovery.BlobDecompressorToDomainV1" to Level.DEBUG,
      "linea.plugin.staterecovery.clients" to Level.DEBUG,
      "test.clients.l1.web3j-default" to Level.INFO,
      "test.clients.l1.web3j.receipt-poller" to Level.INFO,
      "linea.staterecovery.datafetching" to Level.INFO,
    )
    submitDataToL1ContactAndWaitExecution()
  }

  fun createFetcherTask(
    l2StartBlockNumber: ULong,
    debugForceSyncStopBlockNumber: ULong? = null,
    queuesSizeLimit: Int = 2,
  ): SubmissionsFetchingTask {
    val l1EventsClient = LineaSubmissionEventsClientImpl(
      logsSearcher = appClients.ethLogsSearcher,
      smartContractAddress = appClients.lineaContractClient.contractAddress,
      l1LatestSearchBlock = BlockParameter.Tag.LATEST,
      logsBlockChunkSize = 5000,
    )
    val blobDecompressor: BlobDecompressorAndDeserializer = BlobDecompressorToDomainV1(
      decompressor = GoNativeBlobDecompressorFactory.getInstance(BlobDecompressorVersion.V1_2_0),
      staticFields = BlockHeaderStaticFields.localDev,
      vertx = vertx,
    )

    return SubmissionsFetchingTask(
      vertx = vertx,
      l1EarliestBlockWithFinalizationThatSupportRecovery = BlockParameter.Tag.EARLIEST,
      l1PollingInterval = 10.milliseconds,
      l2StartBlockNumberToFetchInclusive = l2StartBlockNumber,
      submissionEventsClient = l1EventsClient,
      blobsFetcher = appClients.blobScanClient,
      transactionDetailsClient = appClients.transactionDetailsClient,
      blobDecompressor = blobDecompressor,
      submissionEventsQueueLimit = queuesSizeLimit,
      compressedBlobsQueueLimit = queuesSizeLimit,
      targetDecompressedBlobsQueueLimit = queuesSizeLimit,
      debugForceSyncStopBlockNumber = debugForceSyncStopBlockNumber,
    )
  }

  private fun submitDataToL1ContactAndWaitExecution(
    aggregationsAndBlobs: List<AggregationAndBlobs> = this.aggregationsAndBlobs,
    blobChunksSize: Int = 9,
    waitTimeout: Duration = 4.minutes,
  ) {
    submitBlobsAndAggregationsAndWaitExecution(
      contractClientForBlobSubmission = contractClientForBlobSubmissions,
      contractClientForAggregationSubmission = contractClientForAggregationSubmissions,
      aggregationsAndBlobs = aggregationsAndBlobs,
      blobChunksMaxSize = blobChunksSize,
      waitTimeout = waitTimeout,
      l1Web3jClient = createWeb3jHttpClient(
        rpcUrl = l1RpcUrl,
        log = LogManager.getLogger("test.clients.l1.web3j.receipt-poller"),
      ),
      log = log,
    )
  }

  @Test
  fun `should fetch submissions since block 1`() {
    assertSubmissionsAreCorrectlyFetched(l2StartBlockNumber = 1UL)
  }

  @Test
  fun `should fetch submissions since middle of an aggregation`() {
    val l2StartBlockNumber = aggregationsAndBlobs[1].aggregation!!.startBlockNumber + 1UL

    assertSubmissionsAreCorrectlyFetched(l2StartBlockNumber = l2StartBlockNumber)
  }

  @Test
  fun `should stop fetching submissions once debugForceSyncStopBlockNumber is reached`() {
    val debugForceSyncStopBlockNumber =
      aggregationsAndBlobs[aggregationsAndBlobs.size - 1].aggregation!!.endBlockNumber - 1UL

    assertSubmissionsAreCorrectlyFetched(
      l2StartBlockNumber = 1UL,
      debugForceSyncStopBlockNumber = debugForceSyncStopBlockNumber,
    )
  }

  private fun assertSubmissionsAreCorrectlyFetched(
    l2StartBlockNumber: ULong,
    debugForceSyncStopBlockNumber: ULong? = null,
  ) {
    val submissionsFetcher = createFetcherTask(
      l2StartBlockNumber = l2StartBlockNumber,
      queuesSizeLimit = 2,
    )
      .also { it.start() }
    val expectedAggregationsAndBlobsToBeFetched =
      aggregationsAndBlobs
        .filter { aggAndBlobs ->
          val agg = aggAndBlobs.aggregation!!
          val isAfterOrContainingStart =
            agg.startBlockNumber >= l2StartBlockNumber || agg.contains(l2StartBlockNumber)
          val isBeforeForcedStop = if (debugForceSyncStopBlockNumber != null) {
            agg.endBlockNumber < debugForceSyncStopBlockNumber
          } else {
            true
          }
          isAfterOrContainingStart && isBeforeForcedStop
        }

    val fetchedSubmissions = CopyOnWriteArrayList<SubmissionEventsAndData<BlockFromL1RecoveredData>>()

    // consume finalizations until all are fetched
    val continueConsumming = AtomicBoolean(true)
    val consumingThread = Thread {
      while (continueConsumming.get()) {
        submissionsFetcher.peekNextFinalizationReadyToImport()
          ?.also { nextSubmission ->
            fetchedSubmissions.add(nextSubmission)
            submissionsFetcher.pruneQueueForElementsUpToInclusive(
              elHeadBlockNumber = nextSubmission.submissionEvents.dataFinalizedEvent.event.endBlockNumber,
            )
          }
        assertThat(submissionsFetcher.finalizationsReadyToImport()).isLessThanOrEqualTo(2)
        Thread.sleep(1.seconds.inWholeMilliseconds)
      }
    }
    consumingThread.start()

    await()
      .atMost(1.minutes.toJavaDuration())
      .untilAsserted {
        val highestFetchedBlockNumber = fetchedSubmissions.lastOrNull()
          ?.let { it.submissionEvents.dataFinalizedEvent.event.endBlockNumber }
          ?: 0UL

        assertThat(highestFetchedBlockNumber)
          .isGreaterThanOrEqualTo(expectedAggregationsAndBlobsToBeFetched.last().aggregation!!.endBlockNumber)
      }

    continueConsumming.set(false)

    assertThat(fetchedSubmissions.map { it.submissionEvents.dataFinalizedEvent.event.intervalString() })
      .isEqualTo(expectedAggregationsAndBlobsToBeFetched.map { it.aggregation!!.intervalString() })

    fetchedSubmissions.forEachIndexed { index, fetchedSubmission ->
      val sotAggAndBlobs = expectedAggregationsAndBlobsToBeFetched[index]
      assertFetchedData(fetchedSubmission!!, sotAggAndBlobs)
    }
  }

  fun assertFetchedData(
    fetchedData: SubmissionEventsAndData<BlockFromL1RecoveredData>,
    sotAggregationData: AggregationAndBlobs,
  ) {
    assertEventMatchesAggregation(
      fetchedData.submissionEvents.dataFinalizedEvent.event,
      sotAggregationData.aggregation!!,
    )
    assertThat(fetchedData.data.first().header.blockNumber)
      .isEqualTo(sotAggregationData.blobs.first().startBlockNumber)
    assertThat(fetchedData.data.last().header.blockNumber)
      .isEqualTo(sotAggregationData.blobs.last().endBlockNumber)
  }

  fun assertEventMatchesAggregation(
    event: DataFinalizedV3,
    aggregation: Aggregation,
  ) {
    assertThat(event.startBlockNumber).isEqualTo(aggregation.startBlockNumber)
    assertThat(event.endBlockNumber).isEqualTo(aggregation.endBlockNumber)
  }
}
