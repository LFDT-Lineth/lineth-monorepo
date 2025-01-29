package net.consensys.linea.testing.submission

import build.linea.domain.BlockInterval
import linea.web3j.waitForTxReceipt
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.domain.BlobRecord
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.minutes

fun assertTxSuccess(
  txHash: String,
  interval: BlockInterval,
  submissionType: String,
  l1Web3jClient: Web3j,
  timeout: Duration = 1.minutes
) {
  l1Web3jClient.waitForTxReceipt(
    txHash = txHash,
    timeout = timeout
  ).also { txReceipt ->
    assertThat(txReceipt.status)
      .isEqualTo("0x1")
      .withFailMessage(
        "submission of $submissionType=${interval.intervalString()}" +
          " failed on L1. receipt=$txReceipt"
      )
  }
}

fun assertTxsSuccess(
  txsAndInterval: List<Pair<String, BlockInterval>>,
  submissionType: String,
  l1Web3jClient: Web3j,
  timeout: Duration = 1.minutes
) {
  SafeFuture.supplyAsync {
    txsAndInterval.forEach { (txHash, interval) ->
      assertTxSuccess(txHash, interval, submissionType, l1Web3jClient, timeout)
    }
  }
    .get(timeout.inWholeMilliseconds, java.util.concurrent.TimeUnit.MILLISECONDS)
}

/**
 * Submits blobs respecting aggregation boundaries
 * returns list of tx hashes, does not wait for txs to be mined
 */
fun submitBlobs(
  contractClient: LineaRollupSmartContractClient,
  aggregationsAndBlobs: List<AggregationAndBlobs>,
  blobChunksSize: Int = 6,
  log: Logger
): List<Pair<String, List<BlobRecord>>> {
  require(blobChunksSize in 1..6) { "blobChunksSize must be between 1..6" }

  return aggregationsAndBlobs
    .map { (agg, aggBlobs) ->
      val blobChunks = aggBlobs.chunked(blobChunksSize)
      blobChunks.map { blobs ->
        val txHash = contractClient.submitBlobs(blobs, gasPriceCaps = null).get()
        log.info(
          "submitting blobs: aggregation={} blobsChunk={} txHash={}",
          agg?.intervalString(),
          blobs.map { it.intervalString() },
          txHash
        )

        txHash to blobs
      }
    }
    .flatten()
}

fun submitBlobsAndAggregationsAndWaitExecution(
  contractClientForBlobSubmission: LineaRollupSmartContractClient,
  contractClientForAggregationSubmission: LineaRollupSmartContractClient = contractClientForBlobSubmission,
  aggregationsAndBlobs: List<AggregationAndBlobs>,
  blobChunksMaxSize: Int = 6,
  l1Web3jClient: Web3j,
  waitTimeout: Duration = 2.minutes,
  log: Logger = LogManager.getLogger("linea.testing.submission")
) {
  val blobSubmissionTxHashes = submitBlobs(
    contractClientForBlobSubmission,
    aggregationsAndBlobs,
    blobChunksMaxSize,
    log
  )

  assertTxsSuccess(
    txsAndInterval = blobSubmissionTxHashes.map { (txHash, blobs) ->
      txHash to BlockInterval(blobs.first().startBlockNumber, blobs.last().endBlockNumber)
    },
    submissionType = "blobs",
    l1Web3jClient = l1Web3jClient,
    timeout = waitTimeout
  )

  val submissions = aggregationsAndBlobs
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
      ).get() to aggregation
    }

  assertTxsSuccess(
    txsAndInterval = submissions,
    submissionType = "aggregation",
    l1Web3jClient = l1Web3jClient,
    timeout = waitTimeout
  )
}
