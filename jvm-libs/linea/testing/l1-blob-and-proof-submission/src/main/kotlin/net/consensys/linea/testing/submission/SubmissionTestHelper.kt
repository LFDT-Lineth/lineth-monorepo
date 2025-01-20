package net.consensys.linea.testing.submission

import linea.web3j.waitForTxReceipt
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.Aggregation
import org.web3j.protocol.Web3j
import kotlin.time.Duration
import kotlin.time.Duration.Companion.minutes

/**
 * Submits blobs respecting aggregation boundaries
 * returns list of tx hashes, does not wait for txs to be mined
 */
fun submitBlobs(
  contractClient: LineaRollupSmartContractClient,
  aggregationsAndBlobs: List<AggregationAndBlobs>,
  blobChunksSize: Int = 6
): List<String> {
  require(blobChunksSize in 1..6) { "blobChunksSize must be between 1..6" }

  return aggregationsAndBlobs
    .map { (_, aggBlobs) ->
      val blobChunks = aggBlobs.chunked(blobChunksSize)
      blobChunks.map { blobs -> contractClient.submitBlobs(blobs, gasPriceCaps = null).get() }
    }
    .flatten()
}

/**
 * Submits blobs respecting aggregation boundaries,
 * then submits aggregations
 *
 * returns list of tx hashes of aggregations submissions only, does not wait for txs to be mined
 */
data class SubmissionTxHashes(
  val blobTxHashes: List<String>,
  val aggregationTxHashes: List<String>
)

fun submitBlobsAndAggregations(
  contractClientForBlobSubmission: LineaRollupSmartContractClient,
  contractClientForAggregationSubmission: LineaRollupSmartContractClient = contractClientForBlobSubmission,
  aggregationsAndBlobs: List<AggregationAndBlobs>,
  blobChunksSize: Int = 6
): SubmissionTxHashes {
  val blobSubmissionTxHashes = submitBlobs(contractClientForBlobSubmission, aggregationsAndBlobs, blobChunksSize)
  return aggregationsAndBlobs
    .filter { it.aggregation != null }
    .mapIndexed { index, (aggregation, aggBlobs) ->
      aggregation as Aggregation
      val parentAgg = aggregationsAndBlobs.getOrNull(index - 1)?.aggregation
      contractClientForAggregationSubmission.finalizeBlocks(
        aggregation = aggregation.aggregationProof!!,
        aggregationLastBlob = aggBlobs.last(),
        parentShnarf = aggBlobs.first().blobCompressionProof!!.prevShnarf,
        parentL1RollingHash = parentAgg?.aggregationProof?.l1RollingHash ?: ByteArray(32),
        parentL1RollingHashMessageNumber = parentAgg?.aggregationProof?.l1RollingHashMessageNumber ?: 0L,
        gasPriceCaps = null
      ).get()
    }
    .let { SubmissionTxHashes(blobSubmissionTxHashes, it) }
}

fun submitBlobsAndAggregationsAndWaitExecution(
  contractClientForBlobSubmission: LineaRollupSmartContractClient,
  contractClientForAggregationSubmission: LineaRollupSmartContractClient = contractClientForBlobSubmission,
  aggregationsAndBlobs: List<AggregationAndBlobs>,
  blobChunksSize: Int = 6,
  l1Web3jClient: Web3j,
  waitTimeout: Duration = 2.minutes
) {
  val submissionTxHashes = submitBlobsAndAggregations(
    contractClientForBlobSubmission = contractClientForAggregationSubmission,
    contractClientForAggregationSubmission = contractClientForAggregationSubmission,
    aggregationsAndBlobs = aggregationsAndBlobs,
    blobChunksSize = blobChunksSize
  )

  l1Web3jClient.waitForTxReceipt(
    txHash = submissionTxHashes.aggregationTxHashes.last(),
    timeout = waitTimeout
  ).also { txReceipt ->
    if (txReceipt.status != "0x1") {
      val lastAggregation = aggregationsAndBlobs.findLast { it.aggregation != null }!!.aggregation!!
      throw IllegalStateException(
        "latest finalization=${lastAggregation.intervalString()} failed on L1. receipt=$txReceipt"
      )
    }
  }
}
