package lineth.submission

import linea.contract.l1.BlobsSubmissionV9
import linea.contract.l1.LinethRollupSmartContractClient
import linea.domain.BlobRecordV2
import linea.domain.BlobSubmittedEvent
import lineth.gaspricing.GasPriceCapProviderV2
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer
import kotlin.time.Clock

class BlobSubmitterAsEIP4844MultipleBlobsPerTxV2(
  private val contract: LinethRollupSmartContractClient,
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
            submitBlobsInSingleTx(blobRecord)
              .thenApply { txHash -> listOfTxHashes + txHash }
          }
        newChainOfFutures
      }
  }

  private fun submitBlobsInSingleTx(blobRecord: BlobRecordV2, isEthCall: Boolean = false): SafeFuture<String> {
    return (
      if (isEthCall) {
        gasPriceCapProvider?.getGasPriceCapsWithCoefficient(blobRecord.startBlockTimestamp)
      } else {
        gasPriceCapProvider?.getGasPriceCaps(blobRecord.startBlockTimestamp)
      } ?: SafeFuture.completedFuture(null)
      )
      .thenCompose { gasPriceCaps ->
        val nonce = contract.currentNonce()
        log.debug(
          "{}submitting blobs: blobs={} nonce={} gasPriceCaps={}",
          if (isEthCall) "eth_call " else "",
          blobRecord.intervalString(),
          nonce,
          gasPriceCaps,
        )

        val blobData = BlobsSubmissionV9(
          blobs = blobRecord.blobsData.map { it.compressedData },
          blobFinalBlockHashes = blobRecord.blobsData.map { it.endBlockHash },
          parentShnarf = blobRecord.parentShnarf,
          finalBlobShnarf = blobRecord.endShnarf,
        )

        contract.submitBlobsV9(blobData, gasPriceCaps, preflightWithEthCall = false, onlyEthCall = isEthCall)
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
              "{}: blobs={} transactionHash={}, nonce={} gasPriceCaps={}",
              if (isEthCall) "eth_call blobs submission passed" else "blobs submitted",
              blobRecord.intervalString(),
              transactionHash,
              nonce,
              gasPriceCaps,
            )
            if (!isEthCall) {
              val blobSubmittedEvent = BlobSubmittedEvent(
                endBlockNumber = blobRecord.endBlockNumber,
                endBlockTimestamp = blobRecord.endBlockTimestamp,
                lastShnarf = blobRecord.endShnarf,
                submissionTimestamp = clock.now(),
                transactionHash = transactionHash.toByteArray(),
              )
              blobSubmittedEventConsumer.accept(blobSubmittedEvent)
            }
          }
      }
  }

  override fun submitBlobCall(blobRecord: BlobRecordV2): SafeFuture<*> {
    return submitBlobsInSingleTx(blobRecord, true)
  }
}
