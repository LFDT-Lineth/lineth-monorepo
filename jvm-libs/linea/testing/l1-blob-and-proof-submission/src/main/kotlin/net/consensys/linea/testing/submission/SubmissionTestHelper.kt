package net.consensys.linea.testing.submission

import linea.domain.BlockInterval
import linea.web3j.waitForTxReceipt
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.domain.BlobRecord
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

fun assertTxSuccess(
  txHash: String,
  interval: BlockInterval,
  submissionType: String,
  l1Web3jClient: Web3j,
  timeout: Duration = 1.minutes,
  log: Logger = LogManager.getLogger("linea.testing.submission")
) {
  l1Web3jClient.waitForTxReceipt(
    txHash = txHash,
    timeout = timeout,
    log = log
  ).also { txReceipt ->
    if (txReceipt.status != "0x1") {
      throw RuntimeException(
        "submission of $submissionType=${interval.intervalString()}" +
          " failed on L1. receipt=$txReceipt"
      )
    }
  }
}

fun assertTxsSuccess(
  txsAndInterval: List<Pair<String, BlockInterval>>,
  submissionType: String,
  l1Web3jClient: Web3j,
  timeout: Duration = 1.minutes,
  log: Logger = LogManager.getLogger("linea.testing.submission")
) {
  SafeFuture.supplyAsync {
    txsAndInterval.forEach { (txHash, interval) ->
      log.debug("waiting for tx to be mined txHash={} ", txHash)
      assertTxSuccess(txHash, interval, submissionType, l1Web3jClient, timeout, log = log)
      log.debug("tx was mined txHash={} ", txHash)
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
  blobChunksSize: Int = 9,
  awaitForPreviousTxBeforeSubmittingNext: Boolean = false,
  l1Web3jClient: Web3j,
  log: Logger
): List<Pair<String, List<BlobRecord>>> {
  require(blobChunksSize in 1..9) { "blobChunksSize must be between 1..9" }

  return aggregationsAndBlobs
    .map { (agg, aggBlobs) ->
      val blobChunks = aggBlobs.chunked(blobChunksSize)
      blobChunks.map { blobs ->
        val txHash = contractClient.submitBlobs(blobs, gasPriceCaps = null).get()
        val blobsLogInfo = blobs.map(BlockInterval::intervalString)
        log.info(
          "submitting blobs: aggregation={} blobsChunk={} txHash={}",
          agg?.intervalString(),
          blobsLogInfo,
          txHash
        )
        if (awaitForPreviousTxBeforeSubmittingNext) {
          log.debug("waiting for blobsChunk={} txHash={} to be mined", blobsLogInfo, txHash)
          assertTxSuccess(txHash, blobs.first(), "blobs", l1Web3jClient, 20.seconds)
          log.info("blobsChunk={} txHash={} mined", blobsLogInfo, txHash)
        }

        txHash to blobs
      }
    }
    .flatten()
}

fun submitBlobsAndAggregationsAndWaitExecution(
  contractClientForBlobSubmission: LineaRollupSmartContractClient,
  contractClientForAggregationSubmission: LineaRollupSmartContractClient = contractClientForBlobSubmission,
  aggregationsAndBlobs: List<AggregationAndBlobs>,
  blobChunksMaxSize: Int = 9,
  l1Web3jClient: Web3j,
  waitTimeout: Duration = 2.minutes,
  log: Logger = LogManager.getLogger("linea.testing.submission")
) {
  val blobSubmissions = submitBlobs(
    contractClientForBlobSubmission,
    aggregationsAndBlobs,
    blobChunksMaxSize,
    awaitForPreviousTxBeforeSubmittingNext = false,
    l1Web3jClient = l1Web3jClient,
    log = log
  )

  assertTxsSuccess(
    txsAndInterval = blobSubmissions.map { (txHash, blobs) ->
      txHash to BlockInterval(blobs.first().startBlockNumber, blobs.last().endBlockNumber)
    },
    submissionType = "blobs",
    l1Web3jClient = l1Web3jClient,
    timeout = waitTimeout
  )
  log.info(
    "blob={} txHash={} executed on L1",
    blobSubmissions.last().second.last().intervalString(),
    blobSubmissions.last().first
  )

  val submissions = aggregationsAndBlobs
    .filter { it.aggregation != null }
    .mapIndexed { index, (aggregation, aggBlobs) ->
      aggregation as Aggregation
      val parentAgg = aggregationsAndBlobs.getOrNull(index - 1)?.aggregation
      val txHash = contractClientForAggregationSubmission.finalizeBlocks(
        aggregation = aggregation.aggregationProof!!,
        aggregationLastBlob = aggBlobs.last(),
        parentL1RollingHash = parentAgg?.aggregationProof?.l1RollingHash ?: ByteArray(32),
        parentL1RollingHashMessageNumber = parentAgg?.aggregationProof?.l1RollingHashMessageNumber ?: 0L,
        gasPriceCaps = null
      ).get()
      log.info(
        "submitting aggregation={} txHash={}",
        aggregation.intervalString(),
        txHash
      )
      txHash to aggregation
    }

  assertTxsSuccess(
    txsAndInterval = submissions,
    submissionType = "aggregation",
    l1Web3jClient = l1Web3jClient,
    timeout = waitTimeout
  )

  log.info(
    "aggregation={} txHash={} executed on L1",
    submissions.last().second.intervalString(),
    submissions.last().first
  )
}
