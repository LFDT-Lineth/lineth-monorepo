package linea.staterecovery

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import lineth.ContractsManager
import lineth.EthApiClientManager
import lineth.LinethRollupDeploymentResult
import lineth.MakeFileDelegatedContractsManager.connectToLinethRollupContract
import lineth.MakeFileDelegatedContractsManager.linethRollupContractErrors
import lineth.contract.events.DataFinalizedV3
import lineth.contract.events.DataSubmittedV3
import lineth.contract.l1.LinethRollupContractVersion
import lineth.domain.Aggregation
import lineth.domain.BlockParameter
import lineth.ethapi.EthLogsSearcherImpl
import lineth.log4j.configureLoggers
import lineth.testing.submission.AggregationAndBlobs
import lineth.testing.submission.loadBlobsAndAggregationsSortedAndGrouped
import lineth.testing.submission.submitBlobsAndAggregationsAndWaitExecution
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeAll
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.junit.jupiter.api.extension.ExtendWith
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class LineaSubmissionEventsClientIntTest {
  private val testDataDir = "testdata/coordinator/prover/v3/submissionAndFinalization/"
  private lateinit var rollupDeploymentResult: LinethRollupDeploymentResult

  // 1-block-per-blob test data has 2 aggregations: 1..10, 11..20 with 1 more than the max blob submission
  private lateinit var aggregationsAndBlobs: List<AggregationAndBlobs>
  private lateinit var submissionEventsFetcher: LinethRollupSubmissionEventsClient

  private fun setupTest(vertx: Vertx) {
    configureLoggers(
      rootLevel = Level.INFO,
      "lineth.contract.Web3JContractAsyncHelper" to Level.WARN, // silence noisy gasPrice Caps logs
      "test.clients.l1.executionlayer" to Level.INFO,
      "test.clients.l1.web3j-default" to Level.INFO,
      "test.clients.l1.linea-contract" to Level.INFO,
      "test.clients.l1.events-fetcher" to Level.DEBUG,
    )

    val rollupDeploymentFuture = ContractsManager.get()
      .deployLinethRollup(numberOfOperators = 2, contractVersion = LinethRollupContractVersion.V6)
    // load files from FS while smc deploy
    aggregationsAndBlobs = loadBlobsAndAggregationsSortedAndGrouped(
      blobsResponsesDir = "$testDataDir/compression/responses",
      aggregationsResponsesDir = "$testDataDir/aggregation/responses",
      numberOfAggregations = 7,
      extraBlobsWithoutAggregation = 3,
    )
    // wait smc deployment finishes
    rollupDeploymentResult = rollupDeploymentFuture.get()
    submissionEventsFetcher = createSubmissionEventsClient(
      vertx = vertx,
      contractAddress = rollupDeploymentResult.contractAddress,
    )

    submitBlobsAndAggregationsAndWaitExecution(
      contractClientForBlobSubmission = rollupDeploymentResult.rollupOperatorClient,
      contractClientForAggregationSubmission = connectToLinethRollupContract(
        contractAddress = rollupDeploymentResult.contractAddress,
        transactionManager = rollupDeploymentResult.rollupOperators[1].txManager,
        smartContractErrors = linethRollupContractErrors,
      ),
      aggregationsAndBlobs = aggregationsAndBlobs,
      blobChunksMaxSize = 9,
      l1EthApiClient = EthApiClientManager.l1Client,
      waitTimeout = 4.minutes,
    )
  }

  private fun createSubmissionEventsClient(vertx: Vertx, contractAddress: String): LinethRollupSubmissionEventsClient {
    val log = LogManager.getLogger("test.clients.l1.events-fetcher")
    val eventsFetcherEthApiClient = EthApiClientManager.buildL1Client(
      log = log,
      requestResponseLogLevel = Level.DEBUG,
      failuresLogLevel = Level.WARN,
    )
    return LineaSubmissionEventsClientImpl(
      logsSearcher = EthLogsSearcherImpl(
        vertx = vertx,
        ethApiClient = eventsFetcherEthApiClient,
        config = EthLogsSearcherImpl.Config(
          loopSuccessBackoffDelay = 1.milliseconds,
        ),
        log = log,
      ),
      smartContractAddress = contractAddress,
      l1LatestSearchBlock = BlockParameter.Tag.LATEST,
      logsBlockChunkSize = 100,
    )
  }

  @BeforeAll
  fun beforeAll(vertx: Vertx) {
    setupTest(vertx)
  }

  @Test
  fun `findFinalizationAndDataSubmissionV3Events should find events when blockNumber is aggregation startBlock`() {
    val expectedSubmissionEventsToFind: List<Pair<DataFinalizedV3, List<DataSubmittedV3>>> =
      getExpectedSubmissionEventsFromRecords(aggregationsAndBlobs)

    expectedSubmissionEventsToFind
      .forEach { (expectedFinalizationEvent, expectedDataSubmittedEvents) ->
        assertThat(
          submissionEventsFetcher
            .findFinalizationAndDataSubmissionV3Events(
              fromL1BlockNumber = BlockParameter.Tag.EARLIEST,
              finalizationStartBlockNumber = expectedFinalizationEvent.startBlockNumber,
            ),
        )
          .succeedsWithin(1.minutes.toJavaDuration())
          .extracting { submissionEvents ->
            val dataFinalizedEvent = submissionEvents?.dataFinalizedEvent?.event
            val dataSubmittedEvents = submissionEvents?.dataSubmittedEvents?.map { it.event }
            Pair(dataFinalizedEvent, dataSubmittedEvents)
          }
          .isEqualTo(Pair(expectedFinalizationEvent, expectedDataSubmittedEvents))
      }
  }

  @Test
  fun `findFinalizationAndDataSubmissionV3Events should return null when not found`() {
    val invalidStartBlockNumber = aggregationsAndBlobs[1].aggregation!!.startBlockNumber + 1UL

    assertThat(
      submissionEventsFetcher
        .findFinalizationAndDataSubmissionV3Events(
          fromL1BlockNumber = BlockParameter.Tag.EARLIEST,
          finalizationStartBlockNumber = invalidStartBlockNumber,
        ).get(),
    )
      .isNull()
  }

  @Test
  fun `findFinalizationAndDataSubmissionV3EventsContainingL2BlockNumber should find events`() {
    val invalidStartBlockNumber = aggregationsAndBlobs[1].aggregation!!.startBlockNumber + 1UL

    submissionEventsFetcher
      .findFinalizationAndDataSubmissionV3EventsContainingL2BlockNumber(
        fromL1BlockNumber = BlockParameter.Tag.EARLIEST,
        l2BlockNumber = invalidStartBlockNumber,
      ).get()
      .also { result ->
        assertThat(result).isNotNull
        assertThat(result!!.dataFinalizedEvent.event.startBlockNumber)
          .isEqualTo(aggregationsAndBlobs[1].aggregation!!.startBlockNumber)
      }
  }

  private fun getExpectedSubmissionEventsFromRecords(
    aggregationsAndBlobs: List<AggregationAndBlobs>,
  ): List<Pair<DataFinalizedV3, List<DataSubmittedV3>>> {
    return aggregationsAndBlobs
      .filter { it.aggregation != null }
      .map { (aggregation, aggBlobs) ->
        val expectedDataSubmittedEvents: List<DataSubmittedV3> = aggBlobs
          .chunked(9)
          .map { blobsChunk ->
            DataSubmittedV3(
              parentShnarf = blobsChunk.first().blobCompressionProof!!.prevShnarf,
              shnarf = blobsChunk.last().expectedShnarf,
              finalStateRootHash = blobsChunk.last().blobCompressionProof!!.finalStateRootHash,
            )
          }

        aggregation as Aggregation
        DataFinalizedV3(
          startBlockNumber = aggregation.startBlockNumber,
          endBlockNumber = aggregation.endBlockNumber,
          shnarf = aggBlobs.last().expectedShnarf,
          parentStateRootHash = aggBlobs.first().blobCompressionProof!!.parentStateRootHash,
          finalStateRootHash = aggBlobs.last().blobCompressionProof!!.finalStateRootHash,
        ) to expectedDataSubmittedEvents
      }
  }
}
