package linea.build.staterecover.clients.smartcontract

import build.linea.contract.l1.LineaContractVersion
import build.linea.staterecover.clients.DataFinalizedV3
import build.linea.staterecover.clients.DataSubmittedV3
import build.linea.staterecover.clients.LineaRollupSubmissionEventsClient
import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import net.consensys.linea.BlockParameter
import net.consensys.linea.contract.Web3JLogsClient
import net.consensys.linea.testing.submission.AggregationAndBlobs
import net.consensys.linea.testing.submission.loadBlobsAndAggregationsSortedAndGrouped
import net.consensys.linea.testing.submission.submitBlobsAndAggregations
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.ethereum.ContractsManager
import net.consensys.zkevm.ethereum.Web3jClientManager
import net.consensys.zkevm.ethereum.waitForTxReceipt
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class LineaSubmissionEventsClientIntTest {
  private lateinit var contractClient: LineaRollupSmartContractClient
  private val testDataDir = "testdata/coordinator/prover/v2/"

  // 1-block-per-blob test data has 3 aggregations: 1..7, 8..14, 15..21.
  // We will upgrade the contract in the middle of 2nd aggregation: 12
  // shall submit blob 12, stop submission, upgrade the contract and resume with blob 13
  // val lastSubmittedBlobs = blobs.filter { it.startBlockNumber == 7UL }
  private lateinit var aggregationsAndBlobs: List<AggregationAndBlobs>
  private lateinit var submissionEventsFetcher: LineaRollupSubmissionEventsClient

  private fun setupTest(
    vertx: Vertx
  ) {
    val rollupDeploymentFuture = ContractsManager.get()
      .deployLineaRollup(numberOfOperators = 2, contractVersion = LineaContractVersion.V6)
    // load files from FS while smc deploy
    aggregationsAndBlobs = loadBlobsAndAggregationsSortedAndGrouped(
      blobsResponsesDir = "$testDataDir/compression/responses",
      aggregationsResponsesDir = "$testDataDir/aggregation/responses"
    )
    // wait smc deployment finishes
    val rollupDeploymentResult = rollupDeploymentFuture.get()
    contractClient = rollupDeploymentResult.rollupOperatorClient
    val eventsFetcherWeb3jClient = Web3jClientManager.buildL1Client(
      log = LogManager.getLogger("test.clients.l1.events-fetcher"),
      requestResponseLogLevel = Level.INFO,
      failuresLogLevel = Level.WARN
    )
    submissionEventsFetcher = LineaSubmissionEventsClientWeb3jIpml(
      logsClient = Web3JLogsClient(
        vertx = vertx,
        web3jClient = eventsFetcherWeb3jClient,
        config = Web3JLogsClient.Config(
          timeout = 30.seconds,
          backoffDelay = 1.seconds,
          lookBackRange = 100
        )
      ),
      smartContractAddress = rollupDeploymentResult.contractAddress,
      mostRecentBlockTag = BlockParameter.Tag.LATEST
    )
  }

  @Test
  @Timeout(3, timeUnit = TimeUnit.MINUTES)
  fun `submission works with contract V6`(
    vertx: Vertx,
    testContext: VertxTestContext
  ) {
    testSubmission(vertx, testContext)
  }

  private fun testSubmission(
    vertx: Vertx,
    testContext: VertxTestContext
  ) {
    setupTest(vertx)

    val submissionTxHashes = submitBlobsAndAggregations(
      contractClient = contractClient,
      aggregationsAndBlobs = aggregationsAndBlobs,
      blobChunksSize = 6
    )

    // wait for all finalizations Txs to be mined
    Web3jClientManager.l1Client.waitForTxReceipt(
      txHash = submissionTxHashes.aggregationTxHashes.last(),
      timeout = 20.seconds
    )

    val expectedSubmissionEventsToFind: List<Pair<DataFinalizedV3, List<DataSubmittedV3>>> =
      getExpectedSubmissionEventsFromRecords(aggregationsAndBlobs)

    expectedSubmissionEventsToFind
      .forEach { (expectedFinalizationEvent, expectedDataSubmittedEvents) ->
        assertThat(
          submissionEventsFetcher
            .findDataSubmittedV3EventsUntilNextFinalization(
              l2StartBlockNumberInclusive = expectedFinalizationEvent.startBlockNumber
            )
        )
          .succeedsWithin(30.seconds.toJavaDuration())
          .extracting { submissionEvents ->
            val dataFinalizedEvent = submissionEvents?.dataFinalizedEvent?.event
            val dataSubmittedEvents = submissionEvents?.dataSubmittedEvents?.map { it.event }
            Pair(dataFinalizedEvent, dataSubmittedEvents)
          }
          .isEqualTo(Pair(expectedFinalizationEvent, expectedDataSubmittedEvents))
      }

    // non happy path
    val invalidStartBlockNumber = expectedSubmissionEventsToFind.last().first.startBlockNumber + 1UL
    assertThat(
      submissionEventsFetcher
        .findDataSubmittedV3EventsUntilNextFinalization(
          l2StartBlockNumberInclusive = invalidStartBlockNumber
        ).get()
    )
      .isNull()

    testContext.completeNow()
  }

  private fun getExpectedSubmissionEventsFromRecords(
    aggregationsAndBlobs: List<AggregationAndBlobs>
  ): List<Pair<DataFinalizedV3, List<DataSubmittedV3>>> {
    return aggregationsAndBlobs
      .filter { it.aggregation != null }
      .map { (aggregation, aggBlobs) ->
        val expectedDataSubmittedEvents: List<DataSubmittedV3> = aggBlobs
          .chunked(6)
          .map { blobsChunk ->
            DataSubmittedV3(
              parentShnarf = blobsChunk.first().blobCompressionProof!!.prevShnarf,
              shnarf = blobsChunk.last().expectedShnarf,
              finalStateRootHash = blobsChunk.last().blobCompressionProof!!.finalStateRootHash
            )
          }

        aggregation as Aggregation
        DataFinalizedV3(
          startBlockNumber = aggregation.startBlockNumber,
          endBlockNumber = aggregation.endBlockNumber,
          shnarf = aggBlobs.last().expectedShnarf,
          parentStateRootHash = aggBlobs.first().blobCompressionProof!!.parentStateRootHash,
          finalStateRootHash = aggBlobs.last().blobCompressionProof!!.finalStateRootHash
        ) to expectedDataSubmittedEvents
      }
  }
}
