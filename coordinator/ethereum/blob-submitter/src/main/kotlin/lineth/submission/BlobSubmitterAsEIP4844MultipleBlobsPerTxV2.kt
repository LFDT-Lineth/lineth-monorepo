package lineth.submission

import linea.contract.l1.BlobsSubmissionV9
import linea.contract.l1.LinethRollupSmartContractClient
import linea.domain.BlobRecordV2
import linea.domain.BlobSubmittedEvent
import linea.domain.DataRollingHashCalculator
import linea.domain.gas.GasPriceCaps
import linea.kotlin.encodeHex
import lineth.gaspricing.GasPriceCapProviderV2
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer
import kotlin.time.Clock

class BlobSubmitterAsEIP4844MultipleBlobsPerTxV2(
  private val contract: LinethRollupSmartContractClient,
  private val dataRollingHashCalculator: DataRollingHashCalculator,
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

  private fun submitBlobRecordInTxs(blobRecord: BlobRecordV2, isEthCall: Boolean = false): SafeFuture<List<String>> {
    return (
      if (isEthCall) {
        gasPriceCapProvider?.getGasPriceCapsWithCoefficient(blobRecord.startBlockTimestamp)
      } else {
        gasPriceCapProvider?.getGasPriceCaps(blobRecord.startBlockTimestamp)
      } ?: SafeFuture.completedFuture(null)
      )
      .thenCompose<List<String>> { gasPriceCaps ->
        var parentDataRollingHash = blobRecord.parentDataRollingHash
        val chunkedBlobs = blobRecord.blobsData.chunked(targetBlobsPerTransaction.toInt())
        (if (isEthCall) listOf(chunkedBlobs.first()) else chunkedBlobs)
          .fold(SafeFuture.completedFuture(emptyList())) { chainOfFutures, blobs ->
            val newChainOfFutures = chainOfFutures
              .thenCompose { listOfTxHashes ->
                val storedDataRollingHash = blobs.map { it.chunkHash }.fold(
                  parentDataRollingHash,
                ) { dataRollingHash, chunkHash ->
                  dataRollingHashCalculator.fold(dataRollingHash, chunkHash)
                }
                submitBlobsInSingleTx(
                  blobs = blobs.map { it.blobBytes },
                  parentDataRollingHash = parentDataRollingHash,
                  storedDataRollingHash = storedDataRollingHash,
                  blobRecord = blobRecord,
                  gasPriceCaps = gasPriceCaps,
                  isEthCall = isEthCall,
                )
                  .thenApply { txHash ->
                    parentDataRollingHash = storedDataRollingHash
                    listOfTxHashes + txHash
                  }
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
