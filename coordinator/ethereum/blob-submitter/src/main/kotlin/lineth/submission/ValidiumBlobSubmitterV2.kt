package lineth.submission

import linea.contract.l1.LineaValidiumSmartContractClient
import linea.domain.BlobRecordV2
import linea.domain.BlobSubmittedEvent
import lineth.gaspricing.GasPriceCapProviderV2
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer
import kotlin.time.Clock

class ValidiumBlobSubmitterV2(
  private val contract: LineaValidiumSmartContractClient,
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

  private fun submitBlobsInSingleTx(blobRecord: BlobRecordV2): SafeFuture<String> {
    return (
      gasPriceCapProvider?.getGasPriceCaps(blobRecord.startBlockTimestamp)
        ?: SafeFuture.completedFuture(null)
      )
      .thenCompose { gasPriceCaps ->
        val nonce = contract.currentNonce()
        log.debug(
          "submitting shnarf data for blobs: blobs={} nonce={} gasPriceCaps={}",
          blobRecord.intervalString(),
          nonce,
          gasPriceCaps,
        )
        contract.acceptDataRollingHash(
          blobRecord.parentDataRollingHash,
          blobRecord.dataRollingHash,
          gasPriceCaps,
        )
          .whenException { th ->
            logSubmissionError(
              log,
              blobRecord.intervalString(),
              th,
              isEthCall = false,
            )
          }
          .thenPeek { transactionHash ->
            log.info(
              "data rolling hashes for blobs submitted: blobs={} transactionHash={}, nonce={} gasPriceCaps={}",
              blobRecord.intervalString(),
              transactionHash,
              nonce,
              gasPriceCaps,
            )
            val blobSubmittedEvent = BlobSubmittedEvent(
              endBlockNumber = blobRecord.endBlockNumber,
              endBlockTimestamp = blobRecord.endBlockTimestamp,
              submissionTimestamp = clock.now(),
            )
            blobSubmittedEventConsumer.accept(blobSubmittedEvent)
          }
      }
  }

  override fun submitBlobCall(blobRecord: BlobRecordV2): SafeFuture<*> {
    return (
      gasPriceCapProvider?.getGasPriceCapsWithCoefficient(blobRecord.startBlockTimestamp)
        ?: SafeFuture.completedFuture(null)
      )
      .thenCompose { gasPriceCaps ->
        val nonce = contract.currentNonce()
        log.debug(
          "eth_call submitting shnarf data for blobs: blobs={} nonce={} gasPriceCaps={}",
          blobRecord.intervalString(),
          nonce,
          gasPriceCaps,
        )
        contract.acceptDataRollingHash(
          blobRecord.parentDataRollingHash,
          blobRecord.dataRollingHash,
          gasPriceCaps,
          isEthCall = true,
        )
          .whenException { th ->
            logSubmissionError(log, blobRecord.intervalString(), th, isEthCall = true)
          }
          .thenPeek { _ ->
            log.debug(
              "eth_call data rolling hashes for blobs submission passed: blob={} nonce={} gasPriceCaps={}",
              blobRecord.intervalString(),
              nonce,
              gasPriceCaps,
            )
          }
      }
  }
}
