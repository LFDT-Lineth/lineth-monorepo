package lineth.submission

import linea.contract.l1.BlobsSubmissionV9
import linea.contract.l1.LinethRollupSmartContractClient
import linea.domain.BlobData
import linea.domain.BlobRecordV2
import linea.domain.BlobSubmittedEvent
import linea.domain.gas.GasPriceCaps
import linea.kotlin.encodeHex
import lineth.gaspricing.GasPriceCapProviderV2
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer
import kotlin.time.Clock

/**
 * Submits each rollup's blobs in stream order. A blob shared with the previous rollup is submitted with that rollup,
 * the one it ends, so it is skipped here: a rollup's end rolling hash must be anchored on L1 for it to finalize.
 */
class BlobSubmitterAsEIP4844MultipleBlobsPerTxV2(
  private val contract: LinethRollupSmartContractClient,
  private val targetBlobsPerTransaction: UInt,
  private val gasPriceCapProvider: GasPriceCapProviderV2?,
  private val blobSubmittedEventConsumer: Consumer<BlobSubmittedEvent> = Consumer<BlobSubmittedEvent> { },
  private val clock: Clock = Clock.System,
) : BlobSubmitterV2 {
  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun submitBlobs(blobsChunks: List<BlobRecordV2>): SafeFuture<List<String>> {
    return blobsChunks
      .fold(SafeFuture.completedFuture(emptyList())) { chainOfFutures, blobRecord ->
        val newChainOfFutures = chainOfFutures
          .thenCompose { listOfTxHashes ->
            submitBlobRecordInTxs(blobRecord)
              .thenApply { txHashes -> listOfTxHashes + txHashes }
          }
        newChainOfFutures
      }
  }

  private fun blobsToSubmit(blobRecord: BlobRecordV2): List<BlobData> =
    if (blobRecord.parentDataTailTakeBytes > 0) blobRecord.blobsData.drop(1) else blobRecord.blobsData

  private fun submitBlobRecordInTxs(blobRecord: BlobRecordV2, isEthCall: Boolean = false): SafeFuture<List<String>> {
    if (blobsToSubmit(blobRecord).isEmpty()) {
      log.debug(
        "rollup={} has no new blobs: its only blob was submitted with the previous rollup",
        blobRecord.intervalString(),
      )
      return SafeFuture.completedFuture(emptyList())
    }
    return (
      if (isEthCall) {
        gasPriceCapProvider?.getGasPriceCapsWithCoefficient(blobRecord.startBlockTimestamp)
      } else {
        gasPriceCapProvider?.getGasPriceCaps(blobRecord.startBlockTimestamp)
      } ?: SafeFuture.completedFuture(null)
      )
      .thenCompose<List<String>> { gasPriceCaps ->
        val chunkedBlobs = blobsToSubmit(blobRecord).chunked(targetBlobsPerTransaction.toInt())
        (if (isEthCall) chunkedBlobs.take(1) else chunkedBlobs)
          .fold(SafeFuture.completedFuture(emptyList())) { chainOfFutures, blobs ->
            val newChainOfFutures = chainOfFutures
              .thenCompose { listOfTxHashes ->
                submitBlobsInSingleTx(
                  blobs = blobs.map { it.blobBytes },
                  parentDataRollingHash = blobs.first().prevDataRollingHash,
                  storedDataRollingHash = blobs.last().dataRollingHash,
                  blobRecord = blobRecord,
                  gasPriceCaps = gasPriceCaps,
                  isEthCall = isEthCall,
                )
                  .thenApply { txHash -> listOfTxHashes + txHash }
              }
            newChainOfFutures
          }
      }
      .thenPeek {
        if (!isEthCall) {
          val blobSubmittedEvent = BlobSubmittedEvent(
            endBlockNumber = blobRecord.endBlockNumber,
            endBlockTimestamp = blobRecord.endBlockTimestamp,
            submissionTimestamp = clock.now(),
          )
          blobSubmittedEventConsumer.accept(blobSubmittedEvent)
        }
      }
  }

  private fun submitBlobsInSingleTx(
    blobs: List<ByteArray>,
    parentDataRollingHash: ByteArray,
    storedDataRollingHash: ByteArray,
    blobRecord: BlobRecordV2,
    gasPriceCaps: GasPriceCaps?,
    isEthCall: Boolean = false,
  ): SafeFuture<String> {
    val nonce = contract.currentNonce()
    log.debug(
      "{}submitting blobs: rollup={} parentDataRollingHash={} storedDataRollingHash={} nonce={} gasPriceCaps={}",
      if (isEthCall) "eth_call " else "",
      blobRecord.intervalString(),
      parentDataRollingHash.encodeHex(),
      storedDataRollingHash.encodeHex(),
      nonce,
      gasPriceCaps,
    )

    val blobData = BlobsSubmissionV9(
      blobs = blobs,
      parentDataRollingHash = parentDataRollingHash,
      storedDataRollingHash = storedDataRollingHash,
    )

    return contract.submitBlobsV9(blobData, gasPriceCaps, preflightWithEthCall = false, onlyEthCall = isEthCall)
      .whenException { th ->
        logSubmissionError(
          log,
          blobRecord.intervalString(),
          th,
          isEthCall = isEthCall,
        )
      }
      .thenPeek { transactionHash ->
        log.info(
          "{}: rollup={} transactionHash={}, parentDataRollingHash={}" +
            "storedDataRollingHash={} nonce={} gasPriceCaps={}",
          if (isEthCall) "eth_call blobs submission passed" else "blobs submitted",
          blobRecord.intervalString(),
          transactionHash,
          parentDataRollingHash.encodeHex(),
          storedDataRollingHash.encodeHex(),
          nonce,
          gasPriceCaps,
        )
      }
  }

  override fun submitBlobCall(blobRecord: BlobRecordV2): SafeFuture<*> {
    return submitBlobRecordInTxs(blobRecord, true)
  }
}
